package promptrouter

import (
	"context"
)

// WizardplanAdapter envuelve un promptrouter.Classifier para que satisfaga
// wizardplan.IntentClassifier (devuelve string en lugar del type Intent).
// Vive en este package para no introducir import cycle issuebuilder →
// wizardplan → promptrouter → issuebuilder.
//
// Wire desde el lado consumer (cmd/domain/main.go):
//
//	analyzer := &wizardplan.Analyzer{
//	  Classifier: &promptrouter.WizardplanAdapter{Inner: heuristic},
//	  Sources:    [...],
//	}
type WizardplanAdapter struct {
	Inner Classifier
}

// Classify implements wizardplan.IntentClassifier signature.
func (a *WizardplanAdapter) Classify(ctx context.Context, rawText string) (string, float64, string, error) {
	intent, conf, reason, err := a.Inner.Classify(ctx, rawText)
	return string(intent), conf, reason, err
}
