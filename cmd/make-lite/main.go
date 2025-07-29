package main

import (
	"fmt"
	"os"
)

func main() {
	// 1. Parse command line arguments.
	cfg := ParseCLI()

	if cfg.ShowHelp {
		printHelp()
		os.Exit(0)
	}

	if cfg.ShowVer {
		printVersion()
		os.Exit(0)
	}

	// 2. Check if the makefile exists.
	if _, err := os.Stat(cfg.Makefile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, ErrorMakefileNotFound, cfg.Makefile)
		os.Exit(1)
	}

	// 3. Initialize core components.
	vars := NewVariableStore()
	parser := NewParser(vars)

	// 4. Parse the makefile.
	makefile, err := parser.ParseFile(cfg.Makefile)
	if err != nil {
		fmt.Fprintf(os.Stderr, ErrorParsingMakefile, err)
		os.Exit(1)
	}

	// 5. Determine the target to build.
	target := cfg.Target
	if target == "" {
		if len(makefile.Rules) == 0 {
			fmt.Fprintln(os.Stderr, ErrorNoRulesNoTarget)
			os.Exit(1)
		}
		target = makefile.Rules[0].Targets[0]
		fmt.Printf(StatusUsingDefaultTarget, target)
	}

	// 6. Create and run the execution engine.
	engine, err := NewEngine(makefile, vars)
	if err != nil {
		fmt.Fprintf(os.Stderr, ErrorInitEngine, err)
		os.Exit(1)
	}

	err = engine.Build(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, ErrorBuildFailed, err)
		os.Exit(1)
	}

	fmt.Println(StatusBuildSuccess)
}
