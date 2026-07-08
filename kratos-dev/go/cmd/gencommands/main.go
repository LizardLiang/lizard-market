// Command gencommands generates plugins/kratos/commands/<god>.md launchers
// and the god-derived regions of plugins/kratos/skills/auto/SKILL.md from
// plugins/kratos/agents/*.md frontmatter. See kratos-dev/codegen/README.md.
package main

import (
	"fmt"
	"os"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/gencmd"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var check, adopt bool
	for _, a := range args {
		switch a {
		case "--check":
			check = true
		case "--adopt":
			adopt = true
		case "-h", "--help":
			printUsage()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "gencommands: unknown flag %q\n", a)
			printUsage()
			return 2
		}
	}
	if check && adopt {
		fmt.Fprintln(os.Stderr, "gencommands: --check and --adopt are mutually exclusive")
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	repoRoot, err := gencmd.FindRepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	plan, err := gencmd.BuildPlan(repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if check {
		return runCheck(plan)
	}
	return runApply(plan, adopt)
}

func runCheck(plan *gencmd.Plan) int {
	drift, err := plan.Check()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(drift) == 0 {
		fmt.Println("gencommands --check: no drift")
		return 0
	}
	fmt.Println("gencommands --check: drift detected")
	for _, d := range drift {
		fmt.Printf("  %s: %s\n", d.Reason, d.Path)
	}
	return 1
}

func runApply(plan *gencmd.Plan, adopt bool) int {
	results, err := plan.Apply(adopt)
	// Print whatever succeeded first — Apply returns partial results
	// alongside a mid-loop error, so this is the only visibility into what
	// was written before the failure.
	for _, r := range results {
		fmt.Printf("%s: %s\n", r.Action, r.Path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: gencommands [--check | --adopt]

  --check   verify generated files match agent frontmatter without writing;
            exits non-zero and lists drifted files if they don't
  --adopt   allow overwriting existing commands/*.md files that lack the
            'generated: true' marker (first-run hand-off only)`)
}
