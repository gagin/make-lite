// cmd/make-lite/engine.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Engine orchestrates the build process.
type Engine struct {
	makefile  *Makefile
	vars      *VariableStore
	built     map[string]bool
	visiting  map[string]bool
	shellPath string
	isDebug   bool
}

// NewEngine creates a new build engine.
func NewEngine(mf *Makefile, vs *VariableStore, isDebug bool) (*Engine, error) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		return nil, fmt.Errorf("could not find 'sh' in PATH. 'make-lite' requires a POSIX-compliant shell")
	}
	return &Engine{
		makefile:  mf,
		vars:      vs,
		built:     make(map[string]bool),
		visiting:  make(map[string]bool),
		shellPath: shell,
		isDebug:   isDebug,
	}, nil
}

// Build is the main entry point to start building a target.
func (e *Engine) Build(targetName string) error {
	expandedTarget, err := e.vars.Expand(targetName)
	if err != nil {
		return fmt.Errorf("failed to expand target name '%s': %w", targetName, err)
	}
	return e.buildRecursive(expandedTarget)
}

// buildRecursive performs the core dependency resolution and execution.
func (e *Engine) buildRecursive(targetName string) error {
	if e.built[targetName] {
		return nil
	}
	if e.visiting[targetName] {
		return fmt.Errorf("circular dependency detected: target '%s' is a dependency of itself", targetName)
	}
	e.visiting[targetName] = true
	defer func() { delete(e.visiting, targetName) }()

	rule, exists := e.makefile.RuleMap[targetName]
	if !exists {
		info, err := os.Stat(targetName)
		if err == nil && !info.IsDir() {
			// If the file exists and has no rule, it's considered built.
			e.built[targetName] = true
			return nil
		}
		// If the file doesn't exist and there's no rule, it's an error.
		// If it's a directory without a rule, we also don't know how to "make" it.
		return fmt.Errorf("don't know how to make target '%s'", targetName)
	}

	for _, sourceName := range rule.Sources {
		expandedSource, err := e.vars.Expand(sourceName)
		if err != nil {
			return fmt.Errorf("failed to expand source name '%s' for target '%s': %w", sourceName, targetName, err)
		}
		// A single source can expand to a list of files.
		sourceFiles := strings.Fields(expandedSource)
		for _, sourceFile := range sourceFiles {
			if err := e.buildRecursive(sourceFile); err != nil {
				return err
			}
		}
	}

	needsRun, reason, err := e.checkFreshness(rule)
	if err != nil {
		return err
	}

	if needsRun {
		if e.isDebug {
			if reason == "" {
				fmt.Printf(StatusBuildingTarget, targetName)
			} else {
				fmt.Printf(StatusBuildingTargetBecause, targetName, reason)
			}
		}
		if err := e.executeRecipe(rule); err != nil {
			// New, more informative error format.
			return fmt.Errorf("recipe for target '%s' failed: %w", targetName, err)
		}
	} else {
		// Only show "up to date" messages in debug mode, and show them for the whole target group.
		if e.isDebug {
			targetList := strings.Join(rule.Targets, "', '")
			fmt.Printf(StatusTargetsUpToDate, targetList)
		}
	}

	for _, t := range rule.Targets {
		// Mark all targets of the rule as built after execution.
		// This prevents duplicate "up to date" messages for the same rule.
		expandedTarget, err := e.vars.Expand(t)
		if err != nil {
			return fmt.Errorf("failed to expand target name '%s': %w", t, err)
		}
		e.built[expandedTarget] = true
	}
	return nil
}

// checkFreshness determines if a rule's recipe needs to be executed per the PRD.
func (e *Engine) checkFreshness(rule *Rule) (bool, string, error) {
	var oldestTargetModTime time.Time
	var isPhony bool

	if len(rule.Targets) == 0 {
		return true, "it has no targets", nil // Should always run if there are no targets to check.
	}

	// 1. Check if any target is missing or is a directory (phony).
	for _, targetName := range rule.Targets {
		expandedTarget, err := e.vars.Expand(targetName)
		if err != nil {
			return false, "", err
		}
		info, err := os.Stat(expandedTarget)
		if err != nil {
			if os.IsNotExist(err) {
				return true, "", nil // Build quietly if target file is missing.
			}
			return false, "", fmt.Errorf("failed to stat target '%s': %w", expandedTarget, err)
		}
		if info.IsDir() {
			isPhony = true // Target is a directory, treat as phony.
			break
		}
		if oldestTargetModTime.IsZero() || info.ModTime().Before(oldestTargetModTime) {
			oldestTargetModTime = info.ModTime()
		}
	}

	if isPhony || (len(rule.Sources) == 0 && oldestTargetModTime.IsZero()) {
		// A directory target, or a target with no sources and no corresponding file, is phony.
		return true, "it is a symbolic target", nil
	}

	// 2. A rule with existing file targets but no sources is considered up-to-date.
	if len(rule.Sources) == 0 {
		return false, "", nil
	}

	// 3. Timestamp check: if ANY source is newer than the OLDEST target.
	for _, sourceName := range rule.Sources {
		expandedSource, err := e.vars.Expand(sourceName)
		if err != nil {
			return false, "", err
		}
		info, err := os.Stat(expandedSource)
		if err != nil {
			if os.IsNotExist(err) {
				// Fatal error: A dependency is missing and there's no rule to make it.
				return false, "", fmt.Errorf(ErrorMissingDependency, expandedSource, rule.Targets[0])
			}
			return false, "", err // Other stat error.
		}
		if info.ModTime().After(oldestTargetModTime) {
			return true, fmt.Sprintf("source '%s' is newer", expandedSource), nil
		}
	}

	return false, "", nil
}

// executeRecipe runs the commands for a given rule.
func (e *Engine) executeRecipe(rule *Rule) error {
	for _, targetName := range rule.Targets {
		expandedTarget, err := e.vars.Expand(targetName)
		if err != nil {
			return fmt.Errorf("failed to expand target name for directory creation '%s': %w", targetName, err)
		}
		dir := filepath.Dir(expandedTarget)
		if dir != "." && dir != "/" && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	for _, cmdLine := range rule.Recipe {
		if strings.TrimSpace(cmdLine) == "" {
			continue
		}

		commandToExecute := cmdLine
		suppressEcho := false
		if strings.HasPrefix(strings.TrimSpace(commandToExecute), "@") {
			suppressEcho = true
			atIndex := strings.Index(commandToExecute, "@")
			commandToExecute = commandToExecute[:atIndex] + commandToExecute[atIndex+1:]
		}

		expandedCmd, err := e.vars.Expand(commandToExecute)
		if err != nil {
			return fmt.Errorf("error expanding command '%s': %w", cmdLine, err)
		}

		if !suppressEcho {
			fmt.Println(expandedCmd)
		}

		if e.isDebug {
			fmt.Fprintf(os.Stderr, DebugExecutingCommand, expandedCmd)
		}

		cmd := exec.Command(e.shellPath, "-c", expandedCmd)
		cmd.Env = e.vars.getEnvironment()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			// Return the raw error from the shell command. The caller will add context.
			return err
		}
	}
	return nil
}
