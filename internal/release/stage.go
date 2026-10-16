package release

import "strings"

// Stage is a release maturity level.
type Stage string

const (
	StageNone  Stage = "none"
	StageAlpha Stage = "alpha"
	StageBeta  Stage = "beta"
	StageRC    Stage = "rc"
	StageFinal Stage = "final"
)

// ParseStage converts a string to a Stage. Unrecognised values become None.
func ParseStage(s string) Stage {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "alpha":
		return StageAlpha
	case "beta":
		return StageBeta
	case "rc":
		return StageRC
	case "final", "stable":
		return StageFinal
	default:
		return StageNone
	}
}

// AllowedNext returns the stages that may follow s. A closed stage (final)
// returns nil.
func (s Stage) AllowedNext() []Stage {
	switch s {
	case StageNone:
		return []Stage{StageAlpha}
	case StageAlpha:
		return []Stage{StageAlpha, StageBeta}
	case StageBeta:
		return []Stage{StageBeta, StageRC}
	case StageRC:
		return []Stage{StageRC, StageFinal}
	case StageFinal:
		return nil
	}
	return []Stage{StageAlpha}
}

// Allows reports whether next is a valid forward transition from s.
func (s Stage) Allows(next Stage) bool {
	for _, a := range s.AllowedNext() {
		if a == next {
			return true
		}
	}
	return false
}

// RegressTarget returns the stage to regress to, or ("", false) if none.
func (s Stage) RegressTarget() (Stage, bool) {
	switch s {
	case StageBeta:
		return StageAlpha, true
	case StageRC:
		return StageBeta, true
	}
	return "", false
}

// Closed reports whether the stage ends the release cycle.
func (s Stage) Closed() bool { return s == StageFinal }

// TagForVersion returns the git tag corresponding to (version, stage).
// Empty string if stage is None.
func TagForVersion(version string, stage Stage) string {
	v := strings.TrimPrefix(version, "v")
	switch stage {
	case StageNone:
		return ""
	case StageFinal:
		return "v" + v
	default:
		return "v" + v + "-" + string(stage)
	}
}

// String is the human-readable stage name.
func (s Stage) String() string { return string(s) }
