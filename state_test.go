package lifecycle

import (
	"context"
	"testing"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateCreated, "Created"},
		{StateRunning, "Running"},
		{StateStopping, "Stopping"},
		{StateStopped, "Stopped"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%s).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestState_Transitions(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))

	if m.State() != StateCreated {
		t.Fatalf("initial state = %s, want Created", m.State())
	}

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if m.State() != StateRunning {
		t.Fatalf("after Start state = %s, want Running", m.State())
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.State() != StateStopped {
		t.Fatalf("after Stop state = %s, want Stopped", m.State())
	}
}

func TestState_InvalidTransitions(t *testing.T) {
	ctx := context.Background()

	t.Run("cannot start twice", func(t *testing.T) {
		m := NewManager(WithLogger(NopLogger()))
		if err := m.Start(ctx); err != nil {
			t.Fatal(err)
		}
		if err := m.Start(ctx); err == nil {
			t.Fatal("expected error on second Start, got nil")
		}
		_ = m.Stop(ctx)
	})

	t.Run("cannot start after stop", func(t *testing.T) {
		m := NewManager(WithLogger(NopLogger()))
		_ = m.Start(ctx)
		_ = m.Stop(ctx)
		if err := m.Start(ctx); err == nil {
			t.Fatal("expected error starting after stop, got nil")
		}
	})

	t.Run("cannot stop before start", func(t *testing.T) {
		m := NewManager(WithLogger(NopLogger()))
		if err := m.Stop(ctx); err == nil {
			t.Fatal("expected error stopping before start, got nil")
		}
	})

	t.Run("cannot add task after start", func(t *testing.T) {
		m := NewManager(WithLogger(NopLogger()))
		_ = m.Start(ctx)
		if err := m.AddTask(NewFuncTask("late", nil, nil)); err == nil {
			t.Fatal("expected error adding task after start, got nil")
		}
		_ = m.Stop(ctx)
	})
}
