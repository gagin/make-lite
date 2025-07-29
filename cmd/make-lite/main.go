package main

import (
	"fmt"
	"os"
)

func main() {
	cfg := ParseCLI()

	if cfg.ShowHelp {
		printHelp()
		os.Exit(0)
	}
	if cfg.ShowVer {
		printVersion()
		os.Exit(0)
	}

	if _, err := os.Stat(cfg.Makefile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, ErrorMakefileNotFound, cfg.Makefile)
		os.Exit(1)
	}

	vars := NewVariableStore()
	parser := NewParser(vars)

	makefile, err := parser.ParseFile(cfg.Makefile)
	if err != nil {
		fmt.Fprintf(os.Stderr, ErrorParsingMakefile, err)
		os.Exit(1)
	}

	target := cfg.Target
	if target == "" {
		if len(makefile.Rules) == 0 {
			fmt.Fprintln(os.Stderr, ErrorNoRulesNoTarget)
			os.Exit(1)
		}
		target = makefile.Rules[0].Targets[0]
		fmt.Printf(StatusUsingDefaultTarget, target)
	}

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
