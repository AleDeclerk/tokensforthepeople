// Package main is the t4p entrypoint.
//
// t4p (tokensforthepeople) interviews the user, validates LLM provider keys
// against their free tiers, and emits configs for downstream tools.
//
// Subcommands are dispatched here. The init wizard lives in
// internal/wizard; the routing matrix lives in internal/routing.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/wizard"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println("t4p", Version)
	default:
		usage()
		os.Exit(2)
	}
}

// runInit wires the init subcommand. Returns the desired exit code so main
// stays trivial and testable.
func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	useCaseFlag := fs.String("use-case", "",
		"non-interactive: coding-agent|general-chat|agentic|rag|other")
	priorityFlag := fs.String("priority", "",
		"non-interactive: quality|latency|balanced|privacy")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var (
		ans wizard.Answers
		err error
	)

	// Non-interactive path is mostly for CI and smoke tests. Both flags must
	// be present together — half-specified is a usage error so we don't
	// silently fall through to the TUI in headless contexts.
	if *useCaseFlag != "" || *priorityFlag != "" {
		if *useCaseFlag == "" || *priorityFlag == "" {
			fmt.Fprintln(os.Stderr, "init: --use-case and --priority must be set together")
			return 2
		}
		ans = wizard.Answers{
			UseCase:  routing.UseCase(*useCaseFlag),
			Priority: routing.Priority(*priorityFlag),
		}
	} else {
		ans, err = wizard.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, "wizard:", err)
			return 1
		}
	}

	if err := wizard.PrintChain(os.Stdout, ans); err != nil {
		fmt.Fprintln(os.Stderr, "preview:", err)
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, `t4p — free LLM tokens for the rest of us

Usage:
  t4p init [--use-case=... --priority=...]   run the wizard (or non-interactive)
  t4p doctor                                  health-check configured providers (TODO)
  t4p serve                                   start a local proxy (TODO)
  t4p update-matrix                           refresh the free-tier matrix (TODO)
  t4p version                                 print version

See https://github.com/AleDeclerk/tokensforthepeople for docs.`)
}
