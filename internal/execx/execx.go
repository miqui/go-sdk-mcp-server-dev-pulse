// Package execx provides an injectable interface for running external
// commands, so callers can swap in fakes for offline unit tests.
package execx

import (
	"context"
	"os/exec"
)

// Runner runs external commands. The production implementation shells out
// via os/exec; tests supply a fake.
type Runner interface {
	// Run executes name with args in dir and returns combined stdout; stderr
	// is folded into the returned error on failure.
	Run(ctx context.Context, dir string, name string, args ...string) (stdout string, err error)
}

// OSRunner is the production Runner backed by os/exec.CommandContext.
type OSRunner struct{}

// Run implements Runner using exec.CommandContext with argv-style arguments
// (never shell string concatenation).
func (OSRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), &RunError{Name: name, Args: args, Output: string(out), Err: err}
	}
	return string(out), nil
}

// RunError wraps a command failure with enough context for a clean tool
// error message.
type RunError struct {
	Name   string
	Args   []string
	Output string
	Err    error
}

// Error implements the error interface.
func (e *RunError) Error() string {
	return e.Name + ": " + e.Err.Error() + ": " + e.Output
}

// Unwrap supports errors.Is / errors.As.
func (e *RunError) Unwrap() error {
	return e.Err
}
