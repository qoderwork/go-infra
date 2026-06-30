# lifecycle

A production-grade Go package for managing application lifecycle — coordinating graceful startup and shutdown of multiple concurrent tasks (HTTP servers, database connections, message consumers, background workers, etc.).

Inspired by [oklog/run](https://github.com/oklog/run) and [Uber Fx Lifecycle](https://pkg.go.dev/go.uber.org/fx#Lifecycle), but deliberately lighter: no dependency injection, no code generation — just clean Go.

## Features

- **State machine** — strict `Created → Running → Stopping → Stopped` progression with invalid-transition guards
- **Priority ordering** — tasks sorted by priority (descending); higher priority starts first, stops last (like `defer`)
- **Serial & Parallel executors** — choose sequential (LIFO stop) or concurrent execution
- **Four-phase hooks** — `OnStart`, `OnReady`, `OnStopping`, `OnStopped` for lifecycle instrumentation
- **Graceful shutdown** — `Wait(ctx)`, `WaitSignal(timeout, sigs...)`, and `Run()` (all-in-one)
- **Shutdown reason tracking** — `Reason()` reports what triggered shutdown (manual / signal / context / error)
- **Panic recovery** — panicking tasks are recovered and converted to errors
- **Metrics integration** — `MetricsHook` callback for Prometheus or other monitoring
- **Data-race free** — all exported methods are safe for concurrent use
- **Zero dependencies** — standard library only

## Requirements

Go 1.22 or later.

## Installation

```bash
go get lifecycle
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "lifecycle"
)

func main() {
    mgr := lifecycle.NewManager(
        lifecycle.WithShutdownTimeout(10 * time.Second),
    )

    // Add an HTTP server task
    mgr.AddTask(&httpTask{addr: ":8080"})

    // Run blocks until SIGINT/SIGTERM, then gracefully shuts down
    if err := mgr.Run(); err != nil {
        log.Fatal(err)
    }
}

type httpTask struct{ addr string }

func (t *httpTask) Name() string              { return "HTTP" }
func (t *httpTask) Priority() int             { return 10 }
func (t *httpTask) Start(ctx context.Context) error {
    go http.ListenAndServe(t.addr, nil)
    return nil
}
func (t *httpTask) Stop(ctx context.Context) error {
    // graceful shutdown logic
    return nil
}
```

## Task Interface

Every managed component implements `Task`:

```go
type Task interface {
    Name() string          // Human-readable identifier
    Priority() int         // Higher = starts first, stops last
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

Use `lifecycle.NewFuncTask` for quick inline tasks:

```go
task := lifecycle.NewFuncTask("redis",
    func(ctx context.Context) error { return redis.Connect() },
    func(ctx context.Context) error { return redis.Close() },
    lifecycle.WithTaskPriority(50),
)
```

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithExecutor(ExecutorSerial)` | Serial | Tasks start/stop one-by-one |
| `WithExecutor(ExecutorParallel)` | — | Tasks start/stop concurrently |
| `WithLogger(logger)` | `slog.Default()` | Custom logger (any `Printf`-compatible) |
| `WithShutdownTimeout(d)` | 30s | Timeout for graceful shutdown |
| `WithPanicRecovery(bool)` | `true` | Recover panicking tasks as errors |
| `WithMetricsHook(hook)` | no-op | Callback for lifecycle metric events |

## Hooks

Four hook points let you inject cross-cutting logic:

```go
mgr.OnStart(func(ctx context.Context) error {
    // Before any task starts — return error to abort startup
    return nil
})

mgr.OnReady(func(ctx context.Context) error {
    // After all tasks started — errors are logged but non-fatal
    return nil
})

mgr.OnStopping(func(ctx context.Context) error {
    // Shutdown initiated, before tasks are stopped
    return nil
})

mgr.OnStopped(func(ctx context.Context) error {
    // All tasks stopped — cleanup
    return nil
})
```

## Shutdown Methods

```go
// Manual stop
mgr.Stop(ctx)

// Stop with specific reason
mgr.ShutdownCtx(ctx, lifecycle.ReasonError)

// Block until context cancelled, then stop
mgr.Wait(ctx)

// Block until OS signal, then stop
mgr.WaitSignal(10*time.Second)           // default: SIGINT, SIGTERM
mgr.WaitSignal(5*time.Second, syscall.SIGTERM)

// All-in-one: Start + WaitSignal + Stop
mgr.Run()
```

After shutdown, inspect the outcome:

```go
mgr.State()   // StateStopped
mgr.Reason()  // ReasonSignal, ReasonContext, ReasonManual, ReasonError
mgr.Err()     // accumulated error (nil if clean)
```

## Metrics

Integrate with Prometheus or other monitoring:

```go
mgr := lifecycle.NewManager(
    lifecycle.WithMetricsHook(func(e lifecycle.MetricEvent) {
        lifecycleDuration.WithLabelValues(string(e.Phase)).Observe(e.Duration.Seconds())
        if e.Error != nil {
            lifecycleErrors.WithLabelValues(e.TaskName).Inc()
        }
    }),
)
```

## Examples

See the [`examples/`](./examples/) directory:

- **[http_server](./examples/http_server/)** — HTTP server + database connection with hooks and metrics
- **[worker](./examples/worker/)** — Background workers with different priorities and graceful drain

## Project Structure

```
lifecycle/
├── state.go          # State enum and transitions
├── task.go           # Task interface and FuncTask adapter
├── executor.go       # Serial and Parallel executors
├── manager.go        # Core Manager (orchestration, state machine)
├── options.go        # Functional options
├── hooks.go          # Hook type definition
├── metrics.go        # MetricEvent and MetricsHook
├── errors.go         # ShutdownReason and error helpers
├── logger.go         # Logger interface and implementations
├── *_test.go         # Unit, race, and benchmark tests
└── examples/
    ├── http_server/  # HTTP server + DB example
    └── worker/       # Background worker example
```

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector (Linux/macOS recommended)
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...
```

## Design Decisions

1. **No dependency injection** — tasks are registered explicitly; no reflection or code generation.
2. **Priority over dependency graph** — simpler mental model; use priority levels to encode "infra first, app last" ordering.
3. **Lock snapshot pattern** — the Manager holds the mutex only for state checks and snapshots, then releases it during executor/hook execution. This prevents deadlocks when tasks call back into the Manager.
4. **sync.Once for shutdown** — guarantees idempotent shutdown regardless of how many goroutines call Stop concurrently.
5. **errors.Join** — all shutdown errors are accumulated; no task failure is silently lost.

## License

MIT
