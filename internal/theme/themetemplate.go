package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// writeTemplateOptions controls template generation.
type WriteTemplateOptions struct {
	// Base is the scheme to copy the field values from (Default/Darkula/Light).
	Base Scheme
	// Overwrite allows replacing an existing file. The default (false) refuses
	// to overwrite an existing theme file to avoid clobbering user work.
	Overwrite bool
}

// WriteTemplate writes the full set of scheme fields to outPath as a YAML file
// with a hint comment per field. outPath is the target .theme file path. It
// returns an error (without writing) if the file already exists and Overwrite
// is false.
func WriteTemplate(name string, opts WriteTemplateOptions, outPath string) error {
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("theme file %s already exists (use --overwrite to replace)", outPath)
		}
	}
	base := opts.Base
	if base == (Scheme{}) {
		base = Darkula()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Color scheme: %s\n", name)
	b.WriteString("# ANSI-256 color numbers (0-255). Edit values, save, and pick\n")
	b.WriteString("# the scheme in the app. Omitted fields fall back to 'default'.\n")

	// Deterministic field order: sorted by YAML tag.
	typ := reflect.TypeOf(base)
	fields := make([]struct {
		tag string
		val string
	}, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		fields = append(fields, struct{ tag, val string }{tag, reflect.ValueOf(base).Field(i).String()})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].tag < fields[j].tag })

	for _, f := range fields {
		fmt.Fprintf(&b, "%s: %s\n", f.tag, f.val)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o644)
}
