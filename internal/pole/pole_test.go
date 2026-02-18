package pole

import (
	"testing"

	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
)

func specNoGPU(ramGB float64, cores int) *hardware.SystemSpecs {
	return &hardware.SystemSpecs{
		TotalRAMGB:     ramGB,
		AvailableRAMGB: ramGB * 0.8,
		TotalCPUCores:  cores,
		CPUName:        "Test CPU",
		HasGPU:         false,
		Backend:        hardware.BackendCpuX86,
	}
}

func specWithGPU(vramGB float64, ramGB float64, unified bool) *hardware.SystemSpecs {
	return &hardware.SystemSpecs{
		TotalRAMGB:     ramGB,
		AvailableRAMGB: ramGB * 0.8,
		TotalCPUCores:  8,
		CPUName:        "Test CPU",
		HasGPU:         true,
		GpuVRAMGB:      &vramGB,
		UnifiedMemory:  unified,
		Backend:        hardware.BackendCuda,
		Gpus:          []hardware.GpuInfo{{Name: "Test GPU", VRAMGB: &vramGB, Backend: hardware.BackendCuda, Count: 1, UnifiedMemory: unified}},
	}
}

func model7B() *models.LlmModel {
	minVram := 6.0
	return &models.LlmModel{
		Name:             "test-7b",
		Provider:         "Test",
		ParameterCount:   "7B",
		MinRAMGB:         8.0,
		RecommendedRAMGB: 12.0,
		MinVRAMGB:        &minVram,
		Quantization:     "Q4_K_M",
		ContextLength:    4096,
		UseCase:          "general",
		IsMoE:            false,
	}
}

func model7BSmallVram() *models.LlmModel {
	minVram := 2.0
	return &models.LlmModel{
		Name:             "test-7b-small",
		ParameterCount:   "7B",
		MinRAMGB:         4.0,
		RecommendedRAMGB: 6.0,
		MinVRAMGB:        &minVram,
		Quantization:     "Q4_K_M",
		ContextLength:    4096,
		IsMoE:            false,
	}
}

func TestAnalyze_CPUOnly(t *testing.T) {
	spec := specNoGPU(32, 8)
	model := model7B()
	fit := Analyze(model, spec)
	if fit.RunMode != RunModeCpuOnly {
		t.Errorf("RunMode = %v, want RunModeCpuOnly", fit.RunMode)
	}
	if fit.FitLevel != FitMarginal {
		t.Errorf("FitLevel = %v, want FitMarginal (CPU-only caps at Marginal)", fit.FitLevel)
	}
	if fit.MemoryAvailableGB != spec.AvailableRAMGB {
		t.Errorf("MemoryAvailableGB = %v, want %v", fit.MemoryAvailableGB, spec.AvailableRAMGB)
	}
}

func TestAnalyze_GPUSufficient(t *testing.T) {
	// 8GB VRAM >= 6GB min -> GPU path, Perfect or Good
	spec := specWithGPU(8, 32, false)
	model := model7B()
	fit := Analyze(model, spec)
	if fit.RunMode != RunModeGpu {
		t.Errorf("RunMode = %v, want RunModeGpu", fit.RunMode)
	}
	if fit.FitLevel != FitPerfect && fit.FitLevel != FitGood && fit.FitLevel != FitMarginal {
		t.Errorf("FitLevel = %v, want Perfect/Good/Marginal", fit.FitLevel)
	}
	if fit.MemoryAvailableGB != 8 {
		t.Errorf("MemoryAvailableGB = %v, want 8", fit.MemoryAvailableGB)
	}
}

func TestAnalyze_GPUInsufficientRAMOk(t *testing.T) {
	// VRAM 2GB < min 6GB, but MinRAMGB 8 <= AvailableRAM -> CPU offload
	spec := specWithGPU(2, 32, false)
	spec.AvailableRAMGB = 16
	model := model7B()
	fit := Analyze(model, spec)
	if fit.RunMode != RunModeCpuOffload {
		t.Errorf("RunMode = %v, want RunModeCpuOffload", fit.RunMode)
	}
}

func TestAnalyze_TooTight(t *testing.T) {
	// No GPU and not enough RAM
	spec := specNoGPU(4, 4)
	spec.AvailableRAMGB = 2
	model := model7B() // needs 8 GB min RAM for CPU
	fit := Analyze(model, spec)
	if fit.FitLevel != FitTooTight {
		t.Errorf("FitLevel = %v, want FitTooTight", fit.FitLevel)
	}
}

func TestRankModelsByFit(t *testing.T) {
	m := model7B()
	fits := []*ModelFit{
		{Model: m, FitLevel: FitTooTight, Score: 50},
		{Model: m, FitLevel: FitGood, Score: 70},
		{Model: m, FitLevel: FitPerfect, Score: 90},
		{Model: m, FitLevel: FitMarginal, Score: 60},
	}
	ranked := RankModelsByFit(fits)
	// TooTight must be last; others by score desc
	if len(ranked) != 4 {
		t.Fatalf("len(ranked) = %d", len(ranked))
	}
	if ranked[3].FitLevel != FitTooTight {
		t.Errorf("last element FitLevel = %v, want FitTooTight", ranked[3].FitLevel)
	}
	if ranked[0].Score != 90 || ranked[1].Score != 70 || ranked[2].Score != 60 {
		t.Errorf("scores order: got %v %v %v", ranked[0].Score, ranked[1].Score, ranked[2].Score)
	}
}

func TestFilterPerfectOnly(t *testing.T) {
	m := model7B()
	fits := []*ModelFit{
		{Model: m, FitLevel: FitPerfect},
		{Model: m, FitLevel: FitGood},
		{Model: m, FitLevel: FitPerfect},
		{Model: m, FitLevel: FitMarginal},
	}
	out := FilterPerfectOnly(fits)
	if len(out) != 2 {
		t.Errorf("FilterPerfectOnly len = %d, want 2", len(out))
	}
	for _, f := range out {
		if f.FitLevel != FitPerfect {
			t.Errorf("got FitLevel %v", f.FitLevel)
		}
	}
}

func TestFilterByUseCase(t *testing.T) {
	spec := specNoGPU(32, 8)
	codeModel := &models.LlmModel{Name: "code-model", UseCase: "coding", ParameterCount: "7B", MinRAMGB: 4, RecommendedRAMGB: 8, Quantization: "Q4_K_M", ContextLength: 4096}
	generalModel := &models.LlmModel{Name: "general-model", UseCase: "general", ParameterCount: "7B", MinRAMGB: 4, RecommendedRAMGB: 8, Quantization: "Q4_K_M", ContextLength: 4096}
	fits := AnalyzeAll([]*models.LlmModel{codeModel, generalModel}, spec)
	out := FilterByUseCase(fits, "coding")
	if len(out) != 1 {
		t.Errorf("FilterByUseCase(coding) len = %d, want 1", len(out))
	}
	if len(out) > 0 && out[0].UseCase != models.UseCaseCoding {
		t.Errorf("got UseCase %v", out[0].UseCase)
	}
	// unknown use case returns all
	out2 := FilterByUseCase(fits, "unknown-uc")
	if len(out2) != 2 {
		t.Errorf("FilterByUseCase(unknown) len = %d, want 2", len(out2))
	}
}

func TestFitLevel_String(t *testing.T) {
	tests := []struct {
		f    FitLevel
		want string
	}{
		{FitPerfect, "Perfect"},
		{FitGood, "Good"},
		{FitMarginal, "Marginal"},
		{FitTooTight, "Too Tight"},
	}
	for _, tt := range tests {
		got := tt.f.String()
		if got != tt.want {
			t.Errorf("FitLevel(%d).String() = %q, want %q", tt.f, got, tt.want)
		}
	}
}

func TestRunMode_String(t *testing.T) {
	tests := []struct {
		r    RunMode
		want string
	}{
		{RunModeGpu, "GPU"},
		{RunModeMoeOffload, "MoE"},
		{RunModeCpuOffload, "CPU+GPU"},
		{RunModeCpuOnly, "CPU"},
	}
	for _, tt := range tests {
		got := tt.r.String()
		if got != tt.want {
			t.Errorf("RunMode(%d).String() = %q, want %q", tt.r, got, tt.want)
		}
	}
}

func TestModelFit_FitEmoji(t *testing.T) {
	m := model7B()
	tests := []struct {
		level FitLevel
		emoji string
	}{
		{FitPerfect, "🟢"},
		{FitGood, "🟡"},
		{FitMarginal, "🟠"},
		{FitTooTight, "🔴"},
	}
	for _, tt := range tests {
		f := &ModelFit{Model: m, FitLevel: tt.level}
		got := f.FitEmoji()
		if got != tt.emoji {
			t.Errorf("FitEmoji() for %v = %q, want %q", tt.level, got, tt.emoji)
		}
	}
}

func TestAnalyzeAll(t *testing.T) {
	spec := specNoGPU(32, 8)
	models := []*models.LlmModel{model7B(), model7BSmallVram()}
	fits := AnalyzeAll(models, spec)
	if len(fits) != 2 {
		t.Fatalf("AnalyzeAll len = %d, want 2", len(fits))
	}
	for i, f := range fits {
		if f.Model != models[i] {
			t.Errorf("fits[%d].Model mismatch", i)
		}
		if f.RunMode != RunModeCpuOnly {
			t.Errorf("fits[%d].RunMode = %v", i, f.RunMode)
		}
	}
}
