package skill_ab_test

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

// TestBucketDeterministic: mismo (slug,user) -> mismo bucket SIEMPRE.
func TestBucketDeterministic(t *testing.T) {
	slug := "code-review"
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	first := Bucket(slug, uid)
	for i := 0; i < 1000; i++ {
		if got := Bucket(slug, uid); got != first {
			t.Fatalf("bucket no determinista: iter %d dio %d, esperaba %d", i, got, first)
		}
	}
	if first < 0 || first >= 100 {
		t.Fatalf("bucket fuera de rango 0..99: %d", first)
	}
}

// TestBucketRange: todos los buckets caen en 0..99 para muchos usuarios.
func TestBucketRange(t *testing.T) {
	for i := 0; i < 5000; i++ {
		b := Bucket("slug", uuid.New())
		if b < 0 || b >= 100 {
			t.Fatalf("bucket fuera de rango: %d", b)
		}
	}
}

// TestPickBoundaries cubre los limites de pick (0/100 split).
func TestPickBoundaries(t *testing.T) {
	// split=0 -> siempre B (ningun bucket < 0).
	for b := 0; b < 100; b++ {
		if pick(b, 0.0) != VariantB {
			t.Fatalf("split=0 bucket %d deberia ser B", b)
		}
	}
	// split=1 -> siempre A (todo bucket < 100).
	for b := 0; b < 100; b++ {
		if pick(b, 1.0) != VariantA {
			t.Fatalf("split=1 bucket %d deberia ser A", b)
		}
	}
	// clamp: split fuera de [0,1].
	if pick(50, -0.5) != VariantB {
		t.Fatal("split negativo deberia clamp a 0 -> B")
	}
	if pick(50, 1.5) != VariantA {
		t.Fatal("split >1 deberia clamp a 1 -> A")
	}
}

// proportionForSplit corre N usuarios random por pick(Bucket) y devuelve la
// fraccion que cayo en A.
func proportionForSplit(slug string, split float64, n int) float64 {
	a := 0
	for i := 0; i < n; i++ {
		if pick(Bucket(slug, uuid.New()), split) == VariantA {
			a++
		}
	}
	return float64(a) / float64(n)
}

// TestSplitProportions verifica que la proporcion empirica se acerca al split
// configurado para 50/50, 70/30 y 0/100.
func TestSplitProportions(t *testing.T) {
	const n = 20000
	const tol = 0.03 // 3 puntos porcentuales de tolerancia

	cases := []struct {
		name  string
		split float64
		want  float64
	}{
		{"50/50", 0.50, 0.50},
		{"70/30", 0.70, 0.70},
		{"0/100", 0.00, 0.00},
		{"100/0", 1.00, 1.00},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := proportionForSplit("my-skill", tc.split, n)
			if math.Abs(got-tc.want) > tol {
				t.Fatalf("split %s: proporcion A=%.4f, esperaba ~%.2f (tol %.2f)",
					tc.name, got, tc.want, tol)
			}
		})
	}
}

// --- fake repo para Router.Route ---

type fakeRouterRepo struct {
	running *ABTest
	getErr  error
}

func (f *fakeRouterRepo) GetRunningBySlug(_ ctxT, _ string) (*ABTest, error) {
	return f.running, f.getErr
}
func (f *fakeRouterRepo) IncrementResult(_ ctxT, _ uuid.UUID, _ Variant, _ bool) error {
	return nil
}

// TestRouteNoRunningTest: sin test running -> InABTest=false (pin normal).
func TestRouteNoRunningTest(t *testing.T) {
	r := NewRouter(&fakeRouterRepo{running: nil}, nil, nil)
	d := r.Route(bg(), "slug", uuid.New())
	if d.InABTest {
		t.Fatal("sin test running deberia caer al pin normal (InABTest=false)")
	}
}

// TestRouteDeterministicVariant: mismo user -> misma variante y version.
func TestRouteDeterministicVariant(t *testing.T) {
	test := &ABTest{
		ID:            uuid.New(),
		SkillSlug:     "slug",
		VersionA:      3,
		VersionB:      7,
		TrafficSplitA: 0.5,
		Status:        StatusRunning,
	}
	r := NewRouter(&fakeRouterRepo{running: test}, nil, nil)
	uid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	first := r.Route(bg(), "slug", uid)
	if !first.InABTest {
		t.Fatal("con test running deberia estar InABTest=true")
	}
	wantVersion := test.VersionFor(first.Variant)
	if first.Version != wantVersion {
		t.Fatalf("version=%d no corresponde a variante %s (esperaba %d)",
			first.Version, first.Variant, wantVersion)
	}
	for i := 0; i < 100; i++ {
		d := r.Route(bg(), "slug", uid)
		if d.Variant != first.Variant || d.Version != first.Version {
			t.Fatalf("ruteo no determinista: iter %d dio %s/%d, esperaba %s/%d",
				i, d.Variant, d.Version, first.Variant, first.Version)
		}
	}
}

// TestRouteDegradesOnError: si el lookup falla, no rompe -> pin normal.
func TestRouteDegradesOnError(t *testing.T) {
	r := NewRouter(&fakeRouterRepo{getErr: errFake}, nil, nil)
	d := r.Route(bg(), "slug", uuid.New())
	if d.InABTest {
		t.Fatal("error en lookup deberia degradar a pin normal")
	}
}
