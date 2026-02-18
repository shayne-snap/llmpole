// Package fetch fetches a single model's metadata from HuggingFace API (for on-demand add to cache).
package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shayne-snap/llmpole/internal/models"
)

const (
	hfAPI        = "https://huggingface.co/api/models"
	timeoutSec   = 30
	runtimeOver  = 1.2
	quantBPPQ4   = 0.5
	defaultCtx   = 4096
)

// hfAPIResponse is the minimal shape of GET /api/models/{repo_id} we need.
type hfAPIResponse struct {
	Config       map[string]interface{} `json:"config"`
	PipelineTag  string                 `json:"pipeline_tag"`
	Safetensors  *struct {
		Total      *uint64            `json:"total"`
		Parameters map[string]uint64  `json:"parameters"`
	} `json:"safetensors"`
}

// configJSON is the shape of config.json for context length.
type configJSON map[string]interface{}

var moeConfigs = map[string]struct{ NumExperts, ActiveExperts int }{
	"mixtral":       {8, 2},
	"deepseek_v2":   {64, 6},
	"deepseek_v3":   {256, 8},
	"qwen3_moe":     {128, 8},
	"llama4":        {16, 1},
	"grok":          {8, 2},
}

var moeActiveParams = map[string]uint64{
	"mistralai/Mixtral-8x7B-Instruct-v0.1":                    12_900_000_000,
	"mistralai/Mixtral-8x22B-Instruct-v0.1":                   39_100_000_000,
	"NousResearch/Nous-Hermes-2-Mixtral-8x7B-DPO":            12_900_000_000,
	"deepseek-ai/DeepSeek-Coder-V2-Lite-Instruct":            2_400_000_000,
	"deepseek-ai/DeepSeek-V3":                                37_000_000_000,
	"deepseek-ai/DeepSeek-R1":                                37_000_000_000,
	"Qwen/Qwen3-30B-A3B":                                      3_300_000_000,
	"Qwen/Qwen3-235B-A22B":                                    22_000_000_000,
	"Qwen/Qwen3-Coder-480B-A35B-Instruct":                     35_000_000_000,
	"meta-llama/Llama-4-Scout-17B-16E-Instruct":              17_000_000_000,
	"meta-llama/Llama-4-Maverick-17B-128E-Instruct":          17_000_000_000,
	"xai-org/grok-1":                                          86_000_000_000,
	"moonshotai/Kimi-K2-Instruct":                             32_000_000_000,
}

var providerMap = map[string]string{
	"meta-llama": "Meta", "mistralai": "Mistral AI", "qwen": "Alibaba",
	"microsoft": "Microsoft", "google": "Google", "deepseek-ai": "DeepSeek",
	"bigcode": "BigCode", "cohereforai": "Cohere", "tinyllama": "Community",
	"stabilityai": "Stability AI", "nomic-ai": "Nomic", "baai": "BAAI",
	"01-ai": "01.ai", "upstage": "Upstage", "tiiuae": "TII",
	"huggingfaceh4": "HuggingFace", "openchat": "OpenChat", "lmsys": "LMSYS",
	"nousresearch": "NousResearch", "wizardlmteam": "WizardLM",
	"allenai": "Allen Institute", "ibm-granite": "IBM", "inclusionai": "Ant Group",
	"baidu": "Baidu", "meituan": "Meituan", "rednote-hilab": "Rednote",
	"moonshotai": "Moonshot", "thudm": "Zhipu AI", "xai-org": "xAI",
}

const userAgent = "llmpole/0.1.0"

// apiBaseForTest, when set by tests, overrides the base URL for FetchModel and fetchConfigJSON.
var apiBaseForTest string

func apiBase() string {
	if apiBaseForTest != "" {
		return apiBaseForTest
	}
	return "https://huggingface.co"
}

// FetchModelList fetches the raw model list JSON from url (e.g. default list URL). Caller should validate and write to cache.
func FetchModelList(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("update-list: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not update list: %v (check network)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not update list: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not update list: %w", err)
	}
	return body, nil
}

// FetchModel fetches one model by repo_id from HuggingFace and returns an LlmModel (or error).
func FetchModel(repoID string) (*models.LlmModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	url := apiBase() + "/api/models/" + repoID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	var info hfAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	totalParams := uint64(0)
	if info.Safetensors != nil && info.Safetensors.Total != nil {
		totalParams = *info.Safetensors.Total
	} else if info.Safetensors != nil && len(info.Safetensors.Parameters) > 0 {
		for _, v := range info.Safetensors.Parameters {
			if v > totalParams {
				totalParams = v
			}
		}
	}
	if totalParams == 0 {
		return nil, fmt.Errorf("no parameter count in API response (gated or private repo?)")
	}

	arch := "unknown"
	if info.Config != nil {
		if v, _ := info.Config["model_type"].(string); v != "" {
			arch = v
		}
	}
	fullConfig := fetchConfigJSON(repoID)
	ctxLen := inferContextLength(fullConfig)
	if ctxLen == 0 && info.Config != nil {
		ctxLen = inferContextLength(info.Config)
	}
	if ctxLen == 0 {
		ctxLen = defaultCtx
	}

	minRAM, recRAM := estimateRAM(totalParams)
	minVRAM := estimateVRAM(totalParams)
	quant := "Q4_K_M"
	isMoE, numExp, activeExp, activeParams := detectMoE(repoID, fullConfig, arch, totalParams)

	m := &models.LlmModel{
		Name:             repoID,
		Provider:         extractProvider(repoID),
		ParameterCount:   formatParamCount(totalParams),
		ParametersRaw:    &totalParams,
		MinRAMGB:         minRAM,
		RecommendedRAMGB: recRAM,
		MinVRAMGB:        &minVRAM,
		Quantization:     quant,
		ContextLength:    uint32(ctxLen),
		UseCase:          inferUseCase(repoID, info.PipelineTag, info.Config),
		IsMoE:            isMoE,
		NumExperts:       numExp,
		ActiveExperts:    activeExp,
		ActiveParameters: activeParams,
	}
	return m, nil
}

func fetchConfigJSON(repoID string) configJSON {
	url := apiBase() + "/" + repoID + "/resolve/main/config.json"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var c configJSON
	if json.NewDecoder(resp.Body).Decode(&c) != nil {
		return nil
	}
	return c
}

func formatParamCount(n uint64) string {
	if n >= 1_000_000_000 {
		val := float64(n) / 1e9
		if val == math.Trunc(val) {
			return strconv.Itoa(int(val)) + "B"
		}
		return fmt.Sprintf("%.1fB", val)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.0fM", float64(n)/1e6)
	}
	return fmt.Sprintf("%.0fK", float64(n)/1e3)
}

func estimateRAM(totalParams uint64) (minRAM, recRAM float64) {
	modelSizeGB := (float64(totalParams) * quantBPPQ4) / (1024 * 1024 * 1024)
	minRAM = modelSizeGB * runtimeOver
	recRAM = modelSizeGB * 2.0
	if minRAM < 1.0 {
		minRAM = 1.0
	}
	if recRAM < 2.0 {
		recRAM = 2.0
	}
	return round1(minRAM), round1(recRAM)
}

func estimateVRAM(totalParams uint64) float64 {
	modelSizeGB := (float64(totalParams) * quantBPPQ4) / (1024 * 1024 * 1024)
	v := modelSizeGB * 1.1
	if v < 0.5 {
		v = 0.5
	}
	return round1(v)
}

func round1(x float64) float64 {
	return math.Round(x*10) / 10
}

func inferContextLength(c configJSON) int {
	if c == nil {
		return 0
	}
	for _, key := range []string{"max_position_embeddings", "max_sequence_length", "seq_length", "n_positions", "sliding_window"} {
		if v, ok := c[key]; ok {
			switch n := v.(type) {
			case float64:
				if n > 0 {
					return int(n)
				}
			case int:
				if n > 0 {
					return n
				}
			}
		}
	}
	return 0
}

func inferUseCase(repoID, pipelineTag string, config map[string]interface{}) string {
	rid := strings.ToLower(repoID)
	if strings.Contains(rid, "embed") || strings.Contains(rid, "bge") {
		return "Text embeddings for RAG"
	}
	if strings.Contains(rid, "coder") || strings.Contains(rid, "starcoder") || strings.Contains(rid, "code") {
		return "Code generation and completion"
	}
	if strings.Contains(rid, "r1") || strings.Contains(rid, "reason") {
		return "Advanced reasoning, chain-of-thought"
	}
	if strings.Contains(rid, "instruct") || strings.Contains(rid, "chat") {
		return "Instruction following, chat"
	}
	if strings.Contains(rid, "tiny") || strings.Contains(rid, "small") || strings.Contains(rid, "mini") {
		return "Lightweight, edge deployment"
	}
	if pipelineTag == "text-generation" {
		return "General purpose text generation"
	}
	return "General purpose"
}

func extractProvider(repoID string) string {
	i := strings.Index(repoID, "/")
	if i <= 0 {
		return repoID
	}
	org := strings.ToLower(repoID[:i])
	if p, ok := providerMap[org]; ok {
		return p
	}
	return org
}

func detectMoE(repoID string, fullConfig configJSON, arch string, totalParams uint64) (isMoE bool, numExperts, activeExperts *uint32, activeParams *uint64) {
	var numExp, activeExp int
	if fullConfig != nil {
		if v, ok := fullConfig["num_local_experts"]; ok {
			if n, ok := toInt(v); ok && n > 0 {
				numExp = n
			}
		}
		if v, ok := fullConfig["num_experts_per_tok"]; ok {
			if n, ok := toInt(v); ok && n > 0 {
				activeExp = n
			}
		}
	}
	if numExp == 0 || activeExp == 0 {
		if c, ok := moeConfigs[arch]; ok {
			numExp = c.NumExperts
			activeExp = c.ActiveExperts
		}
	}
	if numExp == 0 || activeExp == 0 {
		return false, nil, nil, nil
	}
	n := uint32(numExp)
	a := uint32(activeExp)
	isMoE = true
	numExperts = &n
	activeExperts = &a
	if v, ok := moeActiveParams[repoID]; ok {
		activeParams = &v
	} else {
		ap := estimateActiveParams(totalParams, numExp, activeExp)
		activeParams = &ap
	}
	return
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

func estimateActiveParams(totalParams uint64, numExperts, activeExperts int) uint64 {
	sharedFrac := 0.05
	shared := uint64(float64(totalParams) * sharedFrac)
	expertPool := totalParams - shared
	perExpert := expertPool / uint64(numExperts)
	return shared + uint64(activeExperts)*perExpert
}
