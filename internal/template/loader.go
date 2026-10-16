// Package template renders Go text/template files from the templates/ tree.
package template

import (
    "fmt"
    "os"
    "path/filepath"
    "sync"
    texttemplate "text/template"
)

const (
    // DefaultDir is where template files live when no override is set.
    DefaultDir = "templates"
    // EnvDir lets callers override the template directory.
    EnvDir = "ORG_BOOTSTRAP_TEMPLATES"
)

var (
    cacheMu sync.Mutex
    cache   = map[string]*texttemplate.Template{}
)

// ResolveDir returns the effective template directory.
func ResolveDir() string {
    if v := os.Getenv(EnvDir); v != "" {
        return v
    }
    return DefaultDir
}

// Load parses and caches the named template (a filename relative to the
// resolved template directory). Repeated calls for the same name are cheap.
func Load(name string) (*texttemplate.Template, error) {
    cacheMu.Lock()
    defer cacheMu.Unlock()

    if t, ok := cache[name]; ok {
        return t, nil
    }

    path := filepath.Join(ResolveDir(), name)
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read template %s: %w", path, err)
    }

    t, err := texttemplate.New(name).
        Option("missingkey=error").
        Parse(string(data))
    if err != nil {
        return nil, fmt.Errorf("parse template %s: %w", path, err)
    }
    cache[name] = t
    return t, nil
}

// ClearCache drops every cached template. Used by tests and by long-running
// processes that watch the template directory.
func ClearCache() {
    cacheMu.Lock()
    defer cacheMu.Unlock()
    cache = map[string]*texttemplate.Template{}
}
