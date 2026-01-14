package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	// Since main() calls os.Exit, we need to test via subprocess
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		os.Args = []string{"rivian-ls", "version"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestVersionOutput")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Exit code 0 is expected
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				t.Fatalf("Expected exit code 0, got %d", exitErr.ExitCode())
			}
		}
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "rivian-ls version") {
		t.Errorf("Expected version output to contain 'rivian-ls version', got: %s", outputStr)
	}
}

func TestOkOutput(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		os.Args = []string{"rivian-ls"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestOkOutput")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")
	output, err := cmd.CombinedOutput()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				t.Fatalf("Expected exit code 0, got %d", exitErr.ExitCode())
			}
		}
	}

	outputStr := strings.TrimSpace(string(output))
	if !strings.Contains(outputStr, "ok") {
		t.Errorf("Expected output to contain 'ok', got: %s", outputStr)
	}
}
