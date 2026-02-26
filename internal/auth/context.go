package auth

import "context"

type contextKey struct{}

// WithTenantID returns a new context with the tenant ID set.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, tenantID)
}

// TenantID extracts the tenant ID from the context.
// Returns empty string if not set.
func TenantID(ctx context.Context) string {
	v, _ := ctx.Value(contextKey{}).(string)
	return v
}
