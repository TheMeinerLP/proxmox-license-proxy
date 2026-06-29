package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// outputFormat is the value of the global --output/-o flag.
var outputFormat string

// render prints v in the selected output format. For the default "table" format
// it delegates to the table closure (each command owns its columns); json and
// yaml marshal v directly so the structured output is scriptable.
func render(v any, table func() error) error {
	switch outputFormat {
	case "", "table":
		return table()
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		defer func() { _ = enc.Close() }()
		return enc.Encode(v)
	default:
		return fmt.Errorf("unknown --output %q (table|json|yaml)", outputFormat)
	}
}
