package wizardplan

import (
	"context"
	"sync"
	"time"
)

// IntentClassifier abstrae el classifier de intent. Definida acá (no en
// promptrouter) para evitar import cycle hubuilder → wizardplan →
// promptrouter → hubuilder. promptrouter.Classifier satisface esta
// interface trivialmente con un adapter (ver IntentClassifierAdapter).
type IntentClassifier interface {
	Classify(ctx context.Context, rawText string) (intent string, confidence float64, reasoning string, err error)
}

// Analyzer ejecuta sources en paralelo y produce ContextEnvelope.
type Analyzer struct {
	Sources    []Source
	Classifier IntentClassifier // intent classifier (LLM o heurístico)
	// Timeout máximo TOTAL del pipeline. Si una source no termina, se ignora.
	Timeout time.Duration
}

// Analyze corre el pipeline:
//   1. Classifier para obtener intent (sequential, rápido)
//   2. Si intent es chat/idea, NO corre las 4 sources (no es SDD).
//   3. Si intent es work-related, corre las 4 sources en paralelo con timeout.
//   4. Devuelve envelope con todos los findings + slots inferidos.
func (a *Analyzer) Analyze(ctx context.Context, rawPrompt string) (*ContextEnvelope, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	// 1) Classify intent.
	intent := "feature" // default safe
	conf := 0.5
	reasoning := ""
	if a.Classifier != nil {
		i, c, r, err := a.Classifier.Classify(ctx, rawPrompt)
		if err == nil {
			intent = i
			conf = c
			reasoning = r
		}
	}

	env := NewEnvelope(rawPrompt, intent)
	env.Intent = &IntentFinding{Intent: intent, Confidence: conf, Reasoning: reasoning}
	env.Touch(SlotIntent, intent, "intent_classifier", conf, reasoning)
	env.SourceErrors = map[string]string{}

	// 2) Si chat/idea, NO corremos las 4 sources.
	if intent == "chat" || intent == "idea" {
		return env, nil
	}

	// 3) Corre sources en paralelo con timeout total.
	pipelineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, src := range a.Sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					env.SourceErrors[s.Name()] = "panic recovered"
					mu.Unlock()
				}
			}()
			if err := s.Run(pipelineCtx, env); err != nil {
				mu.Lock()
				env.SourceErrors[s.Name()] = err.Error()
				mu.Unlock()
			}
		}(src)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-pipelineCtx.Done():
		// Timeout: las sources que no terminaron quedan con su contribución parcial.
	}

	return env, nil
}
