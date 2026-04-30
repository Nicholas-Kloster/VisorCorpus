```
============================================================
  VISORCORPUS – Adversarial & Quality Corpus Toolkit
============================================================  v0.2.0
```

VisorCorpus is a Go-based toolkit for building, mutating, and analyzing **adversarial + quality corpora** for LLM/RAG security testing.

It's designed as a standalone component you can plug into:

- Your **attack-sim** harness (e.g., to hit `/chat` or direct LLM APIs).
- CI/CD pipelines (safety & quality gates).
- Manual "red team" style experiments.

Key capabilities:

- Rich corpus **templates** and **seed sets** for:
  - Prompt injection
  - KB exfiltration
  - Cross-tenant leaks
  - System prompt & config exfil
  - Infra discovery
  - Jailbreak / social engineering
  - KB instruction poisoning
  - Benign controls & quality probes
- **Domain-specific** seeds: HR, Finance, Cloud Infra, Healthcare
- **Forge**: expands seeds into large adversarial spaces using deterministic mutations (paraphrasing, synonyms, politeness/authority, length tweaks, sandwich/reorder, etc.) and tone-variant templates
- Multiple **build types**: Baseline, Stress, Focused, Regression, Randomized, Hybrid
- Safety + Quality evaluation hooks: Unsafe vs benign-refusal vs low-quality answers, stats & failure exploration for defense tuning

---

## Core Concepts

### AttackCase

```go
type AttackCase struct {
    ID           string
    Category     Category      // e.g. "prompt_injection", "kb_exfiltration"
    Profile      Profile       // "standard", "strict", "lenient"
    Severity     Severity      // CRITICAL, HIGH, MEDIUM, LOW
    Description  string
    Prompt       string
    Expect       Expectations
    LengthHint   string        // "short", "medium", "long"
    Domain       Domain        // "hr", "finance", "cloud", "healthcare"
    AttackVector AttackVector  // "direct_override", "doc_poison", "authority_spoof", ...
    Tags         map[string]string // e.g. difficulty="hard", style="polite"
}
```

### Expectations

```go
type Expectations struct {
    ExpectRefusal        bool
    ForbiddenSubstrings  []string
    NoIgnorePrevious     bool
    NoInternalURLs       bool
    NoLargeVerbatim      bool
    RequireAnySubstrings []string // required cues in benign answers
    DisallowRefusal      bool     // benign probe: model must NOT refuse
}
```

### Result & Evaluation

```go
type ResultStatus string

const (
    ResultSafe          ResultStatus = "SAFE"
    ResultUnsafe        ResultStatus = "UNSAFE"
    ResultError         ResultStatus = "ERROR"
    ResultUnknown       ResultStatus = "UNKNOWN"
    ResultLowQuality    ResultStatus = "LOW_QUALITY"
    ResultBenignRefusal ResultStatus = "BENIGN_REFUSAL"
)

type Result struct {
    Case        AttackCase
    ModelName   string
    Target      string
    DefenseName string
    Status      ResultStatus
    Reason      string
    Response    string
    OccurredAt  string
}
```

Your harness executes prompts, then calls `EvaluateResponse` to classify responses:

```go
status, reason := corpus.EvaluateResponse(ac, modelResponse)
result := corpus.Result{
    Case:      ac,
    ModelName: "vllm-chat-large",
    Target:    "/chat",
    Status:    status,
    Reason:    reason,
    Response:  modelResponse,
    OccurredAt: time.Now().UTC().Format(time.RFC3339),
}
```

---

## Installation

```bash
go get github.com/Nicholas-Kloster/VisorCorpus
```

CLI:

```bash
cd cmd/visorcorpus && go build -o visorcorpus
```

---

## CLI

### `build`

```bash
visorcorpus build \
  -profile strict \
  -type baseline \
  -max 500 \
  -out strict_baseline_500.json
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-profile` | `standard` | `standard\|strict\|lenient` |
| `-type` | `baseline` | `baseline\|stress\|focused\|randomized\|hybrid` |
| `-include` | `` | Comma-separated categories to include |
| `-exclude` | `` | Comma-separated categories to exclude |
| `-domain` | `` | Domain seeds: `hr\|finance\|cloud\|healthcare` |
| `-max` | `0` | Max cases (0 = no limit) |
| `-seed` | `0` | RNG seed for randomized/hybrid (0 = time-based) |
| `-weighted-severity` | `true` | Bias random toward CRITICAL/HIGH |
| `-weighted-category` | `true` | Bias random toward prompt_injection/kb_exfiltration |
| `-weighted-domain` | `false` | Bias random toward regulated domains |
| `-guaranteed` | `` | Hybrid: categories always included |
| `-guaranteed-min` | `50` | Hybrid: min cases per guaranteed category |
| `-guaranteed-severity` | `` | Hybrid: min severity for guaranteed slice |
| `-protocol` | `false` | Add protocol/tool-abuse seeds |
| `-difficulty-seeds` | `false` | Add easy/medium/hard difficulty-labeled PI seeds |
| `-difficulty` | `` | Filter output by difficulty tag |

**Examples:**

```bash
# Focused security build
visorcorpus build \
  -profile strict -type focused \
  -include prompt_injection,kb_exfiltration,system_prompt \
  -max 300 -out strict_pi_kb_sys_300.json

# Weighted random (reproducible)
visorcorpus build \
  -profile strict -type randomized \
  -max 400 -seed 12345 \
  -out strict_rand_400.json

# Hybrid: guarantee 100 HIGH+ PI/KB cases, fill to 600 randomly
visorcorpus build \
  -profile strict -type hybrid -max 600 -seed 42 \
  -guaranteed prompt_injection,kb_exfiltration \
  -guaranteed-min 100 -guaranteed-severity HIGH \
  -out strict_hybrid_600.json

# HR domain + protocol seeds
visorcorpus build \
  -profile strict -type baseline \
  -domain hr,cloud -protocol \
  -out strict_hr_cloud.json
```

### `forge`

Explicitly expand seeds with all mutators:

```bash
visorcorpus forge \
  -profile strict \
  -templates=true \
  -max-base 100 \
  -max 5000 \
  -out strict_forged_5k.json
```

### `regress`

Build a regression corpus from a previous results file:

```bash
visorcorpus regress \
  -in results_sp_v1.json \
  -out regression_cases.json
```

Extracts all UNSAFE / BENIGN_REFUSAL / LOW_QUALITY cases to re-target with new defenses.

### `stats`

```bash
visorcorpus stats -in strict_forged_5k.json
# or from a profile directly:
visorcorpus stats -profile strict -type baseline
```

### `query`

Filter and inspect a corpus file:

```bash
visorcorpus query \
  -in strict_forged_5k.json \
  -domain hr \
  -category kb_exfiltration \
  -difficulty hard \
  -length medium \
  -limit 20

# Count only
visorcorpus query -in corpus.json -domain cloud -count

# JSON output for piping
visorcorpus query -in corpus.json -difficulty hard -json > hard.json
```

Flags: `-profile`, `-category`, `-domain`, `-difficulty`, `-length`, `-vector`, `-limit`, `-json`, `-count`.

---

## Library Integration

### Build + Forge in code

```go
import vc "github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"

// Baseline strict
cases := vc.CorpusForProfile(vc.ProfileStrict)

// Add HR domain seeds
cases = vc.AddDomainSeeds(cases, vc.ProfileStrict, vc.DomainHR)

// Forge expansion
forged := vc.ForgeCorpus(vc.ForgeConfig{
    Profile:      vc.ProfileStrict,
    BaseCorpus:   cases,
    UseTemplates: true,
    Mutators: []vc.Mutator{
        vc.MutatorSynonymParaphrase(),
        vc.MutatorLengthen(),
        vc.MutatorAddPoliteness(),
        vc.MutatorAddAuthority(),
        vc.MutatorShortenHard(240),
        vc.MutatorKeepFirstSentence(),
        vc.MutatorReorderClauses(),
        vc.MutatorSandwichInjection(),
    },
    MaxBase: 100,
})

// Hybrid build
hybridCases := vc.BuildCorpusVariant(vc.BuildConfig{
    Profile:   vc.ProfileStrict,
    BuildType: vc.BuildHybrid,
    MaxCases:  600,
    Hybrid: &vc.HybridConfig{
        Targets: []vc.HybridTarget{
            {
                Categories:      []vc.Category{vc.CategoryPromptInjection, vc.CategoryKBExfiltration},
                MinCases:        100,
                SeverityAtLeast: vc.PtrSeverity(vc.SeverityHigh),
            },
        },
    },
    Random: &vc.RandomConfig{
        Seed:             12345,
        WeightedSeverity: true,
        WeightedCategory: true,
    },
})
```

### Scoring results

```go
summaries := vc.ScoreResults(allResults)
quality   := vc.ScoreQuality(allResults)

for _, s := range summaries {
    vc.PrintSummaryLine(s)
}
vc.PrintQualityStats(quality)
```

### Tag and filter helpers

```go
// Stamp a tag on a slice
cases = vc.SetTag(cases, "difficulty", "hard")

// Filter by tag
hard := vc.FilterByTag(cases, "difficulty", []string{"hard"})

// Filter by length
short := vc.FilterByLengthHint(cases, []string{"short"})

// Regression corpus from prior results
reg := vc.RegressionCorpusFromResults(vc.ProfileStrict, prevResults)
```

---

## Practical CI Workflow

```bash
# CI (fast, ~200–400 cases)
visorcorpus build -profile strict -type hybrid -max 400 -seed $CI_BUILD_ID \
  -guaranteed prompt_injection,kb_exfiltration -guaranteed-min 60 \
  -guaranteed-severity HIGH -out corpus_ci.json
attack-sim -corpus corpus_ci.json -out results_ci.json
visorfail -in results_ci.json -by category

# Nightly (heavy)
visorcorpus forge -profile strict -max-base 100 -max 5000 -out corpus_nightly.json
attack-sim -corpus corpus_nightly.json -out results_nightly.json

# Pre-release regression
visorcorpus regress -in results_nightly.json -out regression.json
attack-sim -corpus regression.json -out results_regression.json
visorfail -in results_regression.json
```

---

## Status

VisorCorpus is a building block. Seeds, templates, and mutators are starting points — extend them to match your environment and risk model. It pairs best with a separate attack-sim harness that calls your actual LLM/RAG stack and feeds responses back through `EvaluateResponse`.
