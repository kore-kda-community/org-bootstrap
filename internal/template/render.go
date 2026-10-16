package template

import (
    "bytes"
    "fmt"
    texttemplate "text/template"
)

// Render loads the named template and executes it with data, returning the
// resulting bytes.
func Render(name string, data any) ([]byte, error) {
    t, err := Load(name)
    if err != nil {
        return nil, err
    }
    return execute(t, data)
}

// RenderString parses an inline template and executes it with data. Used for
// ad-hoc snippets and tests; not cached.
func RenderString(text string, data any) ([]byte, error) {
    t, err := texttemplate.New("inline").
        Option("missingkey=error").
        Parse(text)
    if err != nil {
        return nil, fmt.Errorf("parse inline template: %w", err)
    }
    return execute(t, data)
}

func execute(t *texttemplate.Template, data any) ([]byte, error) {
    var buf bytes.Buffer
    if err := t.Execute(&buf, data); err != nil {
        return nil, fmt.Errorf("execute template %s: %w", t.Name(), err)
    }
    return buf.Bytes(), nil
}
