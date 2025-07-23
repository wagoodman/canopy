package gotest

import (
	"fmt"
	"github.com/scylladb/go-set/strset"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

type Definition struct {
	ImportPath string
	FnName     string
	Start      token.Position
	End        token.Position
	Cases      []string
}

func (d Definition) References() []Reference {
	refs := []Reference{
		{
			// function reference
			Package:  d.ImportPath,
			FuncName: d.FnName,
		},
	}
	for _, c := range d.Cases {
		refs = append(refs, Reference{
			// test case reference
			Package:  d.ImportPath,
			FuncName: d.FnName,
			TRunName: c,
		})
	}
	return refs
}

type Definitions []Definition

func (d Definitions) References() []Reference {
	if len(d) == 0 {
		return nil
	}
	var refs []Reference
	pkgs := strset.New()
	for _, def := range d {
		for _, ref := range def.References() {
			if !pkgs.Has(ref.Package) {
				pkgs.Add(ref.Package)
				refs = append(refs, Reference{
					Package: ref.Package,
				})
			}
			refs = append(refs, ref)
		}
	}
	sort.Sort(References(refs))
	return refs
}

func FindDefinitions(collection *golist.PackageCollection) ([]Definition, error) {
	// find and parse all '_test.go' files in given directory and subdirectories
	fileSet := token.NewFileSet()
	var tests []Definition

	process := func(path string) error {
		log.WithFields("path", path).Debug("parsing test file")

		f, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}

		absFilePath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		curPkg := collection.GetByDir(filepath.Dir(absFilePath))
		var importPath string
		if curPkg != nil {
			importPath = curPkg.ImportPath
		}
		for _, fnDecl := range findTestsInFile(f) {
			_, _, cases := getTableTestCases(fnDecl)
			tests = append(tests, Definition{
				ImportPath: importPath,
				FnName:     fnDecl.Name.Name,
				Start:      fileSet.Position(fnDecl.Pos()),
				End:        fileSet.Position(fnDecl.End()),
				Cases:      rewriteTestNames(cases...),
			})
		}

		return nil
	}

	for _, pkg := range collection.Packages() {
		// find all *_test.go files within pkg.Dir (not recursive)
		files, err := filepath.Glob(filepath.Join(pkg.Dir, "*_test.go"))
		if err != nil {
			return nil, fmt.Errorf("failed to find *_test.go files: %w", err)
		}

		for _, f := range files {
			err := process(f)
			if err != nil {
				return nil, fmt.Errorf("failed to parse test file %q: %w", f, err)
			}
		}
	}

	return tests, nil
}

// findTestsInFile returns a slice of all test functions in the given file AST
func findTestsInFile(file *ast.File) []*ast.FuncDecl {
	var tests []*ast.FuncDecl
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !strings.HasPrefix(funcDecl.Name.Name, "Test") {
			continue
		}
		if len(funcDecl.Type.Params.List) != 1 {
			continue
		}
		paramType, ok := funcDecl.Type.Params.List[0].Type.(*ast.StarExpr)
		if !ok || !isTestingT(paramType) {
			continue
		}
		if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
			continue
		}
		tests = append(tests, funcDecl)
	}
	return tests
}

func isTestingT(expr ast.Expr) bool {
	starExpr, ok := expr.(*ast.StarExpr)
	if !ok || starExpr == nil {
		return false
	}
	selector, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok || selector == nil {
		return false
	}

	id, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == "testing" && selector.Sel.Name == "T"
}

func getTableTestCases(node *ast.FuncDecl) (bool, int, []string) {
	var (
		hasTableTest bool
		testCount    int
		testNames    []string
	)

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.RangeStmt:
			testCount, testNames = detectLoopWithTestRun(x)
			if len(testNames) > 0 || testCount > 0 {
				hasTableTest = true
				return false
			}
		}
		return true
	})

	if hasTableTest {
		return true, testCount, testNames
	}

	return false, testCount, testNames
}

func detectLoopWithTestRun(node ast.Node) (int, []string) {
	var (
		cases      int
		fieldNames []string
	)

	var testNameFieldName string
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if fun, ok := sel.X.(*ast.Ident); ok && fun.Name == "t" && sel.Sel.Name == "Run" {
					name := getVariableNameInCallExpr(x)

					testNameFieldName = name
					return false
				}
			}
		}
		return true
	})

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.RangeStmt:
			cases, fieldNames = testNamesFromRangeOverStructLiteral(x, testNameFieldName)
		}
		return true
	})

	if cases == 0 {
		cases = 1
	}

	return cases, fieldNames
}

func getVariableNameInCallExpr(callExpr *ast.CallExpr) string {
	argExprs := callExpr.Args
	if len(argExprs) == 0 {
		return ""
	}

	selector, ok := argExprs[0].(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return selector.Sel.Name
}

func testNamesFromRangeOverStructLiteral(x *ast.RangeStmt, targetFieldName string) (int, []string) {
	var testNames []string
	count := 0

	switch rangeOver := x.X.(type) {
	case *ast.Ident:
		if rangeOver.Obj == nil {
			break
		}
		switch decl := rangeOver.Obj.Decl.(type) {
		case *ast.AssignStmt:
			for _, expr := range decl.Rhs {
				if lit, ok := expr.(*ast.CompositeLit); ok {
					count, testNames = testNamesFromStructLiteral(lit, targetFieldName)
				}
			}
		}
	case *ast.CompositeLit:
		count, testNames = testNamesFromStructLiteral(rangeOver, targetFieldName)
	}
	return count, testNames
}

func testNamesFromStructLiteral(lit *ast.CompositeLit, targetFieldName string) (int, []string) {
	var testNames []string
	count := len(lit.Elts)

	// get the position of the field name in the struct literal
	fieldNamePos := -1
	if arrayLit, ok := lit.Type.(*ast.ArrayType); ok {
		if structLit, ok := arrayLit.Elt.(*ast.StructType); ok {
		fields:
			for _, field := range structLit.Fields.List {
				for idx, fieldName := range field.Names {
					// TODO: look for multiple test field names, not just "name"
					if fieldName.Name == targetFieldName {
						fieldNamePos = idx
						break fields
					}
				}
			}
		}
	}

	if fieldNamePos == -1 {
		// we couldn't find a field named "name" in the struct literal, so we'll just number the tests
		for i := 0; i < count; i++ {
			testNames = append(testNames, fmt.Sprintf("#%02d", i+1))
		}
		return count, testNames
	}

	for _, elt := range lit.Elts {
		// ELT are the struct literals within the array of structs
		if comp, ok := elt.(*ast.CompositeLit); ok {
			if basicLit, ok := comp.Elts[fieldNamePos].(*ast.BasicLit); ok {
				testNames = append(testNames, strings.ReplaceAll(basicLit.Value, "\"", ""))
			}
		}
	}

	return count, testNames
}
