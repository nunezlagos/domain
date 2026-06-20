package llm

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	name string
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Complete(ctx context.Context, opts CompletionOptions) (*Response, error) {
	return &Response{Content: "stub", Model: opts.Model}, nil
}
func (s *stubProvider) CompleteStream(ctx context.Context, opts CompletionOptions) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Delta: "stub", Done: true}
	close(ch)
	return ch, nil
}

func TestFactory_RegisterAndGet(t *testing.T) {
	f := NewFactory()
	f.Register("openai", &stubProvider{name: "openai"})
	f.Register("anthropic", &stubProvider{name: "anthropic"})

	p, err := f.Get("openai")
	require.NoError(t, err)
	require.Equal(t, "openai", p.Name())
}

func TestFactory_GetUnknownProvider(t *testing.T) {
	f := NewFactory()
	_, err := f.Get("unknown")
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider not found")
}

func TestFactory_GetDefault(t *testing.T) {
	f := NewFactory()
	_, err := f.GetDefault()
	require.Error(t, err, "sin default seteado debe errar")

	f.Register("openai", &stubProvider{name: "openai"})
	f.SetDefault("openai", "")
	p, err := f.GetDefault()
	require.NoError(t, err)
	require.Equal(t, "openai", p.Name())
}

func TestFactory_EmbedderRegistry(t *testing.T) {
	f := NewFactory()
	f.RegisterEmbedder("fake", FakeEmbedder{})
	f.SetDefault("", "fake")
	e, err := f.GetDefaultEmbedder()
	require.NoError(t, err)
	require.Equal(t, 1536, e.Dimensions())
}

func TestFactory_List(t *testing.T) {
	f := NewFactory()
	f.Register("a", &stubProvider{name: "a"})
	f.Register("b", &stubProvider{name: "b"})
	names := f.List()
	require.ElementsMatch(t, []string{"a", "b"}, names)
}

func TestFactory_ThreadSafe(t *testing.T) {
	f := NewFactory()
	f.Register("openai", &stubProvider{name: "openai"})

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			p, err := f.Get("openai")
			if err != nil {
				errs <- err
				return
			}
			if p.Name() != "openai" {
				errs <- nil
				return
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
}

// Test que el Complete del stub funciona end-to-end.
func TestStubProvider_Complete(t *testing.T) {
	p := &stubProvider{name: "stub"}
	resp, err := p.Complete(context.Background(), CompletionOptions{Model: "test"})
	require.NoError(t, err)
	require.Equal(t, "stub", resp.Content)
	require.Equal(t, "test", resp.Model)
}
