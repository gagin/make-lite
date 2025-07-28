package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Rule struct {
	Target       string
	Dependencies []string
	Commands     []string
}

func getFileModTime(path string) (time.Time, error) {
	file, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return file.ModTime(), nil
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx != -1 {
		return line[:idx]
	}
	return line
}

var varRegex = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)
var shellRegex = regexp.MustCompile(`\$\(shell\s+([^)]+)\)`) // Matches $(shell command)

func expandVariables(s string, vars map[string]string) string {
	// First, expand shell commands
	for shellRegex.MatchString(s) {
		s = shellRegex.ReplaceAllStringFunc(s, func(match string) string {
			cmdStr := strings.TrimSpace(shellRegex.FindStringSubmatch(match)[1])
			cmd := exec.Command("bash", "-c", cmdStr)
			out, err := cmd.Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error executing shell command '%s': %v\n", cmdStr, err)
				return "" // Return empty string on error
			}
			return strings.TrimSpace(string(out))
		})
	}

	// Then, expand regular variables
	return varRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[1:] // Remove the leading $
		if val, ok := vars[varName]; ok {
			return val
		}
		return match // Return original if not found
	})
}

func executeTarget(target string, rules map[string]Rule, executed map[string]bool, visiting map[string]bool) error {
	if visiting[target] {
		return fmt.Errorf("circular dependency detected: %s", target)
	}
	visiting[target] = true
	defer func() { delete(visiting, target) }()

	if executed[target] {
		return nil
	}

	rule, ok := rules[target]
	if !ok {
		// If the target is a file and not a rule, we check if it exists.
		_, err := os.Stat(target)
		if os.IsNotExist(err) {
			return fmt.Errorf("don't know how to make target '%s'", target)
		}
		return nil
	}

	for _, dep := range rule.Dependencies {
		if err := executeTarget(dep, rules, executed, visiting); err != nil {
			return err
		}
	}

	targetModTime, err := getFileModTime(target)
	needsUpdate := false
	if os.IsNotExist(err) {
		needsUpdate = true // Target doesn't exist, must run commands
	} else if err != nil {
		return fmt.Errorf("error checking target '%s': %v", target, err)
	}

	for _, dep := range rule.Dependencies {
		depModTime, err := getFileModTime(dep)
		if err != nil {
			// If a dependency doesn't exist, it might be another rule. We assume it was handled by the recursive call.
			// If it's a file that truly doesn't exist, we should probably fail.
			_, isRule := rules[dep]
			if !isRule {
				return fmt.Errorf("error checking dependency '%s': %v", dep, err)
			}
			// If the dependency is a rule, we assume it was just built, so we force an update.
			needsUpdate = true
			continue
		}

		if targetModTime.Before(depModTime) {
			needsUpdate = true
			break
		}
	}

	if needsUpdate {
		fmt.Printf("Executing commands for target: %s\n", target)
		for _, cmdStr := range rule.Commands {
			fmt.Println(cmdStr)
			cmd := exec.Command("bash", "-c", cmdStr)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("command failed for target '%s': %v", target, err)
			}
		}
	} else {
		fmt.Printf("Target '%s' is up to date.\n", target)
	}

	executed[target] = true
	return nil
}

func readMakefile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func combineMakefiles(filename string, visited map[string]bool) (string, error) {
	if visited[filename] {
		return "", fmt.Errorf("circular include detected: %s", filename)
	}
	visited[filename] = true

	fileContent, err := readMakefile(filename)
	if err != nil {
		return "", err
	}

	var combinedContent strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(fileContent))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "include ") {
			includedFile := strings.TrimSpace(strings.TrimPrefix(line, "include "))
			// For simplicity, assume included files are in the same directory
			includedContent, err := combineMakefiles(includedFile, visited)
			if err != nil {
				return "", err
			}
			combinedContent.WriteString(includedContent)
			combinedContent.WriteString("\n")
		} else {
			combinedContent.WriteString(line)
			combinedContent.WriteString("\n")
		}
	}

	return combinedContent.String(), nil
}

func main() {
	const defaultMakefile = "Makefile-lite"

	targetToExecute := ""
	if len(os.Args) > 1 {
		targetToExecute = os.Args[1]
	}

	combinedMakefileContent, err := combineMakefiles(defaultMakefile, make(map[string]bool))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error combining makefiles: %v\n", err)
		os.Exit(1)
	}

	variables := make(map[string]string)

	// Load environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			variables[parts[0]] = parts[1]
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(combinedMakefileContent))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var rules []Rule
	var currentBlock []string

	for _, line := range lines {
		line = stripComment(line)
		trimmedLine := strings.TrimSpace(line)

		if strings.Contains(trimmedLine, "=") && !strings.Contains(trimmedLine, ":") {
			parts := strings.SplitN(trimmedLine, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if strings.HasSuffix(key, "?=") {
				key = strings.TrimSpace(strings.TrimSuffix(key, "?="))
				if _, exists := variables[key]; !exists {
					variables[key] = value
				}
			} else {
				variables[key] = value
			}
		} else if trimmedLine == "" {
			if len(currentBlock) > 0 {
				rules = append(rules, parseRule(currentBlock, variables))
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, line)
		}
	}

	if len(currentBlock) > 0 {
		rules = append(rules, parseRule(currentBlock, variables))
	}

	ruleMap := make(map[string]Rule)
	var orderedTargets []string
	for _, rule := range rules {
		if _, exists := ruleMap[rule.Target]; !exists {
			orderedTargets = append(orderedTargets, rule.Target)
		}
		ruleMap[rule.Target] = rule
	}

	if targetToExecute != "" {
		executed := make(map[string]bool)
		visiting := make(map[string]bool)
		if err := executeTarget(targetToExecute, ruleMap, executed, visiting); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("No target specified and no default target found.")
	}
}

func parseRule(block []string, variables map[string]string) Rule {
	ruleLine := expandVariables(strings.TrimSpace(block[0]), variables)
	parts := strings.SplitN(ruleLine, ":", 2)
	target := strings.TrimSpace(parts[0])
	var deps []string
	if len(parts) == 2 {
		deps = strings.Fields(expandVariables(strings.TrimSpace(parts[1]), variables))
	}
	commands := []string{}
	if len(block) > 1 {
		for _, cmdLine := range block[1:] {
			commands = append(commands, expandVariables(strings.TrimSpace(cmdLine), variables))
		}
	}
	return Rule{Target: target, Dependencies: deps, Commands: commands}
}