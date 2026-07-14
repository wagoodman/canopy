package failure

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	// memoryAddressPattern matches Go memory addresses like 0xc0000a4000.
	memoryAddressPattern = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	// timestampPattern matches common timestamp formats.
	timestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
	// nanosecondPattern matches nanosecond timestamps.
	nanosecondPattern = regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d+`)
	// uuidPattern matches UUID v4 format.
	uuidPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	// goroutineIDPattern matches goroutine IDs like "goroutine 42".
	goroutineIDPattern = regexp.MustCompile(`goroutine \d+`)
)

// ComputeFingerprint generates a semantic hash for a structured failure.
// The fingerprint is designed to identify distinct failure modes while
// ignoring transient details like memory addresses, timestamps, and line numbers.
func ComputeFingerprint(sf *StructuredFailure) string {
	if sf == nil {
		return ""
	}

	var components []string

	// include failure type as the primary discriminator
	components = append(components, string(sf.FailureType))

	switch sf.FailureType {
	case AssertionFailure:
		if sf.Assertion != nil {
			components = append(components, sf.Assertion.Library)
			components = append(components, sf.Assertion.Function)
			// normalize expected/actual values to handle transient data
			components = append(components, normalizeValue(sf.Assertion.Expected))
			components = append(components, normalizeValue(sf.Assertion.Actual))
		}
	case PanicFailure:
		if sf.Panic != nil {
			components = append(components, normalizeValue(sf.Panic.Message))
			// include user function names from stack for semantic grouping
			for _, frame := range sf.Panic.Frames {
				if frame.IsUser {
					components = append(components, frame.Function)
				}
			}
		}
	case DiffFailure:
		if sf.Diff != nil {
			// include diff lines in order for semantic grouping
			for _, line := range sf.Diff.Chunks {
				// only include additions and removals in fingerprint (not context)
				if line.Type == DiffChunkAddition || line.Type == DiffChunkRemoval {
					components = append(components, string(line.Type))
					components = append(components, normalizeValue(line.Content))
				}
			}
		}
	case UnknownFailure:
		// for unknown failures, use the normalized raw output
		components = append(components, normalizeValue(sf.RawOutput))
	}

	// include source file (but not line number) for location context
	if sf.Location.File != "" {
		components = append(components, sf.Location.File)
	}

	// compute SHA-256 hash of all components
	combined := strings.Join(components, "\x00")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // use first 16 bytes (32 hex chars)
}

// Normalize strips transient data (addresses, timestamps, UUIDs, goroutine IDs) from a value so
// semantically-identical failures compare equal. exported for within-run clustering, which keys
// panics on their normalized message.
func Normalize(s string) string {
	return normalizeValue(s)
}

// normalizeValue strips transient data from a value to enable semantic comparison.
// This removes memory addresses, timestamps, UUIDs, and other run-specific data.
func normalizeValue(s string) string {
	result := s

	// replace memory addresses with placeholder
	result = memoryAddressPattern.ReplaceAllString(result, "<addr>")

	// replace timestamps
	result = timestampPattern.ReplaceAllString(result, "<timestamp>")
	result = nanosecondPattern.ReplaceAllString(result, "<time>")

	// replace UUIDs
	result = uuidPattern.ReplaceAllString(result, "<uuid>")

	// replace goroutine IDs
	result = goroutineIDPattern.ReplaceAllString(result, "goroutine <id>")

	// normalize whitespace
	result = strings.TrimSpace(result)

	return result
}
