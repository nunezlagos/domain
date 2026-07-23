package main

import (
	"fmt"
	"sort"
	"time"
)

// result es el desenlace de un ciclo Spawn→Prompt→Close.
type result struct {
	d   time.Duration
	err error
}

func pct(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(p * float64(len(sorted)-1))
	return sorted[i]
}

// summarize imprime latencias p50/p95/p99, error-rate y throughput de un batch.
func summarize(name string, rs []result, wall time.Duration) {
	var oks []time.Duration
	errs := map[string]int{}
	for _, r := range rs {
		if r.err != nil {
			errs[classify(r.err)]++
			continue
		}
		oks = append(oks, r.d)
	}
	sort.Slice(oks, func(i, j int) bool { return oks[i] < oks[j] })
	rate := float64(len(rs)-len(oks)) / float64(len(rs)) * 100
	tput := float64(len(rs)) / wall.Seconds()

	fmt.Printf("\n[%s] n=%d ok=%d err=%.1f%% wall=%s tput=%.0f/s\n",
		name, len(rs), len(oks), rate, wall.Round(time.Millisecond), tput)
	if len(oks) > 0 {
		fmt.Printf("   lat p50=%s p95=%s p99=%s max=%s\n",
			pct(oks, .5).Round(time.Microsecond),
			pct(oks, .95).Round(time.Microsecond),
			pct(oks, .99).Round(time.Microsecond),
			oks[len(oks)-1].Round(time.Microsecond))
	}
	for k, v := range errs {
		fmt.Printf("   err[%s]=%d\n", k, v)
	}
}

func classify(err error) string {
	s := err.Error()
	for _, m := range []string{"initialize", "new session", "prompt", "spawn", "context deadline", "context canceled"} {
		if contains(s, m) {
			return m
		}
	}
	return "other"
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
