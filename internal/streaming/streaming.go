// Package streaming provides utilities for streaming command output over D-Bus signals.
package streaming

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/creack/pty"

	"github.com/godbus/dbus/v5"

	"linyapsmanager/internal/dbusconsts"
)

// OutputCallback is called for each chunk of output from the command.
// data is the output chunk, isStderr indicates if it came from stderr.
type OutputCallback func(operationID string, data string, isStderr bool)

// CompleteCallback is called when the command completes.
// exitCode is the process exit code (0 for success), errorMsg is non-empty on error.
type CompleteCallback func(operationID string, exitCode int, errorMsg string)

var operationCounter uint64

// GenerateOperationID generates a unique operation ID for tracking streaming operations.
func GenerateOperationID() string {
	id := atomic.AddUint64(&operationCounter, 1)
	return fmt.Sprintf("op-%d-%d", os.Getpid(), id)
}

// Emitter wraps a D-Bus connection for emitting streaming signals.
type Emitter struct {
	conn *dbus.Conn
	mu   sync.Mutex
}

// NewEmitter creates a new signal emitter.
func NewEmitter(conn *dbus.Conn) *Emitter {
	return &Emitter{conn: conn}
}

// EmitOutput sends an Output signal with command output data.
func (e *Emitter) EmitOutput(operationID, data string, isStderr bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.conn.Emit(
		dbus.ObjectPath(dbusconsts.ObjectPath),
		dbusconsts.Interface+"."+dbusconsts.SignalOutput,
		operationID, data, isStderr,
	)
}

// EmitComplete sends a Complete signal when operation finishes.
func (e *Emitter) EmitComplete(operationID string, exitCode int, errorMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.conn.Emit(
		dbus.ObjectPath(dbusconsts.ObjectPath),
		dbusconsts.Interface+"."+dbusconsts.SignalComplete,
		operationID, exitCode, errorMsg,
	)
}

// RunCommandStreaming executes a command and streams its output via D-Bus signals.
// Returns the operation ID immediately; the command runs asynchronously.
// The Complete signal will be emitted when the command finishes.
func RunCommandStreaming(ctx context.Context, emitter *Emitter, env []string, cmdPath string, args ...string) (string, error) {
	operationID := GenerateOperationID()

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Stream output in background
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)

		// Stream stdout
		go func() {
			defer wg.Done()
			streamReader(emitter, operationID, stdout, false)
		}()

		// Stream stderr
		go func() {
			defer wg.Done()
			streamReader(emitter, operationID, stderr, true)
		}()

		wg.Wait()

		// Wait for command to finish
		err := cmd.Wait()
		exitCode := 0
		errorMsg := ""
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
				errorMsg = err.Error()
			}
		}

		emitter.EmitComplete(operationID, exitCode, errorMsg)
	}()

	return operationID, nil
}

// RunCommandStreamingPTY executes a command inside a pseudo-terminal and streams its output.
// This is useful for commands that only emit progressive output when attached to a TTY (e.g., ll-cli install).
func RunCommandStreamingPTY(ctx context.Context, emitter *Emitter, env []string, cmdPath string, args ...string) (string, error) {
	operationID := GenerateOperationID()

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = env

	ptyFile, err := pty.StartWithAttrs(cmd, &pty.Winsize{Rows: 24, Cols: 80}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start command with pty: %w", err)
	}

	// Stream output in background
	go func() {
		defer ptyFile.Close()

		streamReader(emitter, operationID, ptyFile, false)

		// Wait for command to finish
		err := cmd.Wait()
		exitCode := 0
		errorMsg := ""
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
				errorMsg = err.Error()
			}
		}

		emitter.EmitComplete(operationID, exitCode, errorMsg)
	}()

	return operationID, nil
}

// RunCommandStreamingSync executes a command and streams its output via D-Bus signals.
// Blocks until the command completes. Returns the exit code and any error.
func RunCommandStreamingSync(ctx context.Context, emitter *Emitter, env []string, cmdPath string, args ...string) (string, int, error) {
	operationID := GenerateOperationID()

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return operationID, -1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return operationID, -1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return operationID, -1, fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		streamReader(emitter, operationID, stdout, false)
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		streamReader(emitter, operationID, stderr, true)
	}()

	wg.Wait()

	// Wait for command to finish
	err = cmd.Wait()
	exitCode := 0
	errorMsg := ""
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			errorMsg = err.Error()
		}
	}

	emitter.EmitComplete(operationID, exitCode, errorMsg)
	return operationID, exitCode, nil
}

// streamReader reads from a reader line by line and emits output signals.
func streamReader(emitter *Emitter, operationID string, r io.Reader, isStderr bool) {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	scanner.Split(scanLinesCR)

	for scanner.Scan() {
		line := scanner.Text() + "\n"
		log.Printf("[streaming] %s: %s", operationID, line)
		if err := emitter.EmitOutput(operationID, line, isStderr); err != nil {
			// Log error but continue streaming
			fmt.Fprintf(os.Stderr, "[streaming] failed to emit output: %v\n", err)
		}
	}
	// Ignore scanner errors - the process may have terminated
}

// scanLinesCR is like bufio.ScanLines but also treats carriage returns as line breaks.
// 有些命令（尤其是带进度条的）会用 \r 覆盖当前行显示进度，不一定带 \n。默认的 ScanLines 只认 \n，
// 会吃不到中途的进度；用这个函数就能把每次 \r 刷新的内容也当成一行，实时发出去。
func scanLinesCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Look for newline or carriage return.
	for i, b := range data {
		if b == '\n' || b == '\r' {
			// We have a full line.
			return i + 1, data[0:i], nil
		}
	}
	// If at EOF, return any remaining data.
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// Receiver handles receiving streaming signals on the client side.
type Receiver struct {
	conn       *dbus.Conn
	signalChan chan *dbus.Signal
	stopChan   chan struct{}
	stopped    bool
	mu         sync.Mutex
}

// NewReceiver creates a new signal receiver.
func NewReceiver(conn *dbus.Conn) (*Receiver, error) {
	signalChan := make(chan *dbus.Signal, 100)

	// Subscribe to Output and Complete signals
	matchOutput := fmt.Sprintf("type='signal',interface='%s',member='%s'",
		dbusconsts.Interface, dbusconsts.SignalOutput)
	matchComplete := fmt.Sprintf("type='signal',interface='%s',member='%s'",
		dbusconsts.Interface, dbusconsts.SignalComplete)

	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchOutput).Err; err != nil {
		return nil, fmt.Errorf("failed to add Output signal match: %w", err)
	}
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchComplete).Err; err != nil {
		return nil, fmt.Errorf("failed to add Complete signal match: %w", err)
	}

	conn.Signal(signalChan)

	return &Receiver{
		conn:       conn,
		signalChan: signalChan,
		stopChan:   make(chan struct{}),
	}, nil
}

// WaitForOperation waits for all output from a specific operation and returns
// when the Complete signal is received. It calls outputFn for each output chunk.
// Returns the exit code and error message from the Complete signal.
func (r *Receiver) WaitForOperation(operationID string, outputFn func(data string, isStderr bool)) (int, string) {
	for {
		select {
		case sig, ok := <-r.signalChan:
			if !ok {
				return -1, "signal channel closed"
			}

			if sig.Path != dbus.ObjectPath(dbusconsts.ObjectPath) {
				continue
			}

			switch sig.Name {
			case dbusconsts.Interface + "." + dbusconsts.SignalOutput:
				if len(sig.Body) >= 3 {
					opID, ok1 := sig.Body[0].(string)
					data, ok2 := sig.Body[1].(string)
					isStderr, ok3 := sig.Body[2].(bool)
					if ok1 && ok2 && ok3 && opID == operationID {
						outputFn(data, isStderr)
					}
				}

			case dbusconsts.Interface + "." + dbusconsts.SignalComplete:
				if len(sig.Body) >= 3 {
					opID, ok1 := sig.Body[0].(string)
					exitCode, ok2 := sig.Body[1].(int32)
					errorMsg, ok3 := sig.Body[2].(string)
					if ok1 && ok2 && ok3 && opID == operationID {
						return int(exitCode), errorMsg
					}
				}
			}

		case <-r.stopChan:
			return -1, "receiver stopped"
		}
	}
}

// Stop stops the receiver.
func (r *Receiver) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.stopped {
		r.stopped = true
		close(r.stopChan)
		r.conn.RemoveSignal(r.signalChan)
	}
}
