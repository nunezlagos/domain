package cursor

import (
	"errors"
	"testing"
	"time"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	c := Cursor{
		LastID:        "abc-123",
		LastSortValue: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
		FiltersHash:   "deadbeef",
		SortDir:       "desc",
	}
	enc := c.Encode()
	dec, err := Decode(enc, "deadbeef", "desc")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec.LastID != c.LastID || dec.SortDir != c.SortDir || dec.FiltersHash != c.FiltersHash {
		t.Fatalf("mismatch: %+v vs %+v", dec, c)
	}
	if !dec.LastSortValue.Equal(c.LastSortValue) {
		t.Fatalf("time mismatch: %v vs %v", dec.LastSortValue, c.LastSortValue)
	}
}

func TestDecode_Tampered(t *testing.T) {
	_, err := Decode("not-base64-!!", "x", "desc")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestDecode_FiltersMismatch(t *testing.T) {
	c := Cursor{LastID: "a", FiltersHash: "h1", SortDir: "desc"}
	_, err := Decode(c.Encode(), "h2", "desc")
	if !errors.Is(err, ErrFiltersMismatch) {
		t.Fatalf("expected ErrFiltersMismatch, got %v", err)
	}
}

func TestDecode_SortMismatch(t *testing.T) {
	c := Cursor{LastID: "a", FiltersHash: "h1", SortDir: "desc"}
	_, err := Decode(c.Encode(), "h1", "asc")
	if !errors.Is(err, ErrSortMismatch) {
		t.Fatalf("expected ErrSortMismatch, got %v", err)
	}
}

func TestHashFilters_Stable(t *testing.T) {
	a := HashFilters(map[string]string{"x": "1", "y": "2"})
	b := HashFilters(map[string]string{"y": "2", "x": "1"})
	if a != b {
		t.Fatalf("expected stable across map order, got %s vs %s", a, b)
	}
}

func TestHashFilters_DistinctOnChange(t *testing.T) {
	a := HashFilters(map[string]string{"x": "1"})
	b := HashFilters(map[string]string{"x": "2"})
	if a == b {
		t.Fatal("hash should change when value changes")
	}
}

func TestNormalizeSort(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"", "desc", false},
		{"DESC", "desc", false},
		{"asc", "asc", false},
		{"Asc ", "asc", false},
		{"random", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizeSort(tc.in)
		if tc.err {
			if err == nil {
				t.Fatalf("%q: expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: want %s, got %s", tc.in, tc.want, got)
		}
	}
}
