package dispatch

import (
	"context"
	"time"
)

// deviceContext derives the context used for a device operation. It keeps the
// request cancellation semantics and bounds the operation with a hard timeout
// so a stalled device can never block the caller forever.
func deviceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 30*time.Second)
}
