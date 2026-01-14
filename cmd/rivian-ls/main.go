package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("rivian-ls version %s\n", version)
		return
	}

	// Minimal initial implementation - just prints "ok"
	fmt.Println("ok")
	os.Exit(0)
}
