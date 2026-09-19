// Command hostfacts prints a concise summary of machine facts: hostname, OS,
// kernel release, architecture, CPU count, load average, memory, uptime and
// boot time. It uses only the Go standard library and degrades gracefully when
// /proc is unavailable.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Conedope/hostfacts"
)

// version is baked into the binary at build time via -ldflags "-X main.version=...";
// the default is the canonical release.
var version = "1.0.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hostfacts", flag.ContinueOnError)
	fs.SetOutput(stderr)

	jsonOut := fs.Bool("json", false, "print facts as JSON")
	key := fs.String("key", "", "print a single fact; valid keys: "+strings.Join(hostfacts.ValidKeys(), ", "))
	noColor := fs.Bool("no-color", false, "disable color output")
	quiet := fs.Bool("quiet", false, "suppress warnings")
	ver := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: hostfacts [--json] [--key KEY] [--no-color] [--quiet] [--version]\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "hostfacts: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}

	if *ver {
		fmt.Fprintf(stdout, "hostfacts %s\n", version)
		return 0
	}

	f, err := hostfacts.Collect()
	if err != nil {
		fmt.Fprintf(stderr, "hostfacts: %v\n", err)
		return 1
	}
	if *quiet {
		f.Warnings = nil
	}

	if *key != "" {
		v, err := hostfacts.FactValue(f, *key)
		if err != nil {
			fmt.Fprintf(stderr, "hostfacts: %v\n", err)
			fmt.Fprintf(stderr, "valid keys: %s\n", strings.Join(hostfacts.ValidKeys(), ", "))
			return 1
		}
		fmt.Fprintln(stdout, v)
		return 0
	}

	out := hostfacts.Render(f, *jsonOut)
	if !*jsonOut && !*noColor && isTerminal(stdout) {
		out = colorize(out)
	}
	fmt.Fprintln(stdout, out)
	return 0
}

// isTerminal reports whether w is attached to a character device (a TTY).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// colorize wraps each "key = value" header in bold cyan. It is only ever called
// for human output on a TTY, so no escapes reach pipes or JSON.
func colorize(out string) string {
	var b strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, " = "); i >= 0 {
			b.WriteString("\x1b[1;36m")
			b.WriteString(line[:i])
			b.WriteString("\x1b[0m = ")
			b.WriteString(line[i+3:])
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}