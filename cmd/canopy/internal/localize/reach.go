package localize

import (
	"fmt"
	"sort"

	"github.com/scylladb/go-set/strset"
)

// Failure is a new-regression failure to localize: its full reference string (subtest and all)
// plus the call-graph node id of the test FUNCTION entrypoint used as the reachability root.
// RootNode is "" when the entrypoint could not be resolved in the graph, which lands the failure
// in the unattributed bucket honestly rather than guessing.
type Failure struct {
	Reference string
	RootNode  string
}

// Candidate is a ranked changed symbol: the change most likely responsible for the failures that
// reach it. ReachedBy is the number of distinct new-regression failures whose entrypoint
// transitively reaches this symbol; References lists them.
type Candidate struct {
	Symbol     string   `json:"symbol"`
	Location   string   `json:"location"`
	ReachedBy  int      `json:"reached_by"`
	References []string `json:"references"`
}

// Unattributed is a new-regression failure that reaches no changed symbol: not explained by this
// diff. As valuable as the ranking, it separates "your change did this" from "already broken /
// not your change" without guessing.
type Unattributed struct {
	Reference string `json:"reference"`
	Note      string `json:"note"`
}

// Result is the localization layer of the triage report: ranked root-cause candidates, the
// explicitly-unattributed failures, and the call-graph resolver used (so the confidence is
// legible; see the ceiling note in Localize).
type Result struct {
	Candidates   []Candidate    `json:"candidates"`
	Unattributed []Unattributed `json:"unattributed"`
	CallGraph    string         `json:"call_graph"`
	Summary      string         `json:"summary"`
}

// symbolInfo carries a changed symbol's presentational fields, resolved from the call-graph node
// it matched (which knows the fully-qualified name and source position).
type symbolInfo struct {
	display  string // fully-qualified symbol, e.g. "importpath.calculateFlakyScore"
	location string // relative "file:line"
}

const noRootCauseNote = "reaches no changed symbol; not explained by this diff"

// invertReachability returns, per root node id, the set of changed-symbol ids reachable from it.
// Pure over the injected adjacency so the inversion is testable without SSA. edges is the call
// adjacency (caller id -> callee ids); symbolByNode maps a node id to the changed-symbol id it
// defines.
func invertReachability(roots []string, edges map[string][]string, symbolByNode map[string]string) map[string]*strset.Set {
	out := make(map[string]*strset.Set, len(roots))
	for _, root := range roots {
		if _, done := out[root]; done {
			continue // subtests share a root; compute each entrypoint once
		}
		out[root] = reachableSymbols(root, edges, symbolByNode)
	}
	return out
}

// reachableSymbols does a forward DFS from root over the call edges and collects the changed
// symbols any reachable node defines.
func reachableSymbols(root string, edges map[string][]string, symbolByNode map[string]string) *strset.Set {
	syms := strset.New()
	visited := strset.New()
	stack := []string{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited.Has(n) {
			continue
		}
		visited.Add(n)
		if sid, ok := symbolByNode[n]; ok {
			syms.Add(sid)
		}
		for _, callee := range edges[n] {
			if !visited.Has(callee) {
				stack = append(stack, callee)
			}
		}
	}
	return syms
}

// rankCandidates builds the ranked result from the per-root reachable-symbol sets. Each failure
// contributes its reference to every changed symbol its entrypoint reaches; a failure reaching
// none lands in unattributed. Candidates sort by reached_by desc, ties by symbol so the order is
// deterministic. Pure so the ranking is golden-testable.
func rankCandidates(failures []Failure, reachByRoot map[string]*strset.Set, symbols map[string]symbolInfo, callGraph string) Result {
	refsBySymbol := map[string]*strset.Set{}
	var unattributed []Unattributed

	for _, f := range failures {
		reached := reachByRoot[f.RootNode]
		var hit []string
		if reached != nil {
			for _, sid := range reached.List() {
				if _, ok := symbols[sid]; ok {
					hit = append(hit, sid)
				}
			}
		}
		if len(hit) == 0 {
			unattributed = append(unattributed, Unattributed{Reference: f.Reference, Note: noRootCauseNote})
			continue
		}
		for _, sid := range hit {
			if refsBySymbol[sid] == nil {
				refsBySymbol[sid] = strset.New()
			}
			refsBySymbol[sid].Add(f.Reference)
		}
	}

	candidates := make([]Candidate, 0, len(refsBySymbol))
	for sid, refs := range refsBySymbol {
		info := symbols[sid]
		list := refs.List()
		sort.Strings(list)
		candidates = append(candidates, Candidate{
			Symbol:     info.display,
			Location:   info.location,
			ReachedBy:  refs.Size(),
			References: list,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].ReachedBy != candidates[j].ReachedBy {
			return candidates[i].ReachedBy > candidates[j].ReachedBy
		}
		return candidates[i].Symbol < candidates[j].Symbol
	})
	sort.SliceStable(unattributed, func(i, j int) bool {
		return unattributed[i].Reference < unattributed[j].Reference
	})

	return Result{
		Candidates:   candidates,
		Unattributed: unattributed,
		CallGraph:    callGraph,
		Summary:      localizeSummary(len(failures), candidates, unattributed),
	}
}

// localizeSummary renders the one-liner: "3 of 4 failures attributed to 1 changed symbol; 1
// unattributed". localization runs over ALL current failures (not just new regressions), so the
// noun is the neutral "failure": a flaky failure that still reaches a changed symbol is attributed
// here even though its verdict stays flaky.
func localizeSummary(total int, candidates []Candidate, unattributed []Unattributed) string {
	attributed := total - len(unattributed)
	s := fmt.Sprintf("%d of %d %s attributed to %d changed %s",
		attributed, total, plural(total, "failure", "failures"),
		len(candidates), plural(len(candidates), "symbol", "symbols"))
	if len(unattributed) > 0 {
		s += fmt.Sprintf("; %d unattributed", len(unattributed))
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
