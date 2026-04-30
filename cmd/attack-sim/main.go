package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Nicholas-Kloster/VisorCorpus/pkg/corpus"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	red    = "\033[31m"
	yellow = "\033[33m"
	green  = "\033[32m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
	orange = "\033[38;5;208m"
)

type chatReq struct {
	Question string `json:"question"`
	TopK     int    `json:"top_k"`
}

type chatResp struct {
	Answer string `json:"answer"`
}

func callChat(ctx context.Context, client *http.Client, baseURL, apiKey, question string, topK int) (string, error) {
	buf, _ := json.Marshal(chatReq{Question: question, TopK: topK})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
	}
	var out chatResp
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("decode: %v body=%s", err, string(b))
	}
	return out.Answer, nil
}

func severityColor(s corpus.Severity) string {
	switch s {
	case corpus.SeverityCritical:
		return red + bold
	case corpus.SeverityHigh:
		return orange + bold
	case corpus.SeverityMedium:
		return yellow
	default:
		return cyan
	}
}

func main() {
	apiBase := flag.String("api", "http://localhost:8080", "RAG API base URL (appends /chat)")
	apiKey := flag.String("key", os.Getenv("CHAT_API_KEY"), "API key (X-API-Key or CHAT_API_KEY env)")
	modelName := flag.String("model", "unknown", "Model name label for results")
	corpusFile := flag.String("corpus", "", "Pre-built corpus JSON from visorcorpus (overrides -profile/-category)")
	profileStr := flag.String("profile", "standard", "Corpus profile: standard|strict|lenient")
	categoryStr := flag.String("category", "all", "Category filter: all|prompt_injection|kb_exfiltration|tenant_cross_leak|system_prompt|config_secrets|infra_discovery|jailbreak|kb_instructions|benign_control")
	topK := flag.Int("topk", 5, "top_k for retrieval")
	tenant := flag.String("tenant", "", "Replace {{TENANT}} in prompts")
	email := flag.String("email", "", "Replace {{EMAIL}} in prompts")
	docID := flag.String("doc", "", "Replace {{DOC_ID}} in prompts")
	format := flag.String("format", "text", "Output format: text|json|csv")
	out := flag.String("out", "", "Write results to file (default stdout)")
	timeout := flag.Int("timeout", 20, "Per-request timeout in seconds")
	dryRun := flag.Bool("dry-run", false, "Print prompts without calling the API")
	failOn := flag.String("fail-on", "", "Exit 2 if any UNSAFE result at or above: CRITICAL|HIGH|MEDIUM|LOW")
	defenseConfig := flag.String("defense-config", "", "Path to JSON DefenseConfig file")
	defenseName := flag.String("defense-name", "", "Name of DefenseConfig to load (tags results)")
	flag.Parse()

	var cases []corpus.AttackCase

	if *corpusFile != "" {
		cf, err := os.Open(*corpusFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "open corpus:", err)
			os.Exit(1)
		}
		defer cf.Close()
		if err := json.NewDecoder(cf).Decode(&cases); err != nil {
			fmt.Fprintln(os.Stderr, "decode corpus:", err)
			os.Exit(1)
		}
	} else {
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

		if *categoryStr == "all" {
			cases = allCases
		} else {
			want := corpus.Category(*categoryStr)
			for _, c := range allCases {
				if c.Category == want {
					cases = append(cases, c)
				}
			}
			if len(cases) == 0 {
				fmt.Fprintf(os.Stderr, "no cases found for category=%s profile=%s\n", *categoryStr, *profileStr)
				os.Exit(1)
			}
		}
	}

	var transforms []corpus.Transform
	if *tenant != "" {
		transforms = append(transforms, corpus.WithTenant(*tenant))
	}
	if *email != "" {
		transforms = append(transforms, corpus.WithEmail(*email))
	}
	if *docID != "" {
		transforms = append(transforms, corpus.WithDocID(*docID))
	}
	if len(transforms) > 0 {
		cases = corpus.ApplyTransform(cases, corpus.Chain(transforms...))
	}

	// Load defense config if provided (used for tagging results)
	activeDefenseName := *defenseName
	if *defenseConfig != "" {
		cfgs, err := corpus.LoadDefenseConfigs(*defenseConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error loading defense config:", err)
			os.Exit(1)
		}
		if *defenseName != "" {
			def, err := corpus.FindDefenseConfig(cfgs, *defenseName)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			activeDefenseName = def.Name
			if *modelName == "unknown" && def.Model.Name != "" {
				*modelName = def.Model.Name
			}
		}
	}

	if *dryRun {
		fmt.Printf("\n%s%s ATTACK-SIM DRY RUN%s  profile=%s  cases=%d\n\n", bold, cyan, reset, *profileStr, len(cases))
		for _, c := range cases {
			fmt.Printf("  %s[%s]%s [%s] %s\n  %s%s%s\n\n",
				severityColor(c.Severity), c.Severity, reset,
				c.Category, c.Description,
				gray, c.Prompt, reset)
		}
		return
	}

	if *apiKey == "" {
		fmt.Fprintln(os.Stderr, "warn: no API key set (-key or CHAT_API_KEY)")
	}

	client := &http.Client{Timeout: time.Duration(*timeout) * time.Second}
	ctx := context.Background()

	var results []corpus.Result

	fmt.Printf("\n%s%s ATTACK-SIM%s  profile=%s  target=%s  cases=%d\n\n",
		bold, cyan, reset, *profileStr, *apiBase, len(cases))

	for _, ac := range cases {
		resp, err := callChat(ctx, client, *apiBase, *apiKey, ac.Prompt, *topK)
		r := corpus.NewResult(ac, *modelName, *apiBase+"/chat", resp, err)
		r.DefenseName = activeDefenseName
		results = append(results, r)

		switch r.Status {
		case corpus.ResultUnsafe:
			fmt.Printf("%s[UNSAFE]%s  %s\n  %s%s%s\n\n",
				red+bold, reset, ac.Description, yellow, r.Reason, reset)
		case corpus.ResultError:
			fmt.Printf("%s[ERROR]%s   %s: %s\n", yellow, reset, ac.ID, r.Reason)
		default:
			fmt.Printf("%s[SAFE]%s    %s\n", green, reset, ac.Description)
		}
	}

	unsafeCount := 0
	for _, r := range results {
		if r.Status == corpus.ResultUnsafe {
			unsafeCount++
		}
	}
	fmt.Printf("\n%sSummary:%s %d/%d UNSAFE\n\n", bold, reset, unsafeCount, len(results))

	summaries := corpus.ScoreResults(results)
	fmt.Println("=== VisorCorpus Security Summary ===")
	for _, s := range summaries {
		corpus.PrintSummaryLine(s)
	}
	corpus.PrintQualityStats(corpus.ScoreQuality(results))
	fmt.Println()

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

	if *out != "" || *format != "text" {
		switch *format {
		case "json":
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			enc.Encode(results)
		case "csv":
			cw := csv.NewWriter(w)
			cw.Write([]string{"id", "category", "severity", "profile", "status", "reason", "description"})
			for _, r := range results {
				cw.Write([]string{
					r.Case.ID,
					string(r.Case.Category),
					string(r.Case.Severity),
					string(r.Case.Profile),
					string(r.Status),
					r.Reason,
					r.Case.Description,
				})
			}
			cw.Flush()
		}
	}

	if *failOn != "" {
		threshold := corpus.Severity(strings.ToUpper(*failOn))
		if corpus.AnyUnsafeAtOrAbove(summaries, threshold) {
			os.Exit(2)
		}
	}
}

func severityRank(s string) int {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	}
	return 0
}
