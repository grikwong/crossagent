package main

import (
	"fmt"
	"os"
)

const Version = "1.0.0"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("crossagent %s\n", Version)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "crossagent %s (Go) — not yet fully wired\n", Version)
	os.Exit(0)
}
