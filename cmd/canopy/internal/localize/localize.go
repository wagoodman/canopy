package localize

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strconv"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// callGraphRTA names the resolver in the output so the confidence level is legible.
const callGraphRTA = "rta"

// resolver builds a call graph over the program, given the reachability roots (the failing test
// entrypoints). It returns the graph and the name recorded in the output. This is the seam the
// plan's T2/T5 call for: RTA is the default, and CHA/VTA drop in the same way (CHA is exercised in
// the tests as the imprecision baseline the upgrade improves on).
type resolver func(prog *ssa.Program, roots []*ssa.Function) (*callgraph.Graph, string)

// rtaResolver is the DEFAULT. RTA is rooted at the failing test entrypoints and prunes dispatch
// edges by which types are actually instantiated along the way, so a test that never constructs
// an Analyzer does not "reach" every Analyzer method. This is what makes the ranking causal: CHA
// (below) instead assumes every address-taken func and every signature-compatible method is a
// possible callee, which — through testify/`t.Run` func values — makes every test reach every
// method, collapsing the ranking to a flat tie. See the ceiling note on Localize.
func rtaResolver(_ *ssa.Program, roots []*ssa.Function) (*callgraph.Graph, string) {
	return rta.Analyze(roots, true).CallGraph, callGraphRTA
}

// Localize builds a static call graph over the affected packages, intersects the changed source
// symbols with what each new-regression test transitively reaches, and returns ranked root-cause
// candidates. loadPatterns should be the affected import paths (scoping the expensive graph
// build); changed comes from ChangedSymbols; failures are the new-regression references to blame.
// Returns nil (not an error) when there is nothing to localize (no changed symbols or no
// failures), so triage silently behaves as today when no diff is present.
//
// ponytail: the resolver is RTA rooted at the failing tests. CHA is the sound but imprecise
// baseline (exercised in the tests); it over-attributes interface and func-value dispatch so
// badly — assuming every signature-compatible method is a callee, which testify/`t.Run` func
// values then make universal — that it ranks every changed symbol as reached by every test (a
// flat tie, useless for "which change"). RTA prunes dispatch by instantiated-type flow, which is
// where the ranking becomes causal. The resolver is emitted as call_graph so the confidence stays
// legible. Upgrade path: VTA (go/callgraph/vta) drops into the same seam for value-flow precision
// beyond RTA; reflection/codegen/build-tag edges remain invisible to all three (the same
// soundness caveat `affected` documents).
func Localize(loadPatterns []string, changed []Symbol, failures []gotest.Reference) (*Result, error) {
	return localizeWith(rtaResolver, loadPatterns, changed, failures)
}

func localizeWith(resolve resolver, loadPatterns []string, changed []Symbol, failures []gotest.Reference) (*Result, error) {
	if len(changed) == 0 || len(failures) == 0 || len(loadPatterns) == 0 {
		return nil, nil
	}

	prog, err := buildSSA(loadPatterns)
	if err != nil {
		return nil, err
	}

	// resolve each failing reference to its test-function SSA root before building the graph, so
	// RTA can be rooted at exactly those entrypoints.
	testFns := indexTestEntries(prog)
	fs := make([]Failure, 0, len(failures))
	var roots []*ssa.Function
	rootSeen := map[string]bool{}
	for _, ref := range failures {
		fn := testFns[rootKey(ref.Package, ref.FuncName)]
		f := Failure{Reference: ref.String(false)}
		if fn != nil {
			f.RootNode = fn.String()
			if !rootSeen[f.RootNode] {
				rootSeen[f.RootNode] = true
				roots = append(roots, fn)
			}
		}
		fs = append(fs, f)
	}

	cg, resolverName := resolve(prog, roots)
	edges, symbolByNode, infos := indexGraph(prog, cg, changed)

	var rootIDs []string
	for id := range rootSeen {
		rootIDs = append(rootIDs, id)
	}

	reach := invertReachability(rootIDs, edges, symbolByNode)
	res := rankCandidates(fs, reach, infos, resolverName)
	return &res, nil
}

// buildSSA loads the given packages (with their tests, so TestXxx entrypoints are present) and
// builds SSA. Load errors are logged but not fatal: a partial program still localizes what did
// load, and the graph resolvers are sound on partial programs.
func buildSSA(patterns []string) (*ssa.Program, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("unable to load packages for localization: %w", err)
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		log.WithFields("errors", n).Debug("package load reported errors during localization; continuing with a partial program")
	}

	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	return prog, nil
}

// indexTestEntries maps (package, name) to the SSA function for every `func TestXxx(*testing.T)`
// in the program, so a failing reference resolves to the function that roots its reachability.
func indexTestEntries(prog *ssa.Program) map[string]*ssa.Function {
	out := map[string]*ssa.Function{}
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || !isTestEntry(fn) {
			continue
		}
		key := rootKey(pkgPathOf(fn), fn.Name())
		if _, exists := out[key]; !exists {
			out[key] = fn
		}
	}
	return out
}

// indexGraph flattens the call graph into a plain adjacency map plus the node->changed-symbol
// mapping the pure inversion consumes. A node matches a changed symbol when its declaration
// position falls inside a changed decl's line span (whole-file granularity today). The matched
// node also supplies the symbol's fully-qualified display name, which the AST extraction alone
// cannot know (it lacks the import path).
func indexGraph(prog *ssa.Program, cg *callgraph.Graph, changed []Symbol) (edges map[string][]string, symbolByNode map[string]string, infos map[string]symbolInfo) {
	changedByFile := map[string][]Symbol{}
	for _, s := range changed {
		changedByFile[s.File] = append(changedByFile[s.File], s)
	}

	edges = map[string][]string{}
	symbolByNode = map[string]string{}
	infos = map[string]symbolInfo{}

	for fn, node := range cg.Nodes {
		if fn == nil {
			continue // cha roots the graph at a synthetic nil-function node; it defines nothing
		}
		id := fn.String()
		for _, e := range node.Out {
			if e.Callee != nil && e.Callee.Func != nil {
				edges[id] = append(edges[id], e.Callee.Func.String())
			}
		}

		if !fn.Pos().IsValid() {
			continue
		}
		pos := prog.Fset.Position(fn.Pos())
		for _, s := range changedByFile[pos.Filename] {
			if pos.Line < s.Line || pos.Line > s.EndLine {
				continue
			}
			sid := symbolID(s)
			symbolByNode[id] = sid
			if _, ok := infos[sid]; !ok {
				infos[sid] = symbolInfo{display: fn.String(), location: relLoc(s.File, s.Line)}
			}
			break
		}
	}
	return edges, symbolByNode, infos
}

// isTestEntry reports whether fn is a `func TestXxx(*testing.T)` — a call-graph reachability root.
func isTestEntry(fn *ssa.Function) bool {
	if len(fn.Name()) < 5 || fn.Name()[:4] != "Test" {
		return false
	}
	sig := fn.Signature
	if sig.Params().Len() != 1 {
		return false
	}
	return sig.Params().At(0).Type().String() == "*testing.T"
}

// pkgPathOf returns the import path of the function's package, or "" for synthetic functions.
func pkgPathOf(fn *ssa.Function) string {
	if fn.Pkg != nil && fn.Pkg.Pkg != nil {
		return fn.Pkg.Pkg.Path()
	}
	if fn.Object() != nil {
		if pkg := packageOf(fn.Object()); pkg != nil {
			return pkg.Path()
		}
	}
	return ""
}

func packageOf(obj types.Object) *types.Package {
	if obj == nil {
		return nil
	}
	return obj.Pkg()
}

func rootKey(pkg, name string) string {
	return pkg + "\x00" + name
}

func symbolID(s Symbol) string {
	return s.File + "\x00" + strconv.Itoa(s.Line)
}

// relLoc renders a source location relative to the working directory for display, falling back to
// the absolute path when relativization fails.
func relLoc(file string, line int) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, file); err == nil {
			return fmt.Sprintf("%s:%d", rel, line)
		}
	}
	return fmt.Sprintf("%s:%d", file, line)
}
