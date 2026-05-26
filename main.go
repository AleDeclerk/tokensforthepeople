// Package main is the t4p entrypoint.
//
// t4p (tokensforthepeople) interviews the user, validates LLM provider keys
// against their free tiers, and emits configs for downstream tools.
//
// Real wiring lives in cmd/. This file only dispatches.
package main

import (
	"fmt"
	"os"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println("t4p", Version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `t4p — free LLM tokens for the rest of us

Usage:
  t4p init                 run the wizard (not implemented yet)
  t4p doctor               health-check configured providers
  t4p serve                start a local OpenAI-compatible proxy
  t4p update-matrix        refresh the free-tier matrix
  t4p version              print version

See https://github.com/AleDeclerk/tokensforthepeople for docs.`)
}
