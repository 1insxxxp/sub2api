package service

import "context"

type gatewayPublicModelContextKey struct{}

// WithGatewayPublicModel records the caller-visible model separately from the
// model rewritten for routing. Gateway forwarders use it only for downstream
// response and usage identity; account mapping continues from the routed body.
func WithGatewayPublicModel(ctx context.Context, model string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if model == "" {
		return ctx
	}
	return context.WithValue(ctx, gatewayPublicModelContextKey{}, model)
}

func gatewayPublicModel(ctx context.Context, fallback string) string {
	if ctx != nil {
		if model, ok := ctx.Value(gatewayPublicModelContextKey{}).(string); ok && model != "" {
			return model
		}
	}
	return fallback
}
