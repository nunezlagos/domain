package flowrunner

import (
	"context"

	"github.com/google/uuid"
)



func BeginStepRowForTest(ctx context.Context, r *Runner, runID uuid.UUID, stepKey string) uuid.UUID {
	return r.beginStepRow(ctx, runID, stepKey)
}

func CompleteStepRowForTest(ctx context.Context, r *Runner, rowID uuid.UUID, output any, stepErr error) error {
	return r.completeStepRow(ctx, rowID, output, stepErr)
}
