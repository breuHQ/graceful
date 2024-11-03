package graceful

import (
	"context"
	"fmt"
	"sync"
)

type (
	Service interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
	}

	ServiceDef struct {
		Service Service
		Name    string
		Deps    []string

		once sync.Once
	}

	Services map[string]*ServiceDef

	Graceful struct {
		svcs  Services
		graph sync.Map
		order []string

		cherr chan error
	}

	GracefulError struct {
		Service string
		Reason  string
		Err     error
	}
)

func (e *GracefulError) Error() string {
	return fmt.Sprintf("Error in service %s: %s: %v", e.Service, e.Reason, e.Err)
}

func NewGracefulError(service, reason string, err error) *GracefulError {
	return &GracefulError{Service: service, Reason: reason, Err: err}
}

func (g *Graceful) sort() ([]string, error) {
	// 1. Calculate in-degree for each node (number of incoming edges)
	degree := make(map[string]int)
	for _, cmp := range g.svcs {
		for _, dep := range cmp.Deps {
			degree[dep]++
		}
	}

	// 2. Initialize a queue with nodes having in-degree 0 (no incoming edges)
	queue := make([]string, 0)
	for name := range g.svcs {
		if _, ok := degree[name]; !ok {
			degree[name] = 0
		}

		if degree[name] == 0 {
			queue = append(queue, name)
		}
	}

	// 3. Initialize an empty slice to store the topological order
	order := make([]string, 0)

	// 4. Perform Kahn's algorithm
	for len(queue) > 0 {
		// 5. Dequeue a node
		name := queue[0]
		queue = queue[1:]

		// 6. Add the node to the topological order
		order = append(order, name)

		// 7. Update in-degree of neighbors (remove outgoing edge)
		deps, ok := g.graph.Load(name)
		if !ok {
			return nil, NewGracefulError(name, "dependency graph missing entry", nil)
		}

		list, ok := deps.([]string)
		if !ok {
			return nil, NewGracefulError(name, "invalid dependency type", nil)
		}

		for _, dep := range list {
			// Check if dep is actually in the degree map
			if _, ok := degree[dep]; ok {
				degree[dep]--
				// 8. If in-degree of neighbor becomes 0, enqueue it
				if degree[dep] == 0 {
					queue = append(queue, dep)
				}
			}
		}
	}

	// 9. If there are still nodes with non-zero in-degree,
	//    the graph has a cycle and is not a DAG
	for _, zero := range degree {
		if zero > 0 {
			return nil, fmt.Errorf("Dependency graph has a cycle")
		}
	}

	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	return order, nil
}

func (g *Graceful) Add(name string, svc Service, deps ...string) {
	g.svcs[name] = &ServiceDef{Service: svc, Name: name, Deps: deps}
	g.graph.Store(name, deps)
}

func (g *Graceful) Start(ctx context.Context) error {
	g.cherr = make(chan error)
	started := make(map[string]bool)

	sorted, err := g.sort()
	if err != nil {
		return err
	}

	for _, name := range sorted {
		svc, ok := g.svcs[name]
		if !ok {
			return NewGracefulError(name, "service not found", nil)
		}

		if svc == nil {
			return NewGracefulError(name, "service is nil", nil)
		}

		svc.once.Do(func() {
			for _, dep := range svc.Deps {
				for {
					_, ok := started[dep]
					if ok {
						break
					}
				}
			}

			go func() {
				if err := svc.Service.Start(ctx); err != nil {
					g.cherr <- NewGracefulError(name, "service start failed", err)
				}
			}()

			g.order = append(g.order, name)
			started[name] = true
		})
	}

	return nil
}

func (g *Graceful) Stop(ctx context.Context) error {
	var wg sync.WaitGroup
	// Use the reverse of the started order to stop services
	for i := len(g.order) - 1; i >= 0; i-- {
		name := g.order[i]

		wg.Add(1)

		go func() {
			defer wg.Done()

			for _, cmp := range g.svcs {
				if cmp.Name == name {
					if err := cmp.Service.Stop(ctx); err != nil {
						g.cherr <- NewGracefulError(name, "service stop failed", err)
					}

					return
				}
			}
		}()
	}
	wg.Wait()

	select {
	case err := <-g.cherr:
		return err
	default:
		return nil
	}
}

func New() *Graceful {
	return &Graceful{svcs: make(Services)}
}
