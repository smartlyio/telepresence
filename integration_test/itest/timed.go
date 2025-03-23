package itest

import (
	"context"
	"time"
)

// TimedRun gives the given function maxDuration time to finish, and returns a timeout error if it doesn't. Otherwise,
// it returns the error from the function.
func TimedRun(ctx context.Context, maxDuration time.Duration, function func(ctx2 context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return function(ctx)
	}
}
