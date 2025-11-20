// Package graceful provides a mechanism for managing the lifecycle of services with dependencies.
//
// It ensures that services are started in the correct order and stopped in the reverse order,
// handling dependencies and errors gracefully.
//
// # Example
//
//	import (
//	  "context"
//	  "fmt"
//	  "time"
//	  "github.com/your/package/graceful"
//	)
//
//	type ServiceA struct{}
//
//	func (a *ServiceA) Start(ctx context.Context) error {
//	  fmt.Println("Service A starting...")
//	  time.Sleep(1 * time.Second) // Simulate service startup
//	  fmt.Println("Service A started.")
//	  return nil
//	}
//
//	func (a *ServiceA) Stop(ctx context.Context) error {
//	  fmt.Println("Service A stopping...")
//	  time.Sleep(1 * time.Second) // Simulate service shutdown
//	  fmt.Println("Service A stopped.")
//	  return nil
//	}
//
//	type ServiceB struct{}
//
//	func (b *ServiceB) Start(ctx context.Context) error {
//	  fmt.Println("Service B starting...")
//	  time.Sleep(1 * time.Second) // Simulate service startup
//	  fmt.Println("Service B started.")
//	  return nil
//	}
//
//	func (b *ServiceB) Stop(ctx context.Context) error {
//	  fmt.Println("Service B stopping...")
//	  time.Sleep(1 * time.Second) // Simulate service shutdown
//	  fmt.Println("Service B stopped.")
//	  return nil
//	}
//
//	func main() {
//	  mgr := graceful.New()
//	  a := &ServiceA{}
//	  b := &ServiceB{}
//
//	  mgr.Add("service-a", a)
//	  mgr.Add("service-b", b, "service-a")
//
//	  ctx := context.Background()
//
//		quit := make(chan os.Signal, 1)
//		signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)
//
//	  // Start all services in the correct order.
//	  if err := mgr.Start(ctx); err != nil {
//	    fmt.Println("Error starting services:", err)
//	    return
//	  }
//
//	  // Wait for a signal to stop services.
//	  fmt.Println("Press Ctrl+C to stop services...")
//	  <-quit
//
//	  // Stop all services gracefully.
//	  if err := mgr.Stop(ctx); err != nil {
//	    fmt.Println("Error stopping services:", err)
//	    return
//	  }
//
//	  fmt.Println("Services stopped gracefully.")
//	}
package graceful

import (
	"context"
	"fmt"
	"sync"
)

type (
	// Service is an interface representing a service that can be started and stopped.
	Service interface {
		// Start starts the service in the given context.
		Start(ctx context.Context) error
		// Stop stops the service in the given context.
		Stop(ctx context.Context) error
	}

	// ServiceDef defines a service with its dependencies.
	ServiceDef struct {
		Service Service   // service implementation
		Name    string    // service name
		Deps    []string  // list of dependencies
		once    sync.Once // Ensures Start is called only once for each service.
	}

	// Services is a map of service names to their definitions.
	Services map[string]*ServiceDef

	// Graceful manages the lifecycle of a set of services with dependencies.
	// It ensures that services are started in the correct order and stopped in the reverse order.
	Graceful struct {
		svcs  Services // Map of services.
		graph sync.Map // Dependency graph of services.
		order []string // Ordered list of service names.
	}

	// GracefulError is an error that occurred during service lifecycle.
	GracefulError struct {
		Service string // Service name that failed
		Reason  string // Reason for the error
		Err     error  // Underlying error
	}
)

// Error returns a formatted error string.
func (e *GracefulError) Error() string {
	return fmt.Sprintf("Error in service %s: %s: %v", e.Service, e.Reason, e.Err)
}

// NewGracefulError creates a new GracefulError.
func NewGracefulError(service, reason string, err error) *GracefulError {
	return &GracefulError{Service: service, Reason: reason, Err: err}
}

// sort calculates the topological order of the services based on their dependencies.
// It implements [Kahn's algorithm] for topological sorting.
//
//   - Time complexity: O(V+E), where V is the number of services and E is the number of dependencies.
//   - Space complexity: O(V+E).
//
// [Kahn's algorithm]: https://www.geeksforgeeks.org/kahns-algorithm-vs-dfs-approach-a-comparative-analysis/
func (g *Graceful) sort() ([]string, error) {
	// Calculate in-degree for each node (number of incoming edges)
	degree := make(map[string]int)
	for _, cmp := range g.svcs {
		for _, dep := range cmp.Deps {
			degree[dep]++
		}
	}

	// Initialize a queue with nodes having in-degree 0 (no incoming edges)
	queue := make([]string, 0)

	for name := range g.svcs {
		if _, ok := degree[name]; !ok {
			degree[name] = 0
		}

		if degree[name] == 0 {
			queue = append(queue, name)
		}
	}

	// Initialize an empty slice to store the topological order
	order := make([]string, 0)

	// Perform Kahn's algorithm
	for len(queue) > 0 {
		// Dequeue a node
		name := queue[0]
		queue = queue[1:]

		// Add the node to the topological order
		order = append(order, name)

		// Update in-degree of neighbors (remove outgoing edge)
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
				// If in-degree of neighbor becomes 0, enqueue it
				if degree[dep] == 0 {
					queue = append(queue, dep)
				}
			}
		}
	}

	// If there are still nodes with non-zero in-degree, the graph has a cycle and is not a DAG
	for _, zero := range degree {
		if zero > 0 {
			return nil, NewGracefulError("", "dependency cycle detected", nil)
		}
	}

	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	return order, nil
}

// Add adds a new service to the graceful manager.
func (g *Graceful) Add(name string, svc Service, deps ...string) {
	g.svcs[name] = &ServiceDef{Service: svc, Name: name, Deps: deps}
	g.graph.Store(name, deps)
}

// Start starts all registered services in the order defined by their dependencies.
//
// It starts services concurrently and waits for all services to start successfully.
func (g *Graceful) Start(ctx context.Context) error {
	sorted, err := g.sort()
	if err != nil {
		return err
	}

	// startctx is used to coordinate the startctx process.
	// It is canceled if any service fails to start, or when Start returns.
	startctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg := sync.WaitGroup{}
	chanerr := make(chan error, 1)
	ready := make(map[string]chan struct{})

	// creating ready signals against each service.
	for svc := range g.svcs {
		ready[svc] = make(chan struct{})
	}

	// start each service when all dependencies are ready.
	for _, name := range sorted {
		svc, ok := g.svcs[name]
		if !ok {
			return NewGracefulError(name, "service not found", nil)
		}

		if svc == nil {
			return NewGracefulError(name, "service is nil", nil)
		}

		wg.Add(1)

		go func(def *ServiceDef) {
			defer wg.Done()

			// wait for all dependencies to be ready
			for _, dep := range def.Deps {
				select {
				case <-ready[dep]: // continue when dependency is ready.
				case <-startctx.Done(): // return when dependency has failed elsewhere.
					return
				}
			}

			// start the service
			svc.once.Do(func() {
				// Pass the original context to the service so it doesn't get canceled
				// when Start returns.
				if err := def.Service.Start(ctx); err != nil {
					select {
					case chanerr <- err:
					default:
					}

					cancel()

					return
				}

				close(ready[def.Name]) // closing this indicates service is ready.
			})
		}(svc)
	}

	// asynchorously wait till all services are ready.
	go func() {
		wg.Wait()
		close(chanerr)
	}()

	g.order = sorted

	// returns error or nil (when channel is closed)
	return <-chanerr
}

// Stop stops all registered services sequentially in the reverse order they were started.
func (g *Graceful) Stop(ctx context.Context) error {
	// Iterate backwards over the start order for correct shutdown.
	for i := len(g.order) - 1; i >= 0; i-- {
		name := g.order[i]
		svc := g.svcs[name]
		if err := svc.Service.Stop(ctx); err != nil {
			return NewGracefulError(svc.Name, "stop failed", err)
		}
	}

	return nil
}

// New creates a new Graceful manager.
func New() *Graceful {
	return &Graceful{svcs: make(Services)}
}
