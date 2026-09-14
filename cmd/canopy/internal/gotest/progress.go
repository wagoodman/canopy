package gotest

// BuildProgress describes how far `go test` has gotten building the test binaries for a run.
type BuildProgress struct {
	// Expected is the number of packages handed to go test, or 0 when they aren't known up front.
	Expected int

	// Started is the number of packages whose test binary has been built (see Result.StartedPackages).
	Started int
}

// Known reports whether the package set is known up front, and therefore whether Expected means anything.
func (p BuildProgress) Known() bool {
	return p.Expected > 0
}

// Building reports whether packages are still being built or waiting their turn to start. Without a known
// package set the most we can say is whether anything has started yet: once something has, treat the build
// as done rather than claim progress we can't measure.
func (p BuildProgress) Building() bool {
	return p.Started == 0 || p.Started < p.Expected
}

// StartedPackages returns the distinct packages that have emitted a "start" event. The go test command emits
// one once a package's test binary is built, just before running it, but holds it until every earlier package
// (in the order the packages were given to go test) has started. The count therefore trails real build
// progress: a slow package holds back packages after it that are already built.
func (r Result) StartedPackages() []string {
	r.lock.RLock()
	defer r.lock.RUnlock()

	refs, ok := r.referencesByAction[StartAction]
	if !ok {
		return nil
	}

	seen := make(map[string]struct{})
	var pkgs []string
	for _, ref := range refs.Values() {
		if _, ok := seen[ref.Package]; ok {
			continue
		}
		seen[ref.Package] = struct{}{}
		pkgs = append(pkgs, ref.Package)
	}
	return pkgs
}

// ExpectedPackages returns the packages handed to go test for this run, or nil when the run doesn't know them
// up front (e.g. replaying recorded events).
func (r Run) ExpectedPackages() []string {
	if r.Config.Packages == nil {
		return nil
	}
	return r.Config.Packages.ImportPaths()
}

// BuildProgress reports how far this run has gotten building test binaries.
func (r Run) BuildProgress() BuildProgress {
	return BuildProgress{
		Expected: len(r.ExpectedPackages()),
		Started:  len(r.Result.StartedPackages()),
	}
}
