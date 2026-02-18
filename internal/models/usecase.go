package models

import "strings"

// UseCaseFromModel infers the use case from the model name and use_case string.
func UseCaseFromModel(m *LlmModel) UseCase {
	name := strings.ToLower(m.Name)
	uc := strings.ToLower(m.UseCase)
	if strings.Contains(uc, "embedding") || strings.Contains(name, "embed") || strings.Contains(name, "bge") {
		return UseCaseEmbedding
	}
	if strings.Contains(name, "code") || strings.Contains(uc, "code") {
		return UseCaseCoding
	}
	if strings.Contains(uc, "vision") || strings.Contains(uc, "multimodal") {
		return UseCaseMultimodal
	}
	if strings.Contains(uc, "reason") || strings.Contains(uc, "chain-of-thought") || strings.Contains(name, "deepseek-r1") {
		return UseCaseReasoning
	}
	if strings.Contains(uc, "chat") || strings.Contains(uc, "instruction") {
		return UseCaseChat
	}
	return UseCaseGeneral
}
