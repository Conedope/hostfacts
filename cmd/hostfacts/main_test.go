// End-to-end tests of the hostfacts CLI: built once in TestMain, then executed
// via os/exec so exit codes, stdout and stderr can be asserted.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hostfacts-bin-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "hostfacts")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("go build failed: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var so, se strings.Builder
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	exit = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			t.Fatalf("running %v: %v", args, err)
		}
	}
	return so.String(), se.String(), exit
}

func TestCLIJSON(t *testing.T) {
	out, _, exit := runCLI(t, "--json")
	if exit != 0 {
		t.Fatalf("--json exit = %d, want 0", exit)
	}
	if !strings.HasPrefix(out, "{") {
		t.Fatalf("--json output must start with {, got: %q", out)
	}
	for _, want := range []string{`"hostname"`, `"cpus"`, `"uptime"`, `"boot"`} {
		if !strings.Contains(out, want) {
			t.Errorf("--json output missing %s: %s", want, out)
		}
	}
}

func TestCLIBogusKey(t *testing.T) {
	out, errOut, exit := runCLI(t, "--key", "bogus")
	if exit != 1 {
		t.Fatalf("--key bogus exit = %d, want 1", exit)
	}
	if !strings.Contains(errOut, "valid keys") {
		t.Errorf("stderr should list valid keys, got: %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout should be empty on unknown key, got: %q", out)
	}
}

func TestCLIValidKeys(t *testing.T) {
	for _, key := range []string{"hostname", "os", "kernel", "arch", "cpus", "load", "mem", "uptime", "boot"} {
		out, _, exit := runCLI(t, "--key", key)
		if exit != 0 {
			t.Errorf("--key %s exit = %d, want 0", key, exit)
		}
		if out == "" {
			t.Errorf("--key %s produced empty output", key)
		}
	}
}

func TestCLIVersionAndHelp(t *testing.T) {
	out, _, exit := runCLI(t, "--version")
	if exit != 0 || !strings.HasPrefix(out, "hostfacts ") {
		t.Errorf("--version = %q (exit %d), want 'hostfacts ...'", out, exit)
	}

	_, errOut, exit := runCLI(t, "--help")
	if exit != 2 {
		t.Errorf("--help exit = %d, want 2", exit)
	}
	if !strings.Contains(errOut, "usage: hostfacts") {
		t.Errorf("--help should print usage, got: %q", errOut)
	}
}

func TestCLIQuietAndNoColor(t *testing.T) {
	out, _, exit := runCLI(t, "--no-color")
	if exit != 0 {
		t.Fatalf("--no-color exit = %d", exit)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("--no-color emitted escape sequences: %q", out)
	}
	if !strings.Contains(out, "=") {
		t.Errorf("human output should contain =, got: %q", out)
	}

	quiet, _, _ := runCLI(t, "--quiet")
	if strings.Contains(quiet, "warnings:") {
		t.Errorf("--quiet should suppress warnings, got: %q", quiet)
	}
}