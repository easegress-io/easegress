package contexttool

import (
	"context"
	"time"
)

// TimeoutContext wraps standard timeout context by calling cancel function automatically.
func TimeoutContext(timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	go func() {
		time.Sleep(timeout)
		cancel()
	}()
	return ctx
}
