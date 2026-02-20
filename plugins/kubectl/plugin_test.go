package kubectl

import (
	"testing"

	"samebits.com/evidra-mcp/pkg/registry"
)

func TestRegisterSucceeds(t *testing.T) {
	r := registry.NewDefaultRegistry()
	p := New()
	if err := p.Register(r); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	def, ok := r.Lookup("kubectl")
	if !ok {
		t.Fatalf("expected kubectl tool to be registered")
	}
	if len(def.SupportedOperations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(def.SupportedOperations))
	}
}

func TestDuplicateRegistrationRejected(t *testing.T) {
	r := registry.NewDefaultRegistry()
	p := New()
	if err := p.Register(r); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := p.Register(r); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}
