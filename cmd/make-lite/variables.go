// cmd/make-lite/variables.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

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

type varSource int

const (
	sourceMakefileConditional varSource = iota
	sourceEnvFile
	sourceShellEnv
	sourceMakefileUnconditional
)

type varEntry struct {
	value  string
	source varSource
}

type VariableStore struct {
	vars              map[string]varEntry
	isDebug           bool
	isExpandingForEnv bool // Flag to prevent shell recursion
}

func NewVariableStore(isDebug bool) *VariableStore {
	vs := &VariableStore{
		vars:    make(map[string]varEntry),
		isDebug: isDebug,
	}
	for _, envPair := range os.Environ() {
		parts := strings.SplitN(envPair, "=", 2)
		if len(parts) == 2 && parts[0] != "" {
			vs.vars[parts[0]] = varEntry{value: parts[1], source: sourceShellEnv}
		}
	}
	return vs
}

func (vs *VariableStore) Set(key, value string, source varSource) {
	existing, exists := vs.vars[key]

	if source == sourceMakefileConditional {
		if !exists {
			vs.vars[key] = varEntry{value: value, source: source}
		}
		return
	}

	if !exists || source >= existing.source {
		vs.vars[key] = varEntry{value: value, source: source}
	}
}

func (vs *VariableStore) Get(key string) (string, bool) {
	entry, ok := vs.vars[key]
	if !ok {
		return "", false
	}
	return entry.value, true
}

func (vs *VariableStore) runShellCmd(command string) (string, error) {
	if vs.isExpandingForEnv {
		return "", nil
	}
	if vs.isDebug {
		fmt.Fprintf(os.Stderr, DebugShellCommand, command)
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = vs.getEnvironment()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if vs.isDebug {
		if stdout.Len() > 0 {
			fmt.Fprintf(os.Stderr, DebugShellStdout, strings.TrimRight(stdout.String(), "\n\r"))
		}
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, DebugShellStderr, strings.TrimRight(stderr.String(), "\n\r"))
		}
	}
	if err != nil {
		return "", fmt.Errorf("shell command '%s' failed: %w\nstderr: %s", command, err, stderr.String())
	}
	return strings.TrimRight(stdout.String(), "\n\r"), nil
}

func (vs *VariableStore) expand(input string, visiting map[string]bool) (string, error) {
	var result strings.Builder
	i := 0
	for i < len(input) {
		nextSpecial := -1
		nextDollar := strings.Index(input[i:], "$")
		nextBackslash := strings.Index(input[i:], "\\")

		if nextDollar != -1 && (nextBackslash == -1 || nextDollar < nextBackslash) {
			nextSpecial = nextDollar
		} else {
			nextSpecial = nextBackslash
		}

		if nextSpecial == -1 {
			result.WriteString(input[i:])
			break
		}

		result.WriteString(input[i : i+nextSpecial])
		i += nextSpecial

		char := input[i]
		if char == '\\' {
			if i+1 < len(input) {
				result.WriteByte(input[i+1])
				i += 2
			} else {
				result.WriteByte('\\')
				i++
			}
			continue
		}

		if i+1 >= len(input) {
			result.WriteByte('$')
			i++
			continue
		}
		switch input[i+1] {
		case '$':
			result.WriteByte('$')
			i += 2
		case '(':
			start := i + 2
			balance := 1
			end := -1
			for j := start; j < len(input); j++ {
				if input[j] == '(' {
					balance++
				} else if input[j] == ')' {
					balance--
					if balance == 0 {
						end = j
						break
					}
				}
			}
			if end == -1 {
				return "", fmt.Errorf("unmatched parenthesis in variable expression: %s", input[i:])
			}
			content := input[start:end]
			i = end + 1

			var expandedContent string
			var err error

			functionName := strings.SplitN(content, " ", 2)[0]
			if _, isUnsupported := unsupportedMakeFunctions[functionName]; isUnsupported {
				return "", fmt.Errorf(ErrorUnsupportedFunction, functionName)
			}

			if strings.HasPrefix(content, "shell ") {
				cmdStr := strings.TrimSpace(content[len("shell"):])
				expandedCmd, err_expand := vs.expand(cmdStr, visiting)
				if err_expand != nil {
					return "", fmt.Errorf("error expanding shell command: %w", err_expand)
				}
				expandedContent, err = vs.runShellCmd(expandedCmd)
			} else if val, ok := vs.Get(content); ok {
				varName := content
				if visiting[varName] {
					return "", fmt.Errorf("circular variable reference detected for '%s'", varName)
				}
				visiting[varName] = true
				expandedContent, err = vs.expand(val, visiting)
				delete(visiting, varName)
			} else {
				expandedCmd, err_expand := vs.expand(content, visiting)
				if err_expand != nil {
					return "", fmt.Errorf("error expanding implicit shell command: %w", err_expand)
				}
				expandedContent, err = vs.runShellCmd(expandedCmd)
			}

			if err != nil {
				return "", err
			}
			result.WriteString(expandedContent)
		default:
			re := regexp.MustCompile(`^[a-zA-Z0-9_]+`)
			varName := re.FindString(input[i+1:])
			if varName == "" {
				result.WriteByte('$')
				i++
				continue
			}
			i += 1 + len(varName)
			if visiting[varName] {
				return "", fmt.Errorf("circular variable reference detected for '%s'", varName)
			}
			visiting[varName] = true
			val, ok := vs.Get(varName)
			if ok {
				expandedVal, err := vs.expand(val, visiting)
				if err != nil {
					return "", err
				}
				result.WriteString(expandedVal)
			}
			delete(visiting, varName)
		}
	}
	return result.String(), nil
}

func (vs *VariableStore) Expand(input string) (string, error) {
	return vs.expand(input, make(map[string]bool))
}

func (vs *VariableStore) getEnvironment() []string {
	if vs.isExpandingForEnv {
		return os.Environ()
	}
	vs.isExpandingForEnv = true
	defer func() { vs.isExpandingForEnv = false }()
	envMap := make(map[string]string)
	for _, pair := range os.Environ() {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	for key, varEntry := range vs.vars {
		if varEntry.source != sourceShellEnv {
			expandedVal, err := vs.Expand(varEntry.value)
			if err != nil {
				if vs.isDebug {
					fmt.Fprintf(os.Stderr, "DEBUG: error expanding env var '%s', using raw value: %v\n", key, err)
				}
				envMap[key] = varEntry.value
			} else {
				envMap[key] = expandedVal
			}
		}
	}
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
