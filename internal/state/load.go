package state

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
)

const (
    // DefaultName is the state file used when no path is given.
    DefaultName = ".state.json"
    // EnvStatePath lets callers override the state location.
    EnvStatePath = "ORG_BOOTSTRAP_STATE"
)

// Load reads state from path, returning an empty State if the file doesn't
// exist. A version mismatch is a hard error.
func Load(path string) (*State, error) {
    resolved := ResolvePath(path)

    st := &State{Version: SchemaVersion, Orgs: map[string]OrgState{}}

    data, err := os.ReadFile(resolved)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return st, nil
        }
        return nil, fmt.Errorf("read %s: %w", resolved, err)
    }

    if err := json.Unmarshal(data, st); err != nil {
        return nil, fmt.Errorf("parse %s: %w", resolved, err)
    }
    if st.Version != SchemaVersion {
        return nil, fmt.Errorf("state version mismatch in %s: file=%d code=%d",
            resolved, st.Version, SchemaVersion)
    }
    if st.Orgs == nil {
        st.Orgs = map[string]OrgState{}
    }
    return st, nil
}

// ResolvePath returns the effective state file path.
func ResolvePath(path string) string {
    if path != "" {
        return path
    }
    if env := os.Getenv(EnvStatePath); env != "" {
        return env
    }
    return DefaultName
}
