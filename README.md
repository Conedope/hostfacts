# hostfacts

A zero-dependency Go CLI that prints a concise summary of machine facts:
hostname, OS, kernel release, architecture, CPU count, load average, memory,
uptime and boot time.

Pure Go **standard library** — no third-party deps. On Linux, memory / load /
uptime are read directly from `/proc`. When the `/proc` files are absent (non-Linux
hosts, restricted containers, chroots) `hostfacts` degrades gracefully: it fills
whatever it can and reports the gaps as `n/a` plus a `warnings` block instead of
crashing.

## Install

```sh
go install github.com/Conedope/hostfacts/cmd/hostfacts@latest
# or build locally
go build -o hostfacts ./cmd/hostfacts
```

The module target is `go 1.22`; it compiles with any newer toolchain.

## Usage

```
hostfacts [--json] [--key KEY] [--no-color] [--quiet] [--version] [--help]
```

### Verified example output

Built and run on 2026-09-19 (Linux, arm64, inside a container on a 4-CPU host):

```
$ ./hostfacts
hostname = localhost
os       = Linux
kernel   = 6.17.0-PRoot-Distro
arch     = aarch64
cpus     = 4
load     = 0.12 0.07 0.02
mem      = 11380220 kB total / 8169068 kB used
uptime   = 124 s
boot     = 2026-09-19 05:07 UTC
```

```
$ ./hostfacts --json
{"hostname":"localhost","os":"Linux","kernel":"6.17.0-PRoot-Distro","arch":"aarch64","cpus":4,"load":[0.12,0.07,0.02],"mem_total_kb":11380220,"mem_available_kb":3210400,"mem_used_kb":8169820,"uptime":124,"boot":1789794450,"warnings":[]}
```

```
$ ./hostfacts --key hostname
localhost

$ ./hostfacts --version
hostfacts 1.0.0

$ ./hostfacts --key bogus
hostfacts: unknown fact key "bogus" (valid: hostname, os, kernel, arch, cpus, load, mem, uptime, boot)
valid keys: hostname, os, kernel, arch, cpus, load, mem, uptime, boot
```

Load and used-memory values drift between runs — they are live readings.

### Flags

| Flag | Description |
|------|-------------|
| `--json` | Print facts as a single-line JSON object. |
| `--key KEY` | Print one fact only. Keys: `hostname`, `os`, `kernel`, `arch`, `cpus`, `load`, `mem`, `uptime`, `boot`. Unknown key → error on stderr, exit 1. |
| `--no-color` | Disable ANSI colors (colors appear only in human output on a TTY; pipes and `--json` never get escapes). |
| `--quiet` | Suppress the warnings section. |
| `--version` | Print `hostfacts <version>` and exit 0. |
| `--help` | Print usage and exit 2. |

Exit codes: `0` success, `1` collection failure or unknown `--key`, `2` flag/usage error.

## Library

The root package is independently testable — collection and formatting are
separated:

- `hostfacts.Collect() (Facts, error)` — snapshot the machine (`uname -srm` via
  `os/exec`, `runtime.NumCPU`, and direct `/proc` reads on Linux). Never
  hard-fails: gaps land in `Facts.Warnings`.
- `hostfacts.ParseLoadavg(s string) ([3]float64, error)`
- `hostfacts.ParseMeminfo(s string) (totalKb, availKb uint64, err error)`
- `hostfacts.ParseUptime(s string) (uint64, error)`
- `hostfacts.FactValue(f Facts, key string) (string, error)`
- `hostfacts.Render(f Facts, jsonOut bool) string`

```go
f, err := hostfacts.Collect()
fmt.Print(hostfacts.Render(f, false))
```

### `Facts`

```go
type Facts struct {
    Hostname     string    // machine hostname
    OS           string    // uname system name (e.g. Linux)
    Kernel       string    // uname release (e.g. 6.17.0-PRoot-Distro)
    Arch         string    // uname machine (e.g. aarch64)
    NumCPU       int       // runtime.NumCPU()
    Load         [3]float64 // 1, 5, 15 min load averages
    MemTotalKb   uint64    // MemTotal from /proc/meminfo
    MemAvailKb   uint64    // MemAvailable from /proc/meminfo
    UptimeSec    uint64    // first field of /proc/uptime
    BootTimeUnix int64     // current unix time minus uptime
    Warnings     []string  // non-fatal collection notes
}
```

`Facts.MemUsedKb()` returns `MemTotalKb - MemAvailKb` (clamped at 0).

## /proc dependency and fallback

On Linux, `Collect` reads three pseudo-files:

| File | Used for |
|------|----------|
| `/proc/loadavg` | 1/5/15 minute load averages |
| `/proc/meminfo` | `MemTotal`, `MemAvailable` (kB) |
| `/proc/uptime` | uptime in seconds (boot time = now − uptime) |

Each read is independent: if a file is missing or unparseable, that single fact
renders as `n/a` and a matching warning is appended — the command never fails
out. On non-Linux hosts the `/proc` reads are skipped entirely and one warning
explains why. Hostname, OS/kernel/arch and CPU count work everywhere (hostname
via `os.Hostname`, OS/kernel/arch via `uname -srm`, CPUs via `runtime.NumCPU`).

## Development

```sh
go vet ./...
go test ./... -v
```

Tests cover parser fixtures (hand-computed `/proc/loadavg`, `/proc/meminfo` and
`/proc/uptime` samples), exact-structure render assertions for both the human
and JSON forms, degraded/`n/a` rendering with warnings, a live `Collect` run,
and end-to-end CLI runs (via a built binary) asserting JSON prefix, valid-key
lookups and the `--key bogus` exit code 1.

## License

MIT © 2026 Conedope. See [LICENSE](LICENSE).