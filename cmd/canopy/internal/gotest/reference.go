package gotest

import (
	"strconv"
	"strings"
)

type Reference struct {
	Package  string // the go package path
	FuncName string // the test function name
	TRunName string // optional instance of t.Run within the function
}

func NewReference(pkg, test string) Reference {
	testFields := strings.SplitN(test, "/", 2)
	var funcName, testName string
	if len(testFields) > 0 {
		funcName = testFields[0]
		if len(testFields) > 1 {
			testName = testFields[1]
		}
	}
	return Reference{
		Package:  pkg,
		FuncName: funcName,
		TRunName: testName,
	}
}

func (r Reference) IsPackage() bool {
	return r.FuncName == "" && r.TRunName == "" && r.Package != ""
}

func (r Reference) PackageRef() Reference {
	return Reference{
		Package: r.Package,
	}
}

func (r Reference) FuncRef() *Reference {
	if r.FuncName == "" {
		return nil
	}
	return &Reference{
		Package:  r.Package,
		FuncName: r.FuncName,
	}
}

func (r Reference) IsSubTest() bool {
	return r.FuncName != "" && r.TRunName != "" && r.Package != ""
}

func (r Reference) SubTestName(clean bool) string {
	tRunNames := r.TRunName
	if clean {
		tRunNames = rewriteTestName(tRunNames)
	}
	return tRunNames
}

func (r Reference) TestName(clean bool) string {
	if r.FuncName == "" {
		return ""
	}

	if r.TRunName == "" {
		return r.FuncName
	}

	return strings.Join([]string{r.FuncName, r.SubTestName(clean)}, "/")
}

func (r Reference) String(clean bool) string {
	testName := r.TestName(clean)
	if testName == "" {
		return r.Package
	}

	return strings.Join([]string{r.Package, testName}, "/")
}

func (r Reference) ParentRef() *Reference {
	if r.FuncName == "" {
		// we're already at the package, there is no parent
		return nil
	}

	if r.TRunName == "" {
		// we're at the function level, return the package
		return &Reference{
			Package: r.Package,
		}
	}

	testFields := strings.Split(r.TRunName, "/")
	if len(testFields) == 1 {
		// return the parent test
		return &Reference{
			Package:  r.Package,
			FuncName: r.FuncName,
		}
	}
	testFields = testFields[:len(testFields)-1]

	return &Reference{
		Package:  r.Package,
		FuncName: r.FuncName,
		TRunName: strings.Join(testFields, "/"),
	}
}

func (r Reference) GuessParentRef() *Reference {
	if !r.IsPackage() {
		return r.ParentRef()
	}

	// we can guess if there is a parent package by removing the last segment of the package path
	parts := strings.Split(r.Package, "/")
	if len(parts) <= 1 {
		// no parent package, we're at the root
		return nil
	}
	// return the parent package by joining all but the last segment
	parts = parts[:len(parts)-1]
	return &Reference{
		Package: strings.Join(parts, "/"),
	}
}

// below functions were copied from the go source repo:
//https://github.com/golang/go/blob/3367475e83eeccd79a5c73c2cc2e91e85e482295/src/testing/match.go#LL284C1-L319C2

func rewriteTestNames(ss ...string) []string {
	m := newMatcher()
	var ret []string
	for _, s := range ss {
		ret = append(ret, m.unique(rewriteTestName(s)))
	}
	return ret
}

// rewriteTestNames rewrites a subname to having only printable characters and no white
// space.
func rewriteTestName(s string) string {
	b := []byte{}
	for _, r := range s {
		switch {
		case isSpace(r):
			b = append(b, '_')
		case !strconv.IsPrint(r):
			s := strconv.QuoteRune(r)
			b = append(b, s[1:len(s)-1]...)
		default:
			b = append(b, string(r)...)
		}
	}
	return string(b)
}

func isSpace(r rune) bool {
	if r < 0x2000 {
		switch r {
		// Note: not the same as Unicode Z class.
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680:
			return true
		}
	} else {
		if r <= 0x200a {
			return true
		}
		switch r {
		case 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
			return true
		}
	}
	return false
}
