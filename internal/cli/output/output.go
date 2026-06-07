// Package output — formato de output para el CLI (json/table/yaml).
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

// Render imprime data en el formato especificado.
// Para FormatTable: si data es []any de objects, imprime tabla.
// Si es object, imprime key:value.
func Render(w io.Writer, data any, format Format) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatTable:
		return renderTable(w, data)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func renderTable(w io.Writer, data any) error {
	if data == nil {
		fmt.Fprintln(w, "(no data)")
		return nil
	}

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return renderSlice(w, v)
	case reflect.Map:
		return renderMap(w, v)
	default:
		fmt.Fprintf(w, "%v\n", data)
		return nil
	}
}

func renderSlice(w io.Writer, v reflect.Value) error {
	if v.Len() == 0 {
		fmt.Fprintln(w, "(empty)")
		return nil
	}

	// Detectar columnas del primer elemento (si es map)
	first := v.Index(0).Interface()
	firstMap, ok := first.(map[string]any)
	if !ok {
		// Fallback: render como lines
		for i := 0; i < v.Len(); i++ {
			fmt.Fprintf(w, "%v\n", v.Index(i).Interface())
		}
		return nil
	}

	// Columnas priorizadas (id, slug, name, status, etc.)
	cols := pickColumns(firstMap)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(toUpper(cols), "\t"))
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

func renderMap(w io.Writer, v reflect.Value) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	iter := v.MapRange()
	for iter.Next() {
		fmt.Fprintf(tw, "%v:\t%v\n", iter.Key().Interface(), formatCell(iter.Value().Interface()))
	}
	return tw.Flush()
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
	// Fill con resto hasta 6
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
