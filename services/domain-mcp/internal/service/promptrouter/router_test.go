package promptrouter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/promptrouter"
)

func TestHeuristicClassify_DetectsFix(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, conf, _, _ := c.Classify(context.Background(),
		"El director no puede descargar la ficha, no funciona el botón Export")
	require.Equal(t, promptrouter.IntentFix, intent)
	require.Greater(t, conf, 0.5)
}

func TestHeuristicClassify_DetectsHotfix(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, conf, _, _ := c.Classify(context.Background(),
		"URGENTE: producción caída, todos los logins fallan, esto es critical bug")
	require.Equal(t, promptrouter.IntentHotfix, intent)
	require.Greater(t, conf, 0.8)
}

func TestHeuristicClassify_DetectsFeature(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(),
		"Quiero implementar export a CSV en la pantalla de runs")
	require.Equal(t, promptrouter.IntentFeature, intent)
}

func TestHeuristicClassify_DetectsRefactor(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(),
		"Necesito refactor del módulo de auth para extract los handlers")
	require.Equal(t, promptrouter.IntentRefactor, intent)
}

func TestHeuristicClassify_DetectsRFC(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(),
		"RFC: diseño arquitectura del nuevo sistema de cache multi-tier")
	require.Equal(t, promptrouter.IntentRFC, intent)
}

func TestHeuristicClassify_DetectsDoc(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(),
		"Hay que actualizar la documentación del README con los nuevos endpoints")
	require.Equal(t, promptrouter.IntentDoc, intent)
}

func TestHeuristicClassify_DetectsChat(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(),
		"Cómo se configuran las migrations de postgres?")
	require.Equal(t, promptrouter.IntentChat, intent)
}

func TestHeuristicClassify_DetectsIdea(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(),
		"Se me ocurre una idea: y si agregamos un modo TUI offline")
	require.Equal(t, promptrouter.IntentIdea, intent)
}

func TestHeuristicClassify_DefaultsToChat(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, conf, _, _ := c.Classify(context.Background(),
		"Cualquier texto sin keywords reconocibles")
	require.Equal(t, promptrouter.IntentChat, intent)
	require.Less(t, conf, 0.5)
}

// Sabotaje: prompt vacío rechaza vía ErrEmptyPrompt al routing.
// Como Route necesita services reales, lo testeamos via la guardia del Router
// con un test integration separado. Acá solo verificamos el HeuristicClassifier.
func TestSabotage_EmptyPromptClassifierDefaults(t *testing.T) {
	c := promptrouter.HeuristicClassifier{}
	intent, _, _, _ := c.Classify(context.Background(), "")

	require.Equal(t, promptrouter.IntentChat, intent)
}
