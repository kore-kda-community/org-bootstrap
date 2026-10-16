package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/config"
	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/protection"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
	"github.com/kore-kda-community/org-bootstrap/internal/state"
)

// repoActions is the per-repo action menu. Loops until [b]ack or [q]uit.
func repoActions(ctx context.Context, c *gh.Client, st *state.State, cfg *config.Config, org string, row repoRow) error {
	for {
		fmt.Print("\033[H\033[2J")
		hdr("Repo: %s/%s", org, row.Name)

		// Refresh the release picture each time we come back.
		sts, err := release.Scan(ctx, c, org, row.Name)
		if err != nil {
			return err
		}

		renderStageHeader(ctx, c, org, row.Name, sts)

		// Menu
		fmt.Println("    [i] Inspect")
		fmt.Println("    [c] Readiness check")
		fmt.Println()

		var (
			canAlpha, canBeta, canRC, canFinal bool
			vAlpha, vBeta, vRC, vFinal         string
			rtarget                            release.Stage
			hasRegress                         bool
		)

		if sts.CurrentBranch == nil {
			fmt.Println("    " + cDim + "no release branch — start one first" + cReset)
			fmt.Printf("      %s[+]%s start new release (first version)\n", cGreen, cReset)
		} else if sts.CurrentStage == release.StageFinal {
			fmt.Println("    " + cGreen + "cycle complete" + cReset + "  " + cDim + "— start a new release for the next version" + cReset)
			fmt.Printf("      %s[+]%s start new release (next version)\n", cGreen, cReset)
		} else {
			allowed := release.AllowedBumps(sts.CurrentStage)
			for _, b := range allowed {
				switch b {
				case release.BumpAlpha:
					canAlpha = true
					vAlpha, _ = release.NextVersion(sts.CurrentVer, release.BumpAlpha)
				case release.BumpBeta:
					canBeta = true
					vBeta, _ = release.NextVersion(sts.CurrentVer, release.BumpBeta)
				case release.BumpRC:
					canRC = true
					vRC, _ = release.NextVersion(sts.CurrentVer, release.BumpRC)
				case release.BumpFinal:
					canFinal = true
					vFinal, _ = release.NextVersion(sts.CurrentVer, release.BumpFinal)
				}
			}
			rtarget, hasRegress = sts.CurrentStage.RegressTarget()

			printStageLine(canAlpha, "A", "alpha", vAlpha)
			printStageLine(canBeta, "B", "beta", vBeta)
			printStageLine(canRC, "R", "rc", vRC)
			printStageLine(canFinal, "F", "final", vFinal)

			if hasRegress {
				rv, _ := release.NextVersion(sts.CurrentVer, release.BumpFromStage(rtarget))
				fmt.Printf("      %s[<]%s regress %s→%s %s (back to %s)\n",
					cGreen, cReset, cDim, cReset, rv, rtarget)
			} else {
				fmt.Printf("      %s[<] regress (none)%s\n", cDim, cReset)
			}
			fmt.Printf("      %s[x]%s bypass — set version manually\n", cGreen, cReset)
			fmt.Printf("      %s[+]%s start new release branch\n", cGreen, cReset)
		}

		fmt.Println()
		lockAction := "Lock main"
		if protection.MainIsLocked(ctx, c, org, row.Name) {
			lockAction = "Unlock main"
		}
		fmt.Printf("    [l] %s\n", lockAction)
		fmt.Printf("    [z] Archive old releases\n")
		fmt.Printf("    [m] Toggle managed status\n")
		fmt.Printf("    [b] Back\n")
		fmt.Printf("    [q] Quit\n")

		pick := strings.TrimSpace(promptLine("  Choice", "i"))

		switch pick {
		case "i":
			inspectRepo(ctx, c, org, row.Name)
			pause()
		case "c":
			readinessCheck(ctx, c, org, row.Name)
			pause()
		case "+":
			if err := createReleaseBranchFlow(ctx, c, st, org, row.Name); err != nil {
				errf("%v", err)
			}
			pause()
		case "A":
			if canAlpha {
				if err := triggerRelease(ctx, c, org, row.Name, release.StageAlpha); err != nil {
					errf("%v", err)
				}
			} else {
				warn("alpha locked (stage: %s)", sts.CurrentStage)
			}
			pause()
		case "B":
			if canBeta {
				if err := triggerRelease(ctx, c, org, row.Name, release.StageBeta); err != nil {
					errf("%v", err)
				}
			} else {
				warn("beta locked (stage: %s)", sts.CurrentStage)
			}
			pause()
		case "R":
			if canRC {
				if err := triggerRelease(ctx, c, org, row.Name, release.StageRC); err != nil {
					errf("%v", err)
				}
			} else {
				warn("rc locked (stage: %s)", sts.CurrentStage)
			}
			pause()
		case "F":
			if canFinal {
				if err := triggerRelease(ctx, c, org, row.Name, release.StageFinal); err != nil {
					errf("%v", err)
				}
			} else {
				warn("final locked (stage: %s)", sts.CurrentStage)
			}
			pause()
		case "<":
			if hasRegress {
				if err := triggerRelease(ctx, c, org, row.Name, rtarget); err != nil {
					errf("%v", err)
				}
			} else {
				warn("no regress from this stage")
			}
			pause()
		case "x":
			bypassRelease(ctx, c, org, row.Name, sts)
			pause()
		case "l":
			toggleLock(ctx, c, org, row.Name)
			pause()
		case "z":
			archiveFlow(ctx, c, org)
			pause()
		case "m":
			toggleManaged(st, cfg, org, row.Name, statePathFromCtx(cfg))
			pause()
		case "b", "":
			return nil
		case "q":
			return errQuit
		}
	}
}

// printStageLine prints a stage option, dimmed if locked.
func printStageLine(enabled bool, key, name, version string) {
	if enabled {
		fmt.Printf("      %s[%s]%s %-7s %s→%s %s\n", cGreen, key, cReset, name, cDim, cReset, version)
	} else {
		fmt.Printf("      %s[%s] %-7s (locked)%s\n", cDim, key, name, cReset)
	}
}

// renderStageHeader prints the current branch/stage/version above the menu.
func renderStageHeader(ctx context.Context, c *gh.Client, org, name string, sts *release.RepoStatus) {
	if sts.CurrentBranch == nil {
		fmt.Printf("    %s•%s no release branch yet\n", cYellow, cReset)
		fmt.Printf("    %sreleases only come from a release branch <project>_<x.y.z>%s\n", cDim, cReset)
	} else if sts.CurrentStage == release.StageFinal {
		fmt.Printf("    %s✓%s %s%s%s  %sreleased%s  %s(cycle closed)%s\n",
			cGreen, cReset, cBold, sts.CurrentTag, cReset, cGreen, cReset, cDim, cReset)
	} else if sts.CurrentStage == release.StageNone {
		fmt.Printf("    %s•%s %s%s%s  %sready — no stage tags yet%s\n",
			cDim, cReset, cBold, sts.CurrentBranch.Name, cReset, cDim, cReset)
	} else {
		fmt.Printf("    %s•%s %s%s%s  %s%s%s  %s%s%s\n",
			cYellow, cReset, cBold, sts.CurrentBranch.Name, cReset,
			cBold, sts.CurrentStage, cReset, cBold, sts.CurrentTag, cReset)
	}

	if protection.MainIsLocked(ctx, c, org, name) {
		fmt.Printf("    %s✓ main locked%s\n", cGreen, cReset)
	} else {
		fmt.Printf("    %s• main unlocked%s\n", cYellow, cReset)
	}
	fmt.Println()
}

// toggleLock applies or removes protection based on current state.
func toggleLock(ctx context.Context, c *gh.Client, org, name string) {
	if protection.MainIsLocked(ctx, c, org, name) {
		if err := protection.UnlockMain(ctx, c, org, name); err != nil {
			errf("unlock: %v", err)
			return
		}
		ok("main protection removed on %s/%s", org, name)
		return
	}
	res, err := protection.LockMain(ctx, c, org, name)
	if err != nil {
		errf("lock: %v", err)
		return
	}
	switch res.Method {
	case "already":
		ok("main is already protected")
	case "protection+lock":
		ok("main locked (protection+lock)")
	case "protection":
		ok("main protected (lock_branch unavailable on this plan)")
	case "ruleset":
		ok("main locked via ruleset")
	case "none":
		warn("%s", res.Warning)
	default:
		ok("main locked (%s)", res.Method)
	}
}

// toggleManaged adds or removes the repo from the managed list, and persists.
func toggleManaged(st *state.State, cfg *config.Config, org, name, statePath string) {
	os := st.Get(org)
	found := -1
	for i, r := range os.ManagedRepos {
		if r == name {
			found = i
			break
		}
	}
	if found >= 0 {
		os.ManagedRepos = append(os.ManagedRepos[:found], os.ManagedRepos[found+1:]...)
		ok("%s/%s removed from managed list", org, name)
	} else {
		os.ManagedRepos = append(os.ManagedRepos, name)
		ok("%s/%s added to managed list", org, name)
	}
	st.Put(org, os)
	if statePath != "" {
		if err := state.Save(statePath, st); err != nil {
			errf("save state: %v", err)
		}
	}
}

// statePathFromCtx returns the state path from cfg. Placeholder — the value
// is threaded explicitly everywhere else.
func statePathFromCtx(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.General.StatePath
}

// errQuit is a sentinel that unwinds up to main.
var errQuit = fmt.Errorf("quit")

// keep strings import alive if unused elsewhere
var _ = strings.TrimSpace
