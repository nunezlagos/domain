package session

import "context"

type ctxKey struct{}

// ToContext mete el Active en el ctx — usado por el middleware tras
// resolver un session token. El handler /me y los endpoints que
// dependen del rol activo lo recuperan con FromContext.
func ToContext(ctx context.Context, a *Active) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

func FromContext(ctx context.Context) (*Active, bool) {
	v, ok := ctx.Value(ctxKey{}).(*Active)
	return v, ok
}
