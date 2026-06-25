package steptypes

import (
	"context"
	"fmt"
	"time"
)

// WaitRunner pauses execution for a duration or until a condition is met.
//
// Config (duration):
//
//	{"duration_seconds": 30}
//
// Config (condition polling):
//
//	{
//	  "until_condition": "steps.s1.result.ready == true",
//	  "poll_interval_seconds": 5,
//	  "timeout_seconds": 300
//	}
//
// Output: {"waited_seconds": <n>, "trigger": "duration"|"condition"|"timeout"}.
type WaitRunner struct{}

func (r *WaitRunner) Run(ctx context.Context, input RunInput) (any, error) {
	durationSec := configInt(input.Config, "duration_seconds")
	untilCondition := configString(input.Config, "until_condition")

	if durationSec > 0 && untilCondition != "" {
		return nil, fmt.Errorf("wait: use either duration_seconds OR until_condition, not both")
	}
	if durationSec <= 0 && untilCondition == "" {
		return nil, fmt.Errorf("wait: duration_seconds or until_condition required")
	}

	start := time.Now()

	if durationSec > 0 {
		return r.waitDuration(ctx, durationSec, start)
	}

	return r.waitCondition(ctx, untilCondition, input, start)
}

func (r *WaitRunner) waitDuration(ctx context.Context, durationSec int, start time.Time) (any, error) {
	dur := time.Duration(durationSec) * time.Second

	select {
	case <-ctx.Done():
		return map[string]any{
			"waited_seconds": int(time.Since(start).Seconds()),
			"trigger":        "cancelled",
		}, ctx.Err()
	case <-time.After(dur):
		return map[string]any{
			"waited_seconds": durationSec,
			"trigger":        "duration",
		}, nil
	}
}

func (r *WaitRunner) waitCondition(ctx context.Context, condition string, input RunInput, start time.Time) (any, error) {
	pollInterval := configInt(input.Config, "poll_interval_seconds")
	if pollInterval <= 0 {
		pollInterval = 5
	}
	timeoutSec := configInt(input.Config, "timeout_seconds")
	if timeoutSec <= 0 {
		timeoutSec = 300 // default 5 minutes
	}

	timeout := time.After(time.Duration(timeoutSec) * time.Second)
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	for {

		resolved := ResolveTemplate(condition, input.Inputs, input.StepOutputs)
		result, err := evalBool(resolved)
		if err == nil && result {
			return map[string]any{
				"waited_seconds": int(time.Since(start).Seconds()),
				"trigger":        "condition",
			}, nil
		}

		select {
		case <-ctx.Done():
			return map[string]any{
				"waited_seconds": int(time.Since(start).Seconds()),
				"trigger":        "cancelled",
			}, ctx.Err()
		case <-timeout:
			return map[string]any{
				"waited_seconds": int(time.Since(start).Seconds()),
				"trigger":        "timeout",
			}, fmt.Errorf("wait: condition not met within %d seconds: %q", timeoutSec, condition)
		case <-ticker.C:

		}
	}
}
