package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestImportDarwinBackingPaths(t *testing.T) {
	request := importInspectionFixture(t, true)
	p, err := namespace()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	paths := &importPathInspection{plan: p, query: importDirectoryPath, paths: make(map[*directory]string)}
	dir, missing, _, err := p.walk(filepath.Dir(request.Input))
	if err != nil || len(missing) != 0 {
		t.Fatalf("fixture parent: %v", err)
	}
	backing, err := importDirectoryPath(dir.fd)
	if err != nil {
		t.Fatal(err)
	}
	var stat unix.Stat_t
	if err := unix.Stat(backing, &stat); err != nil || identity(stat) != dir.id {
		t.Fatalf("backing path identity: %v", err)
	}
	// All created paths stay inside the fixture, including use of its alternative
	// backing spelling. System ancestors are only inspected, never modified.
	a, err := paths.project(request.Input, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := paths.project(backing+"/init.lua", false)
	if err != nil || a != b {
		t.Fatalf("aliases differ: %q %q %v", a, b, err)
	}
	if _, err := InspectImport(request); err != nil {
		t.Fatalf("ordinary nested inspection: %v", err)
	}
	// Protect ancestry above the firmlink jump when the input has a logical name.
	if strings.HasPrefix(backing, "/System/Volumes/Data/") {
		if err := paths.protectedDirectory(request.Input, "/System/Volumes/Data"); err == nil {
			t.Fatal("backing ancestor protection missed")
		}
	} else {
		t.Log("fixture is not on the Data volume; descriptor path checks still exercised")
	}
	if err := paths.protectedDirectory(request.Input, request.Root); err != nil {
		t.Fatalf("unrelated root refused: %v", err)
	}
	if err := paths.revalidate(); err != nil {
		t.Fatal(err)
	}
}

func TestImportDarwinLateBackingRename(t *testing.T) {
	request := importInspectionFixture(t, false)
	parent := filepath.Dir(request.Input)
	ops := nativeImportInspectionOps()
	ops.checkpoint = func() {
		if err := os.Rename(parent, parent+".preserved"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := inspectImport(request, ops); err == nil {
		t.Fatal("renamed pinned directory accepted")
	}
	if _, err := os.Stat(parent + ".preserved/" + filepath.Base(request.Input)); err != nil {
		t.Fatal("input fixture lost")
	}
}

func TestImportDarwinPathOutput(t *testing.T) {
	for _, value := range []string{"", "\x00", "relative\x00", "/unterminated"} {
		if _, err := importDarwinPath([]byte(value)); err == nil {
			t.Fatalf("invalid path buffer accepted: %q", value)
		}
	}
	if path, err := importDarwinPath([]byte("/valid\x00padding")); err != nil || path != "/valid" {
		t.Fatalf("complete path: %q %v", path, err)
	}
	if _, err := importDirectoryPath(-1); err == nil {
		t.Fatal("invalid descriptor accepted")
	}
}
