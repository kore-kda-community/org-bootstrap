package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
	gh "github.com/kore-kda-community/org-bootstrap/internal/gh"
)

const (
	PublicReleasesRepo  = "releases"
	ArchiveReleasesRepo = "releases-archive"
)

// ArchiveCandidate is one release being considered for archival.
type ArchiveCandidate struct {
	Tag        string
	Name       string
	Body       string
	Published  time.Time
	Prerelease bool
	Assets     []*github.ReleaseAsset
}

// ArchivePlan is a pure-data description of what would be archived. Nothing
// happens until Execute is called.
type ArchivePlan struct {
	Org         string
	PublicRepo  string
	ArchiveRepo string
	Candidates  []ArchiveCandidate
}

// Empty reports whether there is nothing to do.
func (p *ArchivePlan) Empty() bool { return len(p.Candidates) == 0 }

// PlanPreviousLine collects all releases matching "v<major-1>.*" where major
// comes from newMajor (e.g. "1.0.0" archives the v0.* line).
func PlanPreviousLine(ctx context.Context, c *gh.Client, org, newMajor string) (*ArchivePlan, error) {
	plan := &ArchivePlan{Org: org, PublicRepo: PublicReleasesRepo, ArchiveRepo: ArchiveReleasesRepo}

	major := parseMajor(newMajor)
	if major <= 0 {
		return plan, nil
	}
	prefix := fmt.Sprintf("v%d.", major-1)

	releases, err := c.ListReleases(ctx, org, PublicReleasesRepo)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	for _, r := range releases {
		if !strings.HasPrefix(r.GetTagName(), prefix) {
			continue
		}
		plan.Candidates = append(plan.Candidates, toCandidate(r))
	}
	return plan, nil
}

// PlanOldReleases collects everything except the latest stable release.
func PlanOldReleases(ctx context.Context, c *gh.Client, org string) (*ArchivePlan, error) {
	plan := &ArchivePlan{Org: org, PublicRepo: PublicReleasesRepo, ArchiveRepo: ArchiveReleasesRepo}

	releases, err := c.ListReleases(ctx, org, PublicReleasesRepo)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	stable := latestStable(releases)
	for _, r := range releases {
		if r.GetTagName() == stable {
			continue
		}
		plan.Candidates = append(plan.Candidates, toCandidate(r))
	}
	return plan, nil
}

func toCandidate(r *github.RepositoryRelease) ArchiveCandidate {
	var pub time.Time
	if r.PublishedAt != nil {
		pub = r.PublishedAt.Time
	}
	name := r.GetName()
	if name == "" {
		name = r.GetTagName()
	}
	return ArchiveCandidate{
		Tag:        r.GetTagName(),
		Name:       name,
		Body:       r.GetBody(),
		Published:  pub,
		Prerelease: r.GetPrerelease(),
		Assets:     r.Assets,
	}
}

var semverOnly = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

func latestStable(rs []*github.RepositoryRelease) string {
	for _, r := range rs {
		if r.GetPrerelease() {
			continue
		}
		if semverOnly.MatchString(r.GetTagName()) {
			return r.GetTagName()
		}
	}
	return ""
}

func parseMajor(v string) int {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '.'); i >= 0 {
		v = v[:i]
	}
	n := 0
	fmt.Sscanf(v, "%d", &n)
	return n
}

// ArchiveResult reports what happened during Execute.
type ArchiveResult struct {
	Moved       []string
	Failed      map[string]error
	CreatedRepo bool
}

// Summary is a one-line count.
func (r *ArchiveResult) Summary() string {
	return fmt.Sprintf("moved %d, failed %d", len(r.Moved), len(r.Failed))
}

// Execute performs the plan. With dryRun=true nothing is written or deleted;
// it only validates that the archive repo would be created if needed.
func Execute(ctx context.Context, c *gh.Client, plan *ArchivePlan, dryRun bool) (*ArchiveResult, error) {
	res := &ArchiveResult{Failed: map[string]error{}}
	if plan.Empty() {
		return res, nil
	}

	if !dryRun {
		exists, err := c.RepoExists(ctx, plan.Org, plan.ArchiveRepo)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := c.CreateRepo(ctx, plan.Org, plan.ArchiveRepo,
				"Archived releases for "+plan.Org, true); err != nil {
				return nil, fmt.Errorf("create archive repo: %w", err)
			}
			res.CreatedRepo = true
		}
	}

	tmp, err := os.MkdirTemp("", "org-archive-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	for _, cand := range plan.Candidates {
		if err := archiveOne(ctx, c, plan, cand, tmp, dryRun); err != nil {
			res.Failed[cand.Tag] = err
			continue
		}
		res.Moved = append(res.Moved, cand.Tag)
	}
	return res, nil
}

func archiveOne(ctx context.Context, c *gh.Client, plan *ArchivePlan, cand ArchiveCandidate, tmp string, dryRun bool) error {
	if dryRun {
		return nil
	}

	newRel, err := c.CreateRelease(ctx, plan.Org, plan.ArchiveRepo,
		cand.Tag, cand.Name, cand.Body)
	if err != nil {
		return fmt.Errorf("create archive release: %w", err)
	}

	for _, asset := range cand.Assets {
		localPath := filepath.Join(tmp, fmt.Sprintf("%d-%s", asset.GetID(), asset.GetName()))
		if _, err := c.DownloadReleaseAsset(ctx, plan.Org, plan.PublicRepo, asset.GetID(), localPath); err != nil {
			return fmt.Errorf("download %s: %w", asset.GetName(), err)
		}
		if err := c.UploadReleaseAsset(ctx, plan.Org, plan.ArchiveRepo,
			newRel.GetID(), localPath, asset.GetName()); err != nil {
			return fmt.Errorf("upload %s: %w", asset.GetName(), err)
		}
	}

	orig, err := c.GetReleaseByTag(ctx, plan.Org, plan.PublicRepo, cand.Tag)
	if err != nil {
		return fmt.Errorf("get original: %w", err)
	}
	if err := c.DeleteRelease(ctx, plan.Org, plan.PublicRepo, orig.GetID()); err != nil {
		return fmt.Errorf("delete original: %w", err)
	}
	return nil
}
