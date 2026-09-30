package slow

import (
	"testing"
	"time"
)

func TestSleep1(t *testing.T) {
	time.Sleep(2000 * time.Millisecond)
}
