package style

// CanceledGlyph replaces the spinner (and the summary status) once a run is interrupted, so in-flight rows don't
// look frozen. It is intentionally distinct from pass/fail/skip markers, and a text glyph (no emoji presentation).
const CanceledGlyph = "⊘"
