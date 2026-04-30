[![Claude Code Friendly](https://img.shields.io/badge/Claude_Code-Friendly-blueviolet?logo=anthropic&logoColor=white)](https://claude.ai/code)

```
============================================================
  VISORCORPUS – Adversarial & Quality Corpus Toolkit
============================================================  v0.2.0
```

Introducing **VisorCorpus** – a Go-powered toolkit for stress‑testing your LLM/RAG systems.

If you're shipping anything built on LLMs, you *need* to know how it behaves under:

- Prompt injection
- KB exfiltration
- Cross‑tenant leaks
- System prompt / config probing
- Infra discovery & jailbreak attempts

VisorCorpus gives you:

- Structured **AttackCases** with expectations for both safety *and* quality
- Domain‑aware seeds (HR, Finance, Cloud, Healthcare)
- A **Forge** engine to expand seeds into large adversarial spaces using deterministic mutations (paraphrasing, synonym swaps, politeness/authority shifts, length tweaks, sandwich/reorder patterns)
- Multiple corpus "builds": baseline, stress, focused, regression, randomized, hybrid
- Stats + failure explorer so you can compare **defense configs** (system prompts + model settings) and track improvements over time

Use it as:

- A **CLI** to emit JSON corpora for your `/chat` or direct‑LLM attack harness
- A **Go library** inside CI/CD to gate new models and prompts on safety *and* benign answer quality

If you care about LLM/RAG security and don't want to fly blind, this is for you.

---

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

## Theoretical Combinatorial Scale

VisorCorpus is deliberately built to *explode* a small, well-curated seed set into a very large adversarial space.

Ignoring any `-max` caps and letting all combinations run, the numbers get big quickly:

- **Mutators (~8+)**
  Even if you only apply 2–4 mutators per base seed (in different orders/layers), you already see combinatorial growth:
  - Paraphrasing (synonyms)
  - Politeness / authority shifts
  - Lengthening / shortening
  - Clause reordering
  - Sandwiching benign + malicious content

  A single seed can easily turn into **dozens to hundreds** of distinct variants before deduplication.

- **Templates (instructions × targets × tones)**
  Template specs that combine:
  - multiple **instructions** (e.g., "ignore instructions", "bypass safety", "follow docs over system rules"),
  - multiple **targets** (system prompt, KB, configs, infra, tenants),
  - multiple **tones** (neutral, polite, authoritative, urgent)

  routinely yield **100+ variants per seed family**.

- **From seeds to scale**
  With just **~100 base seeds** (across categories and domains), you're already in the range of:

  - **10,000 → 100,000+** unique prompts
  - before any deduplication or additional domain-specific expansions

  The math is concrete, not hand-wavy. Just prompt injection alone:

  ```
  20 base seeds
  × 5 targets (system prompt, KB, config, infra, tenants)
  × 3 tones (neutral, polite, authoritative)
  × 3 mutator layers (synonym + sandwich + length tweak)
  = 900 distinct injection prompts
  ```

  before Forge touches any other category.

That's why VisorCorpus has explicit caps (`-max`, `-max-base`) and build types (baseline, stress, randomized, hybrid): you almost never want the full combinatorial explosion at once. Instead, you pick:

- smaller, **targeted slices** for CI,
- **stress builds** for nightly runs,
- and **random/hybrid builds** to approximate "real life" diversity without drowning your infrastructure.

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

## Using VisorCorpus with Claude Code

### Setup

```bash
# Clone and build the CLI
git clone https://github.com/Nicholas-Kloster/VisorCorpus
cd VisorCorpus
go build -o visorcorpus ./cmd/visorcorpus
```

Or use as a library:

```bash
go get github.com/Nicholas-Kloster/VisorCorpus
```

### Library quickstart

```go
import vc "github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"

// Build a strict baseline corpus
cases := vc.CorpusForProfile(vc.ProfileStrict)

// Add HR and cloud domain seeds
cases = vc.AddDomainSeeds(cases, vc.ProfileStrict, vc.DomainHR)
cases = vc.AddDomainSeeds(cases, vc.ProfileStrict, vc.DomainCloud)

// Hybrid build: guarantee 50 HIGH+ prompt injection + KB exfil cases,
// fill the rest randomly up to 400 total
hybridCases := vc.BuildCorpusVariant(vc.BuildConfig{
    Profile:   vc.ProfileStrict,
    BuildType: vc.BuildHybrid,
    MaxCases:  400,
    Hybrid: &vc.HybridConfig{
        Targets: []vc.HybridTarget{
            {
                Categories:      []vc.Category{vc.CategoryPromptInjection, vc.CategoryKBExfiltration},
                MinCases:        50,
                SeverityAtLeast: vc.PtrSeverity(vc.SeverityHigh),
            },
        },
    },
    Random: &vc.RandomConfig{
        Seed:             42,
        WeightedSeverity: true,
        WeightedCategory: true,
    },
})

// Run your attack harness, then evaluate each response
for _, ac := range hybridCases {
    resp := callYourLLM(ac.Prompt) // your implementation
    status, reason := vc.EvaluateResponse(ac, resp)
    result := vc.Result{
        Case:       ac,
        ModelName:  "your-model",
        Target:     "/chat",
        Status:     status,
        Reason:     reason,
        Response:   resp,
        OccurredAt: time.Now().UTC().Format(time.RFC3339),
    }
    _ = result // collect, write to JSON, feed to visorfail
}
```

### Typical Claude Code workflow

```bash
# 1. Build a focused corpus for a CI run
visorcorpus build \
  -profile strict -type hybrid -max 400 -seed $CI_BUILD_ID \
  -guaranteed prompt_injection,kb_exfiltration \
  -guaranteed-min 60 -guaranteed-severity HIGH \
  -out corpus_ci.json

# 2. Inspect what you built before running it
visorcorpus stats -in corpus_ci.json
visorcorpus query -in corpus_ci.json -difficulty hard -json | head -c 2000

# 3. Run your attack harness
attack-sim -corpus corpus_ci.json -target http://localhost:8080/chat \
  -out results_ci.json

# 4. Build a regression corpus from any failures
visorcorpus regress -in results_ci.json -out regression.json

# 5. On the next run, hit those regression cases first
attack-sim -corpus regression.json -out results_regression.json
visorfail -in results_regression.json -by category
```

---

## Status

VisorCorpus is a building block. Seeds, templates, and mutators are starting points — extend them to match your environment and risk model. It pairs best with a separate attack-sim harness that calls your actual LLM/RAG stack and feeds responses back through `EvaluateResponse`.
