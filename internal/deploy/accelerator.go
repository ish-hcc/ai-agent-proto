package deploy

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/innogrid/ai-agent-proto/internal/model"
	"github.com/innogrid/ai-agent-proto/internal/tumblebug"
)

// probeTimeoutMinutes bounds the remote query. It only reads driver state, so it
// either answers in seconds or the node is not reachable.
const probeTimeoutMinutes = 3

// acceleratorProbeCommand asks the driver what it actually has.
//
// The field list follows the one CB-MCMP's monitoring agent uses on hypervisors,
// including mig.mode.current: whether MIG is on decides what is observable at all,
// and reading it is the only way to tell an idle card from one whose counters the
// driver has switched off.
//
// It queries rather than collects. Continuous GPU telemetry is a separate job
// with its own agent and schema; copying that pipeline into a deployment service
// would duplicate a collector without anywhere to put the samples.
const acceleratorProbeCommand = "nvidia-smi --query-gpu=index,uuid,name,memory.total,driver_version,mig.mode.current " +
	"--format=csv,noheader,nounits 2>/dev/null || echo NO_NVIDIA_SMI"

// notAvailable is what the driver prints for a value it cannot report.
//
// It is not zero. Writing it down as zero reads as "idle" or "no memory", and a
// reader cannot tell that apart from a real measurement.
const notAvailable = "N/A"

// Accelerator modes, in the order they win when more than one signal fits.
const (
	modeMIG       = "mig"
	modeBareMetal = "bare_metal"
	modeUnknown   = "unknown"
)

// ProbeAccelerator reads what the deployed nodes actually carry and compares it
// against what the spec catalog promised.
//
// The catalog is a promise made at planning time from a spec record; this is the
// device answering. They disagree often enough to be worth checking: an
// accelerator image whose driver never bound, a spec whose accelerator fields are
// a vGPU profile rather than a card, a node that came up without the device at all.
func (s *Service) ProbeAccelerator(ctx context.Context, nsID, infraID string, app *model.AppSpec) (*model.AcceleratorReport, error) {
	report := &model.AcceleratorReport{
		InfraID:  infraID,
		Mode:     modeUnknown,
		Devices:  []model.AcceleratorDevice{},
		Findings: []string{},
	}
	if app != nil {
		report.Expected = model.AcceleratorExpectation{
			Type:         app.Accelerator.Type,
			Model:        app.Accelerator.Model,
			MinCount:     app.Accelerator.MinCount,
			MinMemoryGiB: app.Accelerator.MinMemoryGiB,
		}
	}

	req := &tumblebug.InfraCmdReq{
		Command:        []string{acceleratorProbeCommand},
		UserName:       probeUserName(app),
		TimeoutMinutes: probeTimeoutMinutes,
	}
	results, err := s.tumblebug.RunCommand(ctx, nsID, infraID, req)
	if err != nil {
		return nil, err
	}

	for _, node := range results.Results {
		devices, driverMissing := parseAcceleratorCSV(node.Stdout["0"])
		if driverMissing {
			report.Findings = append(report.Findings, fmt.Sprintf(
				"%s: no NVIDIA driver tool on the node, so the accelerator could not be read here", node.NodeID))
			continue
		}
		for i := range devices {
			devices[i].NodeID = node.NodeID
		}
		report.Devices = append(report.Devices, devices...)
	}

	report.Probed = true
	report.Mode = acceleratorMode(report.Devices)
	report.Findings = append(report.Findings, compareToExpectation(report, len(results.Results))...)
	report.Message = probeMessage(report)

	log.Info().Str("nsId", nsID).Str("infraId", infraID).Str("mode", report.Mode).
		Int("devices", len(report.Devices)).Int("findings", len(report.Findings)).
		Msg("Probed accelerator")

	return report, nil
}

// parseAcceleratorCSV reads the driver's headerless CSV.
//
// It reports driverMissing separately from an empty device list: a node with no
// driver tool and a node whose driver reports no device are different states, and
// collapsing them hides which one happened.
func parseAcceleratorCSV(stdout string) (devices []model.AcceleratorDevice, driverMissing bool) {
	devices = []model.AcceleratorDevice{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "NO_NVIDIA_SMI") {
			return nil, true
		}
		fields := splitCSV(line)
		if len(fields) < 6 {
			continue
		}
		device := model.AcceleratorDevice{
			// The UUID keeps its "GPU-" prefix. Stripping it splits one card into
			// two identities wherever the prefixed form is also used as a join key.
			UUID:          valueOrEmpty(fields[1]),
			Name:          valueOrEmpty(fields[2]),
			DriverVersion: valueOrEmpty(fields[4]),
		}
		device.Vendor = string(vendorOf(device.Name))
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
		devices = append(devices, device)
	}
	return devices, false
}

// acceleratorMode reports how the accelerator is presented to this node.
//
// MIG wins when any device reports it, because MIG changes what can be measured
// and that has to surface even on a mixed node. Passthrough and bare metal look
// the same from inside a guest, so they are not split here; the hypervisor is the
// only place that can tell them apart.
func acceleratorMode(devices []model.AcceleratorDevice) string {
	if len(devices) == 0 {
		return modeUnknown
	}
	for _, d := range devices {
		if d.MIGKnown && d.MIGEnabled {
			return modeMIG
		}
	}
	return modeBareMetal
}

// compareToExpectation reports where the device disagrees with the catalog.
func compareToExpectation(report *model.AcceleratorReport, nodeCount int) []string {
	findings := make([]string, 0, 4)
	expected := report.Expected

	if expected.Type == "" {
		return findings
	}
	if len(report.Devices) == 0 {
		findings = append(findings, fmt.Sprintf(
			"the application asks for %s but no accelerator answered on any node", expected.Type))
		return findings
	}

	if expected.MinCount > 0 && nodeCount > 0 {
		perNode := len(report.Devices) / nodeCount
		if perNode < expected.MinCount {
			findings = append(findings, fmt.Sprintf(
				"the application asks for %d accelerators per node, the nodes report %d",
				expected.MinCount, perNode))
		}
	}

	for _, d := range report.Devices {
		if expected.MinMemoryGiB > 0 && d.MemoryKnown {
			if gib := float64(d.MemoryMiB) / 1024; gib+0.5 < expected.MinMemoryGiB {
				findings = append(findings, fmt.Sprintf(
					"%s reports %.1f GiB, the application asks for at least %g GiB",
					d.Label(), gib, expected.MinMemoryGiB))
			}
		}
		if expected.Model != "" && d.Name != "" &&
			!strings.Contains(strings.ToLower(d.Name), strings.ToLower(expected.Model)) {
			findings = append(findings, fmt.Sprintf(
				"%s is a %q, the application names %q", d.Label(), d.Name, expected.Model))
		}
	}

	if report.Mode == modeMIG {
		findings = append(findings, "MIG is on, so the driver reports no device level utilization here; "+
			"per instance utilization comes from the hypervisor, not from this node")
	}
	return findings
}

func probeMessage(report *model.AcceleratorReport) string {
	switch {
	case len(report.Devices) == 0:
		return "No accelerator answered on the deployed nodes"
	case len(report.Findings) == 0:
		return fmt.Sprintf("%d accelerator(s) match what the application asked for", len(report.Devices))
	default:
		return fmt.Sprintf("%d accelerator(s) found, %d finding(s) to look at", len(report.Devices), len(report.Findings))
	}
}

func probeUserName(app *model.AppSpec) string {
	if app != nil && app.Install.UserName != "" {
		return app.Install.UserName
	}
	return ""
}

// splitCSV splits the driver's comma separated line and trims each field.
func splitCSV(line string) []string {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// valueOrEmpty drops a value the driver could not report.
func valueOrEmpty(field string) string {
	if strings.EqualFold(field, notAvailable) {
		return ""
	}
	return field
}

// parseNumber reads a numeric field, reporting false for one the driver could not
// report. The caller records that as unknown rather than as zero.
func parseNumber(field string) (float64, bool) {
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
