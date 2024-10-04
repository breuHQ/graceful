# graceful

![GitHub release (with filter)](https://img.shields.io/github/v/release/breuHQ/graceful)
![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/breuHQ/graceful)
[![License](https://img.shields.io/github/license/breuHQ/graceful)](./LICENSE)
![GitHub contributors](https://img.shields.io/github/contributors/breuHQ/graceful)

## Motivation

The `errgroup` package is excellent for coordinating goroutines and collecting errors, but it doesn't directly handle the critical aspect shutting down goroutines.
This package, `graceful`, aims to handle graceful shutdown of shutdown of goroutines.

`graceful` aims to solve the following issues:

- **Asynchronous cleanup of channels:** Many applications involve channels for communication between goroutines.  Graceful shutdown often necessitates closing these channels to prevent further writes and allow existing readers to complete their tasks before the program exits.  This package provides a mechanism for closing channels and handling potential errors during this closing process.
- **Handling initialization errors:** Concurrent goroutines might encounter errors during initialization, potentially halting the entire application.  `graceful` allows applications to catch and report these errors from various concurrently started goroutines.
- **Timeout management:** Cleanup operations, like closing channels, could block indefinitely if the shutdown process isn't time-constrained.  This package includes a timeout mechanism that prevents indefinite blocking during shutdown, gracefully terminating the program if cleanup takes too long.
- **Error aggregation:** `graceful` collects and reports errors encountered during initialization, cleanup tasks, and potentially during channel closure, providing comprehensive diagnostic information during shutdown.

By providing a structured approach for handling channel closure and other cleanup operations, `graceful` promotes program stability, prevents resource leaks, and ensures data integrity during shutdown.

## API

The graceful package provides two core functions for graceful shutdown management:

- `Go(ctx context.Context, fn func() error, errs chan error)`: This function launches fn in a new goroutine, forwarding any initialization errors to the errs channel. This is crucial for collecting errors from concurrently started goroutines.
- `Shutdown(ctx context.Context, cleanups []Cleanup, interrupt chan os.Signal, timeout time.Duration, code int) int`: This function gracefully shuts down the application by first signaling all goroutines (interrupt) to perform their cleanup. It then waits for all cleanups in cleanups to complete within the given timeout. The return code reflects whether the shutdown was successful (0) or if it timed out/failed (non-zero).

The graceful package also provides a Cleanup type, which is a slice of functions that perform cleanup operations. This type is used to collect cleanup functions that need to be executed during shutdown.

Apart from these core functions, the graceful package also provides two utility functions:

- `GrabAndGo(fn func() error, args ...interface{}) func() error`: This function wraps fn to accept variadic arguments and returns a function that can be used with Go. This is useful for functions that require arguments.
- `WrapRelease(fn func(chan any), interrupt chan any) func() error`: This function wraps fn to accept an interrupt channel and returns a function that can be used with Go. This is useful for functions that need to be interrupted during shutdown.

## Getting Started

To get started, install the package using the following command:

```shell
go get go.breu.io/graceful
```

Then, import the package in your code:

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"slog"

	"github.com/labstack/echo/v4"
	"go.breu.io/graceful"
)

func main() {
	ctx := context.Background()
	web := echo.New()

	errs := make(chan error)
	interrupt := make(chan any)
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)

	graceful.Go(ctx, graceful.GrabAndGo(web.Start, "8000"), errs)
	graceful.Go(ctx, graceful.WrapRelease(worker.Run, interrupt), errs)

	cleanups := graceful.Cleanup{
		web.Stop,
	}

	select {
	err <- errs:
		slog.Error("Error encountered during initialization: %v", err)
	case <-terminate:
		slog.Info("Received termination signal")
	}

	code := graceful.Shutdown(ctx, cleanups, interrupt, time.Second*10)

	os.Exit(code)
}
```
