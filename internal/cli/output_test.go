package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestRender(t *testing.T) {
	type sample struct {
		Key string `json:"key" yaml:"key"`
	}
	data := sample{Key: "pbsc-1234567890"}

	t.Run("table delegates to closure", func(t *testing.T) {
		outputFormat = ""
		defer func() { outputFormat = "" }()
		called := false
		out := captureStdout(t, func() {
			_ = render(data, func() error {
				called = true
				return nil
			})
		})
		if !called {
			t.Error("table closure was not called")
		}
		if out != "" {
			t.Errorf("closure did its own printing, render should not add output: %q", out)
		}
	})

	t.Run("json", func(t *testing.T) {
		outputFormat = "json"
		defer func() { outputFormat = "" }()
		out := captureStdout(t, func() {
			if err := render(data, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, `"key": "pbsc-1234567890"`) {
			t.Errorf("json output missing key: %q", out)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		outputFormat = "yaml"
		defer func() { outputFormat = "" }()
		out := captureStdout(t, func() {
			if err := render(data, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, "key: pbsc-1234567890") {
			t.Errorf("yaml output missing key: %q", out)
		}
	})

	t.Run("unknown format errors", func(t *testing.T) {
		outputFormat = "xml"
		defer func() { outputFormat = "" }()
		if err := render(data, func() error { return nil }); err == nil {
			t.Error("expected error for unknown format")
		}
	})
}
