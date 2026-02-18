package models

import (
	"math"
	"testing"
)

func TestQuantBPP(t *testing.T) {
	tests := []struct {
		quant string
		want  float64
	}{
		{"F32", 4.0},
		{"F16", 2.0},
		{"BF16", 2.0},
		{"Q8_0", 1.05},
		{"Q6_K", 0.80},
		{"Q5_K_M", 0.68},
		{"Q4_K_M", 0.58},
		{"Q4_0", 0.58},
		{"Q3_K_M", 0.48},
		{"Q2_K", 0.37},
		{"unknown", 0.58},
	}
	for _, tt := range tests {
		got := QuantBPP(tt.quant)
		if got != tt.want {
			t.Errorf("QuantBPP(%q) = %v, want %v", tt.quant, got, tt.want)
		}
	}
}

func TestQuantSpeedMultiplier(t *testing.T) {
	tests := []struct {
		quant string
		want  float64
	}{
		{"F16", 0.6},
		{"BF16", 0.6},
		{"Q8_0", 0.8},
		{"Q6_K", 0.95},
		{"Q5_K_M", 1.0},
		{"Q4_K_M", 1.15},
		{"Q4_0", 1.15},
		{"Q3_K_M", 1.25},
		{"Q2_K", 1.35},
		{"unknown", 1.0},
	}
	for _, tt := range tests {
		got := QuantSpeedMultiplier(tt.quant)
		if got != tt.want {
			t.Errorf("QuantSpeedMultiplier(%q) = %v, want %v", tt.quant, got, tt.want)
		}
	}
}

func TestQuantQualityPenalty(t *testing.T) {
	tests := []struct {
		quant string
		want  float64
	}{
		{"F16", 0.0},
		{"BF16", 0.0},
		{"Q8_0", 0.0},
		{"Q6_K", -1.0},
		{"Q5_K_M", -2.0},
		{"Q4_K_M", -5.0},
		{"Q4_0", -5.0},
		{"Q3_K_M", -8.0},
		{"Q2_K", -12.0},
		{"unknown", -5.0},
	}
	for _, tt := range tests {
		got := QuantQualityPenalty(tt.quant)
		if got != tt.want {
			t.Errorf("QuantQualityPenalty(%q) = %v, want %v", tt.quant, got, tt.want)
		}
	}
}

func TestLlmModel_ParamsB(t *testing.T) {
	raw7B := uint64(7_000_000_000)
	raw1_5B := uint64(1_500_000_000)
	tests := []struct {
		name   string
		model  *LlmModel
		wantB  float64
	}{
		{"7B string", &LlmModel{ParameterCount: "7B"}, 7.0},
		{"70B string", &LlmModel{ParameterCount: "70B"}, 70.0},
		{"1.5B string", &LlmModel{ParameterCount: "1.5B"}, 1.5},
		{"600M string", &LlmModel{ParameterCount: "600M"}, 0.6},
		{"137M string", &LlmModel{ParameterCount: "137M"}, 0.137},
		{"ParametersRaw 7B", &LlmModel{ParametersRaw: &raw7B, ParameterCount: "?"}, 7.0},
		{"ParametersRaw 1.5B", &LlmModel{ParametersRaw: &raw1_5B}, 1.5},
		{"default", &LlmModel{ParameterCount: ""}, 7.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.ParamsB()
			if math.Abs(got-tt.wantB) > 0.01 {
				t.Errorf("ParamsB() = %v, want %v", got, tt.wantB)
			}
		})
	}
}

func TestLlmModel_EstimateMemoryGB(t *testing.T) {
	m := &LlmModel{ParameterCount: "7B", Quantization: "Q4_K_M"}
	// modelMem = 7 * 0.58, kvCache = 0.000008 * 7 * 4096, overhead = 0.5
	got := m.EstimateMemoryGB("Q4_K_M", 4096)
	wantModel := 7.0 * 0.58
	wantKv := 0.000008 * 7.0 * 4096
	wantOverhead := 0.5
	want := wantModel + wantKv + wantOverhead
	if math.Abs(got-want) > 0.01 {
		t.Errorf("EstimateMemoryGB = %v, want %v (model=%v kv=%v overhead=%v)", got, want, wantModel, wantKv, wantOverhead)
	}
}

func TestLlmModel_BestQuantForBudget(t *testing.T) {
	m := &LlmModel{ParameterCount: "7B", Quantization: "Q4_K_M", ContextLength: 4096}
	// Large budget: should get best quant that fits
	quant, mem := m.BestQuantForBudget(100, 4096)
	if quant != "Q8_0" || mem <= 0 {
		t.Errorf("BestQuantForBudget(100) = %q, %v; want Q8_0 and positive mem", quant, mem)
	}
	// Tiny budget: should fall back to model default
	quant2, mem2 := m.BestQuantForBudget(0.1, 4096)
	if quant2 != m.Quantization || mem2 <= 0 {
		t.Errorf("BestQuantForBudget(0.1) = %q, %v; want model default %q", quant2, mem2, m.Quantization)
	}
}

func TestUseCaseFromModel(t *testing.T) {
	tests := []struct {
		name string
		m    *LlmModel
		want UseCase
	}{
		{"embedding use_case", &LlmModel{Name: "x", UseCase: "Text embeddings for RAG"}, UseCaseEmbedding},
		{"embed in name", &LlmModel{Name: "my-embed-model", UseCase: ""}, UseCaseEmbedding},
		{"bge in name", &LlmModel{Name: "BAAI/bge-large", UseCase: ""}, UseCaseEmbedding},
		{"code in name", &LlmModel{Name: "starcoder-7b", UseCase: ""}, UseCaseCoding},
		{"code use_case", &LlmModel{Name: "x", UseCase: "code generation"}, UseCaseCoding},
		{"vision use_case", &LlmModel{Name: "x", UseCase: "vision"}, UseCaseMultimodal},
		{"reason use_case", &LlmModel{Name: "x", UseCase: "reasoning"}, UseCaseReasoning},
		{"deepseek-r1", &LlmModel{Name: "deepseek-r1-7b", UseCase: ""}, UseCaseReasoning},
		{"chat use_case", &LlmModel{Name: "x", UseCase: "chat"}, UseCaseChat},
		{"instruction", &LlmModel{Name: "x", UseCase: "instruction following"}, UseCaseChat},
		{"general", &LlmModel{Name: "llama-7b", UseCase: "text generation"}, UseCaseGeneral},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UseCaseFromModel(tt.m)
			if got != tt.want {
				t.Errorf("UseCaseFromModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLlmModel_MoeActiveVRAMGB(t *testing.T) {
	activeParams := uint64(3_000_000_000)
	tests := []struct {
		name  string
		model *LlmModel
		want  bool // has value
	}{
		{"not MoE", &LlmModel{IsMoE: false}, false},
		{"MoE no ActiveParameters", &LlmModel{IsMoE: true, ActiveParameters: nil}, false},
		{"MoE with ActiveParameters", &LlmModel{IsMoE: true, ActiveParameters: &activeParams, Quantization: "Q4_K_M"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.MoeActiveVRAMGB()
			if (got != nil) != tt.want {
				t.Errorf("MoeActiveVRAMGB() = %v, want non-nil=%v", got, tt.want)
			}
			if got != nil && *got < 0 {
				t.Errorf("MoeActiveVRAMGB() = %v, should be non-negative", *got)
			}
		})
	}
}

func TestLlmModel_MoeOffloadedRAMGB(t *testing.T) {
	activeParams := uint64(3_000_000_000)
	totalParams := uint64(8_000_000_000)
	tests := []struct {
		name  string
		model *LlmModel
		want  bool
	}{
		{"not MoE", &LlmModel{IsMoE: false}, false},
		{"MoE no ParametersRaw", &LlmModel{IsMoE: true, ActiveParameters: &activeParams, ParametersRaw: nil}, false},
		{"MoE with both", &LlmModel{IsMoE: true, ActiveParameters: &activeParams, ParametersRaw: &totalParams, Quantization: "Q4_K_M"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.MoeOffloadedRAMGB()
			if (got != nil) != tt.want {
				t.Errorf("MoeOffloadedRAMGB() = %v, want non-nil=%v", got, tt.want)
			}
		})
	}
}

func TestNewDB(t *testing.T) {
	db, err := NewDB()
	if err != nil {
		t.Fatalf("NewDB() err = %v", err)
	}
	all := db.GetAllModels()
	if len(all) == 0 {
		t.Error("GetAllModels() returned empty")
	}
}

func TestModelDatabase_FindModel(t *testing.T) {
	db, err := NewDB()
	if err != nil {
		t.Fatalf("NewDB() err = %v", err)
	}
	// From embedded JSON we have e.g. nomic-ai/nomic-embed, BAAI/bge, Qwen, TinyLlama
	results := db.FindModel("nomic")
	if len(results) == 0 {
		t.Error("FindModel(\"nomic\") returned no results")
	}
	results2 := db.FindModel("BAAI")
	if len(results2) == 0 {
		t.Error("FindModel(\"BAAI\") returned no results")
	}
	results3 := db.FindModel("nonexistent-model-xyz")
	if len(results3) != 0 {
		t.Errorf("FindModel(\"nonexistent-model-xyz\") returned %d results", len(results3))
	}
}

func TestUseCase_String(t *testing.T) {
	tests := []struct {
		u    UseCase
		want string
	}{
		{UseCaseGeneral, "General"},
		{UseCaseCoding, "Coding"},
		{UseCaseReasoning, "Reasoning"},
		{UseCaseChat, "Chat"},
		{UseCaseMultimodal, "Multimodal"},
		{UseCaseEmbedding, "Embedding"},
	}
	for _, tt := range tests {
		got := tt.u.String()
		if got != tt.want {
			t.Errorf("UseCase(%d).String() = %q, want %q", tt.u, got, tt.want)
		}
	}
}
