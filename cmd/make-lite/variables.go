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

type varSource int

const (
	sourceMakefileConditional varSource = iota
	sourceEnvFile
	sourceShellEnv
	sourceMakefileUnconditional
)

type varEntry struct {
	value      string
	source     varSource
	originFile string
	originLine int
}

type VariableStore struct {
	vars              map[string]varEntry
	isDebug           bool
	isExpandingForEnv bool // Flag to prevent shell recursion
	cachedEnv         []string
}

func NewVariableStore(isDebug bool) *VariableStore {
	vs := &VariableStore{
		vars:    make(map[string]varEntry),
		isDebug: isDebug,
	}
	for _, envPair := range os.Environ() {
		parts := strings.SplitN(envPair, "=", 2)
		if len(parts) == 2 && parts[0] != "" {
			vs.vars[parts[0]] = varEntry{value: parts[1], source: sourceShellEnv, originFile: "shell environment", originLine: 0}
		}
	}
	return vs
}

func (vs *VariableStore) Set(key, value string, source varSource, originFile string, originLine int) {
	vs.cachedEnv = nil // Invalidate env cache on any variable change.
	existing, exists := vs.vars[key]

	if source == sourceMakefileConditional {
		if !exists {
			vs.vars[key] = varEntry{value: value, source: source, originFile: originFile, originLine: originLine}
		}
		return
	}

	if !exists || source >= existing.source {
		// This is the "action at a distance" case: an unconditional assignment
		// in a makefile (`=`) is overwriting a previous one from a makefile.
		if exists && source == sourceMakefileUnconditional && existing.source == sourceMakefileUnconditional {
			fmt.Fprintf(os.Stderr, WarningVarRedefined, key, originFile, originLine, existing.originFile, existing.originLine)
		}
		vs.vars[key] = varEntry{value: value, source: source, originFile: originFile, originLine: originLine}
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

func (vs *VariableStore) expand(input string, unescape bool, visiting map[string]bool) (string, error) {
	var result strings.Builder
	i := 0
	for i < len(input) {
		char := input[i]
		if unescape && char == '\\' {
			if i+1 < len(input) {
				result.WriteByte(input[i+1])
				i += 2
			} else {
				result.WriteByte('\\')
				i++
			}
			continue
		}

		if char == '$' {
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

				expandedContent, err := vs.expand(content, true, visiting)
				if err != nil {
					return "", err
				}

				var finalValue string
				functionName := strings.SplitN(expandedContent, " ", 2)[0]
				if _, isUnsupported := unsupportedMakeFunctions[functionName]; isUnsupported {
					return "", fmt.Errorf(ErrorUnsupportedFunction, functionName)
				}

				if strings.HasPrefix(expandedContent, "shell ") {
					cmdStr := strings.TrimSpace(expandedContent[len("shell"):])
					finalValue, err = vs.runShellCmd(cmdStr)
				} else if val, ok := vs.Get(expandedContent); ok {
					finalValue = val
				} else {
					finalValue, err = vs.runShellCmd(expandedContent)
				}

				if err != nil {
					return "", err
				}
				result.WriteString(finalValue)
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
				if val, ok := vs.Get(varName); ok {
					result.WriteString(val)
				}
			}
		} else {
			result.WriteByte(char)
			i++
		}
	}
	return result.String(), nil
}

func (vs *VariableStore) Expand(input string, unescape bool) (string, error) {
	return vs.expand(input, unescape, make(map[string]bool))
}

func (vs *VariableStore) getEnvironment() []string {
	if vs.cachedEnv != nil {
		return vs.cachedEnv
	}
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
			envMap[key] = varEntry.value
		}
	}
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	vs.cachedEnv = env
	return env
}
