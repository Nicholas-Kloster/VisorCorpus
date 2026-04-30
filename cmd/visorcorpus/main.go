package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	vc "github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"
)

const version = "0.2.0"

const banner = `
 __     ___           ___              ___
 \ \   / (_)___ _ __ / __\___  _ __   / __\___  _ __  _   _
  \ \ / /| / __| '__/ /  / _ \| '_ \ / /  / _ \| '_ \| | | |
   \ V / | \__ \ | / /__| (_) | | | / /__| (_) | |_) | |_| |
    \_/  |_|___/_| \____/\___/|_| |_\____/\___/| .__/ \__, |
                                               |_|    |___/

  V I S O R C O R P U S  –  Adversarial & Quality Corpus Toolkit  v` + version

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			usage()
			return
		case "-v", "--version", "version":
			fmt.Println("visorcorpus version " + version)
			return
		case "--banner":
			fmt.Println(banner)
			return
		}
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "build", "b":
		buildCmd(args)
	case "forge", "f":
		forgeCmd(args)
	case "regress", "r":
		regressCmd(args)
	case "stats", "s":
		statsCmd(args)
	case "query", "q":
		queryCmd(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(banner)
	fmt.Println(`
Usage: visorcorpus <command> [options]

Commands:
  build  (b)   Build a corpus variant (baseline/stress/focused/randomized/hybrid)
  forge  (f)   Expand seeds into a large adversarial corpus via Forge
  regress (r)  Build a regression corpus from a previous results.json
  stats  (s)   Show stats for a corpus JSON file
  query  (q)   Query/filter a corpus JSON file

Run 'visorcorpus <command> -h' for detailed flags.

Examples:
  visorcorpus build -profile strict -type baseline -max 500 -out strict_500.json
  visorcorpus build -profile strict -type randomized -max 400 -seed 123 -out rand_400.json
  visorcorpus build -profile strict -type hybrid -max 600 \
    -guaranteed prompt_injection,kb_exfiltration -guaranteed-min 100 \
    -guaranteed-severity HIGH -out hybrid_600.json
  visorcorpus build -profile strict -domain hr,cloud -protocol -out hr_cloud.json
  visorcorpus forge -profile strict -templates=true -max-base 100 -max 5000 -out forged_5k.json
  visorcorpus stats -in forged_5k.json
  visorcorpus query -in forged_5k.json -domain hr -category kb_exfiltration -difficulty hard
  visorcorpus regress -in results.json -out regression.json`)
}

// ─── build ────────────────────────────────────────────────────────────────────

func buildCmd(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: visorcorpus build [options]

Build a corpus variant from built-in seeds, templates, and optionally Forge.

Profile/type:
  -profile standard|strict|lenient   (default: standard)
  -type    baseline|stress|focused|randomized|hybrid   (default: baseline)

Filtering:
  -include  comma-separated categories to include (empty = all)
  -exclude  comma-separated categories to exclude
  -domain   domain seeds: hr|finance|cloud|healthcare
  -difficulty  filter output by Tags["difficulty"]: easy|medium|hard

Augmentation:
  -protocol         add protocol-level + tool-abuse seeds
  -difficulty-seeds add easy/medium/hard difficulty-labeled PI seeds

Randomized / hybrid:
  -seed               RNG seed (0 = time-based)
  -weighted-severity  bias toward CRITICAL/HIGH (default: true)
  -weighted-category  bias toward prompt_injection/kb_exfiltration (default: true)
  -weighted-domain    bias toward regulated domains (default: false)
  -guaranteed         hybrid: comma-separated categories always included
  -guaranteed-min     hybrid: min cases per guaranteed category (default: 50)
  -guaranteed-severity hybrid: CRITICAL|HIGH|MEDIUM|LOW threshold (empty = any)

Output:
  -max  max cases (0 = no limit)
  -out  output JSON file (default stdout)

Examples:
  visorcorpus build -profile strict -type baseline -max 500 -out strict_500.json
  visorcorpus build -type randomized -max 200 -seed 42 -out rand_200.json
  visorcorpus build -profile strict -type hybrid -max 600 \
    -guaranteed prompt_injection,kb_exfiltration -guaranteed-min 100 \
    -guaranteed-severity HIGH -out hybrid_600.json`)
	}
	profileStr := fs.String("profile", "standard", "Corpus profile: standard|strict|lenient")
	buildTypeStr := fs.String("type", "baseline", "Build type: baseline|stress|focused|randomized|hybrid")
	includeCats := fs.String("include", "", "Comma-separated categories to include (empty = all)")
	excludeCats := fs.String("exclude", "", "Comma-separated categories to exclude")
	domainStr := fs.String("domain", "", "Comma-separated domain seeds: hr|finance|cloud|healthcare")
	difficulty := fs.String("difficulty", "", "Filter by difficulty tag: easy|medium|hard (comma-separated, empty = all)")
	addProtocol := fs.Bool("protocol", false, "Include protocol-level and tool-abuse seeds")
	addDifficulty := fs.Bool("difficulty-seeds", false, "Include difficulty-layered prompt injection seeds")
	maxCases := fs.Int("max", 0, "Max cases (0 = no limit)")
	// Randomized / hybrid flags
	seed := fs.Int64("seed", 0, "RNG seed (0 = time-based, non-deterministic)")
	weightedSeverity := fs.Bool("weighted-severity", true, "Bias random selection by severity")
	weightedCategory := fs.Bool("weighted-category", true, "Bias random selection toward prompt_injection/kb_exfiltration")
	weightedDomain := fs.Bool("weighted-domain", false, "Bias random selection toward regulated domains")
	// Hybrid-specific
	hybridCats := fs.String("guaranteed", "", "Comma-separated categories always included in hybrid mode")
	hybridMin := fs.Int("guaranteed-min", 50, "Min cases per guaranteed category in hybrid mode")
	hybridSev := fs.String("guaranteed-severity", "", "Min severity for guaranteed slice: CRITICAL|HIGH|MEDIUM|LOW (empty = any)")
	out := fs.String("out", "", "Output JSON file (default stdout)")
	fs.Parse(args)

	p := parseProfile(*profileStr)
	bt := parseBuildType(*buildTypeStr)
	domains := parseDomains(*domainStr)

	rc := &vc.RandomConfig{
		Seed:             *seed,
		WeightedSeverity: *weightedSeverity,
		WeightedCategory: *weightedCategory,
		WeightedDomain:   *weightedDomain,
	}

	cfg := vc.BuildConfig{
		Profile:           p,
		BuildType:         bt,
		IncludeCategories: parseCategories(*includeCats),
		ExcludeCategories: parseCategories(*excludeCats),
		Domains:           domains,
		MaxCases:          *maxCases,
		Random:            rc,
	}

	if bt == vc.BuildHybrid && *hybridCats != "" {
		tgt := vc.HybridTarget{
			Categories: parseCategories(*hybridCats),
			MinCases:   *hybridMin,
		}
		if *hybridSev != "" {
			s := vc.Severity(strings.ToUpper(*hybridSev))
			tgt.SeverityAtLeast = vc.PtrSeverity(s)
		}
		cfg.Hybrid = &vc.HybridConfig{Targets: []vc.HybridTarget{tgt}}
	}

	cases := vc.BuildCorpusVariant(cfg)

	if *addProtocol {
		cases = vc.AddProtocolSeeds(cases, p)
	}
	if *addDifficulty {
		cases = vc.AddDifficultySeeds(cases, p)
	}
	for _, d := range domains {
		cases = vc.AddDomainCrossSeeds(cases, p, d)
	}

	if *difficulty != "" {
		diffs := strings.Split(*difficulty, ",")
		for i := range diffs {
			diffs[i] = strings.TrimSpace(diffs[i])
		}
		cases = vc.FilterByTag(cases, "difficulty", diffs)
	}

	writeJSON(cases, *out)
}

// ─── forge ────────────────────────────────────────────────────────────────────

func forgeCmd(args []string) {
	fs := flag.NewFlagSet("forge", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: visorcorpus forge [options]

Expand seeds into a large adversarial corpus using templates + mutators.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Example:
  visorcorpus forge -profile strict -templates=true -max-base 100 -max 5000 -out forged_5k.json`)
	}
	profileStr := fs.String("profile", "strict", "Profile: standard|strict|lenient")
	useTemplates := fs.Bool("templates", true, "Include template-generated cases")
	maxBase := fs.Int("max-base", 100, "Max base cases as forge seeds")
	maxCases := fs.Int("max", 0, "Max resulting cases (0 = no limit)")
	out := fs.String("out", "", "Output JSON file (default stdout)")
	fs.Parse(args)

	p := parseProfile(*profileStr)
	base := vc.CorpusForProfile(p)

	fc := vc.ForgeConfig{
		Profile:      p,
		BaseCorpus:   base,
		UseTemplates: *useTemplates,
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
		MaxBase: *maxBase,
	}

	cases := vc.ForgeCorpus(fc)
	if *maxCases > 0 && len(cases) > *maxCases {
		cases = cases[:*maxCases]
	}
	writeJSON(cases, *out)
}

// ─── regress ─────────────────────────────────────────────────────────────────

func regressCmd(args []string) {
	fs := flag.NewFlagSet("regress", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: visorcorpus regress -in results.json [-out regression.json]

Extract UNSAFE / BENIGN_REFUSAL / LOW_QUALITY cases from a previous results file
into a new corpus for targeted regression testing.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Example:
  visorcorpus regress -in results_sp_v1.json -out regression_cases.json`)
	}
	in := fs.String("in", "", "Input results JSON from attack-sim (required)")
	out := fs.String("out", "", "Output corpus JSON (default stdout)")
	fs.Parse(args)

	if *in == "" {
		fmt.Fprintln(os.Stderr, "-in is required")
		os.Exit(1)
	}

	f, err := os.Open(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var results []vc.Result
	if err := json.NewDecoder(f).Decode(&results); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	reg := vc.RegressionCorpusFromResults(vc.ProfileStandard, results)
	fmt.Fprintf(os.Stderr, "regression corpus: %d failing cases extracted from %d results\n", len(reg), len(results))
	writeJSON(reg, *out)
}

// ─── stats ────────────────────────────────────────────────────────────────────

func statsCmd(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: visorcorpus stats [-in file.json] [-profile standard|strict|lenient]

Show counts by category, severity, attack vector, domain, and length.
Pass -in for a pre-built file or -profile to build from a profile on the fly.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Examples:
  visorcorpus stats -in forged_5k.json
  visorcorpus stats -profile strict -type baseline`)
	}
	in := fs.String("in", "", "Input corpus JSON file (required)")
	profileStr := fs.String("profile", "", "Build from profile instead of file: standard|strict|lenient")
	buildTypeStr := fs.String("type", "baseline", "Build type when using -profile")
	fs.Parse(args)

	var cases []vc.AttackCase

	switch {
	case *in != "":
		f, err := os.Open(*in)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&cases); err != nil {
			fmt.Fprintf(os.Stderr, "decode: %v\n", err)
			os.Exit(1)
		}
	case *profileStr != "":
		cfg := vc.BuildConfig{
			Profile:   parseProfile(*profileStr),
			BuildType: parseBuildType(*buildTypeStr),
		}
		cases = vc.BuildCorpusVariant(cfg)
	default:
		fmt.Fprintln(os.Stderr, "-in or -profile is required")
		os.Exit(1)
	}

	printCorpusStats(cases)
}

func printCorpusStats(cases []vc.AttackCase) {
	catCounts := make(map[vc.Category]int)
	sevCounts := make(map[vc.Severity]int)
	vecCounts := make(map[vc.AttackVector]int)
	domCounts := make(map[vc.Domain]int)
	lenCounts := make(map[string]int)

	for _, c := range cases {
		catCounts[c.Category]++
		sevCounts[c.Severity]++
		hint := c.LengthHint
		if hint == "" {
			hint = "unset"
		}
		lenCounts[hint]++
		if c.AttackVector != "" {
			vecCounts[c.AttackVector]++
		}
		if c.Domain != vc.DomainNone {
			domCounts[c.Domain]++
		}
	}

	fmt.Printf("Total cases: %d\n\n", len(cases))
	printSortedCounts("CATEGORY", catCounts)
	printSortedCounts("SEVERITY", sevCounts)
	if len(vecCounts) > 0 {
		printSortedCounts("ATTACK VECTOR", vecCounts)
	}
	if len(domCounts) > 0 {
		printSortedCounts("DOMAIN", domCounts)
	}
	printSortedCounts("LENGTH", lenCounts)
}

func printSortedCounts[K ~string](label string, counts map[K]int) {
	type kv struct {
		k string
		n int
	}
	list := make([]kv, 0, len(counts))
	for k, n := range counts {
		list = append(list, kv{string(k), n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].k < list[j].k
	})
	fmt.Printf("%-30s COUNT\n", label)
	fmt.Println(strings.Repeat("─", 38))
	for _, item := range list {
		fmt.Printf("  %-28s %d\n", item.k, item.n)
	}
	fmt.Println()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func parseProfile(s string) vc.Profile {
	switch s {
	case "strict":
		return vc.ProfileStrict
	case "lenient":
		return vc.ProfileLenient
	default:
		return vc.ProfileStandard
	}
}

func parseBuildType(s string) vc.BuildType {
	switch s {
	case "stress":
		return vc.BuildStress
	case "focused":
		return vc.BuildFocused
	case "randomized":
		return vc.BuildRandomized
	default:
		return vc.BuildBaseline
	}
}

func parseCategories(s string) []vc.Category {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]vc.Category, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, vc.Category(p))
		}
	}
	return out
}

func parseDomains(s string) []vc.Domain {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]vc.Domain, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, vc.Domain(p))
		}
	}
	return out
}

// ─── query ────────────────────────────────────────────────────────────────────

func queryCmd(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: visorcorpus query -in file.json [filters]

Filter and inspect a corpus JSON file. All filters are optional; omitting one
means "any value". Multiple values for one filter are comma-separated.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Examples:
  # HR KB exfil, hard difficulty, medium length
  visorcorpus query -in forged_5k.json -domain hr -category kb_exfiltration -difficulty hard -length medium

  # Count cloud config_secrets cases
  visorcorpus query -in forged_5k.json -domain cloud -category config_secrets -count

  # JSON output for piping
  visorcorpus query -in forged_5k.json -difficulty hard -json > hard.json`)
	}
	in := fs.String("in", "", "Input corpus JSON file (required)")
	profileStr := fs.String("profile", "", "Filter by profile: standard|strict|lenient (empty = any)")
	catStr := fs.String("category", "", "Comma-separated categories (empty = any)")
	domStr := fs.String("domain", "", "Comma-separated domains: hr|finance|cloud|healthcare (empty = any)")
	diffStr := fs.String("difficulty", "", "Comma-separated difficulty tags (empty = any)")
	lengthStr := fs.String("length", "", "Comma-separated length hints: short|medium|long (empty = any)")
	vecStr := fs.String("vector", "", "Comma-separated attack vectors (empty = any)")
	limit := fs.Int("limit", 20, "Max matching cases to print (0 = no limit)")
	asJSON := fs.Bool("json", false, "Output matching cases as JSON array")
	count := fs.Bool("count", false, "Print only match count, not cases")
	fs.Parse(args)

	if *in == "" {
		fmt.Fprintln(os.Stderr, "-in is required")
		os.Exit(1)
	}

	f, err := os.Open(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var cases []vc.AttackCase
	if err := json.NewDecoder(f).Decode(&cases); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	profFilter := parseProfilePtr(*profileStr)
	catFilter := parseStringSet(*catStr)
	domFilter := parseStringSet(*domStr)
	diffFilter := parseStringSet(*diffStr)
	lenFilter := parseStringSet(*lengthStr)
	vecFilter := parseStringSet(*vecStr)

	var matches []vc.AttackCase
	for _, c := range cases {
		if !matchProfilePtr(c.Profile, profFilter) {
			continue
		}
		if !matchStringSet(string(c.Category), catFilter) {
			continue
		}
		if !matchStringSet(string(c.Domain), domFilter) {
			continue
		}
		if !matchStringSet(c.LengthHint, lenFilter) {
			continue
		}
		if !matchStringSet(string(c.AttackVector), vecFilter) {
			continue
		}
		if !matchTag(c.Tags, "difficulty", diffFilter) {
			continue
		}
		matches = append(matches, c)
	}

	if *count {
		fmt.Printf("%d\n", len(matches))
		return
	}

	if *limit > 0 && len(matches) > *limit {
		matches = matches[:*limit]
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(matches); err != nil {
			fmt.Fprintf(os.Stderr, "encode: %v\n", err)
			os.Exit(1)
		}
		return
	}

	printMatchesHuman(matches)
}

func parseProfilePtr(s string) *vc.Profile {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return nil
	}
	p := parseProfile(s)
	return &p
}

func parseStringSet(s string) map[string]bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	set := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			set[p] = true
		}
	}
	return set
}

func matchProfilePtr(p vc.Profile, filter *vc.Profile) bool {
	if filter == nil {
		return true
	}
	return p == *filter
}

func matchStringSet(val string, set map[string]bool) bool {
	if set == nil {
		return true
	}
	return set[strings.ToLower(val)]
}

func matchTag(tags map[string]string, key string, set map[string]bool) bool {
	if set == nil {
		return true
	}
	if tags == nil {
		return false
	}
	return set[strings.ToLower(tags[key])]
}

func printMatchesHuman(cases []vc.AttackCase) {
	if len(cases) == 0 {
		fmt.Println("No matching cases.")
		return
	}
	for _, c := range cases {
		fmt.Printf("ID: %s\n", c.ID)
		fmt.Printf("  Profile:   %s\n", c.Profile)
		fmt.Printf("  Category:  %s\n", c.Category)
		if c.Domain != vc.DomainNone {
			fmt.Printf("  Domain:    %s\n", c.Domain)
		}
		if c.AttackVector != "" {
			fmt.Printf("  Vector:    %s\n", c.AttackVector)
		}
		fmt.Printf("  Severity:  %s\n", c.Severity)
		if c.LengthHint != "" {
			fmt.Printf("  Length:    %s\n", c.LengthHint)
		}
		if len(c.Tags) > 0 {
			parts := make([]string, 0, len(c.Tags))
			for k, v := range c.Tags {
				parts = append(parts, k+"="+v)
			}
			sort.Strings(parts)
			fmt.Printf("  Tags:      %s\n", strings.Join(parts, ", "))
		}
		fmt.Printf("  Desc:      %s\n", c.Description)
		fmt.Printf("  Prompt:\n%s\n\n", indentBlock(c.Prompt, "    "))
	}
	fmt.Printf("── %d case(s) ──\n", len(cases))
}

func indentBlock(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func writeJSON(v any, path string) {
	var f *os.File
	var err error
	if path == "" {
		f = os.Stdout
	} else {
		f, err = os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}
