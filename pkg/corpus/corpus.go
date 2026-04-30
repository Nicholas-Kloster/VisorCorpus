package corpus

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
)

type Profile string

const (
	ProfileStandard Profile = "standard"
	ProfileStrict   Profile = "strict"
	ProfileLenient  Profile = "lenient"
)

type Category string

const (
	CategoryPromptInjection Category = "prompt_injection"
	CategoryKBExfiltration  Category = "kb_exfiltration"
	CategoryTenantCrossLeak Category = "tenant_cross_leak"
	CategorySystemPrompt    Category = "system_prompt"
	CategoryConfigSecrets   Category = "config_secrets"
	CategoryInfraDiscovery  Category = "infra_discovery"
	CategoryJailbreak       Category = "jailbreak"
	CategoryKBInstructions  Category = "kb_instructions"
	CategoryBenignControl Category = "benign_control"
	CategoryQualityProbe  Category = "quality_probe"
)

type PromptSpec struct {
	Description string
	Text        string
}

type Expectations struct {
	ExpectRefusal        bool
	ForbiddenSubstrings  []string
	NoIgnorePrevious     bool
	NoInternalURLs       bool
	NoLargeVerbatim      bool
	RequireAnySubstrings []string // quality probes: response must contain at least one
	DisallowRefusal      bool     // quality probes: refusal is a failure
}

type Domain string

const (
	DomainNone       Domain = ""
	DomainHR         Domain = "hr"
	DomainFinance    Domain = "finance"
	DomainCloud      Domain = "cloud"
	DomainHealthcare Domain = "healthcare"
)

type AttackVector string

const (
	AttackVectorDirectOverride AttackVector = "direct_override"
	AttackVectorDocPoison      AttackVector = "doc_poison"
	AttackVectorSandwich       AttackVector = "prompt_sandwich"
	AttackVectorAuthoritySpoof AttackVector = "authority_spoof"
	AttackVectorEmotional      AttackVector = "emotional"
	AttackVectorToolAbuse      AttackVector = "tool_abuse"
	AttackVectorProtocol       AttackVector = "protocol"
	AttackVectorBenignControl  AttackVector = "benign_control"
	AttackVectorQualityProbe   AttackVector = "quality_probe"
)

type AttackCase struct {
	ID           string            `json:"id"`
	Category     Category          `json:"category"`
	Profile      Profile           `json:"profile"`
	Severity     Severity          `json:"severity"`
	Description  string            `json:"description"`
	Prompt       string            `json:"prompt"`
	Expect       Expectations      `json:"expect"`
	LengthHint   string            `json:"length_hint"` // "short" | "medium" | "long"
	Domain       Domain            `json:"domain,omitempty"`
	AttackVector AttackVector      `json:"attack_vector,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"` // difficulty, style, mutation, source
}

func CorpusForProfile(p Profile) []AttackCase {
	var cases []AttackCase
	switch p {
	case ProfileStrict:
		cases = buildStrict()
	case ProfileLenient:
		cases = buildLenient()
	default:
		cases = buildStandard()
	}
	cases = append(cases, BuildQualityProbes(p)...)
	return cases
}
