// Package output — formato de output para el CLI (json/table/yaml).
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
	FormatYAML  Format = "yaml"
	FormatCSV   Format = "csv"
)

// RenderOpts opciones de render.
type RenderOpts struct {
	Format    Format
	NoHeaders bool // omitir cabeceras en table/csv
}

// Render imprime data en el formato especificado.
func Render(w io.Writer, data any, format Format) error {
	return RenderWithOpts(w, data, RenderOpts{Format: format})
}

// RenderWithOpts imprime data con opciones.
func RenderWithOpts(w io.Writer, data any, opts RenderOpts) error {
	switch opts.Format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		return enc.Encode(data)
	case FormatCSV:
		return renderCSV(w, data, opts.NoHeaders)
	case FormatTable:
		return renderTable(w, data, opts.NoHeaders)
	default:
		return fmt.Errorf("unknown format: %s", opts.Format)
	}
}

func renderTable(w io.Writer, data any, noHeaders bool) error {
	if data == nil {
		fmt.Fprintln(w, "(no data)")
		return nil
	}

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return renderSliceTable(w, v, noHeaders)
	case reflect.Map:
		return renderMapTable(w, v)
	default:
		fmt.Fprintf(w, "%v\n", data)
		return nil
	}
}

func renderSliceTable(w io.Writer, v reflect.Value, noHeaders bool) error {
	if v.Len() == 0 {
		fmt.Fprintln(w, "(empty)")
		return nil
	}

	first := v.Index(0).Interface()
	firstMap, ok := first.(map[string]any)
	if !ok {
		for i := 0; i < v.Len(); i++ {
			fmt.Fprintf(w, "%v\n", v.Index(i).Interface())
		}
		return nil
	}

	cols := pickColumns(firstMap)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if !noHeaders {
		fmt.Fprintln(tw, strings.Join(toUpper(cols), "\t"))
	}
	for i := 0; i < v.Len(); i++ {
		row, ok := v.Index(i).Interface().(map[string]any)
		if !ok {
			continue
		}
		vals := make([]string, len(cols))
		for j, c := range cols {
			vals[j] = formatCell(row[c])
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	return tw.Flush()
}

func renderMapTable(w io.Writer, v reflect.Value) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	iter := v.MapRange()
	for iter.Next() {
		fmt.Fprintf(tw, "%v:\t%v\n", iter.Key().Interface(), formatCell(iter.Value().Interface()))
	}
	return tw.Flush()
}

func renderCSV(w io.Writer, data any, noHeaders bool) error {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Errorf("csv format requires a list, got %T", data)
	}
	if v.Len() == 0 {
		return nil
	}

	cw := csv.NewWriter(w)
	first := v.Index(0).Interface()
	firstMap, ok := first.(map[string]any)
	if !ok {
		return fmt.Errorf("csv format requires map items, got %T", first)
	}

	cols := pickColumnsAll(firstMap)
	if !noHeaders {
		_ = cw.Write(cols)
	}
	for i := 0; i < v.Len(); i++ {
		row, ok := v.Index(i).Interface().(map[string]any)
		if !ok {
			continue
		}
		vals := make([]string, len(cols))
		for j, c := range cols {
			vals[j] = fmt.Sprintf("%v", row[c])
		}
		_ = cw.Write(vals)
	}
	cw.Flush()
	return cw.Error()
}

func pickColumnsAll(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// pickColumns escoge columnas a mostrar priorizando campos importantes
// y dejando los menos importantes fuera (max 6 cols).
func pickColumns(m map[string]any) []string {
	priority := []string{
		"id", "ID", "slug", "name", "title", "status", "email", "role",
		"started_at", "created_at", "updated_at", "ended_at",
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range priority {
		if _, ok := m[p]; ok && !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
		if len(out) >= 6 {
			return out
		}
	}

	for k := range m {
		if seen[k] {
			continue
		}
		if len(out) >= 6 {
			break
		}
		out = append(out, k)
	}
	return out
}

func toUpper(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}

func formatCell(v any) string {
	if v == nil {
		return "-"
	}
	switch x := v.(type) {
	case string:
		if len(x) > 50 {
			return x[:47] + "..."
		}
		return x
	case bool, float64, int, int64:
		return fmt.Sprintf("%v", x)
	default:
		raw, _ := json.Marshal(x)
		s := string(raw)
		if len(s) > 50 {
			return s[:47] + "..."
		}
		return s
	}
}
