package external_test

import (
	"testing"
	"time"

	"github.com/wagoodman/canopy/internal/test-fixtures/sharding/external"
)

func TestExternal1(t *testing.T) {
	time.Sleep(150 * time.Millisecond)
	if external.Name == "" {
		t.Fatal("empty name")
	}
}

func TestExternal2(t *testing.T) {
	time.Sleep(150 * time.Millisecond)
}
