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
	expandedTarget, err := e.vars.Expand(targetName, true)
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
			e.built[targetName] = true
			return nil
		}
		return fmt.Errorf("don't know how to make target '%s'", targetName)
	}

	for _, sourceName := range rule.Sources {
		// sourceName is already expanded by the parser
		sourceFiles := strings.Fields(sourceName)
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
			return fmt.Errorf("recipe for target '%s' failed: %w", targetName, err)
		}
	} else {
		if e.isDebug {
			targetList := strings.Join(rule.Targets, "', '")
			fmt.Printf(StatusTargetsUpToDate, targetList)
		}
	}

	for _, t := range rule.Targets {
		e.built[t] = true
	}
	return nil
}

// checkFreshness determines if a rule's recipe needs to be executed per the PRD.
func (e *Engine) checkFreshness(rule *Rule) (bool, string, error) {
	var oldestTargetModTime time.Time
	var isPhony bool

	if len(rule.Targets) == 0 {
		return true, "it has no targets", nil
	}

	for _, targetName := range rule.Targets {
		// targetName is already expanded by parser
		info, err := os.Stat(targetName)
		if err != nil {
			if os.IsNotExist(err) {
				return true, "", nil
			}
			return false, "", fmt.Errorf("failed to stat target '%s': %w", targetName, err)
		}
		if info.IsDir() {
			isPhony = true
			break
		}
		if oldestTargetModTime.IsZero() || info.ModTime().Before(oldestTargetModTime) {
			oldestTargetModTime = info.ModTime()
		}
	}

	if isPhony || (len(rule.Sources) == 0 && oldestTargetModTime.IsZero()) {
		return true, "it is a symbolic target", nil
	}

	if len(rule.Sources) == 0 {
		return false, "", nil
	}

	for _, sourceName := range rule.Sources {
		// sourceName is already expanded by parser
		info, err := os.Stat(sourceName)
		if err != nil {
			if os.IsNotExist(err) {
				// Check if the missing "file" is actually another rule target (a phony dependency).
				if _, isRule := e.makefile.RuleMap[sourceName]; isRule {
					// It's a phony dependency. It has already been run.
					// It does not influence the freshness of the current file-based target.
					// So we just continue to the next source.
					continue
				}
				// Otherwise, it's a genuine missing file dependency.
				return false, "", fmt.Errorf(ErrorMissingDependency, sourceName, rule.Targets[0])
			}
			return false, "", err
		}
		if info.ModTime().After(oldestTargetModTime) {
			return true, fmt.Sprintf("source '%s' is newer", sourceName), nil
		}
	}

	return false, "", nil
}

// executeRecipe runs the commands for a given rule.
func (e *Engine) executeRecipe(rule *Rule) error {
	for _, targetName := range rule.Targets {
		// targetName is already expanded
		dir := filepath.Dir(targetName)
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

		expandedCmd, err := e.vars.Expand(commandToExecute, false)
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
			return err
		}
	}
	return nil
}
