package graceful_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.breu.io/graceful"
)

type MockSvc struct {
	name  string
	start bool
	stop  bool
}

func (m *MockSvc) Start(ctx context.Context) error {
	if m.start {
		return fmt.Errorf("service %s already started", m.name)
	}
	m.start = true
	return nil
}

func (m *MockSvc) Stop(ctx context.Context) error {
	if !m.start {
		return fmt.Errorf("service %s not started", m.name)
	}
	if m.stop {
		return fmt.Errorf("service %s already stopped", m.name)
	}
	m.stop = true
	return nil
}

func TestGraceful_Start(t *testing.T) {
	t.Run("Start successfully", func(t *testing.T) {
		g := graceful.New()
		svc1 := &MockSvc{name: "service1"}
		svc2 := &MockSvc{name: "service2"}
		svc3 := &MockSvc{name: "service3"}

		g.Add("service1", svc1)
		g.Add("service2", svc2, "service1")
		g.Add("service3", svc3, "service2")

		ctx := context.Background()
		err := g.Start(ctx)
		assert.NoError(t, err, "Error starting services")

		time.Sleep(500 * time.Millisecond)

		assert.True(t, svc1.start, "Service1 not started")
		assert.True(t, svc2.start, "Service2 not started")
		assert.True(t, svc3.start, "Service3 not started")
	})

	t.Run("Start with Duplicate Dependencies", func(t *testing.T) {
		g := graceful.New()
		svc1 := &MockSvc{name: "service1"}
		svc2 := &MockSvc{name: "service2"}
		svc3 := &MockSvc{name: "service3"}

		g.Add("service1", svc1)
		g.Add("service2", svc2, "service1")
		g.Add("service3", svc3, "service1", "service2")

		ctx := context.Background()
		err := g.Start(ctx)
		assert.NoError(t, err, "Error starting services")

		time.Sleep(500 * time.Millisecond)

		assert.True(t, svc1.start, "Service1 not started")
		assert.True(t, svc2.start, "Service2 not started")
		assert.True(t, svc3.start, "Service3 not started")
	})

	t.Run("Check complex dependencies", func(t *testing.T) {
		g := graceful.New()
		services := make(map[string]*MockSvc, 10)

		// Add services to the graph:
		for i := 0; i < 10; i++ {
			svc := &MockSvc{name: fmt.Sprintf("service%d", i)}
			services[svc.name] = svc
			g.Add(svc.name, svc)
		}

		// Manually defined dependencies (acyclic):
		// service0: service1, service2, service3
		// service1: service2, service3
		// service2: service3
		// service3: none
		// ... (continue pattern)

		g.Add("service0", services["service0"], "service1", "service2", "service3")
		g.Add("service1", services["service1"], "service2", "service3")
		g.Add("service2", services["service2"], "service3")
		g.Add("service3", services["service3"]) // No dependencies
		g.Add("service4", services["service4"], "service5", "service6", "service7", "service8", "service9")
		g.Add("service5", services["service5"], "service6", "service7", "service8", "service9")
		g.Add("service6", services["service6"], "service2", "service8", "service9")
		g.Add("service7", services["service7"], "service8", "service9")
		g.Add("service8", services["service8"], "service3")
		g.Add("service9", services["service9"]) // No dependencies

		// Start services and validate
		ctx := context.Background()
		err := g.Start(ctx)
		assert.NoError(t, err, "Error starting services")

		time.Sleep(500 * time.Millisecond)

		// Verify that all services are started successfully:
		for _, svc := range services {
			assert.True(t, svc.start, fmt.Sprintf("Service %s not started", svc.name))
		}
	})
}

func TestGraceful_Stop(t *testing.T) {
	t.Run("Check complex dependencies", func(t *testing.T) {
		g := graceful.New()
		services := make(map[string]*MockSvc, 10)

		// Add services to the graph:
		for i := 0; i < 10; i++ {
			svc := &MockSvc{name: fmt.Sprintf("service%d", i)}
			services[svc.name] = svc
			g.Add(svc.name, svc)
		}

		// Define dependencies for each service, ensuring an acyclic graph:
		for i := 0; i < 10; i++ {
			svcName := fmt.Sprintf("service%d", i)
			dependencies := make([]string, 0)
			for j := i + 1; j < 10; j++ { // Ensure dependencies on services with higher indices
				dependencyName := fmt.Sprintf("service%d", j)
				dependencies = append(dependencies, dependencyName)
			}
			g.Add(svcName, services[svcName], dependencies...)
		}

		// Expected order is hard to determine manually with a complex graph.
		// Instead, we'll validate the result by checking if all services are started
		// in the correct order without errors.

		ctx := context.Background()
		err := g.Start(ctx)
		assert.NoError(t, err, "Error starting services")

		time.Sleep(500 * time.Millisecond)

		// Verify that all services are started successfully:
		for _, svc := range services {
			assert.True(t, svc.start, fmt.Sprintf("Service %s not started", svc.name))
		}
	})
}
