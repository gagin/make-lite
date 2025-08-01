// cmd/make-lite/types.go
package main

import (
	"fmt"
)

// Rule represents a single rule in the makefile.
// It consists of targets, sources, and a recipe.
type Rule struct {
	Targets []string
	Sources []string
	Recipe  []string
	Origin  string // For error reporting: "line 10"
}

// String provides a simple string representation for a Rule, useful for debugging.
func (r *Rule) String() string {
	return fmt.Sprintf("Rule(Targets: %v, Sources: %v)", r.Targets, r.Sources)
}

// Makefile represents the entire parsed makefile.
// It holds all the rules and initial variable assignments.
type Makefile struct {
	Rules   []*Rule
	RuleMap map[string]*Rule // Fast lookup of a rule by its target name
}

// NewMakefile creates an initialized Makefile.
func NewMakefile() *Makefile {
	return &Makefile{
		Rules:   []*Rule{},
		RuleMap: make(map[string]*Rule),
	}
}

// AddRule adds a rule to the Makefile and registers all its targets in the RuleMap.
func (m *Makefile) AddRule(rule *Rule) {
	m.Rules = append(m.Rules, rule)
	for _, target := range rule.Targets {
		// Map every target to this rule. If a target is defined in multiple
		// rules, the last one wins, which is standard Make behavior.
		m.RuleMap[target] = rule
	}
}
