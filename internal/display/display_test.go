package display

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shayne-snap/llmpole/internal/hardware"
	"github.com/shayne-snap/llmpole/internal/models"
	"github.com/shayne-snap/llmpole/internal/pole"
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

func specWithGPU(vramGB, ramGB float64) *hardware.SystemSpecs {
	return &hardware.SystemSpecs{
		TotalRAMGB:     ramGB,
		AvailableRAMGB: ramGB * 0.8,
		TotalCPUCores:  8,
		CPUName:        "Test CPU",
		HasGPU:         true,
		GpuVRAMGB:      &vramGB,
		Backend:        hardware.BackendCuda,
		Gpus:           []hardware.GpuInfo{{Name: "Test GPU", VRAMGB: &vramGB, Backend: hardware.BackendCuda, Count: 1}},
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

func oneFit() (*hardware.SystemSpecs, []*pole.ModelFit) {
	spec := specNoGPU(32, 8)
	model := model7B()
	fit := pole.Analyze(model, spec)
	return spec, []*pole.ModelFit{fit}
}

func TestSystem_JSON(t *testing.T) {
	spec := specNoGPU(16, 4)
	var buf bytes.Buffer
	System(&buf, spec, true)
	var out struct {
		System map[string]interface{} `json:"system"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.System["cpu_name"] != "Test CPU" {
		t.Errorf("system.cpu_name = %v", out.System["cpu_name"])
	}
	if _, ok := out.System["total_ram_gb"]; !ok {
		t.Error("system.total_ram_gb missing")
	}
	if _, ok := out.System["gpus"]; !ok {
		t.Error("system.gpus missing")
	}
}

func TestSystem_Table(t *testing.T) {
	spec := specNoGPU(16, 4)
	var buf bytes.Buffer
	System(&buf, spec, false)
	s := buf.String()
	if !strings.Contains(s, "System Specifications") {
		t.Error("output should contain 'System Specifications'")
	}
	if !strings.Contains(s, "Test CPU") {
		t.Error("output should contain CPU name")
	}
	if !strings.Contains(s, "Backend") {
		t.Error("output should contain Backend")
	}
	if !strings.Contains(s, "GPU: Not detected") {
		t.Error("output should contain GPU line when no GPU")
	}
}

func TestSystem_TableWithGPU(t *testing.T) {
	spec := specWithGPU(8, 32)
	var buf bytes.Buffer
	System(&buf, spec, false)
	s := buf.String()
	if !strings.Contains(s, "8.00 GB VRAM") || !strings.Contains(s, "Test GPU") {
		t.Errorf("output should contain GPU info: %s", s)
	}
}

func TestList_Empty(t *testing.T) {
	var buf bytes.Buffer
	List(&buf, nil)
	s := buf.String()
	if !strings.Contains(s, "Total models: 0") {
		t.Errorf("expected 'Total models: 0', got: %s", s)
	}
}

func TestList_NonEmpty(t *testing.T) {
	list := []*models.LlmModel{model7B()}
	var buf bytes.Buffer
	List(&buf, list)
	s := buf.String()
	if !strings.Contains(s, "Total models: 1") {
		t.Errorf("expected 'Total models: 1', got: %s", s)
	}
	if !strings.Contains(s, "Available LLM Models") {
		t.Error("output should contain section title")
	}
	if !strings.Contains(s, "test-7b") {
		t.Error("output should contain model name")
	}
}

func TestPole_Empty(t *testing.T) {
	spec := specNoGPU(16, 4)
	var buf bytes.Buffer
	Pole(&buf, spec, nil, false)
	s := buf.String()
	if !strings.Contains(s, "No compatible models found") {
		t.Errorf("expected empty message, got: %s", s)
	}
}

func TestPole_NonEmpty_JSON(t *testing.T) {
	spec, fits := oneFit()
	var buf bytes.Buffer
	Pole(&buf, spec, fits, true)
	var out struct {
		Models []map[string]interface{} `json:"models"`
		System map[string]interface{}   `json:"system"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(out.Models))
	}
	m := out.Models[0]
	if _, ok := m["fit_level"]; !ok {
		t.Error("model entry should have fit_level")
	}
	if _, ok := m["run_mode"]; !ok {
		t.Error("model entry should have run_mode")
	}
	if _, ok := m["score"]; !ok {
		t.Error("model entry should have score")
	}
}

func TestPole_NonEmpty_Table(t *testing.T) {
	spec, fits := oneFit()
	var buf bytes.Buffer
	Pole(&buf, spec, fits, false)
	s := buf.String()
	if !strings.Contains(s, "Pole Analysis") {
		t.Error("output should contain 'Pole Analysis'")
	}
	if !strings.Contains(s, "compatible model(s)") {
		t.Error("output should contain compatible count")
	}
	// Fit emoji or text (Marginal/Good/Perfect/Too Tight)
	if !strings.Contains(s, "test-7b") {
		t.Error("output should contain model name")
	}
}

func TestSearch_Empty(t *testing.T) {
	var buf bytes.Buffer
	Search(&buf, nil, "nonexistent")
	s := buf.String()
	if !strings.Contains(s, "No models found matching 'nonexistent'") {
		t.Errorf("expected no-results message, got: %s", s)
	}
}

func TestSearch_NonEmpty(t *testing.T) {
	list := []*models.LlmModel{model7B()}
	var buf bytes.Buffer
	Search(&buf, list, "test")
	s := buf.String()
	if !strings.Contains(s, "Search Results") || !strings.Contains(s, "test") {
		t.Errorf("expected Search Results and query, got: %s", s)
	}
	if !strings.Contains(s, "test-7b") {
		t.Error("output should contain model name")
	}
}

func TestInfo_JSON(t *testing.T) {
	spec, fits := oneFit()
	var buf bytes.Buffer
	Info(&buf, spec, fits[0], true)
	var out struct {
		System map[string]interface{}   `json:"system"`
		Models []map[string]interface{} `json:"models"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(out.Models))
	}
	if out.System == nil {
		t.Error("system should be present")
	}
}

func TestInfo_Table(t *testing.T) {
	spec, fits := oneFit()
	var buf bytes.Buffer
	Info(&buf, spec, fits[0], false)
	s := buf.String()
	if !strings.Contains(s, "Score Breakdown") {
		t.Error("output should contain Score Breakdown")
	}
	if !strings.Contains(s, "Resource Requirements") {
		t.Error("output should contain Resource Requirements")
	}
	if !strings.Contains(s, "Min RAM:") {
		t.Error("output should contain Min RAM from ResourceBlock")
	}
}

func TestInfo_Table_MoE(t *testing.T) {
	spec := specNoGPU(32, 8)
	activeParams := uint64(3_000_000_000)
	numExp := uint32(8)
	activeExp := uint32(2)
	offload := 2.5
	model := &models.LlmModel{
		Name:             "moe-model",
		ParameterCount:   "8B",
		MinRAMGB:         4,
		RecommendedRAMGB: 8,
		Quantization:     "Q4_K_M",
		ContextLength:    4096,
		IsMoE:            true,
		NumExperts:       &numExp,
		ActiveExperts:    &activeExp,
		ActiveParameters: &activeParams,
	}
	fit := pole.Analyze(model, spec)
	fit.MoeOffloadedGB = &offload
	var buf bytes.Buffer
	Info(&buf, spec, fit, false)
	s := buf.String()
	if !strings.Contains(s, "MoE") {
		t.Error("output should contain MoE block for MoE model")
	}
}

func TestRecommend_JSON(t *testing.T) {
	spec, fits := oneFit()
	var buf bytes.Buffer
	Recommend(&buf, spec, fits, true)
	var out struct {
		System map[string]interface{}   `json:"system"`
		Models []map[string]interface{} `json:"models"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out.Models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(out.Models))
	}
	if out.System == nil {
		t.Error("system should be present")
	}
}

func TestRecommend_Table(t *testing.T) {
	spec, fits := oneFit()
	var buf bytes.Buffer
	Recommend(&buf, spec, fits, false)
	s := buf.String()
	// Recommend with fits calls System then Pole
	if !strings.Contains(s, "Pole Analysis") {
		t.Error("output should contain Pole Analysis")
	}
	if !strings.Contains(s, "test-7b") {
		t.Error("output should contain model name")
	}
}
