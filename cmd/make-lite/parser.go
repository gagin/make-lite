// cmd/make-lite/parser.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// processedLine holds a line of content along with its original location.
type processedLine struct {
	content    string
	originFile string
	originLine int
}

// rawRule holds an unexpanded rule definition, collected during the first pass.
type rawRule struct {
	definitionLine string
	recipeLines    []string
	originFile     string
	originLine     int
}

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

	// This now returns lines with their origin info preserved.
	processedLines, err := p.processFile(absPath)
	if err != nil {
		return nil, err
	}

	// joinContinuations now also preserves origin info.
	finalLines := p.joinContinuations(processedLines)
	return p.parseContent(finalLines)
}

// processFile handles comment removal and file inclusion, returning lines with origin info.
func (p *Parser) processFile(absPath string) (lines []processedLine, err error) {
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

	var outputLines []processedLine
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		lineContent := scanner.Text()

		var contentPart strings.Builder
		var commentPart strings.Builder
		inComment := false
		isEscaped := false
		for _, r := range lineContent {
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
		lineContent = contentPart.String()

		trimmedLine := strings.TrimSpace(lineContent)
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
			outputLines = append(outputLines, processedLine{
				content:    lineContent,
				originFile: absPath,
				originLine: lineNumber,
			})
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
// It preserves the origin of the first line in a continuation sequence.
func (p *Parser) joinContinuations(lines []processedLine) []processedLine {
	if len(lines) == 0 {
		return nil
	}
	var result []processedLine
	current := lines[0]
	var builder strings.Builder
	builder.WriteString(current.content)

	for i := 1; i < len(lines); i++ {
		trimmedContent := strings.TrimRight(builder.String(), " \t")
		if strings.HasSuffix(trimmedContent, `\`) && !strings.HasSuffix(trimmedContent, `\\`) {
			builder.Reset()
			builder.WriteString(trimmedContent[:len(trimmedContent)-1])
			builder.WriteString(lines[i].content)
			current.content = builder.String()
		} else {
			result = append(result, current)
			current = lines[i]
			builder.Reset()
			builder.WriteString(current.content)
		}
	}
	result = append(result, current)
	return result
}

// parseContent performs the two-pass parse.
func (p *Parser) parseContent(lines []processedLine) (*Makefile, error) {
	// --- Pass 1: Populate VariableStore and collect raw, unexpanded rules ---
	rawRules, err := p.collectVarsAndRawRules(lines)
	if err != nil {
		return nil, err
	}

	// --- Pass 2: Parse the collected raw rules using the now-complete VariableStore ---
	makefile := NewMakefile()
	for _, raw := range rawRules {
		left, right, _ := splitOnUnescaped(raw.definitionLine, ':')

		expandedLeft, err := p.variableStore.Expand(left, true)
		if err != nil {
			return nil, fmt.Errorf("at %s:%d: error expanding targets: %w", raw.originFile, raw.originLine, err)
		}
		expandedRight, err := p.variableStore.Expand(right, true)
		if err != nil {
			return nil, fmt.Errorf("at %s:%d: error expanding sources: %w", raw.originFile, raw.originLine, err)
		}

		targets := strings.Fields(expandedLeft)
		sources := strings.Fields(expandedRight)
		if len(targets) == 0 {
			return nil, fmt.Errorf("at %s:%d: rule with no target: \"%s\"", raw.originFile, raw.originLine, raw.definitionLine)
		}

		rule := &Rule{
			Targets: targets,
			Sources: sources,
			Recipe:  raw.recipeLines,
			Origin:  fmt.Sprintf("%s:%d", raw.originFile, raw.originLine),
		}
		makefile.AddRule(rule)
	}

	return makefile, nil
}

// collectVarsAndRawRules is the first pass, now using processedLine.
func (p *Parser) collectVarsAndRawRules(lines []processedLine) ([]rawRule, error) {
	var collectedRules []rawRule
	for i := 0; i < len(lines); i++ {
		pLine := lines[i]
		trimmedLine := strings.TrimSpace(pLine.content)

		if trimmedLine == "" {
			continue
		}

		if left, right, ok := splitOnUnescaped(trimmedLine, ':'); ok && !strings.Contains(left, "=") {
			if _, _, hasMulti := splitOnUnescaped(right, ':'); hasMulti {
				return nil, fmt.Errorf("at %s:%d: invalid rule with multiple colons: \"%s\"", pLine.originFile, pLine.originLine, trimmedLine)
			}
			raw := rawRule{
				definitionLine: trimmedLine,
				recipeLines:    []string{},
				originFile:     pLine.originFile,
				originLine:     pLine.originLine,
			}
			j := i + 1
			for ; j < len(lines); j++ {
				recipeLine := lines[j].content
				if strings.TrimSpace(recipeLine) == "" {
					raw.recipeLines = append(raw.recipeLines, recipeLine)
					continue
				}
				if !(len(recipeLine) > 0 && (recipeLine[0] == ' ' || recipeLine[0] == '\t')) {
					break
				}
				raw.recipeLines = append(raw.recipeLines, recipeLine)
			}
			i = j - 1
			collectedRules = append(collectedRules, raw)
		} else if left, right, ok := splitOnUnescaped(trimmedLine, '='); ok {
			op := "="
			if strings.HasSuffix(strings.TrimSpace(left), "?") {
				op = "?="
				left = strings.TrimSpace(left[:len(left)-1])
			}
			keyPart := strings.TrimSpace(left)
			keyTokens := strings.Fields(keyPart)
			if len(keyTokens) == 0 {
				return nil, fmt.Errorf("at %s:%d: invalid assignment with no variable name: \"%s\"", pLine.originFile, pLine.originLine, trimmedLine)
			}
			varName := keyTokens[len(keyTokens)-1]
			value, err := p.variableStore.Expand(strings.TrimSpace(right), true)
			if err != nil {
				return nil, fmt.Errorf("at %s:%d: error expanding variable value: %w", pLine.originFile, pLine.originLine, err)
			}
			source := sourceMakefileUnconditional
			if op == "?=" {
				source = sourceMakefileConditional
			}
			p.variableStore.Set(varName, value, source, pLine.originFile, pLine.originLine)
		} else if strings.HasPrefix(trimmedLine, "load_env ") {
			envPath := strings.TrimSpace(trimmedLine[len("load_env"):])
			envPath = trimQuotes(envPath)
			if err := p.loadEnvFile(envPath); err != nil {
				return nil, fmt.Errorf("at %s:%d: %w", pLine.originFile, pLine.originLine, err)
			}
		} else {
			if len(pLine.content) > 0 && (pLine.content[0] == ' ' || pLine.content[0] == '\t') {
				return nil, fmt.Errorf("at %s:%d: unexpected indented line, must follow a rule definition: \"%s\"", pLine.originFile, pLine.originLine, trimmedLine)
			}
			return nil, fmt.Errorf("at %s:%d: not a rule, assignment, or directive: \"%s\"", pLine.originFile, pLine.originLine, trimmedLine)
		}
	}
	return collectedRules, nil
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
	for lineNum := 1; scanner.Scan(); lineNum++ {
		key, val, ok := cleanEnvLine(scanner.Text())
		if ok {
			p.variableStore.Set(key, val, sourceEnvFile, filename, lineNum)
		}
	}
	return scanner.Err()
}
