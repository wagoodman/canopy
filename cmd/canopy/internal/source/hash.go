package source

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

// hashFile returns the xxhash64 digest of the file at the given path as a hex string.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("unable to open file for hashing: %w", err)
	}
	defer f.Close()

	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("unable to hash file: %w", err)
	}

	var buf [8]byte
	sum := h.Sum(buf[:0])
	return hex.EncodeToString(sum), nil
}
