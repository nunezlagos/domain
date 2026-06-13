package mail

import (
	"context"
	"time"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
	SendOTP(ctx context.Context, to, code string, expiresIn time.Duration) error
}
