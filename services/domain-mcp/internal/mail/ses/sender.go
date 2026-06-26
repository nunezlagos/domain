package ses

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type SESAPI interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

type Sender struct {
	Client SESAPI
	From   string
	Logger *slog.Logger
}

func New(region, from string, logger *slog.Logger) (*Sender, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("ses config: %w", err)
	}
	return &Sender{
		Client: sesv2.NewFromConfig(cfg),
		From:   from,
		Logger: logger,
	}, nil
}

func NewWithClient(client SESAPI, from string, logger *slog.Logger) *Sender {
	return &Sender{Client: client, From: from, Logger: logger}
}

func (s *Sender) Send(ctx context.Context, to, subject, body string) error {
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.From),
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(subject),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data: aws.String(body),
					},
				},
			},
		},
	}

	_, err := s.Client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("ses send: %w", err)
	}

	s.Logger.Info("email sent",
		slog.String("provider", "ses"),
		slog.String("to", to),
		slog.String("subject", subject),
	)
	return nil
}
