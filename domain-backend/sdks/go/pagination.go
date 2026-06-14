package domain

import "context"

// PageFetcher es la firma de una función que pide UNA página dado un cursor.
// Devuelve la slice de items, el next cursor (vacío si no hay más) y error.
type PageFetcher[T any] func(ctx context.Context, cursor string) ([]T, string, error)

// Iterator itera todos los items de un listing paginado, transparentemente
// pidiendo páginas adicionales cuando el buffer local se vacía.
//
// Uso típico:
//
//	it := client.Observations.Iter(ctx, params)
//	for {
//	    obs, ok, err := it.Next(ctx)
//	    if err != nil { return err }
//	    if !ok { break }
//	    use(obs)
//	}
type Iterator[T any] struct {
	fetch  PageFetcher[T]
	buf    []T
	cursor string
	done   bool
	err    error
}

// NewIterator construye un iterator. Normalmente las resources exponen
// helpers .Iter() en lugar de instanciarlo directo.
func NewIterator[T any](fetch PageFetcher[T]) *Iterator[T] {
	return &Iterator[T]{fetch: fetch}
}

// Next devuelve el próximo item. El segundo return es false cuando se
// agotaron los items (fin normal) o cuando hubo error (chequear .Err()).
func (it *Iterator[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T
	if it.err != nil {
		return zero, false, it.err
	}
	if len(it.buf) == 0 && !it.done {
		items, next, err := it.fetch(ctx, it.cursor)
		if err != nil {
			it.err = err
			return zero, false, err
		}
		it.buf = items
		it.cursor = next
		if next == "" {
			it.done = true
		}
	}
	if len(it.buf) == 0 {
		return zero, false, nil
	}
	item := it.buf[0]
	it.buf = it.buf[1:]
	return item, true, nil
}

// Err devuelve el último error del iterator (si alguno).
func (it *Iterator[T]) Err() error { return it.err }
