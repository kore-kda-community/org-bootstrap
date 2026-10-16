package audit

import (
    "encoding/json"
    "fmt"
    "io"
    "text/tabwriter"
    "time"
)

// Report is the JSON document written by org-audit.
type Report struct {
    Version      int       `json:"version"`
    Org          string    `json:"org"`
    Repo         string    `json:"repo"`
    CommitSHA    string    `json:"commit_sha,omitempty"`
    StartedAt    time.Time `json:"started_at"`
    FinishedAt   time.Time `json:"finished_at"`
    DurationMS   int64     `json:"duration_ms"`
    Clean        bool      `json:"clean"`
    Findings     []Finding `json:"findings"`
    ScannedFiles int       `json:"scanned_files"`
    SkippedFiles int       `json:"skipped_files"`
}

// Build fills in derived fields from an Outcome.
func Build(out *Outcome, org, repo, sha string, started, finished time.Time) *Report {
    findings := out.Findings
    if findings == nil {
        findings = []Finding{}
    }
    return &Report{
        Version:      1,
        Org:          org,
        Repo:         repo,
        CommitSHA:    sha,
        StartedAt:    started.UTC(),
        FinishedAt:   finished.UTC(),
        DurationMS:   finished.Sub(started).Milliseconds(),
        Clean:        len(findings) == 0,
        Findings:     findings,
        ScannedFiles: out.Scanned,
        SkippedFiles: out.Skipped,
    }
}

// WriteJSON emits the report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(r)
}

// WriteText emits a human-readable report.
func (r *Report) WriteText(w io.Writer) error {
    if r.Clean {
        _, err := fmt.Fprintf(w, "✓ audit clean: %s/%s (%d files in %dms)\n",
            r.Org, r.Repo, r.ScannedFiles, r.DurationMS)
        return err
    }
    if _, err := fmt.Fprintf(w, "✗ audit: %d finding(s) in %s/%s\n",
        len(r.Findings), r.Org, r.Repo); err != nil {
        return err
    }
    tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
    fmt.Fprintln(tw, "  SEVERITY\tRULE\tFILE:LINE\tMATCH")
    for _, f := range r.Findings {
        fmt.Fprintf(tw, "  %s\t%s\t%s:%d\t%s\n",
            f.Severity, f.RuleID, f.File, f.Line, f.MatchPreview)
    }
    return tw.Flush()
}
