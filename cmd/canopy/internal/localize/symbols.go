// Package localize performs change-aware root-cause localization: it intersects the symbols a
// change touched with what each failing test statically reaches, so a new-regression can be
// attributed to the changed function most likely responsible. It is the causal layer that
// composes with triage's symptom grouping (which failures look alike) to answer "which change
// caused the new ones."
package localize

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// Symbol identifies a changed top-level declaration: the func or method a changed region defines.
// v1 granularity is whole-file (every decl in a changed file is suspect), so File+Name+span is
// enough to match a declaration to its call-graph node by source position.
type Symbol struct {
	File    string // absolute path to the source file
	Name    string // declaration name; methods rendered as "Recv.Method"
	Line    int    // 1-based line of the declaration
	EndLine int    // 1-based line of the declaration's end
}

// ChangedSymbols reads and parses each changed .go file (absolute paths) and returns the
// top-level func/method declarations they define. Parse errors on a single file are skipped
// rather than fatal so one unparsable file doesn't sink the whole analysis.
//
// Changed _test.go files are skipped: a test function is never an upstream root cause of a
// failure (it IS the failing code), and including it would make an edited test attribute to
// itself. Only source declarations can be the change that broke something downstream.
func ChangedSymbols(files []string) ([]Symbol, error) {
	var out []Symbol
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		syms, err := symbolsFromSource(f, src)
		if err != nil {
			continue // skip unparsable files rather than failing the whole run
		}
		out = append(out, syms...)
	}
	return out, nil
}

// symbolsFromSource parses Go source and returns every top-level func/method declaration as a
// changed Symbol. Pure over the injected bytes so extraction is testable without a repo. Only
// funcs/methods are emitted: var/const/type decls are not call-graph nodes, so they cannot be
// reached and cannot be attributed by reachability.
//
// ponytail: whole-file granularity, every decl is suspect (matches the affected-package
// over-approximation). Upgrade path if the whole-file set proves too noisy to rank: intersect
// each decl's line span with the diff's changed hunk ranges to keep only actually-touched decls.
func symbolsFromSource(filename string, src []byte) ([]Symbol, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}

	var syms []Symbol
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		syms = append(syms, Symbol{
			File:    filename,
			Name:    funcName(fd),
			Line:    fset.Position(fd.Pos()).Line,
			EndLine: fset.Position(fd.End()).Line,
		})
	}
	return syms, nil
}

// funcName renders a declaration name, prefixing methods with their receiver type (e.g.
// "*Analyzer.Analyze") so a method reads distinctly from a package-level func of the same name.
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return recvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

// recvTypeName extracts the receiver type name, unwrapping pointer and generic-instantiation
// wrappers so "func (a *Analyzer)" and "func (s Stack[T])" resolve to their base type name.
func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return "*" + recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	}
	return ""
}
