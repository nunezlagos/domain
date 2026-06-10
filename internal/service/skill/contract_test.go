package skill_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/skill"
)

func TestValidatePayload_NoSchema_AlwaysValid(t *testing.T) {
	r := skill.ValidatePayload(nil, []byte(`{"x":1}`))
	require.True(t, r.Valid)
	require.Empty(t, r.Errors)
}

func TestValidatePayload_RequiredMissing(t *testing.T) {
	schema := []byte(`{"required":["name","email"],"properties":{"name":{"type":"string"},"email":{"type":"string"}}}`)
	payload := []byte(`{"name":"alice"}`)
	r := skill.ValidatePayload(schema, payload)
	require.False(t, r.Valid)
	require.Len(t, r.Errors, 1)
	require.Equal(t, "email", r.Errors[0].Field)
}

func TestValidatePayload_TypeMismatch(t *testing.T) {
	schema := []byte(`{"properties":{"age":{"type":"integer"}}}`)
	payload := []byte(`{"age":"old"}`)
	r := skill.ValidatePayload(schema, payload)
	require.False(t, r.Valid)
	require.Equal(t, "age", r.Errors[0].Field)
}

func TestValidatePayload_EnumValid(t *testing.T) {
	schema := []byte(`{"properties":{"role":{"type":"string","enum":["admin","user"]}}}`)
	require.True(t, skill.ValidatePayload(schema, []byte(`{"role":"admin"}`)).Valid)
	require.False(t, skill.ValidatePayload(schema, []byte(`{"role":"hacker"}`)).Valid)
}

func TestValidatePayload_BadJSON(t *testing.T) {
	r := skill.ValidatePayload([]byte(`{"required":["x"]}`), []byte(`{not json}`))
	require.False(t, r.Valid)
	require.Equal(t, "$payload", r.Errors[0].Field)
}

// Sabotaje: schema corrupto debe reportar y NO pretender que el payload pasa.
func TestSabotage_CorruptSchemaInvalidates(t *testing.T) {
	r := skill.ValidatePayload([]byte(`{not json`), []byte(`{"x":1}`))
	require.False(t, r.Valid)
	require.Equal(t, "$schema", r.Errors[0].Field)
}
