package ses_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"nunezlagos/domain/internal/mail/ses"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/stretchr/testify/require"
)

type mockSESClient struct {
	sendEmailFn func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

func (m *mockSESClient) SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	return m.sendEmailFn(ctx, params, optFns...)
}

func TestSES_SendOK(t *testing.T) {
	client := &mockSESClient{
		sendEmailFn: func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			require.NotNil(t, params)
			require.Equal(t, "noreply@test.com", *params.FromEmailAddress)
			require.Equal(t, "to@test.com", params.Destination.ToAddresses[0])
			require.Equal(t, "Subject", *params.Content.Simple.Subject.Data)
			require.Equal(t, "Body", *params.Content.Simple.Body.Text.Data)
			return &sesv2.SendEmailOutput{}, nil
		},
	}

	s := ses.NewWithClient(client, "noreply@test.com", slog.Default())
	err := s.Send(context.Background(), "to@test.com", "Subject", "Body")
	require.NoError(t, err)
}

func TestSES_SendFails(t *testing.T) {
	client := &mockSESClient{
		sendEmailFn: func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			return nil, errors.New("mock error")
		},
	}

	s := ses.NewWithClient(client, "noreply@test.com", slog.Default())
	err := s.Send(context.Background(), "to@test.com", "S", "B")
	require.Error(t, err)
	require.Contains(t, err.Error(), "mock error")
}

func TestSES_SendOTP(t *testing.T) {
	var capturedTo string
	client := &mockSESClient{
		sendEmailFn: func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			capturedTo = params.Destination.ToAddresses[0]
			return &sesv2.SendEmailOutput{}, nil
		},
	}

	s := ses.NewWithClient(client, "noreply@test.com", slog.Default())
	err := s.SendOTP(context.Background(), "user@test.com", "654321", 0)
	require.NoError(t, err)
	require.Equal(t, "user@test.com", capturedTo)
}
