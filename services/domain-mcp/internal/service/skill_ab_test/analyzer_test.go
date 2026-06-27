package skill_ab_test

import (
	"math"
	"testing"
)

// TestZTestKnownValue valida el z-test contra un caso calculado a mano.
//
// nA=100, sA=90 (pA=0.90); nB=100, sB=70 (pB=0.70).
//   pooled = 160/200 = 0.80
//   se = sqrt(0.8*0.2*(1/100+1/100)) = sqrt(0.16*0.02) = sqrt(0.0032)
//      = 0.0565685...
//   z  = (0.90-0.70)/0.0565685 = 0.20/0.0565685 = 3.53553...
//   p-value (dos colas) = erfc(|z|/sqrt2) = erfc(2.5) ~= 0.000407
// => p < 0.05, ganador A.
func TestZTestKnownValue(t *testing.T) {
	r := TwoProportionZTest(100, 90, 100, 70, 0.05)

	wantZ := 3.5355339059
	if math.Abs(r.Z-wantZ) > 1e-6 {
		t.Fatalf("z=%.10f, esperaba %.10f", r.Z, wantZ)
	}
	wantP := math.Erfc(2.5) // ~0.0004069520
	if math.Abs(r.PValue-wantP) > 1e-9 {
		t.Fatalf("p-value=%.10f, esperaba %.10f", r.PValue, wantP)
	}
	if r.Winner != WinnerA {
		t.Fatalf("winner=%s, esperaba a", r.Winner)
	}
	if r.Confidence <= 0.99 {
		t.Fatalf("confidence=%.6f, esperaba >0.99", r.Confidence)
	}
}

// TestZTestSymmetry: invertir A y B invierte el signo de z y el ganador, mismo
// |z| y mismo p-value.
func TestZTestSymmetry(t *testing.T) {
	ab := TwoProportionZTest(100, 90, 100, 70, 0.05)
	ba := TwoProportionZTest(100, 70, 100, 90, 0.05)

	if math.Abs(ab.Z+ba.Z) > 1e-9 {
		t.Fatalf("z no es simetrico: ab.Z=%.6f ba.Z=%.6f", ab.Z, ba.Z)
	}
	if math.Abs(ab.PValue-ba.PValue) > 1e-12 {
		t.Fatalf("p-value no es simetrico: %.12f vs %.12f", ab.PValue, ba.PValue)
	}
	if ba.Winner != WinnerB {
		t.Fatalf("invertido deberia ganar B, dio %s", ba.Winner)
	}
}

// TestZTestTieInconclusive: proporciones identicas -> z=0, inconclusive.
func TestZTestTieInconclusive(t *testing.T) {
	r := TwoProportionZTest(500, 400, 500, 400, 0.05)
	if math.Abs(r.Z) > 1e-12 {
		t.Fatalf("empate deberia dar z=0, dio %.6f", r.Z)
	}
	if r.PValue < 0.999 {
		t.Fatalf("empate deberia dar p-value ~1, dio %.6f", r.PValue)
	}
	if r.Winner != WinnerInconclusive {
		t.Fatalf("empate deberia ser inconclusive, dio %s", r.Winner)
	}
}

// TestZTestDegenerate: n=0 o varianza 0 -> inconclusive con p=1.
func TestZTestDegenerate(t *testing.T) {
	cases := []struct {
		name               string
		nA, sA, nB, sB     int
	}{
		{"nA cero", 0, 0, 100, 50},
		{"nB cero", 100, 50, 0, 0},
		{"ambas 100%", 100, 100, 100, 100},
		{"ambas 0%", 100, 0, 100, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := TwoProportionZTest(tc.nA, tc.sA, tc.nB, tc.sB, 0.05)
			if r.Winner != WinnerInconclusive {
				t.Fatalf("%s deberia ser inconclusive, dio %s", tc.name, r.Winner)
			}
			if r.PValue != 1.0 {
				t.Fatalf("%s deberia tener p-value=1, dio %.6f", tc.name, r.PValue)
			}
		})
	}
}

// --- Analyzer (con min_invocations) ---

func vr(version string, n, s int) VariantResult {
	return VariantResult{Version: version, InvocationsCount: n, SuccessCount: s}
}

// TestAnalyzeAClearlyBetter: A mucho mejor con muestra suficiente -> ready, A gana.
func TestAnalyzeAClearlyBetter(t *testing.T) {
	a := NewAnalyzer(0.05)
	v := a.Analyze(vr("a", 200, 180), vr("b", 200, 120), 100)
	if !v.Ready {
		t.Fatal("con 200/200 invocaciones (>=100) deberia estar ready")
	}
	if v.Winner != WinnerA {
		t.Fatalf("A claramente mejor deberia ganar A, dio %s", v.Winner)
	}
	if v.Confidence <= 0.95 {
		t.Fatalf("confidence baja: %.6f", v.Confidence)
	}
}

// TestAnalyzeTie: empate con muestra grande -> ready pero inconclusive.
func TestAnalyzeTie(t *testing.T) {
	a := NewAnalyzer(0.05)
	v := a.Analyze(vr("a", 1000, 800), vr("b", 1000, 800), 100)
	if !v.Ready {
		t.Fatal("muestra grande deberia estar ready")
	}
	if v.Winner != WinnerInconclusive {
		t.Fatalf("empate deberia ser inconclusive, dio %s", v.Winner)
	}
}

// TestAnalyzeSmallSampleNotReady: por debajo de min_invocations -> not ready,
// el cron NO debe declarar ganador aunque A parezca mejor.
func TestAnalyzeSmallSampleNotReady(t *testing.T) {
	a := NewAnalyzer(0.05)
	v := a.Analyze(vr("a", 10, 9), vr("b", 10, 3), 100)
	if v.Ready {
		t.Fatal("muestra chica (<min_invocations) NO deberia estar ready")
	}
	if v.Winner != "" {
		t.Fatalf("not ready no deberia traer winner, dio %q", v.Winner)
	}
}

// TestAnalyzeSmallDiffInconclusive: diferencia pequena con muestra justa al
// minimo no alcanza significancia -> inconclusive.
func TestAnalyzeSmallDiffInconclusive(t *testing.T) {
	a := NewAnalyzer(0.05)
	// pA=0.52, pB=0.50 con n=100 cada una: z chico, p>0.05.
	v := a.Analyze(vr("a", 100, 52), vr("b", 100, 50), 100)
	if !v.Ready {
		t.Fatal("n=100 cada una deberia estar ready")
	}
	if v.Winner != WinnerInconclusive {
		t.Fatalf("diferencia chica deberia ser inconclusive, dio %s (p=%.4f)",
			v.Winner, v.ZTest.PValue)
	}
}

// TestAnalyzeDefaultMinInvocations: minInvocations<=0 cae al default (100).
func TestAnalyzeDefaultMinInvocations(t *testing.T) {
	a := NewAnalyzer(0)
	// 99 < default 100 -> not ready.
	v := a.Analyze(vr("a", 99, 90), vr("b", 99, 50), 0)
	if v.Ready {
		t.Fatal("99 invocaciones con default min=100 NO deberia estar ready")
	}
}
