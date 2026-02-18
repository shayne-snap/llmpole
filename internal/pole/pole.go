// Package pole implements fit analysis and ranking (one model vs system specs, scoring, filters).
package pole

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"

	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
)

// FitLevel is how well a model fits the current hardware (Perfect / Good / Marginal / Too Tight).
type FitLevel int

const (
	FitPerfect FitLevel = iota
	FitGood
	FitMarginal
	FitTooTight
)

func (f FitLevel) String() string {
	switch f {
	case FitPerfect:
		return "Perfect"
	case FitGood:
		return "Good"
	case FitMarginal:
		return "Marginal"
	case FitTooTight:
		return "Too Tight"
	default:
		return "Marginal"
	}
}

// RunMode is how the model would run (GPU, MoE offload, CPU+GPU, or CPU-only).
type RunMode int

const (
	RunModeGpu RunMode = iota
	RunModeMoeOffload
	RunModeCpuOffload
	RunModeCpuOnly
)

func (r RunMode) String() string {
	switch r {
	case RunModeGpu:
		return "GPU"
	case RunModeMoeOffload:
		return "MoE"
	case RunModeCpuOffload:
		return "CPU+GPU"
	case RunModeCpuOnly:
		return "CPU"
	default:
		return "CPU"
	}
}

// ScoreComponents holds the per-dimension scores (quality, speed, fit, context).
type ScoreComponents struct {
	Quality float64 `json:"quality"`
	Speed   float64 `json:"speed"`
	Fit     float64 `json:"fit"`
	Context float64 `json:"context"`
}

// ModelFit holds the analysis result for one model on the current system.
type ModelFit struct {
	Model              *models.LlmModel `json:"-"`
	FitLevel           FitLevel         `json:"fit_level"`
	RunMode            RunMode          `json:"run_mode"`
	MemoryRequiredGB   float64          `json:"memory_required_gb"`
	MemoryAvailableGB  float64          `json:"memory_available_gb"`
	UtilizationPct     float64          `json:"utilization_pct"`
	Notes              []string         `json:"notes"`
	MoeOffloadedGB     *float64         `json:"moe_offloaded_gb,omitempty"`
	Score              float64          `json:"score"`
	ScoreComponents    ScoreComponents  `json:"score_components"`
	EstimatedTPS       float64          `json:"estimated_tps"`
	BestQuant          string           `json:"best_quant"`
	UseCase            models.UseCase   `json:"use_case"`
}

// FitEmoji returns the status emoji for the fit level (e.g. green for Perfect).
func (f *ModelFit) FitEmoji() string {
	switch f.FitLevel {
	case FitPerfect:
		return "🟢"
	case FitGood:
		return "🟡"
	case FitMarginal:
		return "🟠"
	case FitTooTight:
		return "🔴"
	default:
		return "🟠"
	}
}

// FitText returns text for fit level.
func (f *ModelFit) FitText() string {
	return f.FitLevel.String()
}

// RunModeText returns display string for run mode.
func (f *ModelFit) RunModeText() string {
	return f.RunMode.String()
}

// Analyze analyzes one model against system specs and returns fit level, run mode, score, and notes.
func Analyze(model *models.LlmModel, system *hardware.SystemSpecs) *ModelFit {
	minVram := model.MinRAMGB
	if model.MinVRAMGB != nil {
		minVram = *model.MinVRAMGB
	}
	useCase := models.UseCaseFromModel(model)
	var notes []string

	var runMode RunMode
	var memRequired, memAvailable float64

	if system.HasGPU {
		if system.UnifiedMemory {
			if system.GpuVRAMGB != nil {
				notes = append(notes, "Unified memory: GPU and CPU share the same pool")
				if model.IsMoE && model.NumExperts != nil {
					ne := uint32(0)
					if model.ActiveExperts != nil {
						ne = *model.ActiveExperts
					}
					notes = append(notes, fmt.Sprintf("MoE: %d/%d experts active (all share unified memory pool)", ne, *model.NumExperts))
				}
				runMode = RunModeGpu
				memRequired = minVram
				memAvailable = *system.GpuVRAMGB
			} else {
				runMode, memRequired, memAvailable = cpuPath(model, system, &notes)
			}
		} else if system.GpuVRAMGB != nil {
			sysVram := *system.GpuVRAMGB
			if minVram <= sysVram {
				notes = append(notes, "GPU: model loaded into VRAM")
				if model.IsMoE && model.NumExperts != nil {
					notes = append(notes, fmt.Sprintf("MoE: all %d experts loaded in VRAM (optimal)", *model.NumExperts))
				}
				runMode = RunModeGpu
				memRequired = minVram
				memAvailable = sysVram
			} else if model.IsMoE {
				runMode, memRequired, memAvailable = moeOffloadPath(model, system, sysVram, minVram, &notes)
			} else if model.MinRAMGB <= system.AvailableRAMGB {
				notes = append(notes, "GPU: insufficient VRAM, spilling to system RAM")
				notes = append(notes, "Performance will be significantly reduced")
				runMode = RunModeCpuOffload
				memRequired = model.MinRAMGB
				memAvailable = system.AvailableRAMGB
			} else {
				notes = append(notes, "Insufficient VRAM and system RAM")
				notes = append(notes, fmt.Sprintf("Need %.1f GB VRAM or %.1f GB system RAM", minVram, model.MinRAMGB))
				runMode = RunModeGpu
				memRequired = minVram
				memAvailable = sysVram
			}
		} else {
			notes = append(notes, "GPU detected but VRAM unknown")
			runMode, memRequired, memAvailable = cpuPath(model, system, &notes)
		}
	} else {
		runMode, memRequired, memAvailable = cpuPath(model, system, &notes)
	}

	fitLevel := scoreFit(memRequired, memAvailable, model.RecommendedRAMGB, runMode)
	utilPct := math.MaxFloat64
	if memAvailable > 0 {
		utilPct = (memRequired / memAvailable) * 100
	}

	if runMode == RunModeCpuOnly {
		notes = append(notes, "No GPU -- inference will be slow")
	}
	if (runMode == RunModeCpuOffload || runMode == RunModeCpuOnly) && system.TotalCPUCores < 4 {
		notes = append(notes, "Low CPU core count may bottleneck inference")
	}

	var moeOffloaded *float64
	if runMode == RunModeMoeOffload {
		moeOffloaded = model.MoeOffloadedRAMGB()
	}

	bestQuant, _ := model.BestQuantForBudget(memAvailable, model.ContextLength)
	if bestQuant != model.Quantization {
		notes = append(notes, "Best quantization for hardware: "+bestQuant+" (model default: "+model.Quantization+")")
	}
	estimatedTPS := estimateTPS(model, bestQuant, system, runMode)
	sc := computeScores(model, bestQuant, useCase, estimatedTPS, memRequired, memAvailable)
	score := weightedScore(sc, useCase)
	if estimatedTPS > 0 {
		notes = append(notes, fmt.Sprintf("Estimated speed: %.1f tok/s", estimatedTPS))
	}

	return &ModelFit{
		Model:             model,
		FitLevel:          fitLevel,
		RunMode:           runMode,
		MemoryRequiredGB:  memRequired,
		MemoryAvailableGB: memAvailable,
		UtilizationPct:    utilPct,
		Notes:             notes,
		MoeOffloadedGB:    moeOffloaded,
		Score:             score,
		ScoreComponents:   sc,
		EstimatedTPS:      estimatedTPS,
		BestQuant:         bestQuant,
		UseCase:           useCase,
	}
}

// AnalyzeAll runs Analyze for each model.
func AnalyzeAll(models []*models.LlmModel, system *hardware.SystemSpecs) []*ModelFit {
	out := make([]*ModelFit, 0, len(models))
	for _, m := range models {
		out = append(out, Analyze(m, system))
	}
	return out
}

// RankModelsByFit sorts by score descending, with Too Tight entries last.
func RankModelsByFit(fits []*ModelFit) []*ModelFit {
	out := make([]*ModelFit, len(fits))
	copy(out, fits)
	sort.Slice(out, func(i, j int) bool {
		ar, br := out[i].FitLevel != FitTooTight, out[j].FitLevel != FitTooTight
		if ar && !br {
			return true
		}
		if !ar && br {
			return false
		}
		return out[i].Score > out[j].Score
	})
	return out
}

// FilterPerfectOnly keeps only Perfect fit level.
func FilterPerfectOnly(fits []*ModelFit) []*ModelFit {
	var out []*ModelFit
	for _, f := range fits {
		if f.FitLevel == FitPerfect {
			out = append(out, f)
		}
	}
	return out
}

// FilterByUseCase keeps fits matching use case (general, coding, reasoning, chat, multimodal, embedding).
func FilterByUseCase(fits []*ModelFit, useCase string) []*ModelFit {
	uc, ok := useCaseFromString(useCase)
	if !ok {
		return fits
	}
	var out []*ModelFit
	for _, f := range fits {
		if f.UseCase == uc {
			out = append(out, f)
		}
	}
	return out
}

func useCaseFromString(s string) (models.UseCase, bool) {
	switch strings.ToLower(s) {
	case "general":
		return models.UseCaseGeneral, true
	case "coding", "code":
		return models.UseCaseCoding, true
	case "reasoning", "reason":
		return models.UseCaseReasoning, true
	case "chat":
		return models.UseCaseChat, true
	case "multimodal", "vision":
		return models.UseCaseMultimodal, true
	case "embedding", "embed":
		return models.UseCaseEmbedding, true
	default:
		return 0, false
	}
}

func cpuPath(model *models.LlmModel, system *hardware.SystemSpecs, notes *[]string) (RunMode, float64, float64) {
	*notes = append(*notes, "CPU-only: model loaded into system RAM")
	if model.IsMoE {
		*notes = append(*notes, "MoE architecture, but expert offloading requires a GPU")
	}
	return RunModeCpuOnly, model.MinRAMGB, system.AvailableRAMGB
}

func moeOffloadPath(model *models.LlmModel, system *hardware.SystemSpecs, systemVram, totalVram float64, notes *[]string) (RunMode, float64, float64) {
	moeVram := model.MoeActiveVRAMGB()
	if moeVram != nil {
		offload := model.MoeOffloadedRAMGB()
		offloadGB := 0.0
		if offload != nil {
			offloadGB = *offload
		}
		if *moeVram <= systemVram && offloadGB <= system.AvailableRAMGB {
			ne, nn := uint32(0), uint32(0)
			if model.ActiveExperts != nil {
				ne = *model.ActiveExperts
			}
			if model.NumExperts != nil {
				nn = *model.NumExperts
			}
			*notes = append(*notes, fmt.Sprintf("MoE: %d/%d experts active in VRAM (%.1f GB)", ne, nn, *moeVram))
			*notes = append(*notes, fmt.Sprintf("Inactive experts offloaded to system RAM (%.1f GB)", offloadGB))
			return RunModeMoeOffload, *moeVram, systemVram
		}
	}
	if model.MinRAMGB <= system.AvailableRAMGB {
		*notes = append(*notes, "MoE: insufficient VRAM for expert offloading")
		*notes = append(*notes, "Spilling entire model to system RAM")
		*notes = append(*notes, "Performance will be significantly reduced")
		return RunModeCpuOffload, model.MinRAMGB, system.AvailableRAMGB
	}
	*notes = append(*notes, "Insufficient VRAM and system RAM")
	mav := totalVram
	if model.MoeActiveVRAMGB() != nil {
		mav = *model.MoeActiveVRAMGB()
	}
	*notes = append(*notes, fmt.Sprintf("Need %.1f GB VRAM (full) or %.1f GB (MoE offload) + RAM", totalVram, mav))
	return RunModeGpu, totalVram, systemVram
}

func scoreFit(memRequired, memAvailable, recommended float64, runMode RunMode) FitLevel {
	if memRequired > memAvailable {
		return FitTooTight
	}
	switch runMode {
	case RunModeGpu:
		if recommended <= memAvailable {
			return FitPerfect
		}
		if memAvailable >= memRequired*1.2 {
			return FitGood
		}
		return FitMarginal
	case RunModeMoeOffload, RunModeCpuOffload:
		if memAvailable >= memRequired*1.2 {
			return FitGood
		}
		return FitMarginal
	case RunModeCpuOnly:
		return FitMarginal
	default:
		return FitMarginal
	}
}

func estimateTPS(model *models.LlmModel, quant string, system *hardware.SystemSpecs, runMode RunMode) float64 {
	k := 70.0
	switch system.Backend {
	case hardware.BackendCuda:
		k = 220
	case hardware.BackendMetal:
		k = 160
	case hardware.BackendRocm:
		k = 180
	case hardware.BackendVulkan:
		k = 150
	case hardware.BackendSycl:
		k = 100
	case hardware.BackendCpuArm:
		k = 90
	case hardware.BackendCpuX86:
		k = 70
	}
	params := model.ParamsB()
	if params < 0.1 {
		params = 0.1
	}
	base := k / params * models.QuantSpeedMultiplier(quant)
	if system.TotalCPUCores >= 8 {
		base *= 1.1
	}
	switch runMode {
	case RunModeMoeOffload:
		base *= 0.8
	case RunModeCpuOffload:
		base *= 0.5
	case RunModeCpuOnly:
		base *= 0.3
	}
	if runMode == RunModeCpuOnly {
		cpuK := 70.0
		if runtime.GOARCH == "arm64" {
			cpuK = 90
		}
		base = (cpuK / params) * models.QuantSpeedMultiplier(quant)
		if system.TotalCPUCores >= 8 {
			base *= 1.1
		}
	}
	if base < 0.1 {
		base = 0.1
	}
	return base
}

func computeScores(model *models.LlmModel, quant string, useCase models.UseCase, estimatedTPS, memRequired, memAvailable float64) ScoreComponents {
	return ScoreComponents{
		Quality: qualityScore(model, quant, useCase),
		Speed:   speedScore(estimatedTPS, useCase),
		Fit:     fitScore(memRequired, memAvailable),
		Context: contextScore(model, useCase),
	}
}

func qualityScore(model *models.LlmModel, quant string, useCase models.UseCase) float64 {
	params := model.ParamsB()
	base := 30.0
	if params < 1 {
		base = 30
	} else if params < 3 {
		base = 45
	} else if params < 7 {
		base = 60
	} else if params < 10 {
		base = 75
	} else if params < 20 {
		base = 82
	} else if params < 40 {
		base = 89
	} else {
		base = 95
	}
	nameLower := strings.ToLower(model.Name)
	familyBump := 0.0
	if strings.Contains(nameLower, "qwen") {
		familyBump = 2
	} else if strings.Contains(nameLower, "deepseek") {
		familyBump = 3
	} else if strings.Contains(nameLower, "llama") {
		familyBump = 2
	} else if strings.Contains(nameLower, "mistral") || strings.Contains(nameLower, "mixtral") {
		familyBump = 1
	} else if strings.Contains(nameLower, "gemma") {
		familyBump = 1
	} else if strings.Contains(nameLower, "starcoder") {
		familyBump = 1
	}
	qPenalty := models.QuantQualityPenalty(quant)
	taskBump := 0.0
	switch useCase {
	case models.UseCaseCoding:
		if strings.Contains(nameLower, "code") || strings.Contains(nameLower, "starcoder") || strings.Contains(nameLower, "wizard") {
			taskBump = 6
		}
	case models.UseCaseReasoning:
		if params >= 13 {
			taskBump = 5
		}
	case models.UseCaseMultimodal:
		if strings.Contains(nameLower, "vision") || strings.Contains(strings.ToLower(model.UseCase), "vision") {
			taskBump = 6
		}
	}
	v := base + familyBump + qPenalty + taskBump
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

func speedScore(tps float64, useCase models.UseCase) float64 {
	target := 40.0
	if useCase == models.UseCaseReasoning {
		target = 25
	} else if useCase == models.UseCaseEmbedding {
		target = 200
	}
	v := (tps / target) * 100
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

func fitScore(required, available float64) float64 {
	if available <= 0 || required > available {
		return 0
	}
	ratio := required / available
	if ratio <= 0.5 {
		return 60 + (ratio/0.5)*40
	}
	if ratio <= 0.8 {
		return 100
	}
	if ratio <= 0.9 {
		return 70
	}
	return 50
}

func contextScore(model *models.LlmModel, useCase models.UseCase) float64 {
	target := uint32(4096)
	switch useCase {
	case models.UseCaseCoding, models.UseCaseReasoning:
		target = 8192
	case models.UseCaseEmbedding:
		target = 512
	}
	if model.ContextLength >= target {
		return 100
	}
	if model.ContextLength >= target/2 {
		return 70
	}
	return 30
}

func weightedScore(sc ScoreComponents, useCase models.UseCase) float64 {
	var wq, ws, wf, wc float64
	switch useCase {
	case models.UseCaseGeneral:
		wq, ws, wf, wc = 0.45, 0.30, 0.15, 0.10
	case models.UseCaseCoding:
		wq, ws, wf, wc = 0.50, 0.20, 0.15, 0.15
	case models.UseCaseReasoning:
		wq, ws, wf, wc = 0.55, 0.15, 0.15, 0.15
	case models.UseCaseChat:
		wq, ws, wf, wc = 0.40, 0.35, 0.15, 0.10
	case models.UseCaseMultimodal:
		wq, ws, wf, wc = 0.50, 0.20, 0.15, 0.15
	case models.UseCaseEmbedding:
		wq, ws, wf, wc = 0.30, 0.40, 0.20, 0.10
	default:
		wq, ws, wf, wc = 0.45, 0.30, 0.15, 0.10
	}
	raw := sc.Quality*wq + sc.Speed*ws + sc.Fit*wf + sc.Context*wc
	return math.Round(raw*10) / 10
}
