package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"
)

func main() {
	cmd := flag.String("cmd", "list", "Command: list|dump|stats")
	profileStr := flag.String("profile", "standard", "Corpus profile: standard|strict|lenient")
	categoryStr := flag.String("category", "all", "Category filter (or 'all')")
	out := flag.String("out", "", "Output file for dump (default stdout)")
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

	allCases := corpus.CorpusForProfile(p)
	var cases []corpus.AttackCase
	if *categoryStr == "all" {
		cases = allCases
	} else {
		want := corpus.Category(*categoryStr)
		for _, c := range allCases {
			if c.Category == want {
				cases = append(cases, c)
			}
		}
	}

	switch *cmd {
	case "list":
		for _, c := range cases {
			fmt.Printf("%-40s [%s / %s / %s / %s]\n",
				c.ID, c.Profile, c.Category, c.Severity, c.LengthHint)
			fmt.Printf("  %s\n", c.Description)
		}
		fmt.Printf("\nTotal: %d cases\n", len(cases))

	case "stats":
		type key struct{ cat, sev string }
		counts := map[key]int{}
		for _, c := range cases {
			counts[key{string(c.Category), string(c.Severity)}]++
		}
		fmt.Printf("Profile: %s   Total: %d\n\n", *profileStr, len(cases))
		fmt.Printf("%-30s %-10s %s\n", "CATEGORY", "SEVERITY", "COUNT")
		fmt.Printf("%s\n", "──────────────────────────────────────────────────")
		for k, n := range counts {
			fmt.Printf("%-30s %-10s %d\n", k.cat, k.sev, n)
		}

	case "dump":
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

	default:
		fmt.Fprintf(os.Stderr, "unknown cmd: %s\n", *cmd)
		os.Exit(1)
	}
}
