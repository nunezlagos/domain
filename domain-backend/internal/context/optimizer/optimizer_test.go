package optimizer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/context/optimizer"
)

func msg(id, role, content string, tokens int, at time.Time, pinned bool) optimizer.Message {
	return optimizer.Message{ID: id, Role: role, Content: content, Tokens: tokens, CreatedAt: at, Pinned: pinned}
}

func TestOptimize_NoEviction_WhenUnderBudget(t *testing.T) {
	base := time.Now()
	msgs := []optimizer.Message{
		msg("1", "user", "a", 10, base, false),
		msg("2", "assistant", "b", 10, base.Add(time.Minute), false),
	}
	r := optimizer.Optimize(msgs, optimizer.Config{MaxTokens: 100, KeepLast: 1})
	require.Len(t, r.Kept, 2)
	require.Equal(t, 0, r.Evicted)
}

func TestOptimize_EvictsOldestFirst(t *testing.T) {
	base := time.Now()
	msgs := []optimizer.Message{
		msg("1", "user", "oldest", 30, base, false),
		msg("2", "user", "middle", 30, base.Add(time.Minute), false),
		msg("3", "user", "newest", 30, base.Add(2*time.Minute), false),
	}
	r := optimizer.Optimize(msgs, optimizer.Config{MaxTokens: 60, KeepLast: 1})
	require.Equal(t, 1, r.Evicted)
	require.Equal(t, "2", r.Kept[0].ID)
	require.Equal(t, "3", r.Kept[1].ID)
}

func TestOptimize_PreservesPinned(t *testing.T) {
	base := time.Now()
	msgs := []optimizer.Message{
		msg("0", "system", "pinned instruction", 50, base, true),
		msg("1", "user", "older convo", 30, base.Add(time.Minute), false),
		msg("2", "user", "recent", 30, base.Add(2*time.Minute), false),
	}
	r := optimizer.Optimize(msgs, optimizer.Config{MaxTokens: 80, KeepLast: 1})
	// pinned + last1 quedan; el msg "1" (older) se va
	ids := []string{}
	for _, m := range r.Kept {
		ids = append(ids, m.ID)
	}
	require.Contains(t, ids, "0")
	require.Contains(t, ids, "2")
}

func TestOptimize_WithSummary(t *testing.T) {
	base := time.Now()
	msgs := []optimizer.Message{
		msg("1", "user", "first message that gets evicted", 50, base, false),
		msg("2", "user", "second also evicted", 50, base.Add(time.Minute), false),
		msg("3", "user", "kept", 10, base.Add(2*time.Minute), false),
	}
	r := optimizer.Optimize(msgs, optimizer.Config{
		MaxTokens: 30,
		KeepLast:  1,
		SummaryFn: optimizer.SimpleSummary,
	})
	require.NotEmpty(t, r.Summary)
	require.Equal(t, "system", r.Kept[0].Role)
	require.Contains(t, r.Kept[0].Content, "Resumen")
}

func TestOptimize_NoBudget_NoEviction(t *testing.T) {
	base := time.Now()
	msgs := []optimizer.Message{msg("1", "user", "x", 10000, base, false)}
	r := optimizer.Optimize(msgs, optimizer.Config{MaxTokens: 0})
	require.Equal(t, 0, r.Evicted)
}

// Sabotaje: KeepLast > total messages no debe crashear.
func TestSabotage_KeepLastTooLarge(t *testing.T) {
	base := time.Now()
	msgs := []optimizer.Message{msg("1", "user", "x", 10, base, false)}
	r := optimizer.Optimize(msgs, optimizer.Config{MaxTokens: 1, KeepLast: 10})
	require.Len(t, r.Kept, 1)
}
