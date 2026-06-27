package skill_ab_test

import "math"

// analyzer.go — ESTADISTICA PURA. z-test de proporciones de DOS muestras (Wald
// con varianza pooled bajo H0). SIN LLM, SIN deps externas: solo math std.
//
// H0: pA == pB  (las dos variantes tienen la misma tasa de exito real).
// H1: pA != pB  (test de dos colas).
//
// Estadistico:
//
//	pA = sA/nA, pB = sB/nB
//	p  = (sA+sB)/(nA+nB)            (proporcion pooled bajo H0)
//	se = sqrt( p*(1-p)*(1/nA + 1/nB) )
//	z  = (pA - pB) / se
//	p-value (dos colas) = 2 * (1 - Phi(|z|)) = erfc(|z|/sqrt(2))
//
// Phi es la CDF de la normal estandar; usamos la identidad
// 1 - Phi(x) = 0.5*erfc(x/sqrt2), por lo que el p-value de dos colas es
// directamente erfc(|z|/sqrt2) — exacto, sin aproximaciones polinomicas propias
// (math.Erfc es de la stdlib).
//
// Decision: si p-value < alpha -> rechazamos H0 y declaramos ganador la variante
// con mayor proporcion. confidence = 1 - p-value. Si no -> 'inconclusive'.

// ZTestResult es el resultado del z-test de dos proporciones.
type ZTestResult struct {
	PropA   float64 // pA = sA/nA
	PropB   float64 // pB = sB/nB
	Z       float64 // estadistico z (0 si se no se puede computar)
	PValue  float64 // p-value de dos colas (1.0 si degenerado)
	Winner  string  // "a" | "b" | "inconclusive"
	Confidence float64 // 1 - PValue cuando hay ganador; 0 si inconclusive
}

// sqrt2 precomputado.
var sqrt2 = math.Sqrt(2)

// TwoProportionZTest corre el z-test de dos proporciones.
//
//	nA, sA: invocaciones y exitos de la variante A.
//	nB, sB: invocaciones y exitos de la variante B.
//	alpha:  nivel de significancia (ej 0.05). <=0 cae a DefaultAlpha.
//
// Casos degenerados (n<=0 en alguna variante, o se=0 porque ambas son 0% o 100%)
// devuelven 'inconclusive' con PValue=1.0: no hay evidencia para rechazar H0.
func TwoProportionZTest(nA, sA, nB, sB int, alpha float64) ZTestResult {
	if alpha <= 0 {
		alpha = DefaultAlpha
	}
	res := ZTestResult{Winner: WinnerInconclusive, PValue: 1.0}

	if nA <= 0 || nB <= 0 {
		return res
	}

	pA := float64(sA) / float64(nA)
	pB := float64(sB) / float64(nB)
	res.PropA = pA
	res.PropB = pB

	pooled := float64(sA+sB) / float64(nA+nB)
	// Varianza pooled bajo H0. Si pooled es 0 o 1 (ambas variantes 0% o 100%),
	// la varianza es 0 -> z indefinido. Sin dispersion no hay test posible:
	// inconclusive (no podemos distinguir aunque las proporciones difieran, no
	// hay datos de exito/fallo mezclados).
	variance := pooled * (1 - pooled) * (1.0/float64(nA) + 1.0/float64(nB))
	if variance <= 0 {
		return res
	}

	z := (pA - pB) / math.Sqrt(variance)
	res.Z = z

	// p-value de dos colas: erfc(|z|/sqrt2).
	pValue := math.Erfc(math.Abs(z) / sqrt2)
	res.PValue = pValue

	if pValue < alpha {
		if pA > pB {
			res.Winner = WinnerA
		} else {
			res.Winner = WinnerB
		}
		res.Confidence = 1 - pValue
	}
	return res
}

// Analyzer corre el z-test sobre los resultados de un test y produce un veredicto.
// ESTADISTICA PURA: no toca DB ni LLM. El cron resuelve la persistencia.
type Analyzer struct {
	Alpha float64 // <=0 usa DefaultAlpha
}

// NewAnalyzer crea un Analyzer con el alpha dado (0 -> DefaultAlpha).
func NewAnalyzer(alpha float64) *Analyzer {
	return &Analyzer{Alpha: alpha}
}

// Verdict es la decision del Analyzer sobre un test.
type Verdict struct {
	// Ready=false significa que aun NO se alcanzo min_invocations en alguna
	// variante: el cron NO debe declarar ganador todavia.
	Ready  bool
	Winner string // "a" | "b" | "inconclusive" (solo valido si Ready)
	// Confidence = 1 - p-value cuando hay ganador; 0 en inconclusive.
	Confidence float64
	ZTest      ZTestResult
}

// Analyze decide el veredicto de un test dadas las dos variantes y el minimo de
// invocaciones requerido. Si CUALQUIER variante no alcanzo minInvocations,
// devuelve Ready=false (el experimento sigue corriendo). Si ambas lo alcanzaron,
// corre el z-test.
func (a *Analyzer) Analyze(resA, resB VariantResult, minInvocations int) Verdict {
	if minInvocations <= 0 {
		minInvocations = DefaultMinInvocations
	}
	if resA.InvocationsCount < minInvocations || resB.InvocationsCount < minInvocations {
		return Verdict{Ready: false}
	}
	zt := TwoProportionZTest(
		resA.InvocationsCount, resA.SuccessCount,
		resB.InvocationsCount, resB.SuccessCount,
		a.Alpha,
	)
	return Verdict{
		Ready:      true,
		Winner:     zt.Winner,
		Confidence: zt.Confidence,
		ZTest:      zt,
	}
}
