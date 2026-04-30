package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"
)

func main() {
	profileStr := flag.String("profile", "standard", "Base profile: standard|strict|lenient")
	useTemplates := flag.Bool("templates", true, "Include template-generated cases")
	maxBase := flag.Int("max-base", 100, "Max seed cases to mutate (0=all)")
	out := flag.String("out", "", "Output JSON file (default stdout)")
	statsOnly := flag.Bool("stats", false, "Print stats instead of dumping JSON")
	flag.Parse()

	var p corpus.Profile
	switch *profileStr {
	case "strict":
		p = corpus.ProfileStrict
	case "lenient":
		p = corpus.ProfileLenient
	default:
		p = corpus.ProfileStandard
	}

	seeds := corpus.CorpusForProfile(p)

	cfg := corpus.ForgeConfig{
		Profile:      p,
		BaseCorpus:   seeds,
		UseTemplates: *useTemplates,
		Mutators: []corpus.Mutator{
			corpus.MutatorAddPoliteness(),
			corpus.MutatorRemovePoliteness(),
			corpus.MutatorAddAuthority(),
			corpus.MutatorSynonymParaphrase(),
			corpus.MutatorLengthen(),
			corpus.MutatorShortenHard(180),
			corpus.MutatorKeepFirstSentence(),
			corpus.MutatorAddUrgency(),
			corpus.MutatorSandwichInjection(),
			corpus.MutatorReorderClauses(),
		},
		MaxBase: *maxBase,
	}

	cases := corpus.ForgeCorpus(cfg)

	if *statsOnly {
		catCounts := map[corpus.Category]int{}
		for _, c := range cases {
			catCounts[c.Category]++
		}
		fmt.Printf("Profile: %s   Total: %d\n\n", *profileStr, len(cases))
		fmt.Printf("%-30s %s\n", "CATEGORY", "COUNT")
		fmt.Printf("%s\n", "─────────────────────────────────────")
		for cat, n := range catCounts {
			fmt.Printf("%-30s %d\n", cat, n)
		}
		return
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cases); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
