// Package structured provides scoped, structured concurrency primitives.
//
// Every goroutine started by this package has an owner, and the owner waits
// for it. A panic in a child goroutine is not swallowed: it is captured with
// a stack trace from the child and re-raised by Wait (or returned as an error
// with Recovered.AsError).
//
// Pick the smallest tool that fits:
//
//   - WaitGroup: spawn goroutines and wait for all of them.
//   - NewPool / NewPool().WithErrors(): bounded task runner, optionally collecting errors.
//   - NewPool().WithContext(ctx): cancel sibling tasks on the first error.
//   - NewResultPool[T](): bounded task runner returning results in submission order.
//   - ForEach / ForEachIdx / Map / MapErr: concurrent slice operations.
//   - NewStream(): tasks run concurrently, but callbacks run serially in submission order.
//   - Collector[T]: bridge concurrent producers to a synchronous range loop.
//   - Retry: exponential backoff with jitter for fallible operations.
//
// All pools must be waited on. Configuration methods (With*) panic if called
// after the first Go(), so build the pool before submitting work.
//
// This package is independent of github.com/qoderwork/go-infra/conc: that
// package holds concurrent data structures (TTLMap, generic Map) and
// fire-and-forget helpers, while this one holds scoped task runners.
package structured
