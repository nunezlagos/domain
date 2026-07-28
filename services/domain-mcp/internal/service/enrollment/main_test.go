package enrollment

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// sin build tag a propósito: aplica a la suite unitaria y a la de integración
func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}

// atrapa que alguien degrade el cost de producción confundiéndolo con el de tests
func TestBcryptCost_Produccion_NoSeDegrada(t *testing.T) {
	if BcryptCost < 12 {
		t.Fatalf("BcryptCost de producción bajó a %d; el mínimo aceptable es 12", BcryptCost)
	}
	if bcryptCost != bcrypt.MinCost {
		t.Fatalf("bcryptCost en tests es %d; TestMain debería haberlo puesto en MinCost (%d)",
			bcryptCost, bcrypt.MinCost)
	}
}
