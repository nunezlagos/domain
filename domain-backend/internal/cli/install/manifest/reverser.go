package manifest

type Reverser interface {
	CanRevert(entry Entry) bool
	Revert(entry Entry) error
}

type ReverserRegistry struct {
	reversers map[string]Reverser
}

func NewReverserRegistry() *ReverserRegistry {
	return &ReverserRegistry{
		reversers: make(map[string]Reverser),
	}
}

func (r *ReverserRegistry) Register(entryType string, rev Reverser) {
	r.reversers[entryType] = rev
}

func (r *ReverserRegistry) Get(entryType string) (Reverser, bool) {
	rev, ok := r.reversers[entryType]
	return rev, ok
}
