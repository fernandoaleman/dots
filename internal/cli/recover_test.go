package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRecoverUsage(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, args := range [][]string{
		{"--retire"}, {"--retire", ""}, {"--retire", id}, {"--settle"}, {"--revision", "0"}, {"--dry-run"},
		{"--retire", id, "--settle", "--revision", "0"}, {"--retire", "bad", "--revision", "1"},
		{"--retire", strings.Repeat("A", 32), "--revision", "1"}, {"--settle=false"}, {"--dry-run=false"},
		{"--settle", "--revision", "1", "extra"}, {"--settle", "--revision", "1", "--revision", "2"},
		{"--config", "missing"}, {"-n"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, err bytes.Buffer
			if code := Run(append([]string{"recover"}, args...), &out, &err); code != 2 || err.Len() == 0 {
				t.Fatal(code, out.String(), err.String())
			}
		})
	}
	for _, rev := range []string{"", "-1", "+1", "1.0", " 1", "18446744073709551616"} {
		var out, err bytes.Buffer
		if code := Run([]string{"recover", "--settle", "--revision", rev}, &out, &err); code != 2 {
			t.Fatal(rev, code, err.String())
		}
	}
	for _, args := range [][]string{{"recover", "--help"}, {"--help"}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 0 || !strings.Contains(err.String(), "recover") {
			t.Fatal(code, err.String())
		}
	}
	var out, err bytes.Buffer
	if code := Run([]string{"--config", "missing", "recover"}, &out, &err); code != 2 || !strings.Contains(err.String(), "does not use --config") {
		t.Fatal(code, err.String())
	}
}

func TestRecoverAbsentState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "missing-state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "missing-config"))
	before := snapshot(t, root)
	for _, args := range [][]string{{}, {"--settle", "--revision", "0", "--dry-run"}, {"--settle", "--revision", "0"}} {
		var out, err bytes.Buffer
		if code := Run(append([]string{"recover"}, args...), &out, &err); code != 0 || err.Len() != 0 || !strings.Contains(out.String(), "registry") && !strings.Contains(out.String(), "REGISTRY") {
			t.Fatal(code, out.String(), err.String())
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("absent-state recovery wrote files")
		}
	}
	var out, err bytes.Buffer
	if code := Run([]string{"recover", "--settle", "--revision", "1", "--dry-run"}, &out, &err); code != 1 || !strings.Contains(out.String(), "BLOCKED") {
		t.Fatal(code, out.String(), err.String())
	}
	// Invalid XDG values use the normal HOME fallback without initializing it.
	for _, xdg := range []string{"", "relative"} {
		t.Setenv("XDG_STATE_HOME", xdg)
		out.Reset()
		err.Reset()
		if code := Run([]string{"recover"}, &out, &err); code != 0 || !strings.Contains(out.String(), filepath.Join(root, ".local", "state", "dots", "ownership.json")) {
			t.Fatal(code, out.String(), err.String())
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("inspection initialized fallback state")
	}
}

type recoveryShortWriter struct{}

func (recoveryShortWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }

func TestRecoverOutputFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	var err bytes.Buffer
	if code := Run([]string{"recover"}, recoveryShortWriter{}, &err); code != 1 || !strings.Contains(err.String(), io.ErrShortWrite.Error()) {
		t.Fatal(code, err.String())
	}
}
