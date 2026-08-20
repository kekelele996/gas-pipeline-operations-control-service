package platform

import (
	"errors"
	"testing"
)

func TestErrorIsCategory(t *testing.T) {
	e := NotFoundf("segment %s missing", "X")
	if !errors.Is(e, ErrNotFound) {
		t.Fatalf("expected ErrNotFound match")
	}
	if errors.Is(e, ErrConflict) {
		t.Fatalf("must not match ErrConflict")
	}
	if e.Error() == "" {
		t.Fatalf("message empty")
	}
}

func TestWrapChainsUnwrap(t *testing.T) {
	inner := errors.New("io")
	e := Wrap(ErrInvalid, "bad body", inner)
	if !errors.Is(e, ErrInvalid) {
		t.Fatalf("expected ErrInvalid match")
	}
	if !errors.Is(e, inner) {
		t.Fatalf("expected wrapped error match")
	}
}

func TestStateMachineTransitions(t *testing.T) {
	m := NewStateMachine("test", "a", map[string][]string{
		"a": {"b"},
		"b": {"c"},
	})
	got, err := m.Transition("a", "b")
	if err != nil || got != "b" {
		t.Fatalf("a->b: %v %v", got, err)
	}
	if _, err := m.Transition("a", "c"); err == nil {
		t.Fatalf("a->c should fail")
	}
	if m.Allowed("b")[0] != "c" {
		t.Fatalf("allowed(b) wrong: %v", m.Allowed("b"))
	}
}

func TestValidateAccumulates(t *testing.T) {
	v := NewValidate().
		RequireNonEmpty("name", "").
		RequirePositive("v", -1).
		RequireEnum("s", "x", []string{"a", "b"})
	if !v.Has() {
		t.Fatalf("should have errors")
	}
	if err := v.Error(); err == nil {
		t.Fatalf("error nil")
	}
}
