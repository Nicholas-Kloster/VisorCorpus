package corpus

import (
	"math/rand"
	"time"
)

type BuildType string

const (
	BuildBaseline   BuildType = "baseline"
	BuildStress     BuildType = "stress"
	BuildFocused    BuildType = "focused"
	BuildRegression BuildType = "regression"
	BuildRandomized BuildType = "randomized"
	BuildHybrid     BuildType = "hybrid"
)

// CoverageRequirement guarantees at least MinCases from a given Category (and
// optional Domain) before random-fill claims remaining slots.
type CoverageRequirement struct {
	Category Category
	Domain   Domain // empty = any domain
	MinCases int
}

// RandomConfig controls weighted sampling for BuildRandomized and the random
// fill phase of BuildHybrid.
// Seed=0 uses time.Now().UnixNano() (non-deterministic).
type RandomConfig struct {
	Seed             int64
	WeightedSeverity bool // bias toward CRITICAL/HIGH (3×/2× weight)
	WeightedCategory bool // bias toward prompt_injection + kb_exfiltration (2×)
	WeightedDomain   bool // bias toward regulated domains: hr/finance/healthcare (1.5×)
	Coverage         []CoverageRequirement
}

// HybridTarget defines a mandatory slice that must be satisfied before the
// random fill phase. SeverityAtLeast=nil means any severity.
type HybridTarget struct {
	Categories      []Category
	MinCases        int
	SeverityAtLeast *Severity
}

// HybridConfig is used only for BuildHybrid.
type HybridConfig struct {
	Targets []HybridTarget
}

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

	// Random controls sampling for BuildRandomized / hybrid fill. nil = flat weights, seed=time.
	Random *RandomConfig
	// Hybrid configures mandatory targeted slices for BuildHybrid.
	Hybrid *HybridConfig
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
	case BuildHybrid:
		cases = buildHybridCorpus(cfg)
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
	if len(base) == 0 {
		return nil
	}
	rc := cfg.Random
	if rc == nil {
		rc = &RandomConfig{}
	}
	n := cfg.MaxCases
	if n <= 0 {
		n = len(base)
	}
	sampled := randomizedFromPool(base, rc, n)
	m := ChainMutators(MutatorSynonymParaphrase(), MutatorAddPoliteness(), MutatorLengthen())
	return MutateCases("rand", sampled, m)
}

func buildHybridCorpus(cfg BuildConfig) []AttackCase {
	baseCfg := cfg
	baseCfg.BuildType = BuildBaseline
	base := buildBaselineCorpus(baseCfg)
	if len(base) == 0 {
		return nil
	}

	hybrid := cfg.Hybrid
	if hybrid == nil {
		return buildRandomizedCorpus(cfg)
	}

	remaining := make([]AttackCase, len(base))
	copy(remaining, base)

	var out []AttackCase
	for _, tgt := range hybrid.Targets {
		slice := filterByCategories(remaining, tgt.Categories, nil)
		if tgt.SeverityAtLeast != nil {
			slice = filterByMinSeverity(slice, *tgt.SeverityAtLeast)
		}
		if len(slice) == 0 {
			continue
		}
		n := tgt.MinCases
		if n <= 0 || n > len(slice) {
			n = len(slice)
		}
		picked := slice[:n]
		out = append(out, picked...)
		remaining = removeCasesByID(remaining, picked)
	}

	slots := cfg.MaxCases - len(out)
	if slots <= 0 || len(remaining) == 0 {
		return out
	}

	rc := cfg.Random
	if rc == nil {
		rc = &RandomConfig{WeightedSeverity: true, WeightedCategory: true}
	}
	fill := randomizedFromPool(remaining, rc, slots)
	return append(out, fill...)
}

// randomizedFromPool samples up to n cases from pool using weighted selection.
// Coverage requirements are satisfied first, then remaining slots filled randomly.
func randomizedFromPool(pool []AttackCase, rc *RandomConfig, n int) []AttackCase {
	if len(pool) == 0 || n <= 0 {
		return nil
	}

	seed := rc.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(seed))

	remaining := make([]AttackCase, len(pool))
	copy(remaining, pool)

	var out []AttackCase

	// Coverage phase: guarantee minimum per category/domain.
	for _, cov := range rc.Coverage {
		if len(out) >= n {
			break
		}
		slice := filterByCategories(remaining, []Category{cov.Category}, nil)
		if cov.Domain != DomainNone {
			slice = filterByDomain(slice, cov.Domain)
		}
		if len(slice) == 0 {
			continue
		}
		want := cov.MinCases
		if want > n-len(out) {
			want = n - len(out)
		}
		if want > len(slice) {
			want = len(slice)
		}
		sel := slice[:want]
		out = append(out, sel...)
		remaining = removeCasesByID(remaining, sel)
	}

	// Weighted random fill phase.
	slots := n - len(out)
	if slots <= 0 || len(remaining) == 0 {
		return out
	}

	weights := make([]float64, len(remaining))
	for i, c := range remaining {
		weights[i] = randWeight(c, rc)
	}
	normalizeWeights(weights)

	for len(out) < n && totalWeight(weights) > 0 {
		idx := weightedPick(r, weights)
		out = append(out, remaining[idx])
		weights[idx] = 0
		normalizeWeights(weights)
	}

	return out
}

func randWeight(c AttackCase, rc *RandomConfig) float64 {
	w := 1.0
	if rc.WeightedSeverity {
		switch c.Severity {
		case SeverityCritical:
			w *= 3.0
		case SeverityHigh:
			w *= 2.0
		case SeverityLow:
			w *= 0.5
		}
	}
	if rc.WeightedCategory {
		if c.Category == CategoryPromptInjection || c.Category == CategoryKBExfiltration {
			w *= 2.0
		}
	}
	if rc.WeightedDomain {
		switch c.Domain {
		case DomainHR, DomainFinance, DomainHealthcare:
			w *= 1.5
		}
	}
	return w
}

func filterByMinSeverity(cases []AttackCase, min Severity) []AttackCase {
	out := make([]AttackCase, 0, len(cases))
	for _, c := range cases {
		if severityRank(c.Severity) >= severityRank(min) {
			out = append(out, c)
		}
	}
	return out
}

func filterByDomain(cases []AttackCase, d Domain) []AttackCase {
	out := make([]AttackCase, 0, len(cases))
	for _, c := range cases {
		if c.Domain == d {
			out = append(out, c)
		}
	}
	return out
}

func removeCasesByID(pool []AttackCase, toRemove []AttackCase) []AttackCase {
	rm := make(map[string]bool, len(toRemove))
	for _, c := range toRemove {
		rm[c.ID] = true
	}
	out := make([]AttackCase, 0, len(pool)-len(toRemove))
	for _, c := range pool {
		if !rm[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	}
	return 0
}

func normalizeWeights(ws []float64) {
	sum := totalWeight(ws)
	if sum == 0 {
		return
	}
	for i := range ws {
		ws[i] /= sum
	}
}

func totalWeight(ws []float64) float64 {
	sum := 0.0
	for _, w := range ws {
		sum += w
	}
	return sum
}

func weightedPick(r *rand.Rand, ws []float64) int {
	x := r.Float64()
	acc := 0.0
	for i, w := range ws {
		acc += w
		if x <= acc {
			return i
		}
	}
	return len(ws) - 1
}

// PtrSeverity is a convenience for building HybridTarget.SeverityAtLeast inline.
func PtrSeverity(s Severity) *Severity { return &s }

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
