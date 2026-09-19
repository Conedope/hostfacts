// Package hostfacts collects and renders a concise summary of machine facts:
// hostname, OS, kernel release, architecture, CPU count, load average, memory,
// uptime and boot time.
//
// It is pure Go standard library. On Linux, memory / load / uptime are read
// directly from /proc. When the /proc files are absent (non-Linux hosts or
// restricted containers) Collect falls back gracefully: it fills whatever it
// can and records a human-readable note in Facts.Warnings instead of failing.
package hostfacts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Facts is a single snapshot of machine facts.
//
// Load holds the 1, 5 and 15 minute load averages. MemTotalKb / MemAvailKb are
// bytes expressed in kB as reported by /proc/meminfo. BootTimeUnix is a Unix
// timestamp derived from the current time minus uptime. Warnings collects
// non-fatal problems encountered while collecting (they render to "n/a").
type Facts struct {
	Hostname     string
	OS           string
	Kernel       string
	Arch         string
	NumCPU       int
	Load         [3]float64
	MemTotalKb   uint64
	MemAvailKb   uint64
	UptimeSec    uint64
	BootTimeUnix int64
	Warnings     []string

	loadKnown   bool
	memKnown    bool
	uptimeKnown bool
	sysKnown    bool
}

// Collect snapshot the machine. It never hard-fails on missing /proc files or
// a broken uname: whatever cannot be read is left unset and reported through
// Warnings.
func Collect() (Facts, error) {
	f := Facts{}

	hostname, err := os.Hostname()
	if err != nil {
		f.Warnings = append(f.Warnings, "hostname: "+err.Error())
		hostname = "unknown"
	}
	f.Hostname = hostname

	sys, kernel, arch, err := uname()
	if err != nil {
		f.Warnings = append(f.Warnings, "uname: "+err.Error())
		sys, kernel, arch = runtime.GOOS, "", runtime.GOARCH
	} else {
		f.sysKnown = true
	}
	f.OS, f.Kernel, f.Arch = sys, kernel, arch

	f.NumCPU = runtime.NumCPU()

	if runtime.GOOS == "linux" {
		f.collectLinux()
	} else {
		f.Warnings = append(f.Warnings,
			"skipped /proc files: not a linux host ("+runtime.GOOS+")")
	}

	if f.uptimeKnown {
		f.BootTimeUnix = time.Now().Unix() - int64(f.UptimeSec)
	}

	return f, nil
}

// collectLinux reads the /proc pseudo-files that only exist on Linux. Each read
// is independent: a missing or unparseable file only adds a warning.
func (f *Facts) collectLinux() {
	if s, err := os.ReadFile("/proc/loadavg"); err == nil {
		if l, err := ParseLoadavg(string(s)); err == nil {
			f.Load = l
			f.loadKnown = true
		} else {
			f.Warnings = append(f.Warnings, "/proc/loadavg: "+err.Error())
		}
	} else {
		f.Warnings = append(f.Warnings, "/proc/loadavg: "+err.Error())
	}

	if s, err := os.ReadFile("/proc/meminfo"); err == nil {
		if total, avail, err := ParseMeminfo(string(s)); err == nil {
			f.MemTotalKb, f.MemAvailKb = total, avail
			f.memKnown = true
		} else {
			f.Warnings = append(f.Warnings, "/proc/meminfo: "+err.Error())
		}
	} else {
		f.Warnings = append(f.Warnings, "/proc/meminfo: "+err.Error())
	}

	if s, err := os.ReadFile("/proc/uptime"); err == nil {
		if up, err := ParseUptime(string(s)); err == nil {
			f.UptimeSec = up
			f.uptimeKnown = true
		} else {
			f.Warnings = append(f.Warnings, "/proc/uptime: "+err.Error())
		}
	} else {
		f.Warnings = append(f.Warnings, "/proc/uptime: "+err.Error())
	}
}

// uname runs `uname -srm` and returns system name, kernel release and machine
// architecture (OS, Kernel, Arch).
func uname() (sys, kernel, arch string, err error) {
	out, err := exec.Command("uname", "-srm").Output()
	if err != nil {
		return "", "", "", err
	}
	fields := strings.Fields(string(out))
	switch {
	case len(fields) == 0:
		return "", "", "", fmt.Errorf("uname -srm produced no output")
	case len(fields) < 3:
		return "", "", "", fmt.Errorf("uname -srm produced %d fields", len(fields))
	}
	return fields[0], fields[1], strings.Join(fields[2:], " "), nil
}

// ParseLoadavg parses a /proc/loadavg file (e.g. "0.12 0.07 0.02 2/165 765")
// and returns the 1, 5 and 15 minute load averages.
func ParseLoadavg(s string) ([3]float64, error) {
	var l [3]float64
	fields := strings.Fields(s)
	if len(fields) < 3 {
		return l, fmt.Errorf("loadavg: expected at least 3 fields, got %q", strings.TrimSpace(s))
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return l, fmt.Errorf("loadavg: field %d (%q): %w", i+1, fields[i], err)
		}
		l[i] = v
	}
	return l, nil
}

// ParseMeminfo parses a /proc/meminfo file and returns the MemTotal and
// MemAvailable values in kB. An error is returned if either line is missing or
// cannot be parsed.
func ParseMeminfo(s string) (totalKb, availKb uint64, err error) {
	var total, avail uint64
	var haveTotal, haveAvail bool
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasSuffix(fields[0], ":") {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			if total, err = strconv.ParseUint(fields[1], 10, 64); err != nil {
				return 0, 0, fmt.Errorf("meminfo: MemTotal %q: %w", fields[1], err)
			}
			haveTotal = true
		case "MemAvailable:":
			if avail, err = strconv.ParseUint(fields[1], 10, 64); err != nil {
				return 0, 0, fmt.Errorf("meminfo: MemAvailable %q: %w", fields[1], err)
			}
			haveAvail = true
		}
	}
	if !haveTotal {
		return 0, 0, fmt.Errorf("meminfo: missing MemTotal line")
	}
	if !haveAvail {
		return 0, 0, fmt.Errorf("meminfo: missing MemAvailable line")
	}
	return total, avail, nil
}

// ParseUptime parses a /proc/uptime file (e.g. "124.08 932.80") and returns the
// uptime in whole seconds.
func ParseUptime(s string) (uint64, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, fmt.Errorf("uptime: empty input")
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("uptime: %q: %w", fields[0], err)
	}
	if v < 0 {
		return 0, fmt.Errorf("uptime: negative value %v", v)
	}
	return uint64(v), nil
}

// MemUsedKb returns used memory (total minus available), never negative.
func (f Facts) MemUsedKb() uint64 {
	if f.MemAvailKb >= f.MemTotalKb {
		return 0
	}
	return f.MemTotalKb - f.MemAvailKb
}

const (
	keyHostname = "hostname"
	keyOS       = "os"
	keyKernel   = "kernel"
	keyArch     = "arch"
	keyCPUs     = "cpus"
	keyLoad     = "load"
	keyMem      = "mem"
	keyUptime   = "uptime"
	keyBoot     = "boot"
)

// ValidKeys lists the fact keys the CLI accepts for --key, in display order.
func ValidKeys() []string {
	return []string{keyHostname, keyOS, keyKernel, keyArch, keyCPUs, keyLoad, keyMem, keyUptime, keyBoot}
}

// FactValue returns the printable value for a single fact key. It errors on an
// unknown key so the CLI can fail loudly while listing the valid keys.
func FactValue(f Facts, key string) (string, error) {
	switch key {
	case keyHostname:
		return orNA(f.Hostname), nil
	case keyOS:
		return orNA(f.OS), nil
	case keyKernel:
		return orNA(f.Kernel), nil
	case keyArch:
		return orNA(f.Arch), nil
	case keyCPUs:
		return strconv.Itoa(f.NumCPU), nil
	case keyLoad:
		if !f.loadKnown {
			return "n/a", nil
		}
		return loadString(f.Load), nil
	case keyMem:
		if !f.memKnown {
			return "n/a", nil
		}
		return fmt.Sprintf("%d kB total / %d kB used", f.MemTotalKb, f.MemUsedKb()), nil
	case keyUptime:
		if !f.uptimeKnown {
			return "n/a", nil
		}
		return fmt.Sprintf("%d s", f.UptimeSec), nil
	case keyBoot:
		if f.BootTimeUnix <= 0 {
			return "n/a", nil
		}
		return bootString(f.BootTimeUnix), nil
	default:
		return "", fmt.Errorf("unknown fact key %q (valid: %s)", key, strings.Join(ValidKeys(), ", "))
	}
}

// Render formats Facts for display. With jsonOut=true it emits a single-line
// JSON object; otherwise it prints an aligned "key = value" block. Human records
// print as "n/a"; known problems are listed under a warnings header.
func Render(f Facts, jsonOut bool) string {
	if jsonOut {
		return renderJSON(f)
	}
	return renderHuman(f)
}

// renderJSON builds an ordered JSON object covering every fact.
func renderJSON(f Facts) string {
	warnings := f.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	j := struct {
		Hostname   string    `json:"hostname"`
		OS         string    `json:"os"`
		Kernel     string    `json:"kernel"`
		Arch       string    `json:"arch"`
		NumCPU     int       `json:"cpus"`
		Load       [3]float64 `json:"load"`
		MemTotalKb uint64    `json:"mem_total_kb"`
		MemAvailKb uint64    `json:"mem_available_kb"`
		MemUsedKb  uint64    `json:"mem_used_kb"`
		UptimeSec  uint64    `json:"uptime"`
		BootTime   int64     `json:"boot"`
		Warnings   []string  `json:"warnings"`
	}{
		Hostname:   f.Hostname,
		OS:         f.OS,
		Kernel:     f.Kernel,
		Arch:       f.Arch,
		NumCPU:     f.NumCPU,
		Load:       f.Load,
		MemTotalKb: f.MemTotalKb,
		MemAvailKb: f.MemAvailKb,
		MemUsedKb:  f.MemUsedKb(),
		UptimeSec:  f.UptimeSec,
		BootTime:   f.BootTimeUnix,
		Warnings:   warnings,
	}
	out, err := json.Marshal(j)
	if err != nil {
		return `{"error":"` + err.Error() + `"}`
	}
	return string(out)
}

// renderHuman prints the aligned block. Missing values render as "n/a";
// warnings are appended in a compact section.
func renderHuman(f Facts) string {
	rows := []struct{ key, val string }{
		{keyHostname, orNA(f.Hostname)},
		{keyOS, orNA(f.OS)},
		{keyKernel, orNA(f.Kernel)},
		{keyArch, orNA(f.Arch)},
		{keyCPUs, strconv.Itoa(f.NumCPU)},
	}

	if f.loadKnown {
		rows = append(rows, struct{ key, val string }{keyLoad, loadString(f.Load)})
	} else {
		rows = append(rows, struct{ key, val string }{keyLoad, "n/a"})
	}

	if f.memKnown {
		rows = append(rows, struct{ key, val string }{keyMem, fmt.Sprintf("%d kB total / %d kB used", f.MemTotalKb, f.MemUsedKb())})
	} else {
		rows = append(rows, struct{ key, val string }{keyMem, "n/a"})
	}

	uptime := "n/a"
	if f.uptimeKnown {
		uptime = fmt.Sprintf("%d s", f.UptimeSec)
	}
	rows = append(rows, struct{ key, val string }{keyUptime, uptime})

	boot := "n/a"
	if f.BootTimeUnix > 0 {
		boot = bootString(f.BootTimeUnix)
	}
	rows = append(rows, struct{ key, val string }{keyBoot, boot})

	width := 0
	for _, r := range rows {
		if n := len(r.key); n > width {
			width = n
		}
	}

	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%-*s = %s\n", width, r.key, r.val)
	}

	if len(f.Warnings) > 0 {
		b.WriteString("warnings:\n")
		for _, w := range sortStrings(f.Warnings) {
			fmt.Fprintf(&b, "  - %s\n", w)
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func orNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}

func loadString(l [3]float64) string {
	return fmt.Sprintf("%.2f %.2f %.2f", l[0], l[1], l[2])
}

func bootString(unix int64) string {
	return time.Unix(unix, 0).UTC().Format("2006-01-02 15:04 MST")
}

func sortStrings(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}