package observation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-145: la señal de consumo la reporta el CLIENTE al cerrar el turno.
// Estos tests cubren la validación del servicio, que es donde se decide qué
// llega a la BD y qué se descarta en silencio.

type usageRepoSpy struct {
	Repository
	existe      bool
	existeErr   error
	gotPrompt   uuid.UUID
	gotCandidat []uuid.UUID
	gotUsed     []uuid.UUID
	llamadas    int
}

func (s *usageRepoSpy) PromptExists(context.Context, uuid.UUID) (bool, error) {
	return s.existe, s.existeErr
}

func (s *usageRepoSpy) RecordUsage(_ context.Context, in RecordUsageParams) (int64, error) {
	s.llamadas++
	s.gotPrompt, s.gotCandidat, s.gotUsed = in.PromptID, in.CandidateIDs, in.UsedIDs
	return int64(len(in.CandidateIDs)), nil
}

func svcConSpy(spy *usageRepoSpy) *Service {
	return NewService(nil, nil, nil, nil, spy)
}

func TestService_RecordUsage_ReportaCandidatosYUsados_PersisteAmbos(t *testing.T) {
	spy := &usageRepoSpy{existe: true}
	prompt, a, b := uuid.New(), uuid.New(), uuid.New()

	rec, skipped, err := svcConSpy(spy).RecordUsage(context.Background(), prompt, []uuid.UUID{a, b}, []uuid.UUID{a})

	require.NoError(t, err)
	assert.Equal(t, 2, rec)
	assert.Zero(t, skipped)
	assert.ElementsMatch(t, []uuid.UUID{a, b}, spy.gotCandidat)
	assert.Equal(t, []uuid.UUID{a}, spy.gotUsed)
}

// "Ninguna sirvió" es una señal VÁLIDA y hay que registrarla: es la diferencia
// entre "se mostraron y no sirvieron" y "no se mostró nada". Sin este caso, el
// denominador queda sesgado hacia los turnos exitosos.
func TestService_RecordUsage_SinUsadas_IgualRegistraLosCandidatos(t *testing.T) {
	spy := &usageRepoSpy{existe: true}
	prompt, a := uuid.New(), uuid.New()

	rec, _, err := svcConSpy(spy).RecordUsage(context.Background(), prompt, []uuid.UUID{a}, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, rec)
	assert.Equal(t, 1, spy.llamadas, "un reporte vacío de usadas NO es un no-op")
	assert.Empty(t, spy.gotUsed)
}

// Un prompt_id desconocido es un caso esperable —el cliente reporta al final
// del turno y puede equivocarse—, no un error del sistema. Sin la
// pre-validación, la FK violation aborta la tx del handler y devuelve SQL crudo.
func TestService_RecordUsage_PromptDesconocido_NoIntentaInsertar(t *testing.T) {
	spy := &usageRepoSpy{existe: false}

	rec, skipped, err := svcConSpy(spy).RecordUsage(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, nil)

	require.NoError(t, err, "un prompt desconocido no es un error del sistema")
	assert.Zero(t, rec)
	assert.Equal(t, 1, skipped)
	assert.Zero(t, spy.llamadas, "no se toca la BD si el prompt no existe: la FK violation abortaría la tx")
}

// SABOTAJE del invariante que sostiene la métrica: used SIEMPRE ⊆ candidates.
// Si el cliente reporta como usada una observación que no declaró candidata,
// se agrega al denominador en vez de descartarse — si no, la tasa
// usados/candidatos podría superar 1 y la señal dejaría de ser interpretable.
func TestService_RecordUsage_UsadaQueNoEsCandidata_SeAgregaAlDenominador(t *testing.T) {
	spy := &usageRepoSpy{existe: true}
	prompt, candidata, huerfana := uuid.New(), uuid.New(), uuid.New()

	_, _, err := svcConSpy(spy).RecordUsage(context.Background(), prompt, []uuid.UUID{candidata}, []uuid.UUID{huerfana})

	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{candidata, huerfana}, spy.gotCandidat,
		"used debe ser subconjunto de candidates: una usada no declarada se suma al denominador")
	assert.LessOrEqual(t, len(spy.gotUsed), len(spy.gotCandidat),
		"la tasa usados/candidatos nunca puede superar 1")
}

func TestService_RecordUsage_IdsDuplicadosYNulos_SeDescartan(t *testing.T) {
	spy := &usageRepoSpy{existe: true}
	a := uuid.New()

	_, _, err := svcConSpy(spy).RecordUsage(context.Background(), uuid.New(),
		[]uuid.UUID{a, a, uuid.Nil}, []uuid.UUID{a, uuid.Nil})

	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{a}, spy.gotCandidat, "sin duplicados ni uuid.Nil")
	assert.Equal(t, []uuid.UUID{a}, spy.gotUsed)
}

func TestService_RecordUsage_PromptNil_NoTocaLaBD(t *testing.T) {
	spy := &usageRepoSpy{existe: true}

	_, _, err := svcConSpy(spy).RecordUsage(context.Background(), uuid.Nil, []uuid.UUID{uuid.New()}, nil)

	require.Error(t, err)
	assert.Zero(t, spy.llamadas)
}

// El cap acota el payload: el hook inyecta un puñado de memorias por turno, así
// que una lista enorme es señal de un cliente mal implementado, no de uso real.
func TestService_RecordUsage_MasIdsQueElCap_RecortaSinFallar(t *testing.T) {
	spy := &usageRepoSpy{existe: true}
	muchos := make([]uuid.UUID, maxUsageIDsPerTurn+15)
	for i := range muchos {
		muchos[i] = uuid.New()
	}

	_, skipped, err := svcConSpy(spy).RecordUsage(context.Background(), uuid.New(), muchos, nil)

	require.NoError(t, err, "recortar no es fallar: el reporte parcial sigue siendo útil")
	assert.Len(t, spy.gotCandidat, maxUsageIDsPerTurn)
	assert.Equal(t, 15, skipped)
}
