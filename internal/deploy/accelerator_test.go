package deploy

import (
	"strings"
	"testing"
)

// The first case is the device this prototype actually deployed on
// 2026-09-01 (aws+us-west-2+g5g.xlarge). Its name, memory and driver version are
// what nvidia-smi printed on that node:
//
//	name, memory.total [MiB], driver_version
//	NVIDIA T4G, 15360 MiB, 595.71.05
//
// The index, UUID, mig.mode.current and pci.bus_id columns are not from that
// capture; the probe asks for a different column set than the deployment did.
// The remaining cases are shapes the driver is documented to produce.
func TestParseNVIDIA(t *testing.T) {
	tests := []struct {
		name        string
		stdout      string
		wantDevices int
		check       func(t *testing.T, devices []AcceleratorDeviceAlias)
	}{
		{
			name:        "observed T4G",
			stdout:      "0, GPU-6f3a1c9e-2b44-4d18-9a70-1c2d3e4f5a6b, NVIDIA T4G, 15360, 595.71.05, N/A, 00000000:00:1E.0\n",
			wantDevices: 1,
			check: func(t *testing.T, d []AcceleratorDeviceAlias) {
				if d[0].Name != "NVIDIA T4G" || d[0].MemoryMiB != 15360 || d[0].DriverVersion != "595.71.05" {
					t.Errorf("device = %+v", d[0])
				}
				if d[0].Vendor != "nvidia" || d[0].Kind != kindGPU {
					t.Errorf("vendor/kind = %q/%q, want nvidia/gpu", d[0].Vendor, d[0].Kind)
				}
				if !d[0].MemoryKnown {
					t.Error("memory should be known")
				}
				if d[0].MIGKnown {
					t.Error("mig.mode.current was N/A, so MIG state is not known")
				}
				if d[0].PCIBusID != "0000:00:1e.0" {
					t.Errorf("pciBusId = %q, want the sysfs form so the PCI join works", d[0].PCIBusID)
				}
			},
		},
		{
			name:        "keeps the GPU- prefix on the uuid",
			stdout:      "0, GPU-abc, NVIDIA L4, 23034, 570.1, Disabled, 00000000:00:1E.0\n",
			wantDevices: 1,
			check: func(t *testing.T, d []AcceleratorDeviceAlias) {
				if d[0].UUID != "GPU-abc" {
					t.Errorf("uuid = %q, want the prefix kept", d[0].UUID)
				}
				if !d[0].MIGKnown || d[0].MIGEnabled {
					t.Errorf("mig = (%v,%v), want known and off", d[0].MIGKnown, d[0].MIGEnabled)
				}
			},
		},
		{
			name:        "N/A memory is unknown, not zero",
			stdout:      "0, GPU-abc, NVIDIA A100, N/A, 570.1, Enabled, 00000000:00:1E.0\n",
			wantDevices: 1,
			check: func(t *testing.T, d []AcceleratorDeviceAlias) {
				if d[0].MemoryKnown {
					t.Error("memory was N/A and must not be reported as known")
				}
				if d[0].MemoryMiB != 0 {
					t.Errorf("memoryMiB = %d, want the zero value left untouched", d[0].MemoryMiB)
				}
				if !d[0].MIGEnabled {
					t.Error("mig.mode.current was Enabled")
				}
			},
		},
		{
			name: "several devices",
			stdout: "0, GPU-a, NVIDIA L4, 23034, 570.1, Disabled, 00000000:00:1E.0\n" +
				"1, GPU-b, NVIDIA L4, 23034, 570.1, Disabled, 00000000:00:1F.0\n",
			wantDevices: 2,
		},
		{
			name:        "empty output is no device",
			stdout:      "\n",
			wantDevices: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			devices := parseNVIDIA(tc.stdout)
			if len(devices) != tc.wantDevices {
				t.Fatalf("got %d devices, want %d", len(devices), tc.wantDevices)
			}
			if tc.check != nil {
				aliases := make([]AcceleratorDeviceAlias, len(devices))
				for i := range devices {
					aliases[i] = AcceleratorDeviceAlias(devices[i])
				}
				tc.check(t, aliases)
			}
		})
	}
}

// The sysfs lines are the shape the probe script writes. The first is the device
// present on the machine this was developed on, read straight out of
// /sys/bus/pci/devices: class 0x030000, vendor 0x10de, device 0x2184, driver
// nvidia. The rest use the vendor and device identifiers the pci.ids database
// carries for those accelerators.
func TestParsePCIInventory(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantVendor string
		wantKind   string
		wantName   string
	}{
		{
			name:       "observed NVIDIA display device",
			line:       "0000:01:00.0 0x030000 0x10de 0x2184 nvidia -",
			wantVendor: "nvidia",
			wantKind:   kindGPU,
			wantName:   "0x2184",
		},
		{
			name:       "Rebellions NPU is a processing accelerator",
			line:       "0000:51:00.0 0x120000 0x1eff 0x1220 rbln -",
			wantVendor: "rebellions",
			wantKind:   kindNPU,
			wantName:   "RBLN-CA22",
		},
		{
			name:       "FuriosaAI RNGD",
			line:       "0000:27:00.0 0x120000 0x1ed2 0x0001 furiosa -",
			wantVendor: "furiosa",
			wantKind:   kindNPU,
			wantName:   "RNGD",
		},
		{
			name:       "AWS Trainium",
			line:       "0000:00:1e.0 0x120000 0x1d0f 0x7164 neuron -",
			wantVendor: "aws",
			wantKind:   kindNPU,
			wantName:   "Trainium",
		},
		{
			name:       "an unknown vendor still reports as an accelerator",
			line:       "0000:03:00.0 0x120000 0xdead 0xbeef - -",
			wantVendor: "0xdead",
			wantKind:   kindNPU,
			wantName:   "0xbeef",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			devices := parsePCIInventory(tc.line + "\n")
			if len(devices) != 1 {
				t.Fatalf("got %d devices, want 1", len(devices))
			}
			d := devices[0]
			if d.Vendor != tc.wantVendor || d.Kind != tc.wantKind || d.Name != tc.wantName {
				t.Errorf("vendor/kind/name = %q/%q/%q, want %q/%q/%q",
					d.Vendor, d.Kind, d.Name, tc.wantVendor, tc.wantKind, tc.wantName)
			}
			if d.Source != sourcePCI {
				t.Errorf("source = %q, want %q so the reading is not mistaken for a driver's", d.Source, sourcePCI)
			}
			if d.MemoryKnown {
				t.Error("the PCI bus never reports memory, so it must not be marked known")
			}
		})
	}
}

func TestParsePCIInventorySkipsNonAccelerators(t *testing.T) {
	// A network card and a bridge sit in classes the probe must not report.
	stdout := "0000:00:02.0 0x020000 0x8086 0x1502 e1000e -\n" +
		"0000:00:1c.0 0x060400 0x8086 0x1901 pcieport -\n"
	if devices := parsePCIInventory(stdout); len(devices) != 0 {
		t.Errorf("got %d devices, want 0: only display and accelerator classes count", len(devices))
	}
}

func TestParsePCIInventoryMarksVirtualFunctions(t *testing.T) {
	stdout := "0000:51:00.0 0x120000 0x1eff 0x1220 rbln -\n" +
		"0000:51:00.1 0x120000 0x1eff 0x1221 rbln vf\n"
	devices := parsePCIInventory(stdout)
	if len(devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(devices))
	}
	if devices[0].VirtualFunction {
		t.Error("the physical function must not be marked as virtual")
	}
	if !devices[1].VirtualFunction {
		t.Error("the virtual function must be marked, or it doubles the device count")
	}
	if got := len(physicalDevices(devices)); got != 1 {
		t.Errorf("physical devices = %d, want 1", got)
	}
}

func TestSplitProbeSections(t *testing.T) {
	stdout := sectionMarker + sectionPCI + "===\n" +
		"0000:01:00.0 0x030000 0x10de 0x2184 nvidia -\n" +
		sectionMarker + sectionNVIDIA + "===\n" +
		"0, GPU-a, NVIDIA L4, 23034, 570.1, Disabled, 00000000:01:00.0\n" +
		sectionMarker + sectionROCm + "===\n" +
		sectionMarker + "end===\n"

	sections := splitProbeSections(stdout)
	if !strings.Contains(sections[sectionPCI], "0x10de") {
		t.Errorf("pci section = %q", sections[sectionPCI])
	}
	if !strings.Contains(sections[sectionNVIDIA], "NVIDIA L4") {
		t.Errorf("nvidia section = %q", sections[sectionNVIDIA])
	}
	if strings.TrimSpace(sections[sectionROCm]) != "" {
		t.Errorf("rocm section should be empty, got %q", sections[sectionROCm])
	}
	if _, ok := sections["end"]; ok {
		t.Error("the end marker must not become a section")
	}
}

// A node still answering the previous single-command probe has no markers at all.
// Reading that as nvidia output keeps it working instead of reporting nothing.
func TestSplitProbeSectionsFallsBackToNVIDIA(t *testing.T) {
	sections := splitProbeSections("0, GPU-a, NVIDIA L4, 23034, 570.1, Disabled\n")
	if !strings.Contains(sections[sectionNVIDIA], "NVIDIA L4") {
		t.Errorf("unmarked output should be read as nvidia, got %v", sections)
	}
}

// The driver and the bus write the same address differently. If they are not
// normalised the join fails and one card is reported twice.
func TestMergeJoinsVendorReadingOntoPCIEntry(t *testing.T) {
	stdout := sectionMarker + sectionPCI + "===\n" +
		"0000:01:00.0 0x030000 0x10de 0x2184 nvidia -\n" +
		sectionMarker + sectionNVIDIA + "===\n" +
		"0, GPU-a, NVIDIA GeForce GTX 1660 Ti, 6144, 610.57.04, Disabled, 00000000:01:00.0\n" +
		sectionMarker + "end===\n"

	devices, sources := readNodeAccelerators(stdout)
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1: the bus entry and the driver reading are one card", len(devices))
	}
	d := devices[0]
	if d.Source != sectionNVIDIA {
		t.Errorf("source = %q, want the driver to win where it answered", d.Source)
	}
	if d.Name != "NVIDIA GeForce GTX 1660 Ti" || !d.MemoryKnown || d.MemoryMiB != 6144 {
		t.Errorf("device = %+v, want the driver's name and memory", d)
	}
	if d.KernelDriver != "nvidia" {
		t.Errorf("kernelDriver = %q, want the bus fact kept", d.KernelDriver)
	}
	if len(sources) != 2 {
		t.Errorf("sources = %v, want both pci and nvidia-smi", sources)
	}
}

// This is the case the PCI layer exists for: the card is plugged in and no
// driver ever bound to it, so every vendor tool reports nothing at all.
func TestUnboundDeviceIsReportedNotSilentlyMissing(t *testing.T) {
	stdout := sectionMarker + sectionPCI + "===\n" +
		"0000:51:00.0 0x120000 0x1eff 0x1220 - -\n" +
		sectionMarker + sectionNVIDIA + "===\n" +
		sectionMarker + "end===\n"

	devices, _ := readNodeAccelerators(stdout)
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(devices))
	}
	if devices[0].Name != "RBLN-CA22" || devices[0].Kind != kindNPU {
		t.Errorf("device = %+v, want the NPU named off the bus", devices[0])
	}
	if devices[0].KernelDriver != "" {
		t.Errorf("kernelDriver = %q, want empty: nothing bound", devices[0].KernelDriver)
	}

	report := stubReport(devices)
	findings := compareToExpectation(report, 1)
	if !containsSubstring(findings, "no kernel driver bound") {
		t.Errorf("findings = %v, want the unbound driver called out", findings)
	}
}

func TestParseNPUTableReadsBusAndMemory(t *testing.T) {
	// The column wording is not relied on; the address and the memory pair are.
	section := "NPU  Name        Device   PCI BUS ID     Temp   Power   Memory        Util\n" +
		"0    RBLN-CA22   rbln0    0000:51:00.0   41C    38W     0.00/15.62 GiB  0%\n"

	devices := parseNPUTable(section, sectionRBLN, "rebellions")
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(devices))
	}
	d := devices[0]
	if d.PCIBusID != "0000:51:00.0" {
		t.Errorf("pciBusId = %q", d.PCIBusID)
	}
	if d.Kind != kindNPU || d.Vendor != "rebellions" {
		t.Errorf("kind/vendor = %q/%q", d.Kind, d.Vendor)
	}
	if !d.MemoryKnown || d.MemoryMiB != 15994 {
		t.Errorf("memory = (%v,%d), want the total of the used/total pair in MiB", d.MemoryKnown, d.MemoryMiB)
	}
}

func TestParseROCmReadsColumnsByName(t *testing.T) {
	section := "device,Card series,Card vendor,VRAM Total Memory (B),Driver version,PCI Bus\n" +
		"card0,Vega 20,Advanced Micro Devices,17163091968,6.7.0,0000:00:1E.0\n"

	devices := parseROCm(section)
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(devices))
	}
	d := devices[0]
	if d.Name != "Vega 20" || d.DriverVersion != "6.7.0" {
		t.Errorf("device = %+v", d)
	}
	if !d.MemoryKnown || d.MemoryMiB != 16368 {
		t.Errorf("memory = (%v,%d), want bytes converted to MiB", d.MemoryKnown, d.MemoryMiB)
	}
	if d.PCIBusID != "0000:00:1e.0" {
		t.Errorf("pciBusId = %q, want normalised", d.PCIBusID)
	}
}

func TestAcceleratorModeMIGWins(t *testing.T) {
	devices := parseNVIDIA(
		"0, GPU-a, NVIDIA A100, 81920, 570.1, Disabled, 00000000:01:00.0\n" +
			"1, GPU-b, NVIDIA A100, 81920, 570.1, Enabled, 00000000:02:00.0\n")
	if got := acceleratorMode(devices); got != modeMIG {
		t.Errorf("mode = %q, want %q: MIG on any device changes what is observable", got, modeMIG)
	}
}

func TestAcceleratorModeUnknownWithoutDevices(t *testing.T) {
	if got := acceleratorMode(nil); got != modeUnknown {
		t.Errorf("mode = %q, want %q", got, modeUnknown)
	}
}

// The catalog states a class, so an NPU answering where a GPU was asked for is a
// mismatch even when the count and the memory fit.
func TestCompareReportsKindMismatch(t *testing.T) {
	devices := parsePCIInventory("0000:51:00.0 0x120000 0x1eff 0x1220 rbln -\n")
	report := stubReport(devices)
	report.Expected.Type = "gpu"
	report.Expected.MinCount = 1

	if !containsSubstring(compareToExpectation(report, 1), "is a npu, the application asks for gpu") {
		t.Errorf("findings = %v, want the class mismatch reported", compareToExpectation(report, 1))
	}
}

func containsSubstring(findings []string, want string) bool {
	for _, finding := range findings {
		if strings.Contains(finding, want) {
			return true
		}
	}
	return false
}
