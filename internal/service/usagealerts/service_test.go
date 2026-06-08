package usagealerts

import "testing"

func TestCompareThreshold(t *testing.T) {
	cases := []struct {
		obs, thr float64
		cond     string
		want     bool
	}{
		{10, 5, ConditionGT, true},
		{5, 10, ConditionGT, false},
		{5, 5, ConditionGT, false},
		{5, 10, ConditionLT, true},
		{10, 5, ConditionLT, false},
		{5, 5, ConditionEQ, true},
		{5, 6, ConditionEQ, false},
		{0, 0, "unknown_condition", false},
	}
	for _, tc := range cases {
		if got := CompareThreshold(tc.obs, tc.thr, tc.cond); got != tc.want {
			t.Fatalf("Compare(%v, %v, %s) = %v, want %v", tc.obs, tc.thr, tc.cond, got, tc.want)
		}
	}
}

func TestValidMetrics(t *testing.T) {
	for _, m := range []string{MetricCostPerRun, MetricCostPerDay, MetricCostPerMonth,
		MetricTokensPerRun, MetricTokensPerDay} {
		if !validMetrics[m] {
			t.Fatalf("metric %s should be valid", m)
		}
	}
	if validMetrics["random_metric"] {
		t.Fatal("random metric should NOT be valid")
	}
}

func TestValidConditions(t *testing.T) {
	if !validConditions[ConditionGT] || !validConditions[ConditionLT] || !validConditions[ConditionEQ] {
		t.Fatal("all conditions should be valid")
	}
	if validConditions["random"] {
		t.Fatal("random condition should NOT be valid")
	}
}
