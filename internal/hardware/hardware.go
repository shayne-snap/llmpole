// Package hardware detects system specs (RAM, CPU, GPU) for the current machine.
package hardware

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// GpuBackend is the acceleration backend used for inference (CUDA, Metal, Vulkan, etc.).
type GpuBackend int

const (
	BackendCuda GpuBackend = iota
	BackendMetal
	BackendRocm
	BackendVulkan
	BackendSycl
	BackendCpuArm
	BackendCpuX86
)

func (b GpuBackend) String() string {
	switch b {
	case BackendCuda:
		return "CUDA"
	case BackendMetal:
		return "Metal"
	case BackendRocm:
		return "ROCm"
	case BackendVulkan:
		return "Vulkan"
	case BackendSycl:
		return "SYCL"
	case BackendCpuArm:
		return "CPU (ARM)"
	case BackendCpuX86:
		return "CPU (x86)"
	default:
		return "CPU (x86)"
	}
}

// GpuInfo holds one detected GPU (name, VRAM, backend, unified memory).
type GpuInfo struct {
	Name           string     `json:"name"`
	VRAMGB         *float64   `json:"vram_gb,omitempty"`
	Backend        GpuBackend `json:"backend"`
	Count          uint32     `json:"count"`
	UnifiedMemory  bool       `json:"unified_memory"`
}

// SystemSpecs holds detected system specs (RAM, CPU, GPUs).
type SystemSpecs struct {
	TotalRAMGB      float64   `json:"total_ram_gb"`
	AvailableRAMGB  float64   `json:"available_ram_gb"`
	TotalCPUCores   int       `json:"cpu_cores"`
	CPUName         string    `json:"cpu_name"`
	HasGPU          bool      `json:"has_gpu"`
	GpuVRAMGB       *float64  `json:"gpu_vram_gb,omitempty"`
	GpuName         *string   `json:"gpu_name,omitempty"`
	GpuCount        uint32    `json:"gpu_count"`
	UnifiedMemory   bool      `json:"unified_memory"`
	Backend         GpuBackend `json:"backend"`
	Gpus            []GpuInfo `json:"gpus"`
}

const gb = 1024 * 1024 * 1024

// Detect returns system specs for the current machine (RAM, CPU, GPUs per OS).
func Detect() (*SystemSpecs, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("mem: %w", err)
	}
	totalRAMGB := float64(v.Total) / float64(gb)
	availableRAMGB := float64(v.Available) / float64(gb)
	if v.Available == 0 && v.Total > 0 {
		availableRAMGB = availableRAMFallback(totalRAMGB)
	}

	infos, _ := cpu.Info()
	totalCPUCores := runtime.NumCPU()
	cpuName := "Unknown CPU"
	if len(infos) > 0 {
		cpuName = infos[0].ModelName
		if cpuName == "" {
			cpuName = infos[0].VendorID
		}
	}

	gpus := detectAllGPUs(totalRAMGB, availableRAMGB, cpuName)
	sort.Slice(gpus, func(i, j int) bool {
		vi, vj := 0.0, 0.0
		if gpus[i].VRAMGB != nil {
			vi = *gpus[i].VRAMGB
		}
		if gpus[j].VRAMGB != nil {
			vj = *gpus[j].VRAMGB
		}
		return vj < vi // descending
	})

	var primary *GpuInfo
	if len(gpus) > 0 {
		primary = &gpus[0]
	}
	hasGPU := len(gpus) > 0
	var gpuVRAMGB *float64
	var gpuName *string
	gpuCount := uint32(0)
	unified := false
	backend := backendCPU(cpuName)
	if primary != nil {
		gpuVRAMGB = primary.VRAMGB
		gpuName = &primary.Name
		gpuCount = primary.Count
		unified = primary.UnifiedMemory
		backend = primary.Backend
	}

	return &SystemSpecs{
		TotalRAMGB:     totalRAMGB,
		AvailableRAMGB: availableRAMGB,
		TotalCPUCores:  totalCPUCores,
		CPUName:        cpuName,
		HasGPU:         hasGPU,
		GpuVRAMGB:      gpuVRAMGB,
		GpuName:        gpuName,
		GpuCount:       gpuCount,
		UnifiedMemory:  unified,
		Backend:        backend,
		Gpus:           gpus,
	}, nil
}

func backendCPU(cpuName string) GpuBackend {
	lower := strings.ToLower(cpuName)
	if strings.Contains(lower, "apple") || runtime.GOARCH == "arm64" {
		return BackendCpuArm
	}
	return BackendCpuX86
}

func availableRAMFallback(totalGB float64) float64 {
	if runtime.GOOS == "darwin" {
		if avail := availableFromVMStat(); avail > 0 {
			return avail
		}
	}
	return totalGB * 0.8
}

func availableFromVMStat() float64 {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0
	}
	var pageSize uint64 = 16384
	var free, inactive, purgeable uint64
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics:") {
			if i := strings.Index(line, "page size of "); i >= 0 {
				rest := line[i+13:]
				if j := strings.IndexAny(rest, " "); j >= 0 {
					if n, err := strconv.ParseUint(rest[:j], 10, 64); err == nil {
						pageSize = n
					}
				}
			}
		}
		if strings.HasPrefix(line, "Pages free:") {
			fmt.Sscanf(strings.Trim(strings.TrimPrefix(line, "Pages free:"), " ."), "%d", &free)
		}
		if strings.HasPrefix(line, "Pages inactive:") {
			fmt.Sscanf(strings.Trim(strings.TrimPrefix(line, "Pages inactive:"), " ."), "%d", &inactive)
		}
		if strings.HasPrefix(line, "Pages purgeable:") {
			fmt.Sscanf(strings.Trim(strings.TrimPrefix(line, "Pages purgeable:"), " ."), "%d", &purgeable)
		}
	}
	avail := (free + inactive + purgeable) * pageSize
	if avail == 0 {
		return 0
	}
	return float64(avail) / float64(gb)
}

func detectAllGPUs(totalRAMGB, availableRAMGB float64, cpuName string) []GpuInfo {
	var gpus []GpuInfo
	gpus = append(gpus, detectNvidiaGPUs()...)
	if amd := detectAMDROCM(); amd != nil {
		gpus = append(gpus, *amd)
	} else if amd := detectAMDSysfs(); amd != nil {
		gpus = append(gpus, *amd)
	}
	for _, wmi := range detectWindowsGPU() {
		dup := false
		for _, e := range gpus {
			el, wl := strings.ToLower(e.Name), strings.ToLower(wmi.Name)
			if strings.Contains(el, wl) || strings.Contains(wl, el) {
				dup = true
				break
			}
		}
		if !dup {
			gpus = append(gpus, wmi)
		}
	}
	if found, vramGB := detectIntelGPU(); found {
		hasIntel := false
		for _, g := range gpus {
			if strings.Contains(strings.ToLower(g.Name), "intel") {
				hasIntel = true
				break
			}
		}
		if !hasIntel {
			gpus = append(gpus, GpuInfo{
				Name: "Intel Arc", VRAMGB: vramGB, Backend: BackendSycl, Count: 1,
			})
		}
	}
	if vram := detectAppleGPU(totalRAMGB, cpuName); vram > 0 {
		name := "Apple Silicon"
		if strings.Contains(strings.ToLower(cpuName), "apple") {
			name = cpuName
		}
		gpus = append(gpus, GpuInfo{
			Name: name, VRAMGB: &vram, Backend: BackendMetal, Count: 1, UnifiedMemory: true,
		})
	}
	return gpus
}

func detectNvidiaGPUs() []GpuInfo {
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.total,name", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var totalVRAMMB float64
	var count uint32
	var firstName string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) < 1 {
			continue
		}
		var vramMB float64
		if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &vramMB); err != nil {
			continue
		}
		totalVRAMMB += vramMB
		count++
		if firstName == "" && len(parts) > 1 {
			firstName = strings.TrimSpace(parts[1])
		}
	}
	if count == 0 {
		return nil
	}
	if firstName == "" {
		firstName = "NVIDIA GPU"
	}
	vramGB := totalVRAMMB / 1024
	if vramGB < 0.1 {
		est := estimateVRAMFromName(firstName)
		vramGB = est
	}
	var v *float64
	if vramGB > 0 {
		v = &vramGB
	}
	return []GpuInfo{{
		Name: firstName, VRAMGB: v, Backend: BackendCuda, Count: count,
	}}
}

func detectAMDROCM() *GpuInfo {
	cmd := exec.Command("rocm-smi", "--showmeminfo", "vram")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var totalBytes uint64
	var gpuCount uint32
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.ToLower(sc.Text())
		if strings.Contains(line, "total") && !strings.Contains(line, "used") {
			fields := strings.Fields(sc.Text())
			for i := len(fields) - 1; i >= 0; i-- {
				if n, err := strconv.ParseUint(fields[i], 10, 64); err == nil && n > 0 {
					totalBytes += n
					gpuCount++
					break
				}
			}
		}
	}
	if gpuCount == 0 {
		gpuCount = 1
	}
	name := "AMD GPU"
	cmd2 := exec.Command("rocm-smi", "--showproductname")
	if out2, err := cmd2.Output(); err == nil {
		sc2 := bufio.NewScanner(bytes.NewReader(out2))
		for sc2.Scan() {
			l := strings.ToLower(sc2.Text())
			if strings.Contains(l, "card series") || strings.Contains(l, "card model") {
				if idx := strings.Index(sc2.Text(), ":"); idx >= 0 {
					name = strings.TrimSpace(sc2.Text()[idx+1:])
					if name != "" {
						break
					}
				}
			}
		}
	}
	var vramGB *float64
	if totalBytes > 0 {
		v := float64(totalBytes) / float64(gb)
		vramGB = &v
	} else {
		est := estimateVRAMFromName(name)
		if est > 0 {
			vramGB = &est
		}
	}
	return &GpuInfo{
		Name: name, VRAMGB: vramGB, Backend: BackendRocm, Count: gpuCount,
	}
}

func detectAMDSysfs() *GpuInfo {
	if runtime.GOOS != "linux" {
		return nil
	}
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		vendor, _ := os.ReadFile(filepath.Join("/sys/class/drm", name, "device/vendor"))
		if strings.TrimSpace(string(vendor)) != "0x1002" {
			continue
		}
		var vramGB *float64
		data, err := os.ReadFile(filepath.Join("/sys/class/drm", name, "device/mem_info_vram_total"))
		if err == nil {
			var bytes uint64
			if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &bytes); err == nil && bytes > 0 {
				v := float64(bytes) / float64(gb)
				vramGB = &v
			}
		}
		gpuName := getAMDGpuNameLspci()
		if gpuName == "" {
			gpuName = "AMD GPU"
		}
		if vramGB == nil {
			est := estimateVRAMFromName(gpuName)
			if est > 0 {
				vramGB = &est
			}
		}
		return &GpuInfo{
			Name: gpuName, VRAMGB: vramGB, Backend: BackendVulkan, Count: 1,
		}
	}
	return nil
}

func getAMDGpuNameLspci() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		l := strings.ToLower(line)
		if (strings.Contains(l, "vga") || strings.Contains(l, "3d")) && (strings.Contains(l, "amd") || strings.Contains(l, "ati")) {
			parts := strings.Split(line, "]:")
			if len(parts) < 2 {
				continue
			}
			desc := strings.TrimSpace(parts[len(parts)-1])
			if start := strings.LastIndex(desc, "["); start >= 0 {
				if end := strings.LastIndex(desc, "]"); end > start {
					return desc[start+1 : end]
				}
			}
			return desc
		}
	}
	return ""
}

func detectWindowsGPU() []GpuInfo {
	if runtime.GOOS != "windows" {
		return nil
	}
	ps := `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ForEach-Object { $_.Name + '|' + $_.AdapterRAM }`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseWindowsGPUList(string(out))
}

func parseWindowsGPUList(text string) []GpuInfo {
	var gpus []GpuInfo
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		name := strings.TrimSpace(parts[0])
		l := strings.ToLower(name)
		if l == "" || strings.Contains(l, "microsoft") || strings.Contains(l, "basic") || strings.Contains(l, "virtual") {
			continue
		}
		var rawVRAM uint64
		if len(parts) > 1 {
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &rawVRAM)
		}
		backend := inferGPUBackend(name)
		vramGB := resolveWmiVRAM(rawVRAM, name)
		gpus = append(gpus, GpuInfo{
			Name: name, VRAMGB: vramGB, Backend: backend, Count: 1,
		})
	}
	return gpus
}

func resolveWmiVRAM(rawBytes uint64, name string) *float64 {
	vramGB := float64(rawBytes) / float64(gb)
	est := estimateVRAMFromName(name)
	if vramGB < 0.1 || (vramGB <= 4.1 && est > 4.1) {
		if est > 0 {
			vramGB = est
		}
	}
	if vramGB > 0 {
		return &vramGB
	}
	return nil
}

func inferGPUBackend(name string) GpuBackend {
	l := strings.ToLower(name)
	if strings.Contains(l, "nvidia") || strings.Contains(l, "geforce") || strings.Contains(l, "quadro") || strings.Contains(l, "tesla") || strings.Contains(l, "rtx") {
		return BackendCuda
	}
	if strings.Contains(l, "amd") || strings.Contains(l, "radeon") || strings.Contains(l, "ati") {
		return BackendVulkan
	}
	if strings.Contains(l, "intel") || strings.Contains(l, "arc") {
		return BackendSycl
	}
	return BackendVulkan
}

func detectIntelGPU() (found bool, vramGB *float64) {
	if runtime.GOOS == "linux" {
		entries, _ := os.ReadDir("/sys/class/drm")
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			devicePath := filepath.Join("/sys/class/drm", name, "device")
			vendor, _ := os.ReadFile(filepath.Join(devicePath, "vendor"))
			if strings.TrimSpace(string(vendor)) != "0x8086" {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(devicePath, "mem_info_vram_total"))
			if len(data) > 0 {
				var bytes uint64
				if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &bytes); err == nil && bytes > 0 {
					v := float64(bytes) / float64(gb)
					return true, &v
				}
			}
		}
		out, err := exec.Command("lspci").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				l := strings.ToLower(line)
				if strings.Contains(l, "intel") && strings.Contains(l, "arc") {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func detectAppleGPU(totalRAMGB float64, cpuName string) float64 {
	if runtime.GOOS != "darwin" {
		return 0
	}
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return 0
	}
	text := string(out)
	for _, line := range strings.Split(text, "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, "apple m") || strings.Contains(l, "apple gpu") {
			return totalRAMGB
		}
	}
	return 0
}

var (
	wslOnce sync.Once
	wslVal  bool
)

// IsRunningInWSL returns true if running under WSL (Linux only).
func IsRunningInWSL() bool {
	wslOnce.Do(func() {
		if runtime.GOOS != "linux" {
			return
		}
		if os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "" {
			wslVal = true
			return
		}
		for _, p := range []string{"/proc/sys/kernel/osrelease", "/proc/version"} {
			b, _ := os.ReadFile(p)
			if strings.Contains(strings.ToLower(string(b)), "microsoft") {
				wslVal = true
				return
			}
		}
	})
	return wslVal
}

// estimateVRAMFromName estimates VRAM in GB from GPU name when API does not provide it.
func estimateVRAMFromName(name string) float64 {
	l := strings.ToLower(name)
	// NVIDIA RTX 50
	if strings.Contains(l, "5090") { return 32 }
	if strings.Contains(l, "5080") { return 16 }
	if strings.Contains(l, "5070 ti") { return 16 }
	if strings.Contains(l, "5070") { return 12 }
	if strings.Contains(l, "5060 ti") { return 16 }
	if strings.Contains(l, "5060") { return 8 }
	// RTX 40
	if strings.Contains(l, "4090") { return 24 }
	if strings.Contains(l, "4080") { return 16 }
	if strings.Contains(l, "4070 ti") { return 12 }
	if strings.Contains(l, "4070") { return 12 }
	if strings.Contains(l, "4060 ti") { return 16 }
	if strings.Contains(l, "4060") { return 8 }
	// RTX 30
	if strings.Contains(l, "3090") { return 24 }
	if strings.Contains(l, "3080 ti") { return 12 }
	if strings.Contains(l, "3080") { return 10 }
	if strings.Contains(l, "3070") { return 8 }
	if strings.Contains(l, "3060 ti") { return 8 }
	if strings.Contains(l, "3060") { return 12 }
	// Data center
	if strings.Contains(l, "h100") { return 80 }
	if strings.Contains(l, "a100") { return 80 }
	if strings.Contains(l, "l40") { return 48 }
	if strings.Contains(l, "a10") { return 24 }
	if strings.Contains(l, "t4") { return 16 }
	// AMD RX 9000/7000/6000/5000
	if strings.Contains(l, "9070 xt") { return 16 }
	if strings.Contains(l, "9070") { return 12 }
	if strings.Contains(l, "7900 xtx") { return 24 }
	if strings.Contains(l, "7900") { return 20 }
	if strings.Contains(l, "7800") { return 16 }
	if strings.Contains(l, "7700") { return 12 }
	if strings.Contains(l, "7600") { return 8 }
	if strings.Contains(l, "6950") { return 16 }
	if strings.Contains(l, "6900") { return 16 }
	if strings.Contains(l, "6800") { return 16 }
	if strings.Contains(l, "6750") { return 12 }
	if strings.Contains(l, "6700") { return 12 }
	if strings.Contains(l, "6650") { return 8 }
	if strings.Contains(l, "6600") { return 8 }
	if strings.Contains(l, "6500") { return 4 }
	if strings.Contains(l, "5700 xt") { return 8 }
	if strings.Contains(l, "5700") { return 8 }
	if strings.Contains(l, "5600") { return 6 }
	if strings.Contains(l, "5500") { return 4 }
	if strings.Contains(l, "rtx") { return 8 }
	if strings.Contains(l, "gtx") { return 4 }
	if strings.Contains(l, "rx ") || strings.Contains(l, "radeon") { return 8 }
	return 0
}
