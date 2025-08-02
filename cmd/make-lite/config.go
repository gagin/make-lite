package main

// --- Application Metadata ---
var AppVersion = "1.0.0"

const DefaultMakefile = "Makefile.mk-lite"

// --- CLI UI Strings ---
const (
	HelpUsage         = "Usage: make-lite [options] [target]\n\n"
	HelpDescription   = "A simple, predictable build tool inspired by Make."
	HelpOptionsHeader = "\nOptions:"
	VersionFormat     = "make-lite version %s\n"
)

// --- Main Application Flow Messages ---
const (
	ErrorMakefileNotFound    = "Error: Makefile '%s' not found.\n"
	ErrorParsingMakefile     = "Error parsing makefile: %v\n"
	ErrorNoRulesNoTarget     = "Error: No rules found in makefile and no target specified."
	ErrorInitEngine          = "Error initializing build engine: %v\n"
	ErrorBuildFailed         = "Build failed: %v\n"
	StatusUsingDefaultTarget = "make-lite: No target specified, using default target '%s'.\n"
	StatusBuildSuccess       = "make-lite: Build finished successfully."
	ErrorMissingDependency   = "Dependency '%s' not found for target '%s', and no rule available to create it."
	ErrorUnsupportedFunction = "GNU Make function '$(%s ...)' is not supported."
)

// --- Engine Status Messages ---
const (
	StatusBuildingTarget        = "make-lite: Building target '%s'.\n"
	StatusBuildingTargetBecause = "make-lite: Building target '%s' because %s.\n"
	StatusTargetsUpToDate       = "make-lite: Targets '%s' are up to date.\n"
	DebugExecutingCommand       = "DEBUG: executing recipe command: [%s]\n"
	DebugShellCommand           = "DEBUG: executing shell command: [%s]\n"
	DebugShellStdout            = "DEBUG: shell stdout: [%s]\n"
	DebugShellStderr            = "DEBUG: shell stderr: [%s]\n"
)

// --- Parser Configuration ---

// unsupportedMakeFunctions is a set of common GNU Make functions that make-lite
// explicitly does not support. Attempting to use them will result in an error.
var unsupportedMakeFunctions = map[string]struct{}{
	"subst":      {},
	"patsubst":   {},
	"strip":      {},
	"findstring": {},
	"filter":     {},
	"filter-out": {},
	"sort":       {},
	"word":       {},
	"words":      {},
	"wordlist":   {},
	"firstword":  {},
	"lastword":   {},
	"dir":        {},
	"notdir":     {},
	"suffix":     {},
	"basename":   {},
	"addsuffix":  {},
	"addprefix":  {},
	"join":       {},
	"foreach":    {},
	"if":         {},
	"or":         {},
	"and":        {},
	"call":       {},
	"origin":     {},
	"value":      {},
	"info":       {},
	"warning":    {},
	"error":      {},
}
