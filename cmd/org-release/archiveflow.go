package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kore-kda-community/org-bootstrap/internal/gh"
	"github.com/kore-kda-community/org-bootstrap/internal/release"
)

// archiveFlow lets the user move releases from <org>/releases to
// <org>/releases-archive. Two modes: archive the previous major line, or
// archive everything except the latest stable.
func archiveFlow(ctx context.Context, c *gh.Client, org string) {
	hdr("Archive: %s", org)
	fmt.Println("  moves releases from " + cBold + org + "/releases" + cReset +
		" to " + cBold + org + "/releases-archive" + cReset)
	fmt.Println()
	fmt.Println("    [a] archive the previous major line")
	fmt.Println("    [o] archive everything except the latest stable")
	fmt.Println("    [x] cancel")
	pick := strings.ToLower(promptLine("  Choice", "x"))

	var plan *release.ArchivePlan
	var err error

	switch pick {
	case "a":
		fmt.Println()
		fmt.Println("  " + cDim + "example: enter 1.0.0 to archive all v0.x.y releases" + cReset)
		major := promptLine("  New major version (e.g. 1.0.0)", "")
		if major == "" {
			return
		}
		plan, err = release.PlanPreviousLine(ctx, c, org, major)
	case "o":
		plan, err = release.PlanOldReleases(ctx, c, org)
	default:
		return
	}

	if err != nil {
		errf("%v", err)
		return
	}
	if plan.Empty() {
		ok("nothing to archive")
		return
	}

	fmt.Println()
	fmt.Printf("  %s%d release(s) to move:%s\n", cBold, len(plan.Candidates), cReset)
	for _, cand := range plan.Candidates {
		pre := ""
		if cand.Prerelease {
			pre = " " + cDim + "(prerelease)" + cReset
		}
		fmt.Printf("    %-22s  %d asset(s)%s\n", cand.Tag, len(cand.Assets), pre)
	}
	fmt.Println()

	if !promptYN(fmt.Sprintf("Proceed with %d release(s)?", len(plan.Candidates)), false) {
		return
	}

	res, err := release.Execute(ctx, c, plan, false)
	if err != nil {
		errf("%v", err)
		return
	}
	if res.CreatedRepo {
		ok("created %s/%s (private)", org, plan.ArchiveRepo)
	}
	ok("moved %d release(s) to %s/%s",
		len(res.Moved), org, plan.ArchiveRepo)
	for tag, e := range res.Failed {
		errf("%s: %v", tag, e)
	}
}
