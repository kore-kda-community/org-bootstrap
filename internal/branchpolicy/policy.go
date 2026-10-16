// Package branchpolicy encodes the rules that decide whether a push from one
// branch into another is allowed. It is the Go port of release.sh's
// branch_policy_check and is intentionally pure: no network, no state.
package branchpolicy

import (
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/release"
)

// Rule identifies which policy check rejected a push. Useful for logging.
type Rule string

const (
	RuleAllow              Rule = "allow"
	RuleMainImmutable      Rule = "main-immutable"
	RuleSourceMain         Rule = "source-main"
	RuleFinalVersionWrite  Rule = "final-version-not-target"
	RuleFinalVersionSource Rule = "final-version-not-source"
	RuleReleaseRequiresDev Rule = "release-requires-dev"
	RuleVersionMismatch    Rule = "version-mismatch"
	RuleDevToDev           Rule = "dev-to-dev"
	RuleDefaultDeny        Rule = "default-deny"
)

// Decision is the outcome of a policy check.
type Decision struct {
	Allowed bool
	Rule    Rule
	Reason  string // human-readable explanation, empty when Allowed
}

func allow(rule Rule) Decision {
	return Decision{Allowed: true, Rule: rule}
}

func deny(rule Rule, format string, args ...any) Decision {
	return Decision{
		Allowed: false,
		Rule:    rule,
		Reason:  fmt.Sprintf(format, args...),
	}
}

// FinalVersionBranch is the special branch that holds "the current release
// line" for orgs that use it. Every other branch model ignores it.
const FinalVersionBranch = "Current_final_version"

// Check decides whether pushing from source into target is allowed.
func Check(source, target string) Decision {
	if target == "" {
		return deny(RuleDefaultDeny, "target branch is empty")
	}
	if source == "" {
		return deny(RuleDefaultDeny, "source branch is empty")
	}

	if target == "main" {
		return deny(RuleMainImmutable,
			"main is a frozen upstream reference — never writable")
	}
	if source == "main" {
		return deny(RuleSourceMain,
			"cannot push FROM main — it is frozen")
	}

	if target == FinalVersionBranch {
		if !release.ParseBranch(source).IsRelease() {
			return deny(RuleFinalVersionWrite,
				"only release branches can push into %s (source %q is not)",
				FinalVersionBranch, source)
		}
		return allow(RuleAllow)
	}
	if source == FinalVersionBranch {
		return deny(RuleFinalVersionSource,
			"%s is read-only — branch it first",
			FinalVersionBranch)
	}

	srcBranch := release.ParseBranch(source)
	tgtBranch := release.ParseBranch(target)

	if tgtBranch.IsRelease() {
		if !srcBranch.IsDev() {
			return deny(RuleReleaseRequiresDev,
				"only dev branches can push into %s (source %q is not)",
				target, source)
		}
		if tgtBranch.Version != srcBranch.Version {
			return deny(RuleVersionMismatch,
				"version mismatch: %s (%s) cannot push into %s (%s)",
				source, srcBranch.Version, target, tgtBranch.Version)
		}
		return allow(RuleAllow)
	}

	if tgtBranch.IsDev() {
		return allow(RuleDevToDev)
	}

	return deny(RuleDefaultDeny,
		"no policy permits pushing from %q into %q", source, target)
}

// FormatReason renders a Decision for display. Empty when allowed.
func FormatReason(d Decision) string {
	if d.Allowed {
		return ""
	}
	if d.Reason == "" {
		return string(d.Rule)
	}
	return d.Reason
}

// IsReleasePrefix reports whether s looks like a release branch prefix.
func IsReleasePrefix(s string) bool {
	return strings.HasPrefix(s, "release_")
}
