// Package main is the t4p entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/plan"
	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
	"github.com/AleDeclerk/tokensforthepeople/internal/wizard"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
// goreleaser and the Homebrew formula set it; `go install` does not, leaving
// it at "dev" — resolveVersion falls back to the module version baked into the
// binary's build info so `go install` users still see the real tag.
var Version = "dev"

// resolveVersion picks the most specific version available. The ldflags value
// wins when set; otherwise it falls back to the module version from the build
// info ("(devel)" means an unversioned local build, so it is ignored).
func resolveVersion(ldflags, buildInfo string, haveBuildInfo bool) string {
	if ldflags != "dev" {
		return ldflags
	}
	if haveBuildInfo && buildInfo != "" && buildInfo != "(devel)" {
		return buildInfo
	}
	return ldflags
}

// versionString resolves the version using the running binary's build info.
func versionString() string {
	buildInfo, have := "", false
	if info, ok := debug.ReadBuildInfo(); ok {
		buildInfo, have = info.Main.Version, true
	}
	return resolveVersion(Version, buildInfo, have)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println("t4p", versionString())
	default:
		usage()
		os.Exit(2)
	}
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	useCaseFlag := fs.String("use-case", "",
		"non-interactive: coding-agent|general-chat|agentic|rag|other")
	priorityFlag := fs.String("priority", "",
		"non-interactive: quality|latency|balanced|privacy")
	var keyFlags stringSliceFlag
	fs.Var(&keyFlags, "key",
		"non-interactive: provider=value (repeatable). gemini, groq, openrouter, ollama, cerebras.")
	targetsFlag := fs.String("targets", "",
		"non-interactive: comma list of cline,continue,aider,litellm")
	writeFlag := fs.Bool("write", false,
		"persist validated keys (chmod 600) and selected target configs to disk")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var (
		ans wizard.Answers
		err error
	)

	if *useCaseFlag != "" || *priorityFlag != "" || len(keyFlags) > 0 || *targetsFlag != "" {
		if *useCaseFlag == "" || *priorityFlag == "" {
			fmt.Fprintln(os.Stderr, "init: --use-case and --priority must be set together")
			return 2
		}
		ans, err = buildNonInteractiveAnswers(*useCaseFlag, *priorityFlag, keyFlags, *targetsFlag)
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
		return writeOutputs(ans)
	}
	return 0
}

// writeOutputs persists keys.env and every selected target's config.
// Returns the desired process exit code.
func writeOutputs(ans wizard.Answers) int {
	rep, err := plan.Apply(plan.Input{
		UseCase:  ans.UseCase,
		Priority: ans.Priority,
		Keys:     ans.Keys,
		Targets:  ans.Targets,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "\n✓ wrote %s (chmod 600)\n", rep.KeysPath)
	if len(rep.Targets) == 0 {
		fmt.Fprintln(os.Stdout, "  no targets selected — keys.env is enough for direnv / manual use")
		return 0
	}
	failed := 0
	for _, tr := range rep.Targets {
		if !tr.OK {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %s\n", tr.Target, tr.Err)
			failed++
			continue
		}
		verb := "updated"
		if tr.Created {
			verb = "created"
		}
		fmt.Fprintf(os.Stdout, "  ✓ %s %s\n", verb, tr.Path)
		if tr.Backup != "" {
			fmt.Fprintf(os.Stdout, "    backed up to %s\n", tr.Backup)
		}
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d of %d targets failed.\n", failed, len(rep.Targets))
		return 1
	}
	return 0
}

func buildNonInteractiveAnswers(useCase, priority string, keyFlags []string, targetsCSV string) (wizard.Answers, error) {
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
	if targetsCSV != "" {
		for _, t := range strings.Split(targetsCSV, ",") {
			t = strings.TrimSpace(strings.ToLower(t))
			if t == "" {
				continue
			}
			if !isKnownTarget(emit.Target(t)) {
				return ans, fmt.Errorf("--targets: unknown target %q", t)
			}
			ans.Targets = append(ans.Targets, emit.Target(t))
		}
	}
	return ans, nil
}

func isKnownTarget(t emit.Target) bool {
	for _, known := range emit.All {
		if known == t {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Fprintln(os.Stderr, `t4p — free LLM tokens for the rest of us

Usage:
  t4p init [flags]
    Run the wizard. With --use-case and --priority set, runs non-interactive.
    --use-case   coding-agent|general-chat|agentic|rag|other
    --priority   quality|latency|balanced|privacy
    --key        provider=value (repeatable, live-validated)
    --targets    cline,continue,aider,litellm  (comma list)
    --write      persist keys (chmod 600) and emit selected target configs

  t4p doctor                                  health-check configured providers (TODO)
  t4p serve                                   start a local proxy (TODO)
  t4p update-matrix                           refresh the free-tier matrix (TODO)
  t4p version                                 print version

See https://github.com/AleDeclerk/tokensforthepeople for docs.`)
}
