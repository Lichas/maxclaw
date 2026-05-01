package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Lichas/maxclaw/internal/cli"
)

func TestCLI_VersionCommand(t *testing.T) {
	// Isolate config directory to avoid interfering with user config.
	t.Setenv("MAXCLAW_HOME", t.TempDir())

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"maxclaw", "version"}

	var buf bytes.Buffer
	// Capture stdout by replacing the output writer on the root command.
	// This is safe because the root command is recreated in init() at package load.
	// For concurrent safety we run sequentially.
	if err := cli.ExecuteWithOutput(&buf); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "maxclaw") {
		t.Errorf("expected output to contain 'maxclaw', got: %s", out)
	}
	if !strings.Contains(out, "v") {
		t.Errorf("expected output to contain version string, got: %s", out)
	}
}

func TestCLI_GatewayStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Isolate config directory to avoid interfering with user config.
	tmpDir := t.TempDir()
	t.Setenv("MAXCLAW_HOME", tmpDir)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// Use port 0 to let OS assign a free port.
	os.Args = []string{"maxclaw", "gateway", "--port", "0"}

	// Capture stdout because gateway.go uses fmt.Printf directly.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Run gateway with a 5-second timeout context so it shuts down automatically.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		var dummy bytes.Buffer
		done <- cli.ExecuteWithContext(ctx, &dummy)
	}()

	var testErr error
	select {
	case err := <-done:
		// Expect context.Canceled or nil because we cancelled via timeout.
		if err != nil && !strings.Contains(err.Error(), "context canceled") && err != context.Canceled {
			testErr = err
		}
	case <-time.After(10 * time.Second):
		testErr = nil
		t.Fatalf("gateway did not shut down within timeout")
	}

	w.Close()
	os.Stdout = oldStdout

	if testErr != nil {
		var buf bytes.Buffer
		buf.ReadFrom(r)
		t.Fatalf("gateway returned unexpected error: %v\noutput:\n%s", testErr, buf.String())
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "Starting maxclaw gateway") {
		t.Errorf("expected startup message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Gateway ready") {
		t.Errorf("expected ready message in output, got:\n%s", out)
	}
}
