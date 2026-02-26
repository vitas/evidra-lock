package auth

import (
	"context"
	"testing"
)

func TestWithTenantID_TenantID_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := WithTenantID(context.Background(), "tenant-abc")
	got := TenantID(ctx)
	if got != "tenant-abc" {
		t.Errorf("TenantID = %q, want %q", got, "tenant-abc")
	}
}

func TestTenantID_EmptyContext(t *testing.T) {
	t.Parallel()
	got := TenantID(context.Background())
	if got != "" {
		t.Errorf("TenantID on empty context = %q, want empty", got)
	}
}

func TestTenantID_Overwrite(t *testing.T) {
	t.Parallel()
	ctx := WithTenantID(context.Background(), "first")
	ctx = WithTenantID(ctx, "second")
	got := TenantID(ctx)
	if got != "second" {
		t.Errorf("TenantID = %q, want %q", got, "second")
	}
}
