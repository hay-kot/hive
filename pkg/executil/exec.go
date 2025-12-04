// Package executil provides shell execution utilities.
package executil

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Executor runs shell commands.
type Executor interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
	// RunDir executes a command in a specific directory.
	RunDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error)
	// RunStream executes a command and streams stdout/stderr to the provided writers.
	RunStream(ctx context.Context, stdout, stderr io.Writer, cmd string, args ...string) error
	// RunDirStream executes a command in a specific directory and streams output.
	RunDirStream(ctx context.Context, dir string, stdout, stderr io.Writer, cmd string, args ...string) error
}

// RealExecutor calls actual shell commands.
type RealExecutor struct{}

// Run executes a command and returns its combined output.
func (e *RealExecutor) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("exec %s: %w", cmd, err)
	}
	return out, nil
}

// RunDir executes a command in a specific directory.
func (e *RealExecutor) RunDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("exec %s in %s: %w", cmd, dir, err)
	}
	return out, nil
}

// RunStream executes a command and streams stdout/stderr to the provided writers.
func (e *RealExecutor) RunStream(ctx context.Context, stdout, stderr io.Writer, cmd string, args ...string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", cmd, err)
	}
	return nil
}

// RunDirStream executes a command in a specific directory and streams output.
func (e *RealExecutor) RunDirStream(ctx context.Context, dir string, stdout, stderr io.Writer, cmd string, args ...string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("exec %s in %s: %w", cmd, dir, err)
	}
	return nil
}
