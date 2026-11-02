package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstalledInformationCommands(t *testing.T) {
	canonical, err := os.ReadFile("LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	if license != string(canonical) {
		t.Fatal("embedded license differs from LICENSE")
	}
	for _, stamp := range []string{"none", "release", "development"} {
		base := t.TempDir()
		binary := filepath.Join(base, "dots")
		args := []string{"build", "-o", binary}
		revision := "0123456789abcdef0123456789abcdef76543210"
		if stamp != "none" {
			v := "dev"
			if stamp == "release" {
				v = "v0.1.0"
			}
			args = append(args, "-ldflags", "-X github.com/fernandoaleman/dots/internal/cli.version="+v+" -X github.com/fernandoaleman/dots/internal/cli.commit="+revision)
		}
		args = append(args, ".")
		if out, err := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), args...).CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
		cwd := filepath.Join(base, "fixture")
		if err := os.Mkdir(cwd, 0o700); err != nil {
			t.Fatal(err)
		}
		run := func(args ...string) string {
			t.Helper()
			cmd := exec.Command(binary, args...)
			cmd.Dir = cwd
			cmd.Env = []string{"HOME=" + cwd + "/missing-home", "XDG_CONFIG_HOME=" + cwd + "/missing-config", "XDG_STATE_HOME=" + cwd + "/missing-state"}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal(args, err, string(out))
			}
			return string(out)
		}
		want := "dots dev (commit unknown)\n"
		if stamp == "release" {
			want = "dots v0.1.0\n"
		}
		if stamp == "development" {
			want = "dots dev (commit " + revision + ")\n"
		}
		if run("version") != want || run("--version") != want {
			t.Fatal("incorrect executable version")
		}
		if run("license") != string(canonical) {
			t.Fatal("offline license differs")
		}
		for _, command := range []string{"help", "add", "up", "down", "list", "status", "recover", "version", "license"} {
			if !strings.Contains(run(command, "-h"), "Usage:") {
				t.Fatal("missing help", command)
			}
		}
		if stamp != "none" {
			out, err := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "version", "-m", binary).CombinedOutput()
			if err != nil || !strings.Contains(string(out), revision) {
				t.Fatal("source commit missing from executable build metadata", err, string(out))
			}
		}
		entries, err := os.ReadDir(cwd)
		if err != nil || len(entries) != 0 {
			t.Fatal("information commands initialized state", err)
		}
	}
}
