package main

import (
	"fmt"
	"os"
)

func main() {
	// 1. Parse CLI arguments
	cfg := ParseArgs(os.Args)
	if cfg.ShowHelp {
		fmt.Println(generateHelpText(defaultMakefile))
		return
	}
	if cfg.ShowVersion {
		fmt.Printf("make-lite version %s\n", version)
		return
	}

	// 2. Parse the makefile(s) into rules and variables
	makefileVars, rules, orderedTargets, err := ParseMakefile(defaultMakefile, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "make-lite: %v\n", err)
		os.Exit(2)
	}

	// Determine the target to execute
	targetToExecute := cfg.Target
	if targetToExecute == "" {
		if len(orderedTargets) > 0 {
			targetToExecute = orderedTargets[0]
		} else {
			fmt.Fprintln(os.Stderr, "make-lite: *** No targets. Stop.")
			os.Exit(2)
		}
	}

	// 3. Execute the target
	if err := Execute(targetToExecute, rules, makefileVars, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "make-lite: *** %v\n", err)
		os.Exit(1)
	}
}
