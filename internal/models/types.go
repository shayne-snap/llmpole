package models

import (
	"strconv"
	"strings"
)

// UseCase is the model use case (general, coding, reasoning, chat, etc.).
type UseCase int

const (
	UseCaseGeneral UseCase = iota
	UseCaseCoding
	UseCaseReasoning
	UseCaseChat
	UseCaseMultimodal
	UseCaseEmbedding
)

func (u UseCase) String() string {
	switch u {
	case UseCaseGeneral:
		return "General"
	case UseCaseCoding:
		return "Coding"
	case UseCaseReasoning:
		return "Reasoning"
	case UseCaseChat:
		return "Chat"
	case UseCaseMultimodal:
		return "Multimodal"
	case UseCaseEmbedding:
		return "Embedding"
	default:
		return "General"
	}
}

// LlmModel is a single model entry (fields align with hf_models.json and cache).
type LlmModel struct {
	Name               string   `json:"name"`
	Provider           string   `json:"provider"`
	ParameterCount     string   `json:"parameter_count"`
	ParametersRaw      *uint64  `json:"parameters_raw,omitempty"`
	MinRAMGB           float64  `json:"min_ram_gb"`
	RecommendedRAMGB   float64  `json:"recommended_ram_gb"`
	MinVRAMGB          *float64 `json:"min_vram_gb,omitempty"`
	Quantization       string   `json:"quantization"`
	ContextLength      uint32   `json:"context_length"`
	UseCase            string   `json:"use_case"`
	IsMoE              bool     `json:"is_moe"`
	NumExperts         *uint32  `json:"num_experts,omitempty"`
	ActiveExperts      *uint32  `json:"active_experts,omitempty"`
	ActiveParameters   *uint64  `json:"active_parameters,omitempty"`
}

// hfModelEntry for JSON decode (extra fields ignored).
type hfModelEntry struct {
	Name             string   `json:"name"`
	Provider         string   `json:"provider"`
	ParameterCount   string   `json:"parameter_count"`
	ParametersRaw    *uint64  `json:"parameters_raw"`
	MinRAMGB         float64  `json:"min_ram_gb"`
	RecommendedRAMGB float64  `json:"recommended_ram_gb"`
	MinVRAMGB        *float64 `json:"min_vram_gb"`
	Quantization     string   `json:"quantization"`
	ContextLength    uint32   `json:"context_length"`
	UseCase          string   `json:"use_case"`
	IsMoE            bool     `json:"is_moe"`
	NumExperts       *uint32  `json:"num_experts"`
	ActiveExperts    *uint32  `json:"active_experts"`
	ActiveParameters *uint64  `json:"active_parameters"`
}

// ModelDatabase holds the merged model list (embedded + user cache).
type ModelDatabase struct {
	models []*LlmModel
}

// ParamsB returns parameter count in billions for scoring and memory estimates.
func (m *LlmModel) ParamsB() float64 {
	if m.ParametersRaw != nil {
		return float64(*m.ParametersRaw) / 1e9
	}
	s := strings.TrimSpace(strings.ToUpper(m.ParameterCount))
	if strings.HasSuffix(s, "B") {
		n, _ := strconv.ParseFloat(strings.TrimSpace(s[:len(s)-1]), 64)
		return n
	}
	if strings.HasSuffix(s, "M") {
		n, _ := strconv.ParseFloat(strings.TrimSpace(s[:len(s)-1]), 64)
		return n / 1000
	}
	return 7.0
}

// EstimateMemoryGB returns estimated memory in GB for the given quant and context length.
func (m *LlmModel) EstimateMemoryGB(quant string, ctx uint32) float64 {
	bpp := QuantBPP(quant)
	params := m.ParamsB()
	modelMem := params * bpp
	kvCache := 0.000008 * params * float64(ctx)
	overhead := 0.5
	return modelMem + kvCache + overhead
}

// BestQuantForBudget returns the best quantization that fits the given memory budget, and its memory GB.
func (m *LlmModel) BestQuantForBudget(budgetGB float64, ctx uint32) (string, float64) {
	for _, q := range QuantHierarchy {
		mem := m.EstimateMemoryGB(q, ctx)
		if mem <= budgetGB {
			return q, mem
		}
	}
	halfCtx := ctx / 2
	if halfCtx >= 1024 {
		for _, q := range QuantHierarchy {
			mem := m.EstimateMemoryGB(q, halfCtx)
			if mem <= budgetGB {
				return q, mem
			}
		}
	}
	return m.Quantization, m.EstimateMemoryGB(m.Quantization, ctx)
}

func (m *LlmModel) quantBPP() float64 {
	return QuantBPP(m.Quantization)
}

// MoeActiveVRAMGB returns estimated VRAM for active MoE experts, or nil if not MoE.
func (m *LlmModel) MoeActiveVRAMGB() *float64 {
	if !m.IsMoE || m.ActiveParameters == nil {
		return nil
	}
	activeParams := float64(*m.ActiveParameters)
	bpp := m.quantBPP()
	sizeGB := (activeParams * bpp) / float64(1024*1024*1024)
	v := sizeGB * 1.1
	if v < 0.5 {
		v = 0.5
	}
	return &v
}

// MoeOffloadedRAMGB returns RAM for offloaded (inactive) MoE experts, or nil if not MoE.
func (m *LlmModel) MoeOffloadedRAMGB() *float64 {
	if !m.IsMoE || m.ActiveParameters == nil || m.ParametersRaw == nil {
		return nil
	}
	active := float64(*m.ActiveParameters)
	total := float64(*m.ParametersRaw)
	inactive := total - active
	if inactive <= 0 {
		v := 0.0
		return &v
	}
	bpp := m.quantBPP()
	v := (inactive * bpp) / float64(1024*1024*1024)
	return &v
}
