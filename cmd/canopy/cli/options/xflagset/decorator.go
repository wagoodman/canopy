package xflagset

import (
	"fmt"

	"github.com/scylladb/go-set/strset"
	"github.com/spf13/pflag"

	"github.com/anchore/fangs"
)

var _ fangs.FlagSet = (*Decorator)(nil)

// Decorator wraps a fangs.FlagSet to track which flags are set by the user, enabling selective rendering
// of flags for passing to external commands like `go test`.
type Decorator struct {
	upstream fangs.FlagSet
	group    *pflag.FlagSet

	intFlags         map[*int]string
	boolFlags        map[*bool]string
	boolRefFlags     map[**bool]string
	stringFlags      map[*string]string
	stringArrayFlags map[*[]string]string
	float64Flags     map[*float64]string

	noTrack bool
}

// NewDecorator creates a new flag decorator that tracks flag values and associates them with a named flag group.
func NewDecorator(upstream fangs.FlagSet, group *pflag.FlagSet) *Decorator {
	return &Decorator{
		upstream:         upstream,
		group:            group,
		intFlags:         make(map[*int]string),
		boolFlags:        make(map[*bool]string),
		boolRefFlags:     make(map[**bool]string),
		stringFlags:      make(map[*string]string),
		stringArrayFlags: make(map[*[]string]string),
		float64Flags:     make(map[*float64]string),
	}
}

// RenderFlags returns the command-line representation of all tracked flags that have non-default values.
// Flags in the ignore list are excluded from the output.
func (f Decorator) RenderFlags(ignore ...string) []string {
	var flags []string
	ignoreSet := strset.New(ignore...)

	flags = append(flags, f.renderIntFlags(ignoreSet)...)
	flags = append(flags, f.renderBoolFlags(ignoreSet)...)
	flags = append(flags, f.renderBoolRefFlags(ignoreSet)...)
	flags = append(flags, f.renderStringFlags(ignoreSet)...)
	flags = append(flags, f.renderStringArrayFlags(ignoreSet)...)
	flags = append(flags, f.renderFloat64Flags(ignoreSet)...)

	return flags
}

// renderIntFlags processes int flags and returns their string representations.
func (f Decorator) renderIntFlags(ignoreSet *strset.Set) []string {
	var flags []string
	for p, name := range f.intFlags {
		if ignoreSet.Has(name) {
			continue
		}
		if *p == 0 {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s=%d", name, *p))
	}
	return flags
}

// renderBoolFlags processes bool flags and returns their string representations.
func (f Decorator) renderBoolFlags(ignoreSet *strset.Set) []string {
	var flags []string
	for p, name := range f.boolFlags {
		if ignoreSet.Has(name) {
			continue
		}
		if !*p {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s", name))
	}
	return flags
}

// renderBoolRefFlags processes bool reference flags and returns their string representations.
func (f Decorator) renderBoolRefFlags(ignoreSet *strset.Set) []string {
	var flags []string
	for p, name := range f.boolRefFlags {
		if ignoreSet.Has(name) {
			continue
		}
		if *p == nil || !**p {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s", name))
	}
	return flags
}

// renderStringFlags processes string flags and returns their string representations.
func (f Decorator) renderStringFlags(ignoreSet *strset.Set) []string {
	var flags []string
	for p, name := range f.stringFlags {
		if ignoreSet.Has(name) {
			continue
		}
		if *p == "" {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s=%s", name, *p))
	}
	return flags
}

// renderStringArrayFlags processes string array flags and returns their string representations.
func (f Decorator) renderStringArrayFlags(ignoreSet *strset.Set) []string {
	var flags []string
	for p, name := range f.stringArrayFlags {
		if ignoreSet.Has(name) {
			continue
		}
		if len(*p) == 0 {
			continue
		}
		for _, v := range *p {
			flags = append(flags, fmt.Sprintf("-%s=%s", name, v))
		}
	}
	return flags
}

// renderFloat64Flags processes float64 flags and returns their string representations.
func (f Decorator) renderFloat64Flags(ignoreSet *strset.Set) []string {
	var flags []string
	for p, name := range f.float64Flags {
		if ignoreSet.Has(name) {
			continue
		}
		if *p == 0 {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s=%f", name, *p))
	}
	return flags
}

// BoolVarP registers a bool flag and tracks its pointer for rendering.
func (f Decorator) BoolVarP(p *bool, name, shorthand, usage string) {
	if !f.noTrack {
		f.boolFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.BoolVarP(p, name, shorthand, usage)
	f.group.BoolVarP(p, name, shorthand, *p, usage)
}

// BoolPtrVarP registers a bool pointer flag and tracks it for rendering.
func (f Decorator) BoolPtrVarP(p **bool, name, shorthand, usage string) {
	if !f.noTrack {
		f.boolRefFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.BoolPtrVarP(p, name, shorthand, usage)
	var val bool
	if *p != nil {
		val = **p
	}
	f.group.BoolVarP(*p, name, shorthand, val, usage)
}

// Float64VarP registers a float64 flag and tracks its pointer for rendering.
func (f Decorator) Float64VarP(p *float64, name, shorthand, usage string) {
	if !f.noTrack {
		f.float64Flags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.Float64VarP(p, name, shorthand, usage)
	f.group.Float64VarP(p, name, shorthand, *p, usage)
}

// CountVarP registers a count flag and tracks its pointer for rendering.
func (f Decorator) CountVarP(p *int, name, shorthand, usage string) {
	if !f.noTrack {
		f.intFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.CountVarP(p, name, shorthand, usage)
	f.group.CountVarP(p, name, shorthand, usage)
}

// IntVarP registers an int flag and tracks its pointer for rendering.
func (f Decorator) IntVarP(p *int, name, shorthand, usage string) {
	if !f.noTrack {
		f.intFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.IntVarP(p, name, shorthand, usage)
	f.group.IntVarP(p, name, shorthand, *p, usage)
}

// StringVarP registers a string flag and tracks its pointer for rendering.
func (f Decorator) StringVarP(p *string, name, shorthand, usage string) {
	if !f.noTrack {
		f.stringFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.StringVarP(p, name, shorthand, usage)
	f.group.StringVarP(p, name, shorthand, *p, usage)
}

// StringArrayVarP registers a string array flag and tracks its pointer for rendering.
func (f Decorator) StringArrayVarP(p *[]string, name, shorthand, usage string) {
	if !f.noTrack {
		f.stringArrayFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.StringArrayVarP(p, name, shorthand, usage)
	f.group.StringArrayVarP(p, name, shorthand, *p, usage)
}

// WithNoTrack returns a copy of the Decorator with tracking disabled. Flags registered through this copy
// will not be included in RenderFlags output.
func (f Decorator) WithNoTrack() Decorator {
	f.noTrack = true
	return f
}

// nameOrShorthand returns the flag name if provided, otherwise returns the shorthand.
func nameOrShorthand(name, shorthand string) string {
	if name != "" {
		return name
	}
	return shorthand
}
