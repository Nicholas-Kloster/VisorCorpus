package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"
)

func main() {
	in := flag.String("in", "", "Results JSON file from attack-sim (required)")
	by := flag.String("by", "category", "Group by: category|severity|model|reason")
	flag.Parse()

	if *in == "" {
		fmt.Fprintln(os.Stderr, "-in is required")
		os.Exit(1)
	}

	f, err := os.Open(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer f.Close()

	var results []corpus.Result
	if err := json.NewDecoder(f).Decode(&results); err != nil {
		fmt.Fprintln(os.Stderr, "decode:", err)
		os.Exit(1)
	}

	var unsafe []corpus.Result
	for _, r := range results {
		if r.Status == corpus.ResultUnsafe {
			unsafe = append(unsafe, r)
		}
	}

	fmt.Printf("Total: %d  Unsafe: %d  Safe: %d\n\n",
		len(results), len(unsafe), len(results)-len(unsafe))

	if len(unsafe) == 0 {
		fmt.Println("No UNSAFE results.")
		return
	}

	// also show quality failures separately
	var qualityFails []corpus.Result
	for _, r := range results {
		if r.Status == corpus.ResultBenignRefusal || r.Status == corpus.ResultLowQuality {
			qualityFails = append(qualityFails, r)
		}
	}
	if len(qualityFails) > 0 {
		fmt.Printf("Quality failures: %d (benign_refusal + low_quality)\n\n", len(qualityFails))
	}

	switch *by {
	case "severity":
		counts := map[corpus.Severity]int{}
		for _, r := range unsafe {
			counts[r.Case.Severity]++
		}
		printCounts("SEVERITY", counts)

	case "model":
		counts := map[string]int{}
		for _, r := range unsafe {
			counts[r.ModelName]++
		}
		printStringCounts("MODEL", counts)

	case "reason":
		counts := map[string]int{}
		for _, r := range unsafe {
			counts[r.Reason]++
		}
		printStringCounts("REASON", counts)

	default: // category
		counts := map[corpus.Category]int{}
		for _, r := range unsafe {
			counts[r.Case.Category]++
		}
		printCounts("CATEGORY", counts)
	}

	fmt.Println("\n=== Failing cases ===")
	for _, r := range unsafe {
		fmt.Printf("[%s] %s\n  Reason: %s\n  Prompt: %s\n\n",
			r.Case.Severity, r.Case.Description, r.Reason, truncate(r.Case.Prompt, 120))
	}
}

func printCounts[K ~string](label string, counts map[K]int) {
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range counts {
		list = append(list, kv{string(k), n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	fmt.Printf("=== By %s ===\n", label)
	for _, item := range list {
		fmt.Printf("  %-30s %d\n", item.k, item.n)
	}
	fmt.Println()
}

func printStringCounts(label string, counts map[string]int) {
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range counts {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	fmt.Printf("=== By %s ===\n", label)
	for _, item := range list {
		fmt.Printf("  %-40s %d\n", item.k, item.n)
	}
	fmt.Println()
}

func truncate(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max]) + "…"
}
