package handler

import "fmt"

// ErrPackageComplete is a sentinel error returned when a package handler has
// completed processing all events for its package and should be removed from
// the active handler set.
var ErrPackageComplete = fmt.Errorf("package output complete")
