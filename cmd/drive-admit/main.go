package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 5 {
		fatal("usage: drive-admit seed-decision <journal> <ticket> <outdir>")
	}
	if os.Args[1] != "seed-decision" {
		fatal("only seed-decision supported")
	}
	fatal("seed-decision disabled: authoritative Decision persistence must use an authenticated authority path")
}

func fatal(err any) {
	fmt.Fprintln(os.Stderr, "drive-admit:", err)
	os.Exit(2)
}
