package xflagset

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// source: https://github.com/kubernetes/component-base/blob/b5a495af30a7bb04642ce82f4816b47e75f78dbe/cli/flag/sectioned.go#L33-L41

const (
	usageTemplate = `{{.Long}}

Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{.NamedLocalFlags | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:

{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
)

// Named stores named flag sets in the order of calling FlagSet. This enables organizing flags into logical groups
// for improved help text readability.
type Named struct {
	// Order is an ordered list of flag set names.
	Order []string
	// FlagSets stores the flag sets by name.
	FlagSets map[string]*pflag.FlagSet
	// NormalizeNameFunc is the normalize function which used to initialize FlagSets created by Named.
	NormalizeNameFunc func(f *pflag.FlagSet, name string) pflag.NormalizedName
}

// NewNamed creates a new Named flag set collection with no groups.
func NewNamed() *Named {
	return &Named{
		FlagSets: make(map[string]*pflag.FlagSet),
	}
}

// FlagSet returns the flag set with the given name, creating it if it doesn't exist.
func (nfs *Named) FlagSet(name string) *pflag.FlagSet {
	if nfs.FlagSets == nil {
		nfs.FlagSets = make(map[string]*pflag.FlagSet)
	}
	if _, ok := nfs.FlagSets[name]; !ok {
		flagSet := pflag.NewFlagSet(name, pflag.ExitOnError)
		flagSet.SetNormalizeFunc(pflag.CommandLine.GetNormalizeFunc())
		if nfs.NormalizeNameFunc != nil {
			flagSet.SetNormalizeFunc(nfs.NormalizeNameFunc)
		}
		nfs.FlagSets[name] = flagSet
		nfs.Order = append(nfs.Order, name)
	}
	return nfs.FlagSets[name]
}

// printSections renders all flag groups as help text sections with proper wrapping at the specified column width.
// If cols is zero, lines are not wrapped.
func (nfs *Named) printSections(cols int) string {
	w := &bytes.Buffer{}
	for _, name := range nfs.Order {
		fs := nfs.FlagSets[name]
		if !fs.HasFlags() {
			continue
		}

		wideFS := pflag.NewFlagSet("", pflag.ExitOnError)
		wideFS.AddFlagSet(fs)

		var zzz string
		if cols > 24 {
			zzz = strings.Repeat("z", cols-24)
			wideFS.Int(zzz, 0, strings.Repeat("z", cols-24))
		}

		var buf bytes.Buffer
		fmt.Fprintf(&buf, "\n%s flags:\n\n%s", strings.ToUpper(name[:1])+name[1:], wideFS.FlagUsagesWrapped(cols))

		if cols > 24 {
			i := strings.Index(buf.String(), zzz)
			lines := strings.Split(buf.String()[:i], "\n")
			fmt.Fprint(w, strings.Join(lines[:len(lines)-1], "\n"))
			fmt.Fprintln(w)
		} else {
			fmt.Fprint(w, buf.String())
		}
	}
	return w.String()
}

type cmdWrapper struct {
	*cobra.Command
	NamedLocalFlags string
}

// BindUsageAndHelpFunc customizes the usage and help functions for a cobra command to display named flag groups.
// This organizes flags into logical sections for improved help text readability.
func (nfs *Named) BindUsageAndHelpFunc(cmd *cobra.Command, cols int) {
	help := func(cmd *cobra.Command) error {
		return tmpl(
			cmd.OutOrStdout(),
			usageTemplate,
			&cmdWrapper{
				Command:         cmd,
				NamedLocalFlags: nfs.printSections(cols),
			},
		)
	}
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		return help(cmd)
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		_ = help(cmd)
	})
}

// source: from upstream in cobra lib

var templateFuncs = template.FuncMap{
	"trim":                    strings.TrimSpace,
	"trimRightSpace":          trimRightSpace,
	"trimTrailingWhitespaces": trimRightSpace,
	"appendIfNotPresent":      appendIfNotPresent,
	"rpad":                    rpad,
	"gt":                      Gt,
	"eq":                      Eq,
}

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

// appendIfNotPresent will append stringToAppend to the end of s, but only if it's not yet present in s.
func appendIfNotPresent(s, stringToAppend string) string {
	if strings.Contains(s, stringToAppend) {
		return s
	}
	return s + " " + stringToAppend
}

// rpad adds padding to the right of a string.
func rpad(s string, padding int) string {
	formattedString := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(formattedString, s)
}

// Gt takes two types and checks whether the first type is greater than the second. In case of types Arrays, Chans,
// Maps and Slices, Gt will compare their lengths. Ints are compared directly while strings are first parsed as
// ints and then compared.
func Gt(a interface{}, b interface{}) bool {
	var left, right int64
	av := reflect.ValueOf(a)

	switch av.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		left = int64(av.Len())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		left = av.Int()
	case reflect.String:
		left, _ = strconv.ParseInt(av.String(), 10, 64)
	}

	bv := reflect.ValueOf(b)

	switch bv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		right = int64(bv.Len())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		right = bv.Int()
	case reflect.String:
		right, _ = strconv.ParseInt(bv.String(), 10, 64)
	}

	return left > right
}

// Eq takes two types and checks whether they are equal. Supported types are int and string. Unsupported types will panic.
func Eq(a interface{}, b interface{}) bool {
	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	switch av.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		panic("Eq called on unsupported type")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return av.Int() == bv.Int()
	case reflect.String:
		return av.String() == bv.String()
	}
	return false
}

func tmpl(w io.Writer, text string, data interface{}) error {
	t := template.New("top")
	t.Funcs(templateFuncs)
	template.Must(t.Parse(text))
	return t.Execute(w, data)
}

// Merge combines another Named flag set collection into this one, preserving order and merging duplicate groups.
func (nfs *Named) Merge(other *Named) {
	for _, name := range other.Order {
		if _, exists := nfs.FlagSets[name]; exists {
			nfs.FlagSets[name].AddFlagSet(other.FlagSets[name])
			continue
		}
		nfs.FlagSets[name] = other.FlagSets[name]
		nfs.Order = append(nfs.Order, name)
	}
}

// Add adds one or more pflag.FlagSets to a named group, creating or extending the group as needed.
func (nfs *Named) Add(name string, fss ...*pflag.FlagSet) {
	for _, fs := range fss {
		if _, exists := nfs.FlagSets[name]; exists {
			nfs.FlagSets[name].AddFlagSet(fs)
		} else {
			nfs.FlagSets[name] = fs
			nfs.Order = append(nfs.Order, name)
		}
	}
}
