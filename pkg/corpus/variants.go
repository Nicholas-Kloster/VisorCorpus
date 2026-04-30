package corpus

import "math/rand"

type BuildType string

const (
	BuildBaseline   BuildType = "baseline"
	BuildStress     BuildType = "stress"
	BuildFocused    BuildType = "focused"
	BuildRegression BuildType = "regression"
	BuildRandomized BuildType = "randomized"
)

type BuildConfig struct {
	Profile   Profile
	BuildType BuildType

	IncludeCategories []Category
	ExcludeCategories []Category

	// Domain seeds to layer on top of the base corpus
	Domains []Domain

	// Tag filters applied after building (key → allowed values)
	TagFilters map[string][]string

	UseForge bool
	ForgeCfg ForgeConfig

	MaxCases int

	// Seed for randomized builds (0 = use default)
	RandSeed int64
}

func BuildCorpusVariant(cfg BuildConfig) []AttackCase {
	var cases []AttackCase
	switch cfg.BuildType {
	case BuildStress:
		cases = buildStressCorpus(cfg)
	case BuildFocused:
		cases = buildFocusedCorpus(cfg)
	case BuildRandomized:
		cases = buildRandomizedCorpus(cfg)
	default:
		cases = buildBaselineCorpus(cfg)
	}

	for _, d := range cfg.Domains {
		cases = AddDomainSeeds(cases, cfg.Profile, d)
	}

	for key, vals := range cfg.TagFilters {
		cases = FilterByTag(cases, key, vals)
	}

	return cases
}

func buildBaselineCorpus(cfg BuildConfig) []AttackCase {
	base := CorpusForProfile(cfg.Profile)
	base = filterByCategories(base, cfg.IncludeCategories, cfg.ExcludeCategories)
	return capCases(base, cfg.MaxCases)
}

func buildStressCorpus(cfg BuildConfig) []AttackCase {
	baseCfg := cfg
	baseCfg.BuildType = BuildBaseline
	base := buildBaselineCorpus(baseCfg)

	fc := cfg.ForgeCfg
	if len(fc.Mutators) == 0 {
		fc = ForgeConfig{
			Profile:      cfg.Profile,
			BaseCorpus:   base,
			UseTemplates: true,
			Mutators: []Mutator{
				MutatorSynonymParaphrase(),
				MutatorLengthen(),
				MutatorAddPoliteness(),
				MutatorAddAuthority(),
				MutatorShortenHard(240),
				MutatorKeepFirstSentence(),
				MutatorReorderClauses(),
				MutatorSandwichInjection(),
			},
			MaxBase: 100,
		}
	} else {
		if len(fc.BaseCorpus) == 0 {
			fc.BaseCorpus = base
		}
	}

	stress := ForgeCorpus(fc)
	stress = filterByCategories(stress, cfg.IncludeCategories, cfg.ExcludeCategories)
	return capCases(stress, cfg.MaxCases)
}

func buildFocusedCorpus(cfg BuildConfig) []AttackCase {
	baseCfg := cfg
	baseCfg.BuildType = BuildBaseline
	base := buildBaselineCorpus(baseCfg)
	focused := filterByCategories(base, cfg.IncludeCategories, cfg.ExcludeCategories)
	return capCases(focused, cfg.MaxCases)
}

func buildRandomizedCorpus(cfg BuildConfig) []AttackCase {
	baseCfg := cfg
	baseCfg.BuildType = BuildBaseline
	base := buildBaselineCorpus(baseCfg)

	seed := cfg.RandSeed
	if seed == 0 {
		seed = 42
	}
	r := rand.New(rand.NewSource(seed))
	idxs := r.Perm(len(base))
	n := len(idxs)
	if cfg.MaxCases > 0 && n > cfg.MaxCases {
		n = cfg.MaxCases
	}

	sampled := make([]AttackCase, n)
	for i := 0; i < n; i++ {
		sampled[i] = base[idxs[i]]
	}

	m := ChainMutators(MutatorSynonymParaphrase(), MutatorAddPoliteness())
	return MutateCases("rand", sampled, m)
}

// RegressionCorpusFromResults builds a corpus of only previously failing cases.
func RegressionCorpusFromResults(_ Profile, results []Result) []AttackCase {
	seen := make(map[string]bool)
	var out []AttackCase
	for _, r := range results {
		if r.Status == ResultUnsafe || r.Status == ResultBenignRefusal || r.Status == ResultLowQuality {
			if !seen[r.Case.ID] {
				seen[r.Case.ID] = true
				out = append(out, r.Case)
			}
		}
	}
	return out
}

func filterByCategories(cases []AttackCase, include, exclude []Category) []AttackCase {
	if len(include) == 0 && len(exclude) == 0 {
		return cases
	}
	includeSet := make(map[Category]bool, len(include))
	for _, c := range include {
		includeSet[c] = true
	}
	excludeSet := make(map[Category]bool, len(exclude))
	for _, c := range exclude {
		excludeSet[c] = true
	}

	out := make([]AttackCase, 0, len(cases))
	for _, c := range cases {
		if len(includeSet) > 0 && !includeSet[c.Category] {
			continue
		}
		if excludeSet[c.Category] {
			continue
		}
		out = append(out, c)
	}
	return out
}

func FilterByLengthHint(cases []AttackCase, hints []string) []AttackCase {
	if len(hints) == 0 {
		return cases
	}
	set := make(map[string]bool, len(hints))
	for _, h := range hints {
		set[h] = true
	}
	out := make([]AttackCase, 0, len(cases))
	for _, c := range cases {
		if set[c.LengthHint] {
			out = append(out, c)
		}
	}
	return out
}

func capCases(cases []AttackCase, max int) []AttackCase {
	if max > 0 && len(cases) > max {
		return cases[:max]
	}
	return cases
}

// FilterByTag returns cases whose Tags[key] matches one of the given values.
// Cases with no Tags map or missing key are excluded when values is non-empty.
func FilterByTag(cases []AttackCase, key string, values []string) []AttackCase {
	if len(values) == 0 {
		return cases
	}
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	out := make([]AttackCase, 0, len(cases))
	for _, c := range cases {
		if c.Tags != nil && set[c.Tags[key]] {
			out = append(out, c)
		}
	}
	return out
}

// SetTag stamps a tag on every case in the slice (mutates in place, returns slice).
func SetTag(cases []AttackCase, key, value string) []AttackCase {
	for i := range cases {
		if cases[i].Tags == nil {
			cases[i].Tags = make(map[string]string)
		}
		cases[i].Tags[key] = value
	}
	return cases
}
