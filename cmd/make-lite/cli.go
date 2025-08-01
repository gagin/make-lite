package main

import (
	"flag"
	"fmt"
)

// Config holds the final configuration determined from CLI flags and arguments.
type Config struct {
	Makefile string
	Target   string
	ShowHelp bool
	ShowVer  bool
}

// ParseCLI parses command-line arguments and returns a Config struct.
func ParseCLI() *Config {
	cfg := &Config{}

	flag.BoolVar(&cfg.ShowHelp, "h", false, "Display help message.")
	flag.BoolVar(&cfg.ShowHelp, "help", false, "Display help message.")
	flag.BoolVar(&cfg.ShowVer, "v", false, "Display program version.")
	flag.BoolVar(&cfg.ShowVer, "version", false, "Display program version.")

	flag.Usage = printHelp
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 {
		cfg.Target = args[0]
	}

	cfg.Makefile = DefaultMakefile

	return cfg
}

func printHelp() {
	fmt.Print(HelpUsage)
	fmt.Println(HelpDescription)
	fmt.Println(HelpOptionsHeader)
	flag.PrintDefaults()
}

func printVersion() {
	fmt.Printf(VersionFormat, AppVersion)
}
