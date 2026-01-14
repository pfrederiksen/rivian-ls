package main

import (
	"fmt"
	"os"
)

// Version information - set by GoReleaser via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("rivian-ls version %s\n", version)
		if commit != "none" {
			fmt.Printf("  commit: %s\n", commit)
		}
		if date != "unknown" {
			fmt.Printf("  built:  %s\n", date)
		}
		return
	}

	// Minimal initial implementation - just prints "ok"
	fmt.Println("ok")
	os.Exit(0)
}
