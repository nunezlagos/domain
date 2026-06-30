package modes

import "testing"

func TestProviderForModel(t *testing.T) {
	cases := map[string]string{
		"claude-sonnet-4-6": "anthropic",
		"gpt-4o":            "openai",
		"openai-o1":         "openai",
		"gemini-1.5-pro":    "google",
		"google-foo":        "google",
		"MiniMax-M3":        "minimax",
		"minimax-m3":        "minimax",
		"llama3.3:70b":      "ollama",
		"":                  "ollama",
	}
	for model, want := range cases {
		if got := ProviderForModel(model); got != want {
			t.Errorf("ProviderForModel(%q) = %q, want %q", model, got, want)
		}
	}
}
