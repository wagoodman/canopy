package failure

// Parser defines the interface for failure output parsers.
// Each parser specializes in a specific output format (testify, panic, diff, etc.).
type Parser interface {
	// Name returns a human-readable name for this parser.
	Name() string
	// CanParse returns true if this parser can handle the given output.
	CanParse(output string) bool
	// Parse attempts to parse the output into a structured failure.
	// Returns nil if parsing fails or the output doesn't match expected format.
	Parse(output string) *StructuredFailure
}

// Registry holds parsers in priority order and orchestrates parsing attempts.
type Registry struct {
	parsers []Parser
}

// NewRegistry creates a new registry with the default set of parsers.
// Parsers are tried in order: testify, panic, diff, stdlib (fallback).
func NewRegistry() *Registry {
	return &Registry{
		parsers: []Parser{
			&testifyParser{},
			&panicParser{},
			&diffParser{},
			&stdlibParser{}, // fallback parser
		},
	}
}

// Parse attempts to parse the output using registered parsers.
// Parsers are tried in priority order; the first successful parse wins.
// If no parser matches, returns a StructuredFailure with UnknownFailure type.
func (r *Registry) Parse(output string) *StructuredFailure {
	for _, p := range r.parsers {
		if p.CanParse(output) {
			if sf := p.Parse(output); sf != nil {
				// compute fingerprint for the parsed failure
				sf.Fingerprint = ComputeFingerprint(sf)
				return sf
			}
		}
	}

	// no parser matched, return unknown failure
	sf := &StructuredFailure{
		FailureType: UnknownFailure,
		RawOutput:   output,
	}
	sf.Fingerprint = ComputeFingerprint(sf)
	return sf
}

// RegisterParser adds a parser to the registry at the specified priority.
// Lower index = higher priority (tried first).
func (r *Registry) RegisterParser(index int, p Parser) {
	if index < 0 {
		index = 0
	}
	if index >= len(r.parsers) {
		r.parsers = append(r.parsers, p)
		return
	}
	// insert at index
	r.parsers = append(r.parsers[:index], append([]Parser{p}, r.parsers[index:]...)...)
}

// Parsers returns a copy of the registered parsers.
func (r *Registry) Parsers() []Parser {
	result := make([]Parser, len(r.parsers))
	copy(result, r.parsers)
	return result
}
