package skill_ab_test

import (
	"context"
	"errors"
)

// ctxT alias para acortar firmas de los fakes en los tests.
type ctxT = context.Context

// bg devuelve un context.Background() (azucar para los tests).
func bg() context.Context { return context.Background() }

// errFake error generico para simular fallos del repo.
var errFake = errors.New("fake repo error")
