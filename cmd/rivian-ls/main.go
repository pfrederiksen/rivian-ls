package main

import (
	"fmt"
	"io"
	"os"
)

// Version information - set by GoReleaser via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func printVersion(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "rivian-ls version %s\n", version); err != nil {
		return err
	}
	if commit != "none" {
		if _, err := fmt.Fprintf(w, "  commit: %s\n", commit); err != nil {
			return err
		}
	}
	if date != "unknown" {
		if _, err := fmt.Fprintf(w, "  built:  %s\n", date); err != nil {
			return err
		}
	}
	return nil
}

func run(args []string, w io.Writer) int {
	if len(args) > 1 && args[1] == "version" {
		if err := printVersion(w); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error printing version: %v\n", err)
			return 1
		}
		return 0
	}

	// Minimal initial implementation - just prints "ok"
	if _, err := fmt.Fprintln(w, "ok"); err != nil {
		return 1
	}
	return 0
}

func main() {
	exitCode := run(os.Args, os.Stdout)
	os.Exit(exitCode)
}
