package ai

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Sizes in this package are mebibytes: 1 MB here means 1024*1024 bytes, which
// is what /proc/meminfo, nvidia-smi and the model registry all report in.
const (
	mib = 1 << 20
	gib = 1 << 30

	// gpuProbeTimeout caps a vendor tool that has to wake a powered-down GPU.
	gpuProbeTimeout = 3 * time.Second
)

// GPU is one graphics device found on the machine.
type GPU struct {
	// Vendor is "nvidia", "amd" or "intel".
	Vendor string
	// Name is the marketing name when a vendor tool could be asked for it.
	Name string
	// VRAMMB is the board's total memory, or zero when it could not be
	// measured. Zero is not "no memory": an integrated GPU has no fixed
	// amount, and a discrete card with no vendor tool installed cannot be
	// asked. Treat zero as unknown and size models against system memory.
	VRAMMB int
	// Integrated reports graphics that share system memory. They are sized
	// as CPU-tier, whatever the runtime can offload to them.
	Integrated bool
	// Source names where the reading came from: "nvidia-smi", "rocm-smi" or
	// "sysfs".
	Source string
}

// String renders one device for a report line.
func (g GPU) String() string {
	name := g.Name
	if name == "" {
		name = g.Vendor + " GPU"
	}
	if g.VRAMMB > 0 {
		return fmt.Sprintf("%s (%s)", name, formatMB(g.VRAMMB))
	}
	if g.Integrated {
		return name + " (integrated, shares system memory)"
	}
	return name + " (memory unknown)"
}

// Hardware is what the machine can offer a local model.
type Hardware struct {
	CPUModel string
	// Cores is the number of usable logical CPUs, honouring any affinity or
	// container limit. PhysicalCores is what the topology says, when it says.
	Cores         int
	PhysicalCores int
	// RAMTotalMB answers "is this machine capable?" and RAMFreeMB answers
	// "can it run this right now?". The free figure is the kernel's own
	// estimate of what is obtainable without swapping, not merely unused
	// memory.
	RAMTotalMB int
	RAMFreeMB  int
	GPUs       []GPU
	// ModelStorePath is the directory the models live in, and DiskFreeMB is
	// what is available to an unprivileged user on its filesystem.
	ModelStorePath string
	DiskFreeMB     int64
}

// BestVRAMMB returns the largest measured memory on a discrete GPU, or zero
// when the machine has none the tool could measure. Model sizing keys on this.
func (h Hardware) BestVRAMMB() int {
	best := 0
	for _, g := range h.GPUs {
		if !g.Integrated && g.VRAMMB > best {
			best = g.VRAMMB
		}
	}
	return best
}

// HasGPU reports whether a discrete GPU with measured memory was found.
func (h Hardware) HasGPU() bool { return h.BestVRAMMB() > 0 }

// Describe renders the probe as the lines a setup command prints.
func (h Hardware) Describe() []string {
	lines := []string{
		fmt.Sprintf("CPU:   %s (%d cores)", orUnknown(h.CPUModel), h.Cores),
		fmt.Sprintf("RAM:   %s total, %s available", formatMB(h.RAMTotalMB), formatMB(h.RAMFreeMB)),
	}
	if len(h.GPUs) == 0 {
		lines = append(lines, "GPU:   none detected, so models run on the CPU")
	}
	for i, g := range h.GPUs {
		label := "GPU:  "
		if i > 0 {
			label = "       "
		}
		lines = append(lines, fmt.Sprintf("%s %s", label, g))
	}
	lines = append(lines, fmt.Sprintf("Disk:  %s free on %s", formatMB(int(h.DiskFreeMB)), h.ModelStorePath))
	return lines
}

// Probe reads what this machine has. modelStorePath is the directory models
// are kept in; an empty value means the default store for the current user.
//
// It never fails because an optional tool is missing: a machine with no
// nvidia-smi and no AMD sysfs entries simply reports no GPU. The error is
// reserved for the readings that must work, which is memory and disk.
func Probe(modelStorePath string) (Hardware, error) {
	hw := Hardware{
		Cores:          runtime.NumCPU(),
		ModelStorePath: modelStorePath,
	}
	if hw.ModelStorePath == "" {
		hw.ModelStorePath = DefaultModelStore()
	}

	hw.CPUModel, hw.PhysicalCores = probeCPU()
	total, free, err := probeMemory()
	if err != nil {
		return hw, err
	}
	hw.RAMTotalMB, hw.RAMFreeMB = total, free

	hw.GPUs = probeGPUs()

	freeDisk, err := freeDiskMB(hw.ModelStorePath)
	if err != nil {
		return hw, fmt.Errorf("checking free space on %s: %w", hw.ModelStorePath, err)
	}
	hw.DiskFreeMB = freeDisk
	return hw, nil
}

// DefaultModelStore returns where the local runtime keeps its models, honouring
// the runtime's own environment variable.
//
// There are two plausible locations and picking the wrong one reports a
// directory nothing is reading. A runtime installed as a system service runs
// under its own account and keeps models under /usr/share; a runtime started by
// hand keeps them under the invoking user's home. Both can exist on the same
// machine, because pulling a model before installing the service leaves the
// user copy behind, so existence alone does not settle it. A populated store is
// the better signal: whichever one actually holds manifests is the one in use,
// and the service copy wins a tie because a registered service is what serves
// the endpoint.
func DefaultModelStore() string {
	if dir := strings.TrimSpace(os.Getenv("OLLAMA_MODELS")); dir != "" {
		return dir
	}

	const systemStore = "/usr/share/ollama/.ollama/models"
	userStore := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		userStore = filepath.Join(home, ".ollama", "models")
	}

	for _, dir := range []string{systemStore, userStore} {
		if dir == "" {
			continue
		}
		if entries, err := os.ReadDir(filepath.Join(dir, "manifests")); err == nil && len(entries) > 0 {
			return dir
		}
	}
	for _, dir := range []string{systemStore, userStore} {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		} else if errors.Is(err, fs.ErrPermission) {
			// Being refused is itself the answer. The path exists and belongs to
			// another account, which is exactly what a service-managed store
			// looks like from an ordinary user's shell.
			return dir
		}
	}
	if userStore != "" {
		return userStore
	}
	return systemStore
}

// probeCPU reads the model name and physical core count. The flags line in
// /proc/cpuinfo is very long, so the scanner gets a bigger buffer.
func probeCPU() (model string, physical int) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	cores := map[string]bool{}
	var physID, coreID string
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			// A blank line ends one processor block.
			physID, coreID = "", ""
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "model name", "Model name":
			if model == "" {
				model = value
			}
		case "physical id":
			physID = value
		case "core id":
			coreID = value
		}
		if physID != "" && coreID != "" {
			cores[physID+"/"+coreID] = true
			physID, coreID = "", ""
		}
	}
	return model, len(cores)
}

// probeMemory reads total and available memory in MB.
//
// The reading comes from /proc/meminfo, which is a Linux interface. A system
// without it is told so plainly rather than shown a file error, because the
// answer there is "this cannot be measured here", not "something went wrong".
func probeMemory() (totalMB, freeMB int, err error) {
	f, err := os.Open("/proc/meminfo")
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, errors.New("this system has no /proc/meminfo, so the memory a model would need cannot be measured here")
	}
	if err != nil {
		return 0, 0, fmt.Errorf("reading system memory: %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		kb, convErr := strconv.Atoi(fields[0])
		if convErr != nil {
			continue
		}
		// Values are kibibytes wherever a unit is present at all; entries
		// without one are counters this does not read.
		switch strings.TrimSpace(key) {
		case "MemTotal":
			totalMB = kb / 1024
		case "MemAvailable":
			freeMB = kb / 1024
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, fmt.Errorf("reading system memory: %w", err)
	}
	if totalMB == 0 {
		return 0, 0, fmt.Errorf("reading system memory: /proc/meminfo reported no total")
	}
	return totalMB, freeMB, nil
}

// probeGPUs asks the vendor tools first, because they are the only source of a
// memory figure, then fills in anything they missed from sysfs.
func probeGPUs() []GPU {
	var gpus []GPU
	seen := map[string]bool{}

	for _, g := range probeNvidiaSMI() {
		gpus = append(gpus, g)
		seen[g.Vendor] = true
	}
	for _, g := range probeROCmSMI() {
		gpus = append(gpus, g)
		seen[g.Vendor] = true
	}
	for _, g := range probeSysfsGPUs() {
		if seen[g.Vendor] {
			continue
		}
		gpus = append(gpus, g)
	}
	return gpus
}

// probeNvidiaSMI reads name and total memory. memory.total is always in MiB,
// and nounits strips the suffix. Names can contain commas, so the split is on
// the last one.
func probeNvidiaSMI() []GPU {
	out, ok := runTool("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	if !ok {
		return probeNvidiaProc()
	}
	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.LastIndex(line, ",")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		vram, err := strconv.Atoi(strings.TrimSpace(line[idx+1:]))
		if err != nil {
			vram = 0
		}
		gpus = append(gpus, GPU{Vendor: "nvidia", Name: name, VRAMMB: vram, Source: "nvidia-smi"})
	}
	if len(gpus) == 0 {
		return probeNvidiaProc()
	}
	return gpus
}

// probeNvidiaProc is the fallback when the vendor tool is absent: the driver's
// own files name the card but do not report its memory.
func probeNvidiaProc() []GPU {
	entries, err := filepath.Glob("/proc/driver/nvidia/gpus/*/information")
	if err != nil || len(entries) == 0 {
		return nil
	}
	var gpus []GPU
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name := ""
		for _, line := range strings.Split(string(data), "\n") {
			if rest, ok := strings.CutPrefix(line, "Model:"); ok {
				name = strings.TrimSpace(rest)
				break
			}
		}
		gpus = append(gpus, GPU{Vendor: "nvidia", Name: name, Source: "driver"})
	}
	return gpus
}

// probeROCmSMI reads AMD memory, preferring sysfs because it needs no
// subprocess and reports bytes directly.
func probeROCmSMI() []GPU {
	if gpus := probeAMDSysfs(); len(gpus) > 0 {
		return gpus
	}
	out, ok := runTool("rocm-smi", "--showmeminfo", "vram", "--csv")
	if !ok {
		return nil
	}
	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, ",")
		if len(fields) < 2 || !strings.HasPrefix(strings.ToLower(fields[0]), "card") {
			continue
		}
		bytesTotal, err := strconv.ParseInt(strings.TrimSpace(fields[len(fields)-1]), 10, 64)
		if err != nil {
			continue
		}
		gpus = append(gpus, GPU{Vendor: "amd", Name: "AMD GPU", VRAMMB: int(bytesTotal / mib), Source: "rocm-smi"})
	}
	return gpus
}

// probeAMDSysfs reads the amdgpu driver's own memory file, which is in bytes.
func probeAMDSysfs() []GPU {
	entries, err := filepath.Glob("/sys/class/drm/card*/device/mem_info_vram_total")
	if err != nil {
		return nil
	}
	var gpus []GPU
	for _, path := range entries {
		if strings.Contains(filepath.Base(filepath.Dir(filepath.Dir(path))), "-") {
			continue // a connector, not a card
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		total, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			continue
		}
		gpus = append(gpus, GPU{Vendor: "amd", Name: "AMD GPU", VRAMMB: int(total / mib), Source: "sysfs"})
	}
	return gpus
}

// probeSysfsGPUs identifies devices by PCI vendor when no vendor tool is
// installed. It reports presence, never capacity.
func probeSysfsGPUs() []GPU {
	entries, err := filepath.Glob("/sys/class/drm/card*/device/vendor")
	if err != nil {
		return nil
	}
	var gpus []GPU
	for _, path := range entries {
		card := filepath.Base(filepath.Dir(filepath.Dir(path)))
		if strings.Contains(card, "-") {
			continue // connector entries such as card0-eDP-1
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		vendor := ""
		integrated := false
		switch strings.TrimSpace(string(data)) {
		case "0x10de":
			vendor = "nvidia"
		case "0x1002":
			vendor = "amd"
		case "0x8086":
			vendor, integrated = "intel", true
		default:
			continue
		}
		name := vendor + " graphics"
		if integrated {
			name = "Intel integrated graphics"
		}
		gpus = append(gpus, GPU{Vendor: vendor, Name: name, Integrated: integrated, Source: "sysfs"})
	}
	return gpus
}

// runTool runs an optional vendor tool, returning false when it is not
// installed or does not answer in time. A missing tool is normal.
func runTool(name string, args ...string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// freeDiskMB reports what an unprivileged user can still write on the
// filesystem holding path. When the directory does not exist yet, the nearest
// existing parent is measured, because that is where it will be created.
//
// The measurement itself is a system call with a different name on every
// family of operating system, so it lives in a file of its own per platform.
// Where there is no such call at all it reports zero, which every caller reads
// as unknown rather than as full.
func freeDiskMB(path string) (int64, error) {
	probe := path
	for {
		avail, err := availableBytes(probe)
		if err == nil {
			return int64(avail / mib), nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return 0, err
		}
		probe = parent
	}
}

// formatMB renders a size the way a person reads it.
func formatMB(mb int) string {
	switch {
	case mb <= 0:
		return "unknown"
	case mb >= 1024:
		return fmt.Sprintf("%.1f GiB", float64(mb)/1024)
	default:
		return fmt.Sprintf("%d MiB", mb)
	}
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}
