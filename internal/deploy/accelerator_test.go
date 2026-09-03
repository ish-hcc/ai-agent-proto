package deploy

import "testing"

// The first case is the device this prototype actually deployed on
// 2026-09-01 (aws+us-west-2+g5g.xlarge). Its name, memory and driver version are
// what nvidia-smi printed on that node:
//
//	name, memory.total [MiB], driver_version
//	NVIDIA T4G, 15360 MiB, 595.71.05
//
// The index, UUID and mig.mode.current columns are not from that capture; the
// probe asks for a different column set than the deployment did. The remaining
// cases are shapes the driver is documented to produce.
func TestParseAcceleratorCSV(t *testing.T) {
	tests := []struct {
		name        string
		stdout      string
		wantDevices int
		wantMissing bool
		check       func(t *testing.T, devices []AcceleratorDeviceAlias)
	}{
		{
			name:        "observed T4G",
			stdout:      "0, GPU-6f3a1c9e-2b44-4d18-9a70-1c2d3e4f5a6b, NVIDIA T4G, 15360, 595.71.05, N/A\n",
			wantDevices: 1,
			check: func(t *testing.T, d []AcceleratorDeviceAlias) {
				if d[0].Name != "NVIDIA T4G" || d[0].MemoryMiB != 15360 || d[0].DriverVersion != "595.71.05" {
					t.Errorf("device = %+v", d[0])
				}
				if d[0].Vendor != "nvidia" {
					t.Errorf("vendor = %q, want nvidia", d[0].Vendor)
				}
				if !d[0].MemoryKnown {
					t.Error("memory should be known")
				}
				if d[0].MIGKnown {
					t.Error("mig.mode.current was N/A, so MIG state is not known")
				}
			},
		},
		{
			name:        "keeps the GPU- prefix on the uuid",
			stdout:      "0, GPU-abc, NVIDIA L4, 23034, 570.1, Disabled\n",
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
			stdout:      "0, GPU-abc, NVIDIA A100, N/A, 570.1, Enabled\n",
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
			name:        "several devices",
			stdout:      "0, GPU-a, NVIDIA L4, 23034, 570.1, Disabled\n1, GPU-b, NVIDIA L4, 23034, 570.1, Disabled\n",
			wantDevices: 2,
		},
		{
			name:        "no driver tool on the node",
			stdout:      "NO_NVIDIA_SMI\n",
			wantMissing: true,
		},
		{
			name:        "empty output is no device, not a missing driver",
			stdout:      "\n",
			wantDevices: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			devices, missing := parseAcceleratorCSV(tc.stdout)
			if missing != tc.wantMissing {
				t.Fatalf("driverMissing = %v, want %v", missing, tc.wantMissing)
			}
			if missing {
				return
			}
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

func TestAcceleratorModeMIGWins(t *testing.T) {
	devices, _ := parseAcceleratorCSV(
		"0, GPU-a, NVIDIA A100, 81920, 570.1, Disabled\n1, GPU-b, NVIDIA A100, 81920, 570.1, Enabled\n")
	if got := acceleratorMode(devices); got != modeMIG {
		t.Errorf("mode = %q, want %q: MIG on any device changes what is observable", got, modeMIG)
	}
}

func TestAcceleratorModeUnknownWithoutDevices(t *testing.T) {
	if got := acceleratorMode(nil); got != modeUnknown {
		t.Errorf("mode = %q, want %q", got, modeUnknown)
	}
}
