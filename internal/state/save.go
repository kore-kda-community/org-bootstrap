package state

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// Save writes state atomically. UpdatedAt is set to the current time.
func Save(path string, st *State) error {
    resolved := ResolvePath(path)
    st.UpdatedAt = time.Now().UTC()

    data, err := json.MarshalIndent(st, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal state: %w", err)
    }
    data = append(data, '\n')

    dir := filepath.Dir(resolved)
    if dir != "" && dir != "." {
        if err := os.MkdirAll(dir, 0o755); err != nil {
            return fmt.Errorf("mkdir %s: %w", dir, err)
        }
    }

    tmp := resolved + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("write %s: %w", tmp, err)
    }
    if err := os.Rename(tmp, resolved); err != nil {
        return fmt.Errorf("rename %s -> %s: %w", tmp, resolved, err)
    }
    return nil
}
