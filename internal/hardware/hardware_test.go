package hardware

import (
	"runtime"
	"testing"
)

func TestParseWindowsGPUList(t *testing.T) {
	text := "NVIDIA GeForce RTX 4090|25769803776\nMicrosoft Basic Display|0\n\nAMD Radeon RX 7800|17179869184\n"
	gpus := parseWindowsGPUList(text)
	// Microsoft and empty lines skipped -> 2 GPUs
	if len(gpus) != 2 {
		t.Fatalf("parseWindowsGPUList len = %d, want 2", len(gpus))
	}
	// First: NVIDIA
	if gpus[0].Name != "NVIDIA GeForce RTX 4090" {
		t.Errorf("gpus[0].Name = %q", gpus[0].Name)
	}
	if gpus[0].Backend != BackendCuda {
		t.Errorf("gpus[0].Backend = %v", gpus[0].Backend)
	}
	// Second: AMD
	if gpus[1].Name != "AMD Radeon RX 7800" {
		t.Errorf("gpus[1].Name = %q", gpus[1].Name)
	}
	if gpus[1].Backend != BackendVulkan {
		t.Errorf("gpus[1].Backend = %v", gpus[1].Backend)
	}
}

func TestResolveWmiVRAM(t *testing.T) {
	// rawBytes small but name known -> use estimate
	got := resolveWmiVRAM(0, "NVIDIA GeForce RTX 4090")
	if got == nil {
		t.Fatal("resolveWmiVRAM(0, RTX 4090) = nil")
	}
	if *got != 24 {
		t.Errorf("resolveWmiVRAM(0, RTX 4090) = %v, want 24", *got)
	}
	// rawBytes large -> use raw
	got2 := resolveWmiVRAM(32*1024*1024*1024, "Unknown GPU")
	if got2 == nil {
		t.Fatal("resolveWmiVRAM(32GB, Unknown) = nil")
	}
	if *got2 != 32 {
		t.Errorf("resolveWmiVRAM(32GB, Unknown) = %v, want 32", *got2)
	}
}

func TestInferGPUBackend(t *testing.T) {
	tests := []struct {
		name string
		want GpuBackend
	}{
		{"NVIDIA GeForce RTX 3080", BackendCuda},
		{"AMD Radeon RX 7900", BackendVulkan},
		{"Intel Arc A770", BackendSycl},
		{"Unknown GPU", BackendVulkan},
	}
	for _, tt := range tests {
		got := inferGPUBackend(tt.name)
		if got != tt.want {
			t.Errorf("inferGPUBackend(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEstimateVRAMFromName(t *testing.T) {
	tests := []struct {
		name string
		want float64
	}{
		{"NVIDIA GeForce RTX 4090", 24},
		{"RTX 4080", 16},
		{"H100", 80},
		{"A100", 80},
		{"AMD Radeon RX 7900 XTX", 24},
		{"RX 7800", 16},
		{"RTX 3060", 12},
		{"Unknown", 0},
	}
	for _, tt := range tests {
		got := estimateVRAMFromName(tt.name)
		if got != tt.want {
			t.Errorf("estimateVRAMFromName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBackendCPU(t *testing.T) {
	// Apple / apple in name -> BackendCpuArm
	got := backendCPU("Apple M1 Pro")
	if got != BackendCpuArm {
		t.Errorf("backendCPU(Apple M1 Pro) = %v, want BackendCpuArm", got)
	}
	got2 := backendCPU("apple silicon")
	if got2 != BackendCpuArm {
		t.Errorf("backendCPU(apple silicon) = %v, want BackendCpuArm", got2)
	}
	// Intel on x86 -> BackendCpuX86; on arm64 -> BackendCpuArm (due to GOARCH)
	got3 := backendCPU("Intel Xeon")
	if runtime.GOARCH == "arm64" {
		if got3 != BackendCpuArm {
			t.Errorf("backendCPU(Intel Xeon) on arm64 = %v, want BackendCpuArm", got3)
		}
	} else {
		if got3 != BackendCpuX86 {
			t.Errorf("backendCPU(Intel Xeon) on %s = %v, want BackendCpuX86", runtime.GOARCH, got3)
		}
	}
}
