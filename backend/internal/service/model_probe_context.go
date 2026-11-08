package service

import "context"

type modelProbeExecutionContextKey struct{}

// WithModelProbeExecution marks internal, non-billable health probes. It cannot
// be selected by public request headers or payloads. A stopped probe does not
// need the post-disconnect usage drain used for billable customer requests.
func WithModelProbeExecution(ctx context.Context) context.Context {
	return context.WithValue(ctx, modelProbeExecutionContextKey{}, true)
}
