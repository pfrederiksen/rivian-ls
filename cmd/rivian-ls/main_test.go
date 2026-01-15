package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// errorWriter always returns an error when Write is called
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func TestPrintVersion(t *testing.T) {
	tests := []struct {
		name            string
		version         string
		commit          string
		date            string
		expectedContent []string
	}{
		{
			name:            "dev version",
			version:         "dev",
			commit:          "none",
			date:            "unknown",
			expectedContent: []string{"rivian-ls version dev"},
		},
		{
			name:    "release version with build info",
			version: "v1.0.0",
			commit:  "abc123def",
			date:    "2026-01-14T12:34:56Z",
			expectedContent: []string{
				"rivian-ls version v1.0.0",
				"commit: abc123def",
				"built:  2026-01-14T12:34:56Z",
			},
		},
		{
			name:    "version with commit only",
			version: "v0.1.0",
			commit:  "deadbeef",
			date:    "unknown",
			expectedContent: []string{
				"rivian-ls version v0.1.0",
				"commit: deadbeef",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origVersion, origCommit, origDate := version, commit, date
			defer func() {
				version, commit, date = origVersion, origCommit, origDate
			}()

			// Set test values
			version, commit, date = tt.version, tt.commit, tt.date

			var buf bytes.Buffer
			if err := printVersion(&buf); err != nil {
				t.Fatalf("printVersion failed: %v", err)
			}

			output := buf.String()
			for _, expected := range tt.expectedContent {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got: %s", expected, output)
				}
			}

			// Verify commit/date are not printed when they have default values
			if tt.commit == "none" && strings.Contains(output, "commit:") {
				t.Errorf("Should not print commit when it's 'none', got: %s", output)
			}
			if tt.date == "unknown" && strings.Contains(output, "built:") {
				t.Errorf("Should not print date when it's 'unknown', got: %s", output)
			}
		})
	}
}

func TestPrintVersionError(t *testing.T) {
	ew := &errorWriter{}
	err := printVersion(ew)
	if err == nil {
		t.Error("Expected error when writer fails, got nil")
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedExit   int
	}{
		{
			name:           "version command",
			args:           []string{"rivian-ls", "version"},
			expectedOutput: "rivian-ls version",
			expectedExit:   0,
		},
		{
			name:           "default output",
			args:           []string{"rivian-ls"},
			expectedOutput: "ok",
			expectedExit:   0,
		},
		{
			name:           "no arguments",
			args:           []string{},
			expectedOutput: "ok",
			expectedExit:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			exitCode := run(tt.args, &buf)

			if exitCode != tt.expectedExit {
				t.Errorf("Expected exit code %d, got %d", tt.expectedExit, exitCode)
			}

			output := buf.String()
			if !strings.Contains(output, tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got: %s", tt.expectedOutput, output)
			}
		})
	}
}

func TestRunVersionError(t *testing.T) {
	ew := &errorWriter{}
	exitCode := run([]string{"rivian-ls", "version"}, ew)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 on version error, got %d", exitCode)
	}
}

func TestRunOkError(t *testing.T) {
	ew := &errorWriter{}
	exitCode := run([]string{"rivian-ls"}, ew)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 on write error, got %d", exitCode)
	}
}

func TestPrintVersionCommitError(t *testing.T) {
	// Save original values
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	// Set values so commit branch is taken
	version, commit, date = "v1.0.0", "abc123", "unknown"

	ew := &errorWriter{}
	err := printVersion(ew)
	if err == nil {
		t.Error("Expected error when writer fails on version line, got nil")
	}
}

func TestPrintVersionDateError(t *testing.T) {
	// Save original values
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	// Set values so date branch is taken
	version, commit, date = "v1.0.0", "none", "2026-01-14"

	ew := &errorWriter{}
	err := printVersion(ew)
	if err == nil {
		t.Error("Expected error when writer fails, got nil")
	}
}
