package audit

import (
    "bufio"
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "sync"

    "golang.org/x/sync/errgroup"
)

// Finding is a single match.
type Finding struct {
    RuleID       string `json:"rule_id"`
    Pattern      string `json:"pattern"`
    File         string `json:"file"`
    Line         int    `json:"line"`
    MatchPreview string `json:"match_preview"`
    Severity     string `json:"severity"`
}

// Outcome is the aggregate result of a scan.
type Outcome struct {
    Findings []Finding
    Scanned  int
    Skipped  int
}

// Scan walks dir and runs every rule against every non-excluded file.
// Uses runtime.NumCPU()*2 concurrent workers.
func Scan(dir string, rules []Rule, ex Excludes) (*Outcome, error) {
    files, skipped, err := collectFiles(dir, ex)
    if err != nil {
        return nil, err
    }

    var (
        mu       sync.Mutex
        findings []Finding
    )
    g := new(errgroup.Group)
    g.SetLimit(runtime.NumCPU() * 2)

    for _, f := range files {
        f := f
        g.Go(func() error {
            hits, err := scanFile(f, dir, rules)
            if err != nil || len(hits) == 0 {
                return nil
            }
            mu.Lock()
            findings = append(findings, hits...)
            mu.Unlock()
            return nil
        })
    }
    _ = g.Wait()

    return &Outcome{
        Findings: findings,
        Scanned:  len(files),
        Skipped:  skipped,
    }, nil
}

// collectFiles walks dir and returns paths that pass the exclusion rules.
func collectFiles(dir string, ex Excludes) ([]string, int, error) {
    var files []string
    skipped := 0

    err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
        if err != nil {
            return nil // skip unreadable entries
        }
        rel, _ := filepath.Rel(dir, p)

        if d.IsDir() {
            if excludedPath(rel, ex.Paths) {
                skipped++
                return filepath.SkipDir
            }
            return nil
        }
        if excludedPath(rel, ex.Paths) || excludedExt(rel, ex.Extensions) {
            skipped++
            return nil
        }
        files = append(files, p)
        return nil
    })
    if err != nil {
        return nil, 0, fmt.Errorf("walk %s: %w", dir, err)
    }
    return files, skipped, nil
}

func excludedPath(rel string, patterns []string) bool {
    if rel == "." {
        return false
    }
    for _, p := range patterns {
        p = strings.TrimSuffix(p, "/")
        if rel == p || strings.HasPrefix(rel, p+"/") {
            return true
        }
        // also match if the pattern appears as a segment
        if strings.Contains(rel, "/"+p+"/") {
            return true
        }
    }
    return false
}

func excludedExt(rel string, exts []string) bool {
    ext := strings.TrimPrefix(filepath.Ext(rel), ".")
    for _, e := range exts {
        if e == ext {
            return true
        }
    }
    return false
}

func scanFile(path, root string, rules []Rule) ([]Finding, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    rel, _ := filepath.Rel(root, path)

    var out []Finding
    sc := bufio.NewScanner(f)
    sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

    line := 0
    for sc.Scan() {
        line++
        s := sc.Text()
        for _, r := range rules {
            m := r.Re.FindString(s)
            if m == "" {
                continue
            }
            out = append(out, Finding{
                RuleID:       r.ID,
                Pattern:      r.Pattern,
                File:         rel,
                Line:         line,
                MatchPreview: redact(m),
                Severity:     r.Severity,
            })
        }
    }
    return out, sc.Err()
}

// redact trims a matched string down to its first few characters plus a
// redaction marker, so findings can be shown without leaking the secret.
func redact(s string) string {
    if len(s) <= 8 {
        return "***REDACTED***"
    }
    return s[:4] + "***REDACTED***"
}
