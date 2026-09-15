package deploy

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/innogrid/ai-agent-proto/internal/model"
)

// notAvailable is what a driver prints for a value it cannot report.
//
// It is not zero. Writing it down as zero reads as "idle" or "no memory", and a
// reader cannot tell that apart from a real measurement.
const notAvailable = "N/A"

// bdfPattern matches a PCI address in either the four digit form sysfs writes or
// the eight digit form nvidia-smi and the NPU tools write.
var bdfPattern = regexp.MustCompile(`[0-9a-fA-F]{4,8}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-9a-fA-F]`)

// memoryPattern matches a memory figure with its unit, as the NPU tools print it
// inside a used/total pair.
var memoryPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(MiB|GiB|MB|GB)`)

// parseNVIDIA reads the headerless CSV from nvidia-smi --query-gpu.
//
// The column order is fixed by the query in acceleratorProbeCommand:
// index, uuid, name, memory.total, driver_version, mig.mode.current, pci.bus_id.
func parseNVIDIA(section string) []model.AcceleratorDevice {
	devices := []model.AcceleratorDevice{}

	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := splitCSV(line)
		if len(fields) < 6 {
			continue
		}

		device := model.AcceleratorDevice{
			Source: sectionNVIDIA,
			Kind:   kindGPU,
			Vendor: "nvidia",
			// The UUID keeps its "GPU-" prefix. Stripping it splits one card into
			// two identities wherever the prefixed form is also used as a join key.
			UUID:          valueOrEmpty(fields[1]),
			Name:          valueOrEmpty(fields[2]),
			DriverVersion: valueOrEmpty(fields[4]),
		}
		if index, err := strconv.Atoi(fields[0]); err == nil {
			device.Index = index
		}
		if mib, ok := parseNumber(fields[3]); ok {
			device.MemoryMiB = int(mib)
			device.MemoryKnown = true
		}
		if mig, ok := parseMIGMode(fields[5]); ok {
			device.MIGEnabled = mig
			device.MIGKnown = true
		}
		if len(fields) > 6 {
			device.PCIBusID = normalizeBusID(valueOrEmpty(fields[6]))
		}

		devices = append(devices, device)
	}

	return devices
}

// parseHLSMI reads Intel Gaudi's hl-smi, which answers in the same headerless CSV
// shape as nvidia-smi but has no MIG column.
func parseHLSMI(section string) []model.AcceleratorDevice {
	devices := []model.AcceleratorDevice{}

	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := splitCSV(line)
		if len(fields) < 5 {
			continue
		}

		device := model.AcceleratorDevice{
			Source:        sectionHL,
			Kind:          kindNPU,
			Vendor:        "intel",
			UUID:          valueOrEmpty(fields[1]),
			Name:          valueOrEmpty(fields[2]),
			DriverVersion: valueOrEmpty(fields[4]),
		}
		if index, err := strconv.Atoi(fields[0]); err == nil {
			device.Index = index
		}
		if mib, ok := parseNumber(fields[3]); ok {
			device.MemoryMiB = int(mib)
			device.MemoryKnown = true
		}
		if len(fields) > 5 {
			device.PCIBusID = normalizeBusID(valueOrEmpty(fields[5]))
		}

		devices = append(devices, device)
	}

	return devices
}

// parseROCm reads rocm-smi --csv.
//
// Columns are looked up by header name rather than by position: rocm-smi changes
// both the order and the wording between releases, and a positional read would
// put a driver version in the memory field without erroring.
func parseROCm(section string) []model.AcceleratorDevice {
	devices := []model.AcceleratorDevice{}

	var header []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := splitCSV(line)
		if header == nil {
			if len(fields) > 1 && strings.EqualFold(fields[0], "device") {
				header = fields
			}
			continue
		}
		if len(fields) < 2 || !strings.HasPrefix(strings.ToLower(fields[0]), "card") {
			continue
		}

		column := func(want ...string) string {
			for i, name := range header {
				if i >= len(fields) {
					break
				}
				lowered := strings.ToLower(name)
				for _, w := range want {
					if strings.Contains(lowered, w) {
						return strings.TrimSpace(fields[i])
					}
				}
			}
			return ""
		}

		device := model.AcceleratorDevice{
			Source:        sectionROCm,
			Kind:          kindGPU,
			Vendor:        "amd",
			Index:         len(devices),
			UUID:          valueOrEmpty(column("unique id", "uuid")),
			Name:          valueOrEmpty(column("card series", "card model", "product name")),
			DriverVersion: valueOrEmpty(column("driver version")),
			PCIBusID:      normalizeBusID(valueOrEmpty(column("pci bus", "bus"))),
		}
		// rocm-smi reports VRAM in bytes, unlike every other tool here.
		if bytes, ok := parseNumber(column("vram total memory")); ok && bytes > 0 {
			device.MemoryMiB = int(bytes / (1024 * 1024))
			device.MemoryKnown = true
		}

		devices = append(devices, device)
	}

	return devices
}

// parseNPUTable reads the text tables the NPU tools print.
//
// rbln-stat and furiosa-smi both lay out one device per row with a PCI address in
// it, but neither has a documented stable column order and neither offers a
// machine format this probe can rely on across releases. So the row is scanned
// for the things that are unambiguous wherever they sit - the PCI address, the
// device index, a memory figure with its unit - and anything else is left to the
// PCI inventory, which already named the card.
//
// The result of a parse that finds nothing is not an error: the inventory has the
// device either way, and what is lost is the memory reading, which the report says
// is unknown rather than guessing at it.
func parseNPUTable(section, source, vendor string) []model.AcceleratorDevice {
	devices := []model.AcceleratorDevice{}

	for _, line := range strings.Split(section, "\n") {
		busID := bdfPattern.FindString(line)
		if busID == "" {
			continue
		}

		device := model.AcceleratorDevice{
			Source:   source,
			Kind:     kindNPU,
			Vendor:   vendor,
			Index:    len(devices),
			PCIBusID: normalizeBusID(busID),
		}
		if index, ok := parseDeviceIndex(line); ok {
			device.Index = index
		}
		if mib, ok := parseLargestMemory(line); ok {
			device.MemoryMiB = mib
			device.MemoryKnown = true
		}

		devices = append(devices, device)
	}

	return devices
}

// parseTPUInfo reads the tpu-info listing.
//
// A Cloud TPU is not on a PCI bus the guest can enumerate the way an NPU card is,
// so unlike the NPU tables this is the only source for the device and a row
// without a PCI address still counts.
func parseTPUInfo(section string) []model.AcceleratorDevice {
	devices := []model.AcceleratorDevice{}

	for _, line := range strings.Split(section, "\n") {
		lowered := strings.ToLower(line)
		if !strings.Contains(lowered, "chip") && !strings.Contains(lowered, "device") {
			continue
		}
		index, ok := parseDeviceIndex(line)
		if !ok {
			continue
		}

		device := model.AcceleratorDevice{
			Source: sectionTPU,
			Kind:   kindTPU,
			Vendor: "google",
			Index:  index,
			Name:   parseTPUAccelerator(line),
		}
		if mib, ok := parseLargestMemory(line); ok {
			device.MemoryMiB = mib
			device.MemoryKnown = true
		}

		devices = append(devices, device)
	}

	return devices
}

// tpuGeneration matches the accelerator names Cloud TPU uses, for example v5e.
var tpuGeneration = regexp.MustCompile(`(?i)\bv\d+[a-z]*\b`)

func parseTPUAccelerator(line string) string {
	if match := tpuGeneration.FindString(line); match != "" {
		return "TPU " + strings.ToLower(match)
	}
	return ""
}

// deviceIndexPattern matches the device labels the NPU and TPU tools print.
var deviceIndexPattern = regexp.MustCompile(`(?i)\b(?:npu|device|chip|card)\s*#?(\d+)\b`)

func parseDeviceIndex(line string) (int, bool) {
	match := deviceIndexPattern.FindStringSubmatch(line)
	if match == nil {
		return 0, false
	}
	index, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return index, true
}

// parseLargestMemory reads the biggest memory figure on a row, in MiB.
//
// These tools print memory as a used/total pair, and the total is the larger of
// the two. Taking the larger is what makes the reading comparable to the memory
// floor the catalog states, which is a capacity and not a usage.
func parseLargestMemory(line string) (int, bool) {
	matches := memoryPattern.FindAllStringSubmatch(line, -1)
	if matches == nil {
		return 0, false
	}

	largest := 0
	for _, match := range matches {
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(match[2]) {
		case "gib":
			value *= 1024
		case "gb":
			value = value * 1000 * 1000 * 1000 / (1024 * 1024)
		case "mb":
			value = value * 1000 * 1000 / (1024 * 1024)
		}
		if int(value) > largest {
			largest = int(value)
		}
	}

	if largest == 0 {
		return 0, false
	}
	return largest, true
}

// splitCSV splits a comma separated line and trims each field.
func splitCSV(line string) []string {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// valueOrEmpty drops a value the driver could not report.
func valueOrEmpty(field string) string {
	if strings.EqualFold(field, notAvailable) || strings.EqualFold(field, "N/A ") {
		return ""
	}
	return strings.TrimSpace(field)
}

// parseNumber reads a numeric field, reporting false for one the driver could not
// report. The caller records that as unknown rather than as zero.
func parseNumber(field string) (float64, bool) {
	field = strings.TrimSpace(field)
	if strings.EqualFold(field, notAvailable) || field == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(field, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// parseMIGMode reads mig.mode.current. A card that does not support MIG answers
// N/A, which is not the same as MIG being off.
func parseMIGMode(field string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "enabled":
		return true, true
	case "disabled":
		return false, true
	default:
		return false, false
	}
}
