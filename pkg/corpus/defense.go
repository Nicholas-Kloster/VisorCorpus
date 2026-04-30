package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ModelConfig struct {
	Name        string  `json:"name"`
	ModelID     string  `json:"model_id"`
	Temperature float32 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float32 `json:"top_p"`
}

type SystemPromptConfig struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type DefenseConfig struct {
	Name         string             `json:"name"`
	SystemPrompt SystemPromptConfig `json:"system_prompt"`
	Model        ModelConfig        `json:"model"`
	Profile      string             `json:"profile"`
	Notes        string             `json:"notes,omitempty"`
}

// LoadDefenseConfigs loads a JSON array of DefenseConfig from path.
func LoadDefenseConfigs(path string) ([]DefenseConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfgs []DefenseConfig
	if err := json.Unmarshal(b, &cfgs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfgs, nil
}

// FindDefenseConfig returns the named config from the slice.
func FindDefenseConfig(cfgs []DefenseConfig, name string) (*DefenseConfig, error) {
	for i := range cfgs {
		if cfgs[i].Name == name {
			return &cfgs[i], nil
		}
	}
	var names []string
	for _, c := range cfgs {
		names = append(names, c.Name)
	}
	return nil, fmt.Errorf("defense config %q not found; available: %s", name, strings.Join(names, ", "))
}
