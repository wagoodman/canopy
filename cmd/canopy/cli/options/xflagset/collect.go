package xflagset

import (
	"reflect"

	"github.com/spf13/cobra"
)

// namedType holds the reflect.Type for *Named to enable type comparison during struct traversal.
var namedType = reflect.TypeOf((*Named)(nil))

// CollectNamedFlagSets recursively walks a struct and collects all *Named fields,
// returning a merged Named containing all discovered flag sets.
func CollectNamedFlagSets(v any) *Named {
	result := NewNamed()
	collectNamedFlagSets(reflect.ValueOf(v), result, make(map[uintptr]bool))
	return result
}

// collectNamedFlagSets recursively traverses the value and merges any *Named fields into result.
// The visited map tracks pointer addresses to prevent infinite loops from circular references.
func collectNamedFlagSets(v reflect.Value, result *Named, visited map[uintptr]bool) {
	// dereference pointers
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		// track visited pointers to prevent infinite loops
		if v.Kind() == reflect.Ptr {
			addr := v.Pointer()
			if visited[addr] {
				return
			}
			visited[addr] = true
		}
		v = v.Elem()
	}

	// only process structs
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// check if this field is a *Named
		if fieldType.Type == namedType {
			if !field.IsNil() {
				named := field.Interface().(*Named)
				result.Merge(named)
			}
			continue
		}

		// recurse into structs and pointers to structs
		switch field.Kind() {
		case reflect.Struct:
			collectNamedFlagSets(field, result, visited)
		case reflect.Ptr:
			if !field.IsNil() {
				collectNamedFlagSets(field, result, visited)
			}
		}
	}
}

// BindCobraHelpFromOpts configures the cobra command's help and usage functions to display
// named flag groups collected from the options struct. It wraps the original help function
// to preserve default behavior while adding organized flag sections.
func BindCobraHelpFromOpts(cmd *cobra.Command, opts any) {
	ogHelp := cmd.Help
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		nfs := CollectNamedFlagSets(opts)
		nfs.BindUsageAndHelpFunc(cmd, -1)
		_ = ogHelp()
	})
}
