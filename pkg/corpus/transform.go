package corpus

import "strings"

type Transform func(AttackCase) AttackCase

func WithTenant(tenant string) Transform {
	return func(c AttackCase) AttackCase {
		c.Prompt = strings.ReplaceAll(c.Prompt, "{{TENANT}}", tenant)
		return c
	}
}

func WithDocID(docID string) Transform {
	return func(c AttackCase) AttackCase {
		c.Prompt = strings.ReplaceAll(c.Prompt, "{{DOC_ID}}", docID)
		return c
	}
}

func WithEmail(email string) Transform {
	return func(c AttackCase) AttackCase {
		c.Prompt = strings.ReplaceAll(c.Prompt, "{{EMAIL}}", email)
		return c
	}
}

func Chain(ts ...Transform) Transform {
	return func(c AttackCase) AttackCase {
		for _, t := range ts {
			c = t(c)
		}
		return c
	}
}

func ApplyTransform(cases []AttackCase, t Transform) []AttackCase {
	out := make([]AttackCase, len(cases))
	for i, c := range cases {
		out[i] = t(c)
	}
	return out
}
