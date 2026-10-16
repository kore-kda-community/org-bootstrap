// Package audit scans a directory for high-signal secret patterns using
// configurable TOML rules.
package audit

import (
    "fmt"
    "regexp"

    "github.com/BurntSushi/toml"
)

// RulesFile is the on-disk TOML schema.
type RulesFile struct {
    Version  int       `toml:"version"`
    Rules    []RuleDef `toml:"rules"`
    Excludes Excludes  `toml:"excludes"`
}

// RuleDef is a single rule in the TOML file.
type RuleDef struct {
    ID       string `toml:"id"`
    Pattern  string `toml:"pattern"`
    Severity string `toml:"severity"`
}

// Excludes controls which paths/extensions are skipped.
type Excludes struct {
    Paths      []string `toml:"paths"`
    Extensions []string `toml:"extensions"`
}

// Rule is a compiled rule ready for scanning.
type Rule struct {
    ID       string
    Pattern  string
    Severity string
    Re       *regexp.Regexp
}

// LoadRules reads and compiles rules from a TOML file.
func LoadRules(path string) ([]Rule, Excludes, error) {
    var rf RulesFile
    if _, err := toml.DecodeFile(path, &rf); err != nil {
        return nil, Excludes{}, fmt.Errorf("parse %s: %w", path, err)
    }
    rules := make([]Rule, 0, len(rf.Rules))
    for _, d := range rf.Rules {
        if d.ID == "" {
            return nil, Excludes{}, fmt.Errorf("rule missing id in %s", path)
        }
        re, err := regexp.Compile(d.Pattern)
        if err != nil {
            return nil, Excludes{}, fmt.Errorf("rule %s: bad pattern %q: %w",
                d.ID, d.Pattern, err)
        }
        rules = append(rules, Rule{
            ID:       d.ID,
            Pattern:  d.Pattern,
            Severity: d.Severity,
            Re:       re,
        })
    }
    return rules, rf.Excludes, nil
}
