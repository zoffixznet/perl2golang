package ai

import (
	"fmt"
	"slices"
	"strings"
)

// ModelSpec describes one model this tool is willing to recommend.
//
// Every model in the catalogue is under a licence that is free for any user -
// Apache-2.0 or MIT - and can be downloaded without an approval step. Models
// under research, non-production or behavioural-use licences are listed
// separately by [Excluded] so the tool can explain why it will not fetch one,
// rather than pretending it does not exist.
type ModelSpec struct {
	// Name is the tag to pull, for example "qwen2.5-coder:7b".
	Name string
	// Family groups the sizes of one model line.
	Family string
	// Licence is the SPDX identifier of the licence the weights are under.
	Licence string
	// LicenceInModel reports whether the downloaded model carries its licence
	// text, so the tool can confirm it locally after pulling. When this is
	// false the licence is known from the publisher only, and the runtime
	// will report no licence at all.
	LicenceInModel bool
	// ParamsB is the parameter count in billions. ActiveParamsB is the count
	// actually computed per token, which is smaller for a mixture-of-experts
	// model: those need memory for everything and compute for a fraction,
	// which is what makes them the sensible choice without a GPU.
	ParamsB       float64
	ActiveParamsB float64
	// DownloadMB is the real size of the download.
	DownloadMB int
	// MinVRAMMB is the graphics memory needed to run it at ContextTokens,
	// and MinRAMMB the system memory needed to run it on the CPU instead.
	MinVRAMMB int
	MinRAMMB  int
	// ContextTokens is the context window this tier can afford. It is always
	// requested explicitly, because the runtime's own default is 4096 on any
	// machine with less than 23 GiB of graphics memory.
	ContextTokens int
	// Quality is the honest one-word tier: "entry", "good" or "strong".
	Quality string
	// Notes is one sentence on the tradeoff at this tier.
	Notes string
	// Rank is the editorial preference between models that all fit. Higher
	// is better.
	Rank int
}

// MoE reports whether only a fraction of the weights is computed per token.
func (m ModelSpec) MoE() bool { return m.ActiveParamsB > 0 && m.ActiveParamsB < m.ParamsB }

// DiskNeedMB is the free space a download needs: the model itself with room
// for the manifest and the temporary blob, plus a margin so the pull cannot
// fill the filesystem it lands on.
func (m ModelSpec) DiskNeedMB() int { return m.DownloadMB*5/4 + 2048 }

// DownloadSize renders the download the way a person reads it.
func (m ModelSpec) DownloadSize() string { return formatMB(m.DownloadMB) }

// formatMB renders a size in binary units, one decimal place past a gigabyte.
func formatMB(mb int) string {
	if mb < 1024 {
		return fmt.Sprintf("%d MB", mb)
	}
	return fmt.Sprintf("%.1f GB", float64(mb)/1024)
}

// FitsGPU reports whether the model runs on a card with the given memory.
func (m ModelSpec) FitsGPU(vramMB int) bool { return vramMB > 0 && m.MinVRAMMB <= vramMB }

// FitsCPU reports whether the model runs on system memory alone.
func (m ModelSpec) FitsCPU(ramMB int) bool { return m.MinRAMMB <= ramMB }

// catalogue is the shipped model list. Sizes are the summed layer sizes from
// the model registry, so they are the real download, not an estimate.
var catalogue = []ModelSpec{
	{
		Name: "qwen2.5-coder:1.5b", Family: "Qwen2.5-Coder", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 1.5, DownloadMB: 944, MinVRAMMB: 2048, MinRAMMB: 5120, ContextTokens: 8192,
		Quality: "entry", Rank: 10,
		Notes: "Runs almost anywhere, and its suggestions need the most scrutiny; useful mainly to prove the setup works.",
	},
	{
		Name: "granite4.1:3b", Family: "Granite 4.1", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 3, DownloadMB: 2003, MinVRAMMB: 4096, MinRAMMB: 6144, ContextTokens: 8192,
		Quality: "entry", Rank: 20,
		Notes: "The best small option on a 4 GiB card; it follows instructions well for its size.",
	},
	{
		Name: "qwen3.5:4b", Family: "Qwen3.5", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 4, DownloadMB: 3233, MinVRAMMB: 6144, MinRAMMB: 8192, ContextTokens: 8192,
		Quality: "entry", Rank: 25,
		Notes: "Fast and recent, but its attention layout makes long context unusually expensive.",
	},
	{
		Name: "qwen2.5-coder:7b", Family: "Qwen2.5-Coder", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 7.6, DownloadMB: 4463, MinVRAMMB: 8192, MinRAMMB: 10240, ContextTokens: 16384,
		Quality: "good", Rank: 40,
		Notes: "The frugal choice: an older model whose memory per token of context is less than half of any newer one, which is what lets it hold a whole file.",
	},
	{
		Name: "rnj-1:8b", Family: "Rnj-1", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 8, DownloadMB: 4873, MinVRAMMB: 8192, MinRAMMB: 11264, ContextTokens: 8192,
		Quality: "good", Rank: 35,
		Notes: "A general model rather than a code model; sound on prose, less sharp on code review.",
	},
	{
		Name: "granite4.1:8b", Family: "Granite 4.1", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 8, DownloadMB: 5102, MinVRAMMB: 8192, MinRAMMB: 11264, ContextTokens: 8192,
		Quality: "good", Rank: 34,
		Notes: "Steady and literal, which suits a job where following the rules matters more than flair.",
	},
	{
		Name: "ornith:9b", Family: "Ornith 1.0", Licence: "MIT", LicenceInModel: true,
		ParamsB: 9, DownloadMB: 5369, MinVRAMMB: 11264, MinRAMMB: 12288, ContextTokens: 32768,
		Quality: "strong", Rank: 50,
		Notes: "The best fit for a 12 GiB card: it holds a large context and reviews code well.",
	},
	{
		Name: "qwen3.5:9b", Family: "Qwen3.5", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 9, DownloadMB: 6285, MinVRAMMB: 11264, MinRAMMB: 13312, ContextTokens: 16384,
		Quality: "strong", Rank: 38,
		Notes: "Recent and capable, but it spends much more memory per token of context than its size suggests.",
	},
	{
		Name: "gemma4:12b", Family: "Gemma 4", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 12, DownloadMB: 7210, MinVRAMMB: 15360, MinRAMMB: 15360, ContextTokens: 8192,
		Quality: "strong", Rank: 42,
		Notes: "Strong general reasoning; the first Gemma release under a plain open source licence.",
	},
	{
		Name: "qwen2.5-coder:14b", Family: "Qwen2.5-Coder", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 14.8, DownloadMB: 8574, MinVRAMMB: 15360, MinRAMMB: 16384, ContextTokens: 16384,
		Quality: "strong", Rank: 55,
		Notes: "A clear step up on code review over the 7B, at roughly twice the memory.",
	},
	{
		Name: "gpt-oss:20b", Family: "gpt-oss", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 21, ActiveParamsB: 3.6, DownloadMB: 13151, MinVRAMMB: 16384, MinRAMMB: 22528, ContextTokens: 32768,
		Quality: "strong", Rank: 65,
		Notes: "Computes only a fraction of its weights per token, so it is fast for its size and leaves real headroom.",
	},
	{
		Name: "devstral-small-2:24b", Family: "Devstral Small 2", Licence: "Apache-2.0",
		ParamsB: 24, DownloadMB: 14477, MinVRAMMB: 20480, MinRAMMB: 24576, ContextTokens: 16384,
		Quality: "strong", Rank: 62,
		Notes: "Built for software work; the download carries no licence text, so the tool cannot confirm its licence locally.",
	},
	{
		Name: "qwen3.6:27b", Family: "Qwen3.6", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 27, DownloadMB: 16613, MinVRAMMB: 22528, MinRAMMB: 26624, ContextTokens: 16384,
		Quality: "strong", Rank: 68,
		Notes: "A dense model of real capability, and correspondingly slow without a large card.",
	},
	{
		Name: "granite4.1:30b", Family: "Granite 4.1", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 30, ActiveParamsB: 3, DownloadMB: 16680, MinVRAMMB: 22528, MinRAMMB: 26624, ContextTokens: 32768,
		Quality: "strong", Rank: 66,
		Notes: "Large but sparse, so it stays responsive where a dense model of the same size would not.",
	},
	{
		Name: "muse-glimmer:30b", Family: "Muse Glimmer", Licence: "Apache-2.0",
		ParamsB: 30, DownloadMB: 17319, MinVRAMMB: 23552, MinRAMMB: 27648, ContextTokens: 16384,
		Quality: "strong", Rank: 64,
		Notes: "Capable general model; the download carries no licence text, so the tool cannot confirm its licence locally.",
	},
	{
		Name: "qwen3-coder:30b", Family: "Qwen3-Coder", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 30.5, ActiveParamsB: 3, DownloadMB: 17700, MinVRAMMB: 23552, MinRAMMB: 27648, ContextTokens: 32768,
		Quality: "strong", Rank: 75,
		Notes: "The best code model here that a workstation can actually run, and the only large one worth running without a GPU.",
	},
	{
		Name: "north-mini-code-1.0", Family: "North Mini Code", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 30, DownloadMB: 17729, MinVRAMMB: 23552, MinRAMMB: 28672, ContextTokens: 16384,
		Quality: "strong", Rank: 70,
		Notes: "A code-focused model for a large card; dense, so it is slow on the CPU.",
	},
	{
		Name: "glm-4.7-flash", Family: "GLM 4.7", Licence: "MIT", LicenceInModel: true,
		ParamsB: 32, DownloadMB: 18139, MinVRAMMB: 24576, MinRAMMB: 28672, ContextTokens: 16384,
		Quality: "strong", Rank: 72,
		Notes: "Strong on structured output, which is most of what this tool asks for.",
	},
	{
		Name: "olmo-3.1:32b", Family: "Olmo 3.1", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 32, DownloadMB: 18577, MinVRAMMB: 24576, MinRAMMB: 29696, ContextTokens: 16384,
		Quality: "strong", Rank: 67,
		Notes: "Fully open, training data included; a good citizen and a solid reviewer.",
	},
	{
		Name: "qwen2.5-coder:32b", Family: "Qwen2.5-Coder", Licence: "Apache-2.0", LicenceInModel: true,
		ParamsB: 32.8, DownloadMB: 18930, MinVRAMMB: 24576, MinRAMMB: 29696, ContextTokens: 16384,
		Quality: "strong", Rank: 74,
		Notes: "The strongest of the older code line; dense, so give it a card rather than the CPU.",
	},
}

// ExcludedModel is a model the tool knows about and will not recommend or
// download, with the reason in the user's terms.
type ExcludedModel struct {
	Name    string
	Licence string
	Reason  string
}

// excluded are the popular tags that a search would turn up but whose licences
// are not free for every user. They are listed so the tool can explain itself:
// the runtime will happily download any of them without a word of warning.
var excluded = []ExcludedModel{
	{
		Name: "qwen2.5-coder:3b", Licence: "qwen-research",
		Reason: "the only size in its family under a research licence, which does not allow commercial use",
	},
	{
		Name: "codestral:22b", Licence: "MNPL",
		Reason: "a non-production licence that bars use in the course of a commercial activity, whether paid or free",
	},
	{
		Name: "gemma3", Licence: "Gemma Terms of Use",
		Reason: "custom terms with obligations that pass on to whoever receives the output; the Gemma 4 release is plain Apache-2.0 instead",
	},
	{
		Name: "codegemma", Licence: "Gemma Terms of Use",
		Reason: "custom terms with obligations that pass on to whoever receives the output; the Gemma 4 release is plain Apache-2.0 instead",
	},
	{
		Name: "starcoder2", Licence: "BigCode OpenRAIL-M",
		Reason: "behavioural use restrictions that pass on downstream, so it is open weight but not open source",
	},
	{
		Name: "deepseek-coder", Licence: "DeepSeek licence",
		Reason: "a custom licence with a use-restriction appendix that has to be passed on downstream",
	},
	{
		Name: "codellama", Licence: "Llama 2 Community Licence",
		Reason: "a community licence with conditions, and the weights need an approval step before download",
	},
}

// Catalogue returns every model the tool recommends, in catalogue order.
func Catalogue() []ModelSpec { return slices.Clone(catalogue) }

// Excluded returns the models the tool deliberately does not offer, with the
// licence reason for each.
func Excluded() []ExcludedModel { return slices.Clone(excluded) }

// Lookup finds a catalogue entry by tag.
func Lookup(name string) (ModelSpec, bool) {
	for _, m := range catalogue {
		if m.Name == name {
			return m, true
		}
	}
	return ModelSpec{}, false
}

// PreferredModel picks which of the models a runtime already has to use.
//
// The rule is "use what is there". A model store is a machine-wide resource
// that other projects share, so this never asks for a download to get a better
// answer: it ranks what is installed and takes the best of it. A model this
// tool has an opinion about wins over one it does not, and a model trained on
// code wins over a general one, because every job here is about code.
func PreferredModel(installed []string) string {
	best, bestScore := "", -1<<30
	for _, name := range installed {
		score := installedScore(name)
		if score > bestScore || (score == bestScore && name < best) {
			best, bestScore = name, score
		}
	}
	return best
}

// installedScore ranks one installed model tag.
func installedScore(name string) int {
	score := 0
	if spec, ok := Lookup(name); ok {
		score += 1000 + spec.Rank
	} else if spec, ok := Lookup(strings.TrimSuffix(name, ":latest")); ok {
		score += 1000 + spec.Rank
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "coder") || strings.Contains(lower, "code") {
		score += 100
	}
	// An embedding model cannot answer a chat request at all, so it is worse
	// than anything else that is present.
	if strings.Contains(lower, "embed") {
		score -= 5000
	}
	return score
}

// ExcludedReason reports why a model tag is not offered, matching either the
// exact tag or its family prefix. The empty string means it is not excluded.
func ExcludedReason(name string) string {
	base, _, _ := strings.Cut(name, ":")
	for _, e := range excluded {
		eBase, _, _ := strings.Cut(e.Name, ":")
		if e.Name == name || eBase == base {
			return fmt.Sprintf("%s is under %s: %s", name, e.Licence, e.Reason)
		}
	}
	return ""
}
