package corpus

import (
	"fmt"
	"strings"
)

type ForgeConfig struct {
	Profile      Profile
	BaseCorpus   []AttackCase
	UseTemplates bool
	Mutators     []Mutator
	MaxBase      int // 0 = all
}

func ForgeCorpus(cfg ForgeConfig) []AttackCase {
	var final []AttackCase

	base := cfg.BaseCorpus
	if cfg.MaxBase > 0 && len(base) > cfg.MaxBase {
		base = base[:cfg.MaxBase]
	}

	if cfg.UseTemplates {
		final = append(final, buildPromptInjectionGenerated()...)
		final = append(final, buildKBExfilGenerated()...)
	}

	final = append(final, base...)

	for i, mut := range cfg.Mutators {
		prefix := fmt.Sprintf("mut%d", i+1)
		final = append(final, MutateCases(prefix, base, mut)...)
	}

	return dedupeByPrompt(final)
}

func dedupeByPrompt(cases []AttackCase) []AttackCase {
	seen := make(map[string]bool, len(cases))
	out := make([]AttackCase, 0, len(cases))
	for _, c := range cases {
		key := strings.TrimSpace(c.Prompt)
		if !seen[key] {
			seen[key] = true
			out = append(out, c)
		}
	}
	return out
}
