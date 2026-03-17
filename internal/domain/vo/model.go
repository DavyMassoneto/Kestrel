package vo

import "fmt"

// supportedModels maps raw model names to their canonical Claude model names.
var supportedModels = map[string]string{
	// Claude 4 family
	"claude-sonnet-4-20250514": "claude-sonnet-4-20250514",
	"claude-opus-4-20250514":   "claude-opus-4-20250514",
	"claude-haiku-4-20250514":  "claude-haiku-4-20250514",

	// Claude 3.5 family
	"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022":  "claude-3-5-haiku-20241022",

	// Claude 3 family
	"claude-3-opus-20240229":   "claude-3-opus-20240229",
	"claude-3-sonnet-20240229": "claude-3-sonnet-20240229",
	"claude-3-haiku-20240307":  "claude-3-haiku-20240307",
}

// ModelName encapsulates a model name with parsing and validation.
type ModelName struct {
	Raw      string
	Resolved string
}

// ParseModelName creates a validated ModelName from a string.
func ParseModelName(raw string) (ModelName, error) {
	if raw == "" {
		return ModelName{}, fmt.Errorf("model name cannot be empty")
	}
	resolved, ok := supportedModels[raw]
	if !ok {
		return ModelName{}, fmt.Errorf("unsupported model: %q", raw)
	}
	return ModelName{Raw: raw, Resolved: resolved}, nil
}

// IsValid returns true if the model is supported.
func (m ModelName) IsValid() bool {
	_, ok := supportedModels[m.Resolved]
	return ok
}
