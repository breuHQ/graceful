package graceful_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.breu.io/graceful"
)

type MockSvc struct {
	name        string
	mu          sync.Mutex
	started     bool
	stopped     bool
	failStart   bool
	failStop    bool
	stopOrder   *[]string
	stopOrderMu *sync.Mutex
}

func NewMockSvc(name string, stopOrder *[]string, mu *sync.Mutex) *MockSvc {
	return &MockSvc{
		name:        name,
		stopOrder:   stopOrder,
		stopOrderMu: mu,
	}
}

func (m *MockSvc) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failStart {
		return errors.New("failed to start " + m.name)
	}
	m.started = true
	return nil
}

func (m *MockSvc) Stop(ctx context.Context) error {
	m.mu.Lock()
	if m.failStop {
		m.mu.Unlock()
		return errors.New("failed to stop " + m.name)
	}
	if !m.started {
		m.mu.Unlock()
		return errors.New(m.name + " was stopped before starting")
	}
	m.stopped = true
	m.mu.Unlock()

	if m.stopOrder != nil {
		m.stopOrderMu.Lock()
		*m.stopOrder = append(*m.stopOrder, m.name)
		m.stopOrderMu.Unlock()
	}
	return nil
}

func (m *MockSvc) FailOnStart() *MockSvc {
	m.failStart = true
	return m
}

func TestGraceful_Lifecycle(t *testing.T) {
	t.Run("Starts and Stops services in correct order", func(t *testing.T) {
		g := graceful.New()
		stopOrder := make([]string, 0)
		var stopOrderMu sync.Mutex

		svcA := NewMockSvc("A", &stopOrder, &stopOrderMu)
		svcB := NewMockSvc("B", &stopOrder, &stopOrderMu)
		svcC := NewMockSvc("C", &stopOrder, &stopOrderMu)

		// Dependency chain: C -> B -> A
		g.Add("A", svcA)
		g.Add("B", svcB, "A")
		g.Add("C", svcC, "B")

		ctx := context.Background()

		err := g.Start(ctx)
		assert.NoError(t, err)

		err = g.Stop(ctx)
		assert.NoError(t, err)

		// Assert the stop order was correct (reverse of start order).
		expectedStopOrder := []string{"C", "B", "A"}
		assert.Equal(t, expectedStopOrder, stopOrder)
	})
}

func TestGraceful_Failures(t *testing.T) {
	t.Run("Start fails if a service fails to start", func(t *testing.T) {
		g := graceful.New()
		svcA := NewMockSvc("A", nil, nil)
		svcB := NewMockSvc("B", nil, nil).FailOnStart()
		g.Add("A", svcA)
		g.Add("B", svcB, "A")

		ctx := context.Background()
		err := g.Start(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start B")
	})

	t.Run("Start fails when a dependency cycle is detected", func(t *testing.T) {
		g := graceful.New()
		svcA := NewMockSvc("A", nil, nil)
		svcB := NewMockSvc("B", nil, nil)
		g.Add("A", svcA, "B")
		g.Add("B", svcB, "A")

		ctx := context.Background()
		err := g.Start(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dependency cycle detected")
	})
}

type MonitorSvc struct {
	ContextCanceled bool
	Done            chan struct{}
}

func (s *MonitorSvc) Start(ctx context.Context) error {
	s.Done = make(chan struct{})
	go func() {
		<-ctx.Done()
		s.ContextCanceled = true
		close(s.Done)
	}()
	return nil
}

func (s *MonitorSvc) Stop(ctx context.Context) error {
	return nil
}

func TestGraceful_ContextSurvival(t *testing.T) {
	g := graceful.New()
	svc := &MonitorSvc{}
	g.Add("monitor", svc)

	ctx := context.Background()
	err := g.Start(ctx)
	assert.NoError(t, err)

	// Allow some time to see if the deferred cancel in Start (if it existed incorrectly) would trigger
	// We need to wait a bit because the cancellation propagation is asynchronous relative to this main thread
	// if it happens via a goroutine or defer cleanup.
	// In the fixed version, it shouldn't happen.
	select {
	case <-svc.Done:
		if svc.ContextCanceled {
			t.Fatal("Context passed to Start was canceled after Start returned")
		}
	case <-time.After(100 * time.Millisecond):
		// This is the expected path if context is NOT canceled
	}
}
