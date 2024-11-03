package graceful_test

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
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

	t.Run("Start with complex dependencies", func(t *testing.T) {
		g := graceful.New()
		services := make(map[string]*MockSvc)
		for i := 10; i < 100; i++ {
			svc := &MockSvc{name: fmt.Sprintf("service%d", i)}
			services[svc.name] = svc
			g.Add(svc.name, svc)
		}

		g.Add("service1", &MockSvc{name: "service1"})
		g.Add("service2", &MockSvc{name: "service2"})
		g.Add("service3", &MockSvc{name: "service3"})
		g.Add("service4", &MockSvc{name: "service4"})
		g.Add("service5", &MockSvc{name: "service5"})
		g.Add("service6", &MockSvc{name: "service6"})
		g.Add("service7", &MockSvc{name: "service7"})
		g.Add("service8", &MockSvc{name: "service8"})
		g.Add("service9", &MockSvc{name: "service9"})
		g.Add("service10", &MockSvc{name: "service0"})

		for i := 0; i < 100; i++ {
			svcName := fmt.Sprintf("service%d", i)
			svc := services[svcName]
			numDependencies := rand.Intn(5)
			dependencies := make([]string, 0, numDependencies)
			for j := 0; j < numDependencies; j++ {
				dependencyName := fmt.Sprintf("service%d", rand.Intn(100))
				if dependencyName != svcName && !slices.Contains(dependencies, dependencyName) {
					dependencies = append(dependencies, dependencyName)
				}
			}

			fmt.Println(svcName, dependencies)
			g.Add(svcName, svc, dependencies...)
		}

		ctx := context.Background()
		err := g.Start(ctx)
		assert.NoError(t, err, "Error starting services")

		time.Sleep(500 * time.Millisecond)

		for _, svc := range services {
			assert.True(t, svc.start, fmt.Sprintf("Service %s not started", svc.name))
		}
	})
}

func TestGraceful_Stop(t *testing.T) {
	t.Run("Stop successfully", func(t *testing.T) {
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

		err = g.Stop(ctx)
		assert.NoError(t, err, "Error stopping services")

		assert.True(t, svc1.stop, "Service1 not stopped")
		assert.True(t, svc2.stop, "Service2 not stopped")
		assert.True(t, svc3.stop, "Service3 not stopped")
	})
}
