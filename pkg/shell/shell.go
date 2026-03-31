package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultTimeout = 30 * time.Minute

// Run executes a command with the executor's default timeout.
// It return stdout on success, or a descriptive error on failure.
func Run(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	return RunWithContext(ctx, name, args...)
}

// RunWithContext executes a command with the provided context (supports cancellation).
func RunWithContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("empty command")
	}

	if ctx == nil {
		return nil, errors.New("nil context")
	}

	cmdStr := buildCmdStr(name, args)
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		// Distinguish context cancellation from command failure.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%q timed out or cancelled:%w", cmdStr, ctx.Err())
		}
		errMsg := strings.TrimSpace(output.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return output.Bytes(), &ExecError{
			Cmd:    cmdStr,
			Stderr: errMsg,
			Err:    err,
		}
	}

	return output.Bytes(), nil
}

// RunShell executes a shell script string via /bin/sh -c.
func RunShell(script string) ([]byte, error) {
	return Run("/bin/sh", "-c", script)
}

// RunShellContext executes a shell script string via /bin/sh -c with context.
func RunShellWithContext(ctx context.Context, script string) ([]byte, error) {
	return RunWithContext(ctx, "/bin/sh", "-c", script)
}

// Exists checks if a command/binary is available in PATH.
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ============================================================
// ExecError
// ============================================================

// ExecError is returned when a command exits with a non-zero status.
type ExecError struct {
	Cmd    string
	Stderr string
	Err    error
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("command %q failed: %s", e.Cmd, e.Stderr)
}

func (e *ExecError) Unwrap() error {
	return e.Err
}

// IsExecError returns true if the error is an ExecError.
func IsExecError(err error) bool {
	_, ok := err.(*ExecError)
	return ok
}

// ============================================================
// Helpers
// ============================================================

func buildCmdStr(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}
