package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanImportPath(t *testing.T) {
	cfg := importConfig(t, "source")
	home, cwd := filepath.Dir(cfg.TargetDir), cfg.TargetDir
	for _, input := range []string{".zshrc", "./.zshrc", "././.zshrc", cfg.TargetDir + "/.zshrc", "~/target/.zshrc"} {
		p, err := PlanImportPath(cfg, "", input, home, cwd)
		if err != nil || p.Link.Destination != cfg.TargetDir+"/.zshrc" || p.Link.Source != cfg.DotfilesDirs[0]+"/zshrc" {
			t.Fatal(input, p, err)
		}
	}
	for _, root := range []string{cfg.DotfilesDirs[0], cfg.DotfilesDirs[0] + "/", "source", "./source", "~/source"} {
		p, err := PlanImportPath(cfg, root, "target/.zshrc", home, home)
		if err != nil || p.Root != cfg.DotfilesDirs[0] {
			t.Fatal(root, p, err)
		}
	}
	for _, root := range []string{".", "./"} {
		if _, err := PlanImportPath(cfg, root, cfg.TargetDir+"/.zshrc", home, cfg.DotfilesDirs[0]); err != nil {
			t.Fatal(root, err)
		}
	}
	for _, input := range []string{"", "../target/.zshrc", ".config/../.zshrc", ".config/./file", ".zshrc/", ".", home + "/other/.zshrc"} {
		if _, err := PlanImportPath(cfg, "", input, home, cwd); err == nil {
			t.Fatal("accepted ambiguous/outside locator", input)
		}
	}
	if _, err := PlanImportPath(cfg, "", ".zshrc", "relative", cwd); err == nil {
		t.Fatal("relative home accepted")
	}
}

func TestPlanImportPathConfiguredTraversal(t *testing.T) {
	cfg := importConfig(t, "source")
	base := filepath.Dir(cfg.TargetDir)
	if err := os.MkdirAll(base+"/actual/deep", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(base+"/actual/deep", base+"/route"); err != nil {
		t.Fatal(err)
	}
	cfg.TargetDir = base + "/route/.."
	p, err := PlanImportPath(cfg, "", cfg.TargetDir+"/.zshrc", base, base)
	if err != nil || p.Link.Destination != cfg.TargetDir+"/.zshrc" || !strings.Contains(p.Link.Destination, "/route/../") {
		t.Fatal("configured traversal was cleaned", p, err)
	}
	if _, err := PlanImportPath(cfg, "", base+"/actual/.zshrc", base, base); err == nil {
		t.Fatal("alternate namespace silently substituted")
	}
}

func TestPlanImportPathDoesNotReinterpretRelativeLocators(t *testing.T) {
	cfg := importConfig(t, "source")
	home := filepath.Dir(cfg.TargetDir)
	cfg.TargetDir = home
	cwd := home + "/work"
	for _, input := range []string{"./~/.zshrc", "././~/.zshrc", ".//" + strings.TrimPrefix(home, "/") + "/.zshrc"} {
		if p, err := PlanImportPath(cfg, "", input, home, cwd); err == nil {
			t.Fatal("relative input reinterpreted", input, p)
		}
	}
	for _, root := range []string{"./~/source", "././~/source", ".//" + strings.TrimPrefix(cfg.DotfilesDirs[0], "/")} {
		if p, err := PlanImportPath(cfg, root, home+"/.zshrc", home, cwd); err == nil {
			t.Fatal("relative root reinterpreted", root, p)
		}
	}
	if _, err := PlanImportPath(cfg, "~/source", "~/.zshrc", home, cwd); err != nil {
		t.Fatal("explicit home locator refused", err)
	}
	literal := cwd + "/~/source"
	if err := os.MkdirAll(literal, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.DotfilesDirs = []string{literal}
	if p, err := PlanImportPath(cfg, "./~/source", "~/.zshrc", home, cwd); err != nil || p.Root != literal {
		t.Fatal("literal relative tilde root refused", p, err)
	}
}
