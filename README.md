# graceful

![GitHub release (with filter)](https://img.shields.io/github/v/release/breuHQ/graceful)
![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/breuHQ/graceful)
[![License](https://img.shields.io/github/license/breuHQ/graceful)](./LICENSE)
![GitHub contributors](https://img.shields.io/github/contributors/breuHQ/graceful)

## Motivation

`graceful` is a lightweight Go package for managing the graceful startup and shutdown of services with dependencies. It provides a simple and flexible API for handling the lifecycle of services in a structured and efficient manner.

### Why not use `uber/fx`?

While `uber/fx` is a powerful dependency injection framework for Go, we chose to create `graceful` because it offers a more focused and lightweight solution specifically for graceful shutdown. While `uber/fx` encompasses a wider range of functionalities including dependency injection and lifecycle management, `graceful` provides a streamlined API tailored to the needs of graceful shutdown. This narrower focus makes it easier to understand and integrate into projects, particularly for simpler use cases where the full scope of `uber/fx` might be unnecessary.

## API

The `graceful` package provides a mechanism for managing the lifecycle of services with dependencies. It ensures that services are started in the correct order and stopped in the reverse order, handling dependencies and errors gracefully.

Here's a breakdown of the core components:

- **`Service` Interface:** Represents a service that can be started and stopped.
- **`ServiceDef` Structure:** Defines a service with its dependencies.
- **`Services` Map:** Stores a collection of service definitions.
- **`Graceful` Structure:** Manages the lifecycle of a set of services, ensuring proper startup and shutdown order.

### Key Features

- **Dependency Management:**  Ensures services start in the correct order based on their dependencies.
- **Topological Sorting:**  Utilizes Kahn's algorithm to efficiently determine the service startup order.
- **Concurrent Start/Stop:** Allows for parallel service initiation and termination for faster operation.
- **Error Handling:**  Gracefully propagates errors encountered during service start/stop operations.
- **GracefulError:** Provides a specialized error type to track service-specific failures.

## Getting Started

```go
package main

import (
	"context"
	"fmt"
	"time"

	"go.breu.io/graceful"
)

type (
	// ExampleService implements the Service interface.
	ExampleService struct {
		name string
	}
)

// Start starts the ExampleService.
func (s *ExampleService) Start(ctx context.Context) error {
	fmt.Printf("Starting service: %s\n", s.name)
	// Perform service start logic here...
	time.Sleep(1 * time.Second)
	return nil
}

// Stop stops the ExampleService.
func (s *ExampleService) Stop(ctx context.Context) error {
	fmt.Printf("Stopping service: %s\n", s.name)
	// Perform service stop logic here...
	time.Sleep(1 * time.Second)
	return nil
}

func main() {
	ctx := context.Background()
	terminate := make(chan os.Signal, 1)

	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)


	// Create a new Graceful manager.
	g := graceful.New()

	// Add services to the manager.
	g.Add("service1", &ExampleService{name: "service1"})
	g.Add("service2", &ExampleService{name: "service2"}, "service1")
	g.Add("service3", &ExampleService{name: "service3"}, "service2")

	// Start all services.
	if err := g.Start(ctx); err != nil {
		fmt.Printf("Error starting services: %v\n", err)
		return
	}

	<- terminate

	// Stop all services gracefully.
	if err := g.Stop(ctx); err != nil {
		fmt.Printf("Error stopping services: %v\n", err)
		return
	}

	fmt.Println("All services stopped gracefully.")
}

```

This code demonstrates how to use the `graceful` package to manage the lifecycle of three services with dependencies. The `service2` depends on `service1` and `service3` depends on `service2`, ensuring they are started in the correct order. The `graceful.Stop()` function handles the graceful shutdown process, stopping the services in the reverse order they were started.

## Contributing

Contributions to this project are welcome! If you have any issues or feature requests, please submit them through the GitHub issue tracker.

## License

This project is licensed under the MIT License.
