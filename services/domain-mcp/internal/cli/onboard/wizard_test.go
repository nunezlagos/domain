package onboard

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)












func TestAsk_RequiredValue(t *testing.T) {
	w := &Wizard{In: strings.NewReader("admin@saargo.com\n"), Err: io.Discard}
	got, err := w.ask("Email", "", false)
	require.NoError(t, err)
	require.Equal(t, "admin@saargo.com", got)
}

func TestAsk_DefaultValue_EmptyReturnsDefault(t *testing.T) {

	w := &Wizard{In: strings.NewReader("\n"), Err: io.Discard}
	got, err := w.ask("Server URL", "http://localhost:8000", false)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8000", got)
}

func TestAsk_DefaultValue_NonEmptyOverridesDefault(t *testing.T) {
	w := &Wizard{In: strings.NewReader("http://staging:9000\n"), Err: io.Discard}
	got, err := w.ask("Server URL", "http://localhost:8000", false)
	require.NoError(t, err)
	require.Equal(t, "http://staging:9000", got)
}

func TestAsk_NonInteractive_NoDefault_Fails(t *testing.T) {
	w := &Wizard{In: strings.NewReader(""), Err: io.Discard}
	_, err := w.ask("Email", "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-interactive")
}

func TestAsk_NonInteractive_WithDefault(t *testing.T) {
	w := &Wizard{In: strings.NewReader(""), Err: io.Discard}
	got, err := w.ask("Email", "default@x.com", true)
	require.NoError(t, err)
	require.Equal(t, "default@x.com", got)
}

func TestAsk_TrimsWhitespace(t *testing.T) {
	w := &Wizard{In: strings.NewReader("  admin@saargo.com  \n"), Err: io.Discard}
	got, err := w.ask("Email", "", false)
	require.NoError(t, err)
	require.Equal(t, "admin@saargo.com", got)
}

func TestAsk_StdinClosed_Error(t *testing.T) {
	w := &Wizard{In: strings.NewReader(""), Err: io.Discard}
	_, err := w.ask("Email", "", false)
	require.Error(t, err)
	require.True(t, errors.Is(err, errors.New("stdin cerrado")) ||
		strings.Contains(err.Error(), "stdin cerrado"))
}



func TestAskYesNo_YesExplicit(t *testing.T) {
	w := &Wizard{In: strings.NewReader("y\n"), Err: io.Discard}
	got, err := w.askYesNo("Continue?", true)
	require.NoError(t, err)
	require.True(t, got)
}

func TestAskYesNo_YesFullWord(t *testing.T) {
	w := &Wizard{In: strings.NewReader("yes\n"), Err: io.Discard}
	got, err := w.askYesNo("Continue?", false)
	require.NoError(t, err)
	require.True(t, got)
}

func TestAskYesNo_NoExplicit(t *testing.T) {
	w := &Wizard{In: strings.NewReader("n\n"), Err: io.Discard}
	got, err := w.askYesNo("Continue?", true)
	require.NoError(t, err)
	require.False(t, got)
}

func TestAskYesNo_CaseInsensitive(t *testing.T) {
	cases := []string{"Y", "y", "YES", "Yes", "N", "n", "NO", "No"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			w := &Wizard{In: strings.NewReader(c + "\n"), Err: io.Discard}
			got, err := w.askYesNo("?", false)
			require.NoError(t, err)
			switch c[0] {
			case 'Y', 'y':
				require.True(t, got, "%q debe ser yes", c)
			case 'N', 'n':
				require.False(t, got, "%q debe ser no", c)
			}
		})
	}
}

func TestAskYesNo_DefaultYes_EmptyInput(t *testing.T) {
	w := &Wizard{In: strings.NewReader("\n"), Err: io.Discard}
	got, err := w.askYesNo("Continue?", true)
	require.NoError(t, err)
	require.True(t, got, "empty input con defaultYes=true → true")
}

func TestAskYesNo_DefaultNo_EmptyInput(t *testing.T) {
	w := &Wizard{In: strings.NewReader("\n"), Err: io.Discard}
	got, err := w.askYesNo("Continue?", false)
	require.NoError(t, err)
	require.False(t, got, "empty input con defaultYes=false → false")
}

func TestAskYesNo_NonInteractive(t *testing.T) {
	w := &Wizard{In: strings.NewReader(""), Err: io.Discard, NonInteractive: true}

	got, err := w.askYesNo("?", true)
	require.NoError(t, err)
	require.True(t, got)

	got, err = w.askYesNo("?", false)
	require.NoError(t, err)
	require.False(t, got)
}



func TestCredentials_JSONShape(t *testing.T) {

	c := &Credentials{
		APIKey:   "domk_live_test",
		APIKeyID: mustUUID("11111111-1111-1111-1111-111111111111"),
		UserID:   mustUUID("22222222-2222-2222-2222-222222222222"),
		OrgID:    mustUUID("33333333-3333-3333-3333-333333333333"),
		Email:    "admin@saargo.com",
		BaseURL:  "http://localhost:8000",
	}
	require.NotEmpty(t, c.APIKey)
	require.NotEmpty(t, c.Email)
}



func TestBoolLabel(t *testing.T) {
	require.Equal(t, "yes", boolLabel(true, "yes", "no"))
	require.Equal(t, "no", boolLabel(false, "yes", "no"))
}

func mustUUID(s string) (u uuid.UUID) {
	u, _ = uuid.Parse(s)
	return
}
