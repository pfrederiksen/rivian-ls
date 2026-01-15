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

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "rivian-ls version %s\n", version)
	if commit != "none" {
		fmt.Fprintf(w, "  commit: %s\n", commit)
	}
	if date != "unknown" {
		fmt.Fprintf(w, "  built:  %s\n", date)
	}
}

func run(args []string, w io.Writer) int {
	if len(args) > 1 && args[1] == "version" {
		printVersion(w)
		return 0
	}

	// Minimal initial implementation - just prints "ok"
	fmt.Fprintln(w, "ok")
	return 0
}

func main() {
	exitCode := run(os.Args, os.Stdout)
	os.Exit(exitCode)
}
