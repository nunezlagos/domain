//go:build integration

package issuebuilder_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	hb "nunezlagos/domain/internal/service/issuebuilder"
)

// mockAttSvc fake que evita pegar al S3 real durante tests del wizard.
type mockAttSvc struct {
	uploadsCalled int
	promotesCalled int
	lastFromKind, lastToKind string
}

func (m *mockAttSvc) InitUpload(_ context.Context, entityType, entityIDStr, filename, mimeType, createdBy string, size int64) (*hb.AttachmentInitResult, error) {
	m.uploadsCalled++
	return &hb.AttachmentInitResult{
		AttachmentID: uuid.New(),
		UploadURL:    "https://s3.mock/" + entityType + "/" + entityIDStr + "/" + filename,
		Filename:     filename,
	}, nil
}

func (m *mockAttSvc) PromoteEntity(_ context.Context, fromKind, toKind string, _, _ uuid.UUID) (int, error) {
	m.promotesCalled++
	m.lastFromKind = fromKind
	m.lastToKind = toKind
	return 2, nil
}

func TestAttachToDraft_InitsUploadAndPersistsRef(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	mock := &mockAttSvc{}
	svc.Attachments = mock
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeBugFix, "El director no puede descargar la ficha", nil)
	require.NoError(t, err)

	res, err := svc.AttachToDraft(ctx, d.ID, "screenshot.png", "image/png", 102400)
	require.NoError(t, err)
	require.Equal(t, 1, mock.uploadsCalled)
	require.NotEmpty(t, res.UploadURL)
	require.Equal(t, "screenshot.png", res.Filename)

	// Verifica que el attachment_id se persistió en answers.
	got, err := svc.Get(ctx, d.ID)
	require.NoError(t, err)
	require.Contains(t, string(got.Answers), "attachments")
	require.Contains(t, string(got.Answers), "screenshot.png")
}

func TestAttachToDraft_MultipleAttachmentsAccumulate(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	svc.Attachments = &mockAttSvc{}
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeBugFix, "Bug con multiple screenshots", nil)
	require.NoError(t, err)

	_, err = svc.AttachToDraft(ctx, d.ID, "before.png", "image/png", 50_000)
	require.NoError(t, err)
	_, err = svc.AttachToDraft(ctx, d.ID, "after.png", "image/png", 60_000)
	require.NoError(t, err)
	_, err = svc.AttachToDraft(ctx, d.ID, "log.txt", "text/plain", 3_000)
	require.NoError(t, err)

	got, _ := svc.Get(ctx, d.ID)
	require.Contains(t, string(got.Answers), "before.png")
	require.Contains(t, string(got.Answers), "after.png")
	require.Contains(t, string(got.Answers), "log.txt")
}

func TestAttachToDraft_RejectsAbandonedDraft(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	svc.Attachments = &mockAttSvc{}
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "idea", nil)
	require.NoError(t, err)
	require.NoError(t, svc.Abandon(ctx, d.ID, "changed mind"))

	_, err = svc.AttachToDraft(ctx, d.ID, "x.png", "image/png", 1024)
	require.ErrorIs(t, err, hb.ErrInvalidStatus)
}

func TestAttachToDraft_NotConfigured(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	svc.Attachments = nil
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "idea", nil)
	require.NoError(t, err)

	_, err = svc.AttachToDraft(ctx, d.ID, "x.png", "image/png", 1024)
	require.ErrorIs(t, err, hb.ErrAttachmentsNotConfigured)
}

func TestPromoteAttachmentsToHU_AfterCommit(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	mock := &mockAttSvc{}
	svc.Attachments = mock
	ctx := context.Background()

	d, _, err := svc.Start(ctx, hb.ModeFeature, "feature con screenshot", nil)
	require.NoError(t, err)

	// Answer todos los 8 steps + attach
	answers := []any{"feature", "dx-engineer", "REQ-04-opsx-sdd",
		"M", "alta", "test-slug", "goal", "summary"}
	for _, a := range answers {
		_, _, _ = svc.Answer(ctx, d.ID, a)
	}
	_, err = svc.AttachToDraft(ctx, d.ID, "mockup.png", "image/png", 8000)
	require.NoError(t, err)

	committed, err := svc.Commit(ctx, d.ID)
	require.NoError(t, err)
	require.Equal(t, hb.StatusCommitted, committed.Status)

	// El agente IA creó el user_story; promueve attachments.
	issueID := uuid.New()
	moved, err := svc.PromoteAttachmentsToHU(ctx, d.ID, issueID)
	require.NoError(t, err)
	require.Equal(t, 2, moved)
	require.Equal(t, "hu_draft", mock.lastFromKind)
	require.Equal(t, "user_story", mock.lastToKind)
}

// Sabotaje: PromoteAttachmentsToHU sobre draft no committed debe rechazar.
func TestSabotage_PromoteBeforeCommit(t *testing.T) {
	svc, cleanup := setupHB(t)
	defer cleanup()
	svc.Attachments = &mockAttSvc{}
	ctx := context.Background()

	d, _, _ := svc.Start(ctx, hb.ModeFeature, "idea", nil)
	_, err := svc.PromoteAttachmentsToHU(ctx, d.ID, uuid.New())
	require.ErrorIs(t, err, hb.ErrInvalidStatus)
}
