// Package main is the t4p entrypoint.
//
// Subcommands are dispatched here. The init wizard lives in
// internal/wizard; the routing matrix lives in internal/routing.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/keystore"
	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
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

// stringSliceFlag lets the user repeat --key flag in non-interactive mode.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// runInit wires the init subcommand. Returns the desired exit code so main
// stays trivial and testable.
func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	useCaseFlag := fs.String("use-case", "",
		"non-interactive: coding-agent|general-chat|agentic|rag|other")
	priorityFlag := fs.String("priority", "",
		"non-interactive: quality|latency|balanced|privacy")
	var keyFlags stringSliceFlag
	fs.Var(&keyFlags, "key",
		"non-interactive: provider=value (repeatable). Providers: gemini, groq, openrouter, ollama, cerebras.")
	writeFlag := fs.Bool("write", false,
		"after the wizard, write validated keys to ~/.config/t4p/keys.env (chmod 600)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var (
		ans wizard.Answers
		err error
	)

	// Non-interactive path: --use-case + --priority + (optionally) --key flags.
	if *useCaseFlag != "" || *priorityFlag != "" || len(keyFlags) > 0 {
		if *useCaseFlag == "" || *priorityFlag == "" {
			fmt.Fprintln(os.Stderr, "init: --use-case and --priority must be set together")
			return 2
		}
		ans, err = buildNonInteractiveAnswers(*useCaseFlag, *priorityFlag, keyFlags)
		if err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			return 2
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

	if *writeFlag {
		if len(ans.Keys) == 0 {
			fmt.Fprintln(os.Stderr, "--write requested but no keys collected; nothing to write.")
			return 1
		}
		path, err := keystore.DefaultPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "init: resolve config dir:", err)
			return 1
		}
		if err := keystore.Write(path, ans.Keys); err != nil {
			fmt.Fprintln(os.Stderr, "init: write keys:", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "\n✓ wrote %s (chmod 600)\n", path)
	}
	return 0
}

// buildNonInteractiveAnswers constructs Answers without firing the TUI.
// Keys passed via --key are validated live, same as in the wizard, so the
// non-interactive path can't accept a key the interactive path would reject.
func buildNonInteractiveAnswers(useCase, priority string, keyFlags []string) (wizard.Answers, error) {
	ans := wizard.Answers{
		UseCase:  routing.UseCase(useCase),
		Priority: routing.Priority(priority),
		Keys:     map[string]string{},
	}
	for _, raw := range keyFlags {
		name, value, ok := strings.Cut(raw, "=")
		if !ok || name == "" || value == "" {
			return ans, fmt.Errorf("--key must be provider=value (got %q)", raw)
		}
		p, ok := providers.ByID(routing.Provider(strings.ToLower(name)))
		if !ok {
			return ans, fmt.Errorf("--key: unknown provider %q", name)
		}
		res, err := validation.Ping(p, value, 5*time.Second)
		if err != nil {
			return ans, fmt.Errorf("validate %s: %w", p.Display, err)
		}
		switch res.Status {
		case validation.StatusOK, validation.StatusQuotaExceeded:
			ans.Keys[p.EnvVar] = value
		case validation.StatusInvalid:
			return ans, fmt.Errorf("invalid %s key (HTTP %d)", p.Display, res.HTTPStatus)
		case validation.StatusNetworkError:
			return ans, fmt.Errorf("could not reach %s (%s)", p.Display, res.Detail)
		}
	}
	return ans, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `t4p — free LLM tokens for the rest of us

Usage:
  t4p init [flags]
    Run the wizard. With --use-case and --priority set, runs non-interactive.
    --use-case   coding-agent|general-chat|agentic|rag|other
    --priority   quality|latency|balanced|privacy
    --key        provider=value (repeatable, live-validated)
    --write      persist validated keys to ~/.config/t4p/keys.env (chmod 600)

  t4p doctor                                  health-check configured providers (TODO)
  t4p serve                                   start a local proxy (TODO)
  t4p update-matrix                           refresh the free-tier matrix (TODO)
  t4p version                                 print version

See https://github.com/AleDeclerk/tokensforthepeople for docs.`)
}
