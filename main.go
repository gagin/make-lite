package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const version = "1.5.0"
const defaultMakefile = "Makefile.mk-lite"

// generateHelpText creates the help message using the default makefile name.
func generateHelpText(makefile string) string {
	return fmt.Sprintf(`make-lite: A simple, sequential build tool.

Usage:
  go run main.go [target]

If no target is specified, the first target defined in %s is executed.

Flags:
  --help, -h       Show this help message.
  --version, -v    Show program version.
`, makefile)
}

type Rule struct {
	Targets      []string
	Dependencies []string
	Commands     []string
}

// getFileModTime returns the modification time of a file.
// It returns an error if the path is a directory or does not exist.
func getFileModTime(path string) (time.Time, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err // Path does not exist or other error.
	}
	if fileInfo.IsDir() {
		return time.Time{}, os.ErrNotExist // Treat directories as if they don't exist.
	}
	return fileInfo.ModTime(), nil
}

// stripComment removes a comment from a line, unless it's inside a shell command.
func stripComment(line string) string {
	inShell := false
	for i, r := range line {
		if i+8 <= len(line) && line[i:i+7] == "$(shell" {
			inShell = true
		}
		if inShell && r == ')' {
			inShell = false
		}
		if r == '#' && !inShell {
			return line[:i]
		}
	}
	return line
}

// expandShellCommands finds and executes $(shell ...) commands in a string.
func expandShellCommands(s string, vars map[string]string) string {
	for {
		startIndex := strings.Index(s, "$(shell ")
		if startIndex == -1 {
			break
		}

		commandStart := startIndex + len("$(shell ")
		parenCount := 1
		endIndex := -1

		// Robust parenthesis matching that is aware of escaped characters.
		i := commandStart
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				// Skip over the escaped character, whatever it is.
				i += 2
				continue
			}
			if s[i] == '(' {
				parenCount++
			} else if s[i] == ')' {
				parenCount--
				if parenCount == 0 {
					endIndex = i
					break
				}
			}
			i++
		}

		if endIndex == -1 {
			// Unmatched parenthesis, stop expanding.
			break
		}

		// Extract, expand variables inside, and execute the command.
		cmdStr := s[commandStart:endIndex]

		// The ONLY un-escaping we do is for parens, which are special to our parser.
		// The shell is responsible for handling all other escape sequences.
		cmdStr = strings.ReplaceAll(cmdStr, "\\(", "(")
		cmdStr = strings.ReplaceAll(cmdStr, "\\)", ")")

		expandedCmdStr := expandVariables(cmdStr, vars)

		cmd := exec.Command("bash", "-c", expandedCmdStr)
		out, err := cmd.Output()
		var result string
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing shell command '%s': %v\n", expandedCmdStr, err)
			result = "" // Return empty string on error.
		} else {
			result = strings.TrimSpace(string(out))
		}
		s = s[:startIndex] + result + s[endIndex+1:]
	}
	return s
}

// Regex for $(VAR) or $VAR. Allows letters, numbers, underscore, hyphen.
var varRegex = regexp.MustCompile(`\$\(([a-zA-Z0-9_-]+)\)|\$([a-zA-Z0-9_-]+)`)

// expandVariables replaces all variable references in a string with their values.
func expandVariables(s string, vars map[string]string) string {
	// First, expand any $(shell ...) commands.
	s = expandShellCommands(s, vars)

	// Then, expand regular variables like $VAR or $(VAR).
	return varRegex.ReplaceAllStringFunc(s, func(match string) string {
		submatches := varRegex.FindStringSubmatch(match)
		// One of the capture groups will have the name.
		varName := submatches[1]
		if varName == "" {
			varName = submatches[2]
		}

		if val, ok := vars[varName]; ok {
			return val
		}
		return "" // Return empty string if var not found, as per standard make behavior.
	})
}

func executeTarget(target string, rules map[string]*Rule, executedRules map[*Rule]bool, visiting map[string]bool, isDebug bool, makefileVars map[string]string, shellVars map[string]string) error {
	if visiting[target] {
		return fmt.Errorf("circular dependency detected: %s", target)
	}

	rule, ok := rules[target]
	if !ok {
		// If the target is not a rule, check if it's a file on disk.
		// getFileModTime correctly treats directories as an error.
		_, err := getFileModTime(target)
		if err == nil {
			// It's a plain file that exists and is not a rule. Nothing to do.
			return nil
		}
		if os.IsNotExist(err) {
			return fmt.Errorf("no rule to make target '%s'. Stop", target)
		}
		return fmt.Errorf("error checking target '%s': %v", target, err)
	}

	// If the rule that makes this target has already run, we're done.
	if executedRules[rule] {
		return nil
	}

	visiting[target] = true
	defer func() { delete(visiting, target) }()

	for _, dep := range rule.Dependencies {
		if err := executeTarget(dep, rules, executedRules, visiting, isDebug, makefileVars, shellVars); err != nil {
			return err
		}
	}

	// --- Freshness Check ---
	needsUpdate := false
	var earliestTargetModTime time.Time
	var latestDepModTime time.Time

	// Check if any target is missing. If so, update.
	for _, t := range rule.Targets {
		modTime, err := getFileModTime(t)
		if err != nil {
			needsUpdate = true // Target missing or is a directory.
			break
		}
		if earliestTargetModTime.IsZero() || modTime.Before(earliestTargetModTime) {
			earliestTargetModTime = modTime
		}
	}

	// If not updating yet, check if any dependency is newer than the oldest target.
	if !needsUpdate {
		for _, dep := range rule.Dependencies {
			modTime, err := getFileModTime(dep)
			if err != nil {
				needsUpdate = true
				break
			}
			if modTime.After(latestDepModTime) {
				latestDepModTime = modTime
			}
		}
		if !latestDepModTime.IsZero() && latestDepModTime.After(earliestTargetModTime) {
			needsUpdate = true
		}
	}
	if len(rule.Dependencies) == 0 && !needsUpdate {
	} else if len(rule.Dependencies) == 0 && needsUpdate {
	}

	if needsUpdate {
		if len(rule.Commands) > 0 && isDebug {
			fmt.Printf("Executing commands for target(s): %s\n", strings.Join(rule.Targets, " "))
		}
		for _, t := range rule.Targets {
			dir := filepath.Dir(t)
			if dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory for target '%s': %v", t, err)
				}
			}
		}

		// --- Deferred Variable Expansion ---
		// Build the final variable map with correct precedence for command execution.
		finalVars := make(map[string]string)
		for k, v := range makefileVars {
			finalVars[k] = v
		}
		for k, v := range shellVars {
			finalVars[k] = v // Shell variables have highest priority.
		}

		// Build environment for subprocess from the final variable map.
		var env []string
		for k, v := range finalVars {
			env = append(env, k+"="+v)
		}

		for _, rawCmdStr := range rule.Commands {
			// Expand variables at the last moment using the final, combined map.
			expandedCmd := expandVariables(rawCmdStr, finalVars)

			commandToRun := expandedCmd
			printCommand := true
			if strings.HasPrefix(commandToRun, "@") {
				commandToRun = strings.TrimPrefix(commandToRun, "@")
				printCommand = false
			}

			if printCommand {
				fmt.Println(commandToRun)
			}

			cmd := exec.Command("bash", "-c", commandToRun)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = env
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("command failed for rule producing '%s': %v", rule.Targets[0], err)
			}
		}
	} else {
		if isDebug {
			fmt.Printf("Target(s) '%s' are up to date.\n", strings.Join(rule.Targets, " "))
		}
	}

	executedRules[rule] = true
	return nil
}

func loadEnvFile(filename string, vars map[string]string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not load env file '%s': %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			if len(value) > 1 {
				if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
					(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
					value = value[1 : len(value)-1]
				}
			}
			vars[key] = value
		}
	}
	return scanner.Err()
}

func isRuleDefinition(line string) bool {
	return strings.Contains(line, ":") && !strings.Contains(line, ":=")
}

func main() {
	helpText := generateHelpText(defaultMakefile)
	isDebug := os.Getenv("LOG_LEVEL") == "DEBUG"

	shellVars := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			shellVars[pair[0]] = pair[1]
		}
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			fmt.Println(helpText)
			return
		case "--version", "-v":
			fmt.Printf("make-lite version %s\n", version)
			return
		}
	}

	targetToExecute := ""
	if len(os.Args) > 1 {
		targetToExecute = os.Args[1]
	}

	makefileVars := make(map[string]string)

	// --- Pre-processing Pass for includes and load_env ---
	var allLines []string
	var processFile func(string, map[string]bool) error
	processFile = func(filename string, visited map[string]bool) error {
		if visited[filename] {
			return fmt.Errorf("circular include detected: %s", filename)
		}
		visited[filename] = true

		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := scanner.Text()
			cleanLine := strings.TrimSpace(stripComment(line))

			if strings.HasPrefix(cleanLine, "include ") {
				includeFile := strings.TrimSpace(strings.TrimPrefix(cleanLine, "include "))
				if err := processFile(includeFile, visited); err != nil {
					return err
				}
			} else if strings.HasPrefix(cleanLine, "load_env ") {
				envFile := strings.TrimSpace(strings.TrimPrefix(cleanLine, "load_env "))
				if err := loadEnvFile(envFile, makefileVars); err != nil {
					if isDebug {
						fmt.Printf("Debug: %v\n", err)
					}
				}
			} else {
				allLines = append(allLines, line)
			}
		}
		delete(visited, filename)
		return nil
	}

	if err := processFile(defaultMakefile, make(map[string]bool)); err != nil {
		fmt.Fprintf(os.Stderr, "make-lite: %v\n", err)
		os.Exit(2)
	}

	// --- Two-Pass Parsing on combined content ---
	// Pass 1: Variables
	for _, line := range allLines {
		cleanLine := stripComment(line)
		if !isRuleDefinition(cleanLine) && strings.Contains(cleanLine, "=") {
			var key, value string
			if strings.Contains(cleanLine, "?=") {
				parts := strings.SplitN(cleanLine, "?=", 2)
				key = strings.TrimSpace(parts[0])
				// Important: for ?=, we must also check the shellVars for existence.
				if _, shellExists := shellVars[key]; !shellExists {
					if _, mfExists := makefileVars[key]; !mfExists {
						value = expandVariables(strings.TrimSpace(parts[1]), makefileVars)
						makefileVars[key] = value
					}
				}
			} else {
				parts := strings.SplitN(cleanLine, "=", 2)
				key = strings.TrimSpace(parts[0])
				value = expandVariables(strings.TrimSpace(parts[1]), makefileVars)
				makefileVars[key] = value
			}
		}
	}

	// Pass 2: Rules
	ruleMap := make(map[string]*Rule)
	var orderedTargets []string
	var currentRule *Rule

	finalizeRule := func() {
		if currentRule != nil {
			if len(orderedTargets) == 0 && len(currentRule.Targets) > 0 {
				orderedTargets = append(orderedTargets, currentRule.Targets[0])
			}
			for _, t := range currentRule.Targets {
				ruleMap[t] = currentRule
			}
		}
		currentRule = nil
	}

	for _, line := range allLines {
		cleanLine := strings.TrimSpace(stripComment(line))
		if cleanLine == "" {
			finalizeRule()
			continue
		}

		if isRuleDefinition(cleanLine) {
			finalizeRule() // Finalize previous rule before starting a new one.

			ruleLine := expandVariables(cleanLine, makefileVars)
			parts := strings.SplitN(ruleLine, ":", 2)
			targets := strings.Fields(parts[0])
			var deps []string
			if len(parts) > 1 {
				deps = strings.Fields(parts[1])
			}
			// Commands are NOT expanded here.
			currentRule = &Rule{Targets: targets, Dependencies: deps}
		} else if currentRule != nil {
			// This is a command for the current rule. Store it raw.
			currentRule.Commands = append(currentRule.Commands, strings.TrimLeft(line, " \t"))
		}
	}
	finalizeRule() // Finalize the last rule if the file doesn't end with a newline

	if targetToExecute == "" {
		if len(orderedTargets) > 0 {
			targetToExecute = orderedTargets[0]
		} else {
			fmt.Fprintln(os.Stderr, "make-lite: *** No targets. Stop.")
			os.Exit(2)
		}
	}

	executedRules := make(map[*Rule]bool)
	visiting := make(map[string]bool)
	if err := executeTarget(targetToExecute, ruleMap, executedRules, visiting, isDebug, makefileVars, shellVars); err != nil {
		fmt.Fprintf(os.Stderr, "make-lite: *** %v\n", err)
		os.Exit(1)
	}
}
