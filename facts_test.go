package hostfacts

import (
	"strings"
	"testing"
)

const (
	loadavgFixture = "0.12 0.07 0.02 2/165 765\n"
	uptimeFixture  = "124.08 932.80\n"
)

// meminfoFixture is a realistic /proc/meminfo sample. Hand-computed from the
// source: MemTotal 11380220 and MemAvailable 3113012, both in kB.
const meminfoFixture = `MemTotal:       11380220 kB
MemFree:         4800000 kB
MemAvailable:    3113012 kB
Buffers:         1000000 kB
Cached:          5555555 kB
SwapCached:            0 kB
Active:          2000000 kB
Inactive:        3000000 kB
Active(anon):    1000000 kB
Inactive(anon):  2000000 kB
Active(file):    1000000 kB
Inactive(file):  1000000 kB
Unevictable:        1220 kB
Mlocked:            1220 kB
SwapTotal:             0 kB
SwapFree:              0 kB
Dirty:               100 kB
Writeback:             0 kB
AnonPages:       3000000 kB
Mapped:           500000 kB
Shmem:            100000 kB
Slab:              60000 kB
SReclaimable:      40000 kB
SUnreclaim:        20000 kB
KernelStack:        7000 kB
PageTables:         8000 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:     5690110 kB
Committed_AS:     400000 kB
VmallocTotal:   34359738367 kB
VmallocUsed:        5000 kB
VmallocChunk:          0 kB
Percpu:             1000 kB
HardwareCorrupted:     0 kB
AnonHugePages:         0 kB
ShmemHugePages:        0 kB
ShmemPmdMapped:        0 kB
FileHugePages:         0 kB
FilePmdMapped:         0 kB
CmaTotal:              0 kB
CmaFree:               0 kB
HugePages_Total:      0
HugePages_Free:       0
HugePages_Rsvd:       0
HugePages_Surp:       0
Hugepagesize:       2048 kB
Hugetlb:               0 kB
DirectMap4k:      200000 kB
DirectMap2M:    11000000 kB
DirectMap1G:           0 kB
`

func TestParseLoadavg(t *testing.T) {
	got, err := ParseLoadavg(loadavgFixture)
	if err != nil {
		t.Fatalf("ParseLoadavg(%q) error: %v", loadavgFixture, err)
	}
	want := [3]float64{0.12, 0.07, 0.02}
	if got != want {
		t.Fatalf("ParseLoadavg(%q) = %v, want %v", loadavgFixture, got, want)
	}

	// Errors: too few fields, then a non-numeric member.
	for _, bad := range []string{"0.12 0.07\n", "0.12 nope 0.02\n", "\n"} {
		if _, err := ParseLoadavg(bad); err == nil {
			t.Errorf("ParseLoadavg(%q) expected error, got nil", bad)
		}
	}
}

func TestParseMeminfo(t *testing.T) {
	total, avail, err := ParseMeminfo(meminfoFixture)
	if err != nil {
		t.Fatalf("ParseMeminfo error: %v", err)
	}
	if total != 11380220 {
		t.Errorf("total = %d, want 11380220", total)
	}
	if avail != 3113012 {
		t.Errorf("avail = %d, want 3113012", avail)
	}

	// Missing MemAvailable (older kernels) must error.
	missingAvail := strings.Replace(meminfoFixture, "MemAvailable:    3113012 kB\n", "", 1)
	if _, _, err := ParseMeminfo(missingAvail); err == nil {
		t.Error("expected error when MemAvailable is missing, got nil")
	}

	// Garbage value must error.
	garbage := strings.Replace(meminfoFixture, "MemTotal:       11380220 kB", "MemTotal:       not-a-number kB", 1)
	if _, _, err := ParseMeminfo(garbage); err == nil {
		t.Error("expected error on garbage MemTotal, got nil")
	}
}

func TestParseUptime(t *testing.T) {
	got, err := ParseUptime(uptimeFixture)
	if err != nil {
		t.Fatalf("ParseUptime(%q) error: %v", uptimeFixture, err)
	}
	if got != 124 {
		t.Fatalf("ParseUptime(%q) = %d, want 124", uptimeFixture, got)
	}

	for _, bad := range []string{"", "abc 932.80\n", "-5.0 932.80\n"} {
		if _, err := ParseUptime(bad); err == nil {
			t.Errorf("ParseUptime(%q) expected error, got nil", bad)
		}
	}
}

// fullFacts is a fully-populated, fully-"known" snapshot used by the render
// tests. Boot time 1760000000 == 2025-10-09 08:53 UTC (verified by hand with
// `date -u -d @1760000000`).
func fullFacts() Facts {
	return Facts{
		Hostname:     "localhost",
		OS:           "Linux",
		Kernel:       "6.17.0-PRoot-Distro",
		Arch:         "aarch64",
		NumCPU:       4,
		Load:         [3]float64{0.12, 0.07, 0.02},
		MemTotalKb:   11380220,
		MemAvailKb:   3113012,
		UptimeSec:    124,
		BootTimeUnix: 1760000000,
		loadKnown:    true,
		memKnown:     true,
		uptimeKnown:  true,
	}
}

func TestRenderHuman(t *testing.T) {
	got := Render(fullFacts(), false)
	want := `hostname = localhost
os       = Linux
kernel   = 6.17.0-PRoot-Distro
arch     = aarch64
cpus     = 4
load     = 0.12 0.07 0.02
mem      = 11380220 kB total / 8267208 kB used
uptime   = 124 s
boot     = 2025-10-09 08:53 UTC`
	if got != want {
		t.Fatalf("Render human mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderJSON(t *testing.T) {
	got := Render(fullFacts(), true)
	want := `{"hostname":"localhost","os":"Linux","kernel":"6.17.0-PRoot-Distro","arch":"aarch64","cpus":4,"load":[0.12,0.07,0.02],"mem_total_kb":11380220,"mem_available_kb":3113012,"mem_used_kb":8267208,"uptime":124,"boot":1760000000,"warnings":[]}`
	if got != want {
		t.Fatalf("Render json mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderHumanDegraded(t *testing.T) {
	f := Facts{
		NumCPU:   2,
		Warnings: []string{"/proc/loadavg: no such file or directory", "skipped /proc files: not a linux host (windows)"},
	}
	got := Render(f, false)
	want := `hostname = n/a
os       = n/a
kernel   = n/a
arch     = n/a
cpus     = 2
load     = n/a
mem      = n/a
uptime   = n/a
boot     = n/a
warnings:
  - /proc/loadavg: no such file or directory
  - skipped /proc files: not a linux host (windows)`
	if got != want {
		t.Fatalf("Render degraded mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderJSONWarningsNotNull(t *testing.T) {
	f := fullFacts()
	f.Warnings = nil
	got := Render(f, true)
	if !strings.Contains(got, `"warnings":[]`) {
		t.Fatalf("expected empty warnings array, got: %s", got)
	}
}

func TestFactValue(t *testing.T) {
	f := fullFacts()
	cases := map[string]string{
		"hostname": "localhost",
		"os":       "Linux",
		"kernel":   "6.17.0-PRoot-Distro",
		"arch":     "aarch64",
		"cpus":     "4",
		"load":     "0.12 0.07 0.02",
		"mem":      "11380220 kB total / 8267208 kB used",
		"uptime":   "124 s",
		"boot":     "2025-10-09 08:53 UTC",
	}
	for key, want := range cases {
		got, err := FactValue(f, key)
		if err != nil {
			t.Errorf("FactValue(%q) error: %v", key, err)
			continue
		}
		if got != want {
			t.Errorf("FactValue(%q) = %q, want %q", key, got, want)
		}
	}

	if _, err := FactValue(f, "bogus"); err == nil {
		t.Error("FactValue(bogus) expected error, got nil")
	} else if !strings.Contains(err.Error(), "valid") {
		t.Errorf("unknown-key error should mention valid keys, got: %v", err)
	}

	// Missing values degrade to n/a rather than erroring.
	for _, key := range []string{"load", "mem", "uptime", "boot", "hostname", "os"} {
		if got, err := FactValue(Facts{}, key); err != nil || got != "n/a" {
			t.Errorf("FactValue(Facts{}, %q) = %q, %v; want %q", key, got, err, "n/a")
		}
	}
}

func TestMemUsedKb(t *testing.T) {
	if got := fullFacts().MemUsedKb(); got != 8267208 {
		t.Errorf("MemUsedKb() = %d, want 8267208", got)
	}
	// Available above total never goes negative.
	if got := (Facts{MemTotalKb: 100, MemAvailKb: 500}).MemUsedKb(); got != 0 {
		t.Errorf("MemUsedKb() oversubscribed = %d, want 0", got)
	}
}

// TestCollect runs against the real machine: it must not error, must return a
// non-empty hostname and a sane positive CPU count inside this container.
func TestCollect(t *testing.T) {
	f, err := Collect()
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if f.Hostname == "" {
		t.Error("Collect() returned empty hostname")
	}
	if f.NumCPU <= 0 {
		t.Errorf("Collect() NumCPU = %d, want > 0", f.NumCPU)
	}
	if f.Arch == "" {
		t.Error("Collect() returned empty arch")
	}
}