package main

import (
	"fmt"
	"os"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", toolName, err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], dir, os.Stdout, os.Stderr))
}
