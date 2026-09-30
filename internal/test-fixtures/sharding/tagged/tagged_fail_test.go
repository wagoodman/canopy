//go:build shardfail

package tagged

import "testing"

// only compiled with -tags shardfail, to exercise the failing-tests path
func TestShardFail(t *testing.T) {
	t.Fatal("failing on purpose (shardfail build tag)")
}
