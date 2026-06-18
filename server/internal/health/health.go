// Package health provides liveness and readiness checks for the control plane
// (endpoints.md / architecture.md). Liveness (/healthz) reports the process is
// up; readiness (/readyz) reports the process can serve traffic — its
// dependencies (the repository, in production also PostgreSQL/MinIO) are
// reachable.
package health

import "context"

// Checker reports liveness and readiness.
type Checker interface {
	// Live reports process liveness. It is cheap and dependency-free.
	Live() bool
	// Ready reports whether the process can serve requests (dependencies
	// reachable). It may consult ctx for cancellation/timeout.
	Ready(ctx context.Context) bool
}

// ReadyFunc is the readiness probe a Checker runs against its dependencies.
type ReadyFunc func(ctx context.Context) bool

// checker is the default Checker implementation.
type checker struct {
	ready ReadyFunc
}

// New builds a Checker whose readiness is the supplied probe. A nil probe is
// treated as "always ready" (the dependency-free MVP default).
func New(ready ReadyFunc) Checker {
	if ready == nil {
		ready = func(context.Context) bool { return true }
	}
	return &checker{ready: ready}
}

// Live always returns true while the process is running and able to execute
// this method.
func (c *checker) Live() bool { return true }

// Ready runs the configured readiness probe.
func (c *checker) Ready(ctx context.Context) bool { return c.ready(ctx) }
