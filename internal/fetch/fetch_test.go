package fetch

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatParamCount(t *testing.T) {
	tests := []struct {
		n    uint64
		want string
	}{
		{7_000_000_000, "7B"},
		{1_500_000_000, "1.5B"},
		{600_000_000, "600M"},
		{137_000_000, "137M"},
		{1_000_000, "1M"},
		{1_000, "1K"},
		{70_000_000_000, "70B"},
	}
	for _, tt := range tests {
		got := formatParamCount(tt.n)
		if got != tt.want {
			t.Errorf("formatParamCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestEstimateRAM(t *testing.T) {
	minRAM, recRAM := estimateRAM(7_000_000_000)
	if minRAM < 3 || minRAM > 5 {
		t.Errorf("estimateRAM(7B) minRAM = %v, want ~3.7", minRAM)
	}
	if recRAM < 6 || recRAM > 8 {
		t.Errorf("estimateRAM(7B) recRAM = %v, want ~6–8", recRAM)
	}
	minRAM2, recRAM2 := estimateRAM(100_000)
	if minRAM2 < 1 {
		t.Errorf("estimateRAM(small) minRAM = %v, want >= 1", minRAM2)
	}
	if recRAM2 < 2 {
		t.Errorf("estimateRAM(small) recRAM = %v, want >= 2", recRAM2)
	}
}

func TestEstimateVRAM(t *testing.T) {
	v := estimateVRAM(7_000_000_000)
	if v < 0.5 {
		t.Errorf("estimateVRAM(7B) = %v, want >= 0.5", v)
	}
	v2 := estimateVRAM(70_000_000_000)
	if v2 <= v {
		t.Errorf("estimateVRAM(70B) = %v should be > estimateVRAM(7B) = %v", v2, v)
	}
}

func TestInferContextLength(t *testing.T) {
	if inferContextLength(nil) != 0 {
		t.Error("inferContextLength(nil) should be 0")
	}
	cfg := configJSON{"max_position_embeddings": float64(8192)}
	if got := inferContextLength(cfg); got != 8192 {
		t.Errorf("inferContextLength(max_position_embeddings) = %d, want 8192", got)
	}
	cfg2 := configJSON{"max_sequence_length": 4096}
	if got := inferContextLength(cfg2); got != 4096 {
		t.Errorf("inferContextLength(max_sequence_length) = %d, want 4096", got)
	}
	cfg3 := configJSON{"n_positions": float64(2048)}
	if got := inferContextLength(cfg3); got != 2048 {
		t.Errorf("inferContextLength(n_positions) = %d, want 2048", got)
	}
}

func TestInferUseCase(t *testing.T) {
	tests := []struct {
		repoID       string
		pipelineTag  string
		wantContains string
	}{
		{"org/embed-x", "", "embedding"},
		{"org/coder-7b", "", "Code generation"},
		{"org/starcoder", "", "Code generation"},
		{"org/r1-model", "", "reasoning"},
		{"org/instruct-7b", "", "Instruction following"},
		{"org/chat-model", "", "Instruction following"},
		{"org/tiny-llama", "", "Lightweight"},
		{"org/small-model", "", "Lightweight"},
		{"org/any", "text-generation", "General purpose text generation"},
		{"org/other", "", "General purpose"},
	}
	for _, tt := range tests {
		got := inferUseCase(tt.repoID, tt.pipelineTag, nil)
		if !strings.Contains(got, tt.wantContains) {
			t.Errorf("inferUseCase(%q, %q) = %q, want containing %q", tt.repoID, tt.pipelineTag, got, tt.wantContains)
		}
	}
}

func TestExtractProvider(t *testing.T) {
	tests := []struct {
		repoID string
		want   string
	}{
		{"meta-llama/Llama-7b", "Meta"},
		{"unknown-org/model", "unknown-org"},
		{"noreal-slash", "noreal-slash"},
		{"/leading", "/leading"},
	}
	for _, tt := range tests {
		got := extractProvider(tt.repoID)
		if got != tt.want {
			t.Errorf("extractProvider(%q) = %q, want %q", tt.repoID, got, tt.want)
		}
	}
}

func TestToInt(t *testing.T) {
	if n, ok := toInt(float64(42)); !ok || n != 42 {
		t.Errorf("toInt(42.0) = %d, %v", n, ok)
	}
	if n, ok := toInt(43); !ok || n != 43 {
		t.Errorf("toInt(43) = %d, %v", n, ok)
	}
	if n, ok := toInt("x"); ok || n != 0 {
		t.Errorf("toInt(\"x\") = %d, %v", n, ok)
	}
}

func TestDetectMoE_FromConfig(t *testing.T) {
	cfg := configJSON{
		"num_local_experts":    8,
		"num_experts_per_tok":  2,
	}
	isMoE, numExp, activeExp, activeParams := detectMoE("org/repo", cfg, "unknown", 7_000_000_000)
	if !isMoE {
		t.Error("detectMoE from config: want isMoE true")
	}
	if numExp == nil || *numExp != 8 {
		t.Errorf("numExperts = %v", numExp)
	}
	if activeExp == nil || *activeExp != 2 {
		t.Errorf("activeExperts = %v", activeExp)
	}
	if activeParams == nil {
		t.Error("activeParams should be set")
	}
}

func TestDetectMoE_FromArch(t *testing.T) {
	isMoE, numExp, activeExp, _ := detectMoE("org/repo", nil, "mixtral", 7_000_000_000)
	if !isMoE {
		t.Error("detectMoE from arch: want isMoE true")
	}
	if numExp == nil || *numExp != 8 {
		t.Errorf("numExperts = %v", numExp)
	}
	if activeExp == nil || *activeExp != 2 {
		t.Errorf("activeExperts = %v", activeExp)
	}
}

func TestDetectMoE_KnownRepo(t *testing.T) {
	_, _, _, activeParams := detectMoE("mistralai/Mixtral-8x7B-Instruct-v0.1", nil, "mixtral", 7_000_000_000)
	if activeParams == nil {
		t.Fatal("activeParams nil")
	}
	if *activeParams != 12_900_000_000 {
		t.Errorf("activeParams = %d, want 12900000000", *activeParams)
	}
}

func TestEstimateActiveParams(t *testing.T) {
	// total 8B, 8 experts, 2 active -> shared 5%, expert pool split, 2*perExpert + shared
	total := uint64(8_000_000_000)
	got := estimateActiveParams(total, 8, 2)
	shared := uint64(float64(total) * 0.05)
	expertPool := total - shared
	perExpert := expertPool / 8
	expectedApprox := shared + 2*perExpert
	diff := math.Abs(float64(got) - float64(expectedApprox))
	if diff > float64(expectedApprox)*0.01 {
		t.Errorf("estimateActiveParams = %d, expected approx %d", got, expectedApprox)
	}
}

func TestFetchModel_Success(t *testing.T) {
	apiResp := map[string]interface{}{
		"safetensors": map[string]interface{}{
			"total": float64(7_000_000_000),
		},
		"config": map[string]interface{}{
			"model_type":             "llama",
			"max_position_embeddings": float64(4096),
		},
		"pipeline_tag": "text-generation",
	}
	body, _ := json.Marshal(apiResp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/models/org/repo" {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
			return
		}
		if r.URL.Path == "/org/repo/resolve/main/config.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	apiBaseForTest = server.URL
	defer func() { apiBaseForTest = "" }()

	m, err := FetchModel("org/repo")
	if err != nil {
		t.Fatalf("FetchModel: %v", err)
	}
	if m == nil {
		t.Fatal("FetchModel returned nil model")
	}
	if m.Name != "org/repo" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.ParameterCount != "7B" {
		t.Errorf("ParameterCount = %q", m.ParameterCount)
	}
	if m.ContextLength != 4096 {
		t.Errorf("ContextLength = %d", m.ContextLength)
	}
	if m.Provider == "" {
		t.Error("Provider should be set")
	}
}

func TestFetchModel_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	apiBaseForTest = server.URL
	defer func() { apiBaseForTest = "" }()

	_, err := FetchModel("org/repo")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetchModel_NoParams(t *testing.T) {
	apiResp := map[string]interface{}{
		"safetensors": map[string]interface{}{},
	}
	body, _ := json.Marshal(apiResp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()
	apiBaseForTest = server.URL
	defer func() { apiBaseForTest = "" }()

	_, err := FetchModel("org/repo")
	if err == nil {
		t.Fatal("expected error when safetensors has no total/parameters")
	}
}

func TestFetchModelList(t *testing.T) {
	validBody := []byte(`[{"name":"org/model","provider":"Org","parameter_count":"7B"}]`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/list" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("User-Agent = %q, want %q", r.Header.Get("User-Agent"), userAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(validBody)
	}))
	defer server.Close()

	ctx := context.Background()
	body, err := FetchModelList(ctx, server.URL+"/list")
	if err != nil {
		t.Fatalf("FetchModelList: %v", err)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(body, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}
}

func TestFetchModelList_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := FetchModelList(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}
