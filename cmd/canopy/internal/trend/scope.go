package trend

import (
	"path/filepath"
	"strings"
)

// ponytail: package/test glob matching duplicated from internal/flaky (its copy is
// unexported). two copies, not four, since duration/failures/count all route through
// Scope here. upgrade path if a third consumer appears: lift this into one shared home
// and switch flaky over to it.

// matchesPackage reports whether pkg passes the include/exclude patterns.
// no include patterns means everything not excluded matches.
func (s *Scope) matchesPackage(pkg string) bool {
	for _, pattern := range s.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, pkg); matched {
			return false
		}
		if matchGlobPrefix(pattern, pkg) {
			return false
		}
	}

	if len(s.PackagePatterns) == 0 {
		return true
	}

	for _, pattern := range s.PackagePatterns {
		if matched, _ := filepath.Match(pattern, pkg); matched {
			return true
		}
		if matchGlobPrefix(pattern, pkg) {
			return true
		}
	}
	return false
}

// matchesTest reports whether the test name matches the configured pattern (nil = all).
func (s *Scope) matchesTest(testName string) bool {
	if s.TestPattern == nil {
		return true
	}
	return s.TestPattern.MatchString(testName)
}

// matchGlobPrefix handles Go-style "..." patterns and "**" prefixes.
func matchGlobPrefix(pattern, pkg string) bool {
	// trailing /... (common in Go): "foo/..." matches "foo" and "foo/bar" but not "foobar"
	if prefix, ok := strings.CutSuffix(pattern, "/..."); ok {
		return pkg == prefix || strings.HasPrefix(pkg, prefix+"/")
	}

	// "**/..." patterns: match the portion after **/ against every path suffix
	if after, ok := strings.CutPrefix(pattern, "**"); ok {
		rest := strings.TrimPrefix(after, "/")
		if rest == "" {
			return true
		}
		segs := strings.Split(pkg, "/")
		for i := range segs {
			if matched, _ := filepath.Match(rest, strings.Join(segs[i:], "/")); matched {
				return true
			}
		}
	}

	return false
}
