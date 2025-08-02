// cmd/make-lite/parser.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Parser is responsible for reading and parsing makefiles.
type Parser struct {
	variableStore *VariableStore
	includeStack  map[string]bool // For detecting circular includes
}

// NewParser creates a new parser instance.
func NewParser(vs *VariableStore) *Parser {
	return &Parser{
		variableStore: vs,
		includeStack:  make(map[string]bool),
	}
}

// ParseFile is the main entry point for parsing. It reads the root makefile and returns a structured Makefile object.
func (p *Parser) ParseFile(filename string) (*Makefile, error) {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("could not determine absolute path for %s: %w", filename, err)
	}

	processedLines, err := p.processFile(absPath)
	if err != nil {
		return nil, err
	}

	fullContent := p.joinContinuations(processedLines)
	return p.parseContent(fullContent)
}

// processFile handles comment removal and file inclusion.
func (p *Parser) processFile(absPath string) (lines []string, err error) {
	if p.includeStack[absPath] {
		return nil, fmt.Errorf("circular include detected: %s", absPath)
	}
	p.includeStack[absPath] = true
	defer func() { delete(p.includeStack, absPath) }()

	file, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) && strings.HasSuffix(absPath, ".env") {
			return nil, nil // Silently ignore missing .env files
		}
		return nil, fmt.Errorf("could not open makefile %s: %w", absPath, err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	var outputLines []string
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Comment Removal (respecting escapes)
		var contentPart strings.Builder
		var commentPart strings.Builder
		inComment := false
		isEscaped := false
		for _, r := range line {
			if isEscaped {
				if inComment {
					commentPart.WriteRune(r)
				} else {
					contentPart.WriteRune(r)
				}
				isEscaped = false
				continue
			}
			if r == '\\' {
				isEscaped = true
				if inComment {
					commentPart.WriteRune(r)
				} else {
					contentPart.WriteRune(r)
				}
				continue
			}
			if r == '#' {
				inComment = true
			}
			if inComment {
				commentPart.WriteRune(r)
			} else {
				contentPart.WriteRune(r)
			}
		}

		if strings.HasSuffix(strings.TrimSpace(commentPart.String()), `\`) {
			return nil, fmt.Errorf("ambiguous line continuation in comment at %s:%d", absPath, lineNumber)
		}
		line = contentPart.String()

		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "include ") {
			includePathStr := strings.TrimSpace(trimmedLine[len("include"):])
			includePathStr = trimQuotes(includePathStr)
			if includePathStr == "" {
				return nil, fmt.Errorf("empty include path at %s:%d", absPath, lineNumber)
			}
			includePath := filepath.Join(filepath.Dir(absPath), includePathStr)
			includedLines, err := p.processFile(includePath)
			if err != nil {
				return nil, fmt.Errorf("error in included file %s (from %s:%d): %w", includePathStr, absPath, lineNumber, err)
			}
			outputLines = append(outputLines, includedLines...)
		} else {
			outputLines = append(outputLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading makefile %s: %w", absPath, err)
	}

	return outputLines, nil
}

// splitOnUnescaped splits a string by a separator, honoring backslash escapes.
func splitOnUnescaped(s string, sep rune) (string, string, bool) {
	isEscaped := false
	for i, r := range s {
		if isEscaped {
			isEscaped = false
			continue
		}
		if r == '\\' {
			isEscaped = true
			continue
		}
		if r == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// joinContinuations processes lines, joining those ending in an unescaped backslash.
func (p *Parser) joinContinuations(lines []string) string {
	var builder strings.Builder
	for i, line := range lines {
		trimmedLine := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmedLine, `\`) && !strings.HasSuffix(trimmedLine, `\\`) {
			builder.WriteString(trimmedLine[:len(trimmedLine)-1])
		} else {
			builder.WriteString(line)
			if i < len(lines)-1 {
				builder.WriteByte('\n')
			}
		}
	}
	return builder.String()
}

// parseContent performs the final parse of the fully processed string buffer.
func (p *Parser) parseContent(content string) (*Makefile, error) {
	makefile := NewMakefile()
	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" {
			continue
		}

		isIndented := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
		if isIndented {
			return nil, fmt.Errorf("invalid line %d: unexpected indented line, must follow a rule definition: \"%s\"", i+1, trimmedLine)
		}

		if left, right, ok := splitOnUnescaped(trimmedLine, ':'); ok && !strings.Contains(left, "=") {
			if _, _, hasMulti := splitOnUnescaped(right, ':'); hasMulti {
				return nil, fmt.Errorf("invalid rule with multiple colons on line %d: \"%s\"", i+1, trimmedLine)
			}

			expandedLeft, err := p.variableStore.Expand(left, true)
			if err != nil {
				return nil, fmt.Errorf("on line %d: error expanding targets: %w", i+1, err)
			}
			expandedRight, err := p.variableStore.Expand(right, true)
			if err != nil {
				return nil, fmt.Errorf("on line %d: error expanding sources: %w", i+1, err)
			}

			targets := strings.Fields(expandedLeft)
			sources := strings.Fields(expandedRight)
			if len(targets) == 0 {
				return nil, fmt.Errorf("rule with no target on line %d: \"%s\"", i+1, trimmedLine)
			}

			rule := &Rule{Targets: targets, Sources: sources, Recipe: []string{}, Origin: fmt.Sprintf("line %d", i+1)}

			j := i + 1
			for ; j < len(lines); j++ {
				recipeLine := lines[j]
				if strings.TrimSpace(recipeLine) == "" {
					rule.Recipe = append(rule.Recipe, recipeLine)
					continue
				}
				recipeIsIndented := len(recipeLine) > 0 && (recipeLine[0] == ' ' || recipeLine[0] == '\t')
				if !recipeIsIndented {
					break
				}
				rule.Recipe = append(rule.Recipe, recipeLine)
			}
			i = j - 1
			makefile.AddRule(rule)

		} else if left, right, ok := splitOnUnescaped(trimmedLine, '='); ok {
			op := "="
			if strings.HasSuffix(strings.TrimSpace(left), "?") {
				op = "?="
				left = strings.TrimSpace(left[:len(left)-1])
			}
			keyPart := strings.TrimSpace(left)
			keyTokens := strings.Fields(keyPart)
			if len(keyTokens) == 0 {
				return nil, fmt.Errorf("invalid assignment with no variable name on line %d: \"%s\"", i+1, trimmedLine)
			}
			varName := keyTokens[len(keyTokens)-1]

			value, err := p.variableStore.Expand(strings.TrimSpace(right), true)
			if err != nil {
				return nil, fmt.Errorf("on line %d: error expanding variable value: %w", i+1, err)
			}

			source := sourceMakefileUnconditional
			if op == "?=" {
				source = sourceMakefileConditional
			}
			p.variableStore.Set(varName, value, source)

		} else if strings.HasPrefix(trimmedLine, "load_env ") {
			envPath := strings.TrimSpace(trimmedLine[len("load_env"):])
			envPath = trimQuotes(envPath)
			if err := p.loadEnvFile(envPath); err != nil {
				return nil, fmt.Errorf("on line %d: %w", i+1, err)
			}
		} else {
			return nil, fmt.Errorf("invalid line %d: not a rule, assignment, or directive: \"%s\"", i+1, trimmedLine)
		}
	}
	return makefile, nil
}

// loadEnvFile reads a .env file and populates the variable store.
func (p *Parser) loadEnvFile(filename string) (err error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Silently ignore missing .env files
		}
		return fmt.Errorf("could not load env file %s: %w", filename, err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, val, ok := cleanEnvLine(scanner.Text())
		if ok {
			p.variableStore.Set(key, val, sourceEnvFile)
		}
	}
	return scanner.Err()
}
