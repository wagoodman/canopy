package xflagset

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/anchore/fangs"
)

var _ fangs.FlagSet = (*Decorator)(nil)

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

func (f Decorator) RenderFlags() []string {
	var flags []string

	for p, name := range f.intFlags {
		if *p == 0 {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s=%d", name, *p))
	}

	for p, name := range f.boolFlags {
		if !*p {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s", name))
	}

	for p, name := range f.boolRefFlags {
		if *p == nil || !**p {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s", name))
	}

	for p, name := range f.stringFlags {
		if *p == "" {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s=%s", name, *p))
	}

	for p, name := range f.stringArrayFlags {
		if len(*p) == 0 {
			continue
		}
		for _, v := range *p {
			flags = append(flags, fmt.Sprintf("-%s=%s", name, v))
		}
	}

	for p, name := range f.float64Flags {
		if *p == 0 {
			continue
		}
		flags = append(flags, fmt.Sprintf("-%s=%f", name, *p))
	}

	return flags
}

func (f Decorator) BoolVarP(p *bool, name, shorthand, usage string) {
	if !f.noTrack {
		f.boolFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.BoolVarP(p, name, shorthand, usage)
	f.group.BoolVarP(p, name, shorthand, *p, usage)
}

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

func (f Decorator) Float64VarP(p *float64, name, shorthand, usage string) {
	if !f.noTrack {
		f.float64Flags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.Float64VarP(p, name, shorthand, usage)
	f.group.Float64VarP(p, name, shorthand, *p, usage)
}

func (f Decorator) CountVarP(p *int, name, shorthand, usage string) {
	if !f.noTrack {
		f.intFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.CountVarP(p, name, shorthand, usage)
	f.group.CountVarP(p, name, shorthand, usage)
}

func (f Decorator) IntVarP(p *int, name, shorthand, usage string) {
	if !f.noTrack {
		f.intFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.IntVarP(p, name, shorthand, usage)
	f.group.IntVarP(p, name, shorthand, *p, usage)
}

func (f Decorator) StringVarP(p *string, name, shorthand, usage string) {
	if !f.noTrack {
		f.stringFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.StringVarP(p, name, shorthand, usage)
	f.group.StringVarP(p, name, shorthand, *p, usage)
}

func (f Decorator) StringArrayVarP(p *[]string, name, shorthand, usage string) {
	if !f.noTrack {
		f.stringArrayFlags[p] = nameOrShorthand(name, shorthand)
	}
	f.upstream.StringArrayVarP(p, name, shorthand, usage)
	f.group.StringArrayVarP(p, name, shorthand, *p, usage)
}

func (f Decorator) WithNoTrack() Decorator {
	f.noTrack = true
	return f
}

func nameOrShorthand(name, shorthand string) string {
	if name != "" {
		return name
	}
	return shorthand
}
