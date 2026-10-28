package filesystem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func liveImport(t *testing.T, q ImportRequest) *preparedImport {
	t.Helper()
	p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, nativeImportPrepareOps())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func assertRelocated(t *testing.T, q ImportRequest, original importFile, content []byte, linked bool) {
	t.Helper()
	var s unix.Stat_t
	if err := unix.Lstat(q.Source, &s); err != nil {
		t.Fatal(err)
	}
	if !sameImportFile(original, importFile{fingerprintOf(s), s.Uid, s.Gid, uint64(s.Nlink)}) {
		t.Fatal("original file identity/metadata lost")
	}
	got, err := os.ReadFile(q.Source)
	if err != nil || !bytes.Equal(content, got) {
		t.Fatal("source content lost", err)
	}
	if linked {
		assertLink(t, q.Input, q.Source)
	} else if _, err := os.Lstat(q.Input); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unexpected original-slot occupant", err)
	}
}

func assertImportBlocked(t *testing.T, q ImportRequest) {
	t.Helper()
	before := treeState(t, filepath.Dir(q.Root))
	for _, dry := range []bool{false, true} {
		if _, err := Down(q.Ownership, dry); err == nil {
			t.Fatal("pending import allowed down")
		}
	}
	if _, err := InstallOwned(nil, q.Ownership); err == nil {
		t.Fatal("pending import allowed empty up")
	}
	if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
		t.Fatal("refusal modified paths")
	}
}

func TestPrivateImportCompletion(t *testing.T) {
	for _, nested := range []bool{false, true} {
		for _, version := range []int{1, 2} {
			t.Run(fmt.Sprintf("nested=%t/v%d", nested, version), func(t *testing.T) {
				q := preparationFixture(t, nested, version)
				managed := Destination{Path: q.Ownership.Target + "/.managed", Source: q.Root + "/managed"}
				fixtureFile(t, managed.Source)
				installFixture(t, q.Ownership, []Destination{managed})
				q.Entries = append(q.Entries, managed)
				history := ownershipSnapshot(t, q.Ownership).Records
				content, _ := os.ReadFile(q.Input)
				p := liveImport(t, q)
				original := p.intent.OriginalFile
				var stages []string
				p.ops.checkpoint = func(stage string) { stages = append(stages, stage) }
				result, err := p.complete(nativeImportCompleteOps())
				if err != nil || result != (importCompletion{true, true, true}) {
					t.Fatal(result, err)
				}
				assertRelocated(t, q, original, content, true)
				s := ownershipSnapshot(t, q.Ownership)
				if s.Version != 2 || s.Unfinished || len(s.Imports) != 0 || len(s.Records) != len(history)+1 {
					t.Fatal("completion state", s)
				}
				for _, r := range history {
					found := false
					for _, got := range s.Records {
						found = found || reflect.DeepEqual(r, got)
					}
					if !found {
						t.Fatal("unrelated history changed")
					}
				}
				for _, stage := range []string{"relocation", "file-sync", "input-sync", "source-sync", "relocated", "creating", "link", "owned", "completion"} {
					if !strings.Contains(strings.Join(stages, ","), stage) {
						t.Fatal("missing transition", stage)
					}
				}
				if _, err := p.complete(nativeImportCompleteOps()); err == nil {
					t.Fatal("completion reused")
				}
				if p.steps < 4 {
					t.Fatal("revision reservation consumed recovery budget")
				}
				// M3 removal recognizes only the link captured by this live transaction.
				if _, err := Down(q.Ownership, false); err != nil {
					t.Fatal(err)
				}
				assertRelocated(t, q, original, content, false)
				if info, err := os.Stat(parentPath(q.Input)); err != nil || !info.IsDir() {
					t.Fatal("input parents removed", err)
				}
				if _, err := Down(q.Ownership, false); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestImportCompletionRefusals(t *testing.T) {
	for _, mode := range []string{"occupied file", "occupied symlink", "occupied directory", "input replacement", "input contents", "source parent", "input parent", "lock", "registry", "rename unsupported", "cross device", "permission", "symlink failure", "unexpected input after rename"} {
		t.Run(mode, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			content, _ := os.ReadFile(q.Input)
			p := liveImport(t, q)
			original := p.intent.OriginalFile
			ops := nativeImportCompleteOps()
			called := 0
			native := ops.rename
			ops.rename = func(a int, b string, c int, d string) error { called++; return native(a, b, c, d) }
			switch mode {
			case "occupied file":
				fixtureFile(t, q.Source)
			case "occupied symlink":
				if err := os.Symlink("missing", q.Source); err != nil {
					t.Fatal(err)
				}
			case "occupied directory":
				if err := os.Mkdir(q.Source, 0o700); err != nil {
					t.Fatal(err)
				}
			case "input replacement":
				if err := os.Rename(q.Input, q.Input+".saved"); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, q.Input)
			case "input contents":
				if err := os.WriteFile(q.Input, []byte("edited"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "source parent":
				if err := os.Rename(parentPath(q.Source), parentPath(q.Source)+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parentPath(q.Source), 0o700); err != nil {
					t.Fatal(err)
				}
			case "input parent":
				if err := os.Rename(parentPath(q.Input), parentPath(q.Input)+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parentPath(q.Input), 0o700); err != nil {
					t.Fatal(err)
				}
			case "lock":
				if err := os.Rename(importLockPath(q.Ownership.Registry), importLockPath(q.Ownership.Registry)+".saved"); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, importLockPath(q.Ownership.Registry))
			case "registry":
				p.ops.checkpoint = func(stage string) {
					if stage == "before-relocation" {
						writeImportSnapshot(t, q.Ownership, ownershipSnapshot(t, q.Ownership))
					}
				}
			case "rename unsupported", "cross device", "permission":
				injected := map[string]error{"rename unsupported": unix.ENOTSUP, "cross device": unix.EXDEV, "permission": unix.EACCES}[mode]
				ops.rename = func(int, string, int, string) error { called++; return injected }
			case "symlink failure":
				ops.symlink = func(string, int, string) error { return unix.EACCES }
			case "unexpected input after rename":
				p.ops.checkpoint = func(stage string) {
					if stage == "relocation" {
						fixtureFile(t, q.Input)
					}
				}
			}
			before := treeState(t, filepath.Dir(q.Root))
			result, err := p.complete(ops)
			if err == nil {
				t.Fatal("unsafe completion succeeded")
			}
			if _, err := p.complete(ops); err == nil {
				t.Fatal("failed capability reused")
			}
			if called > 1 {
				t.Fatal("rename retried")
			}
			if !result.Relocated {
				if mode != "registry" && !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
					t.Fatal("pre-rename refusal wrote state")
				}
			} else {
				got, e := os.ReadFile(q.Source)
				if e != nil || !bytes.Equal(got, content) {
					t.Fatal("relocated content lost", e)
				}
				var stat unix.Stat_t
				if e := unix.Lstat(q.Source, &stat); e != nil || idOf(stat) != original.Fingerprint.ID {
					t.Fatal("source identity lost", e)
				}
				if !strings.Contains(err.Error(), "original pathname may be absent") {
					t.Fatal("missing post-relocation diagnostic", err)
				}
			}
			if mode != "lock" && mode != "registry" {
				assertImportBlocked(t, q)
			}
		})
	}
}

func TestImportCompletionPublicationFaults(t *testing.T) {
	for _, phase := range []string{"relocated", "creating", "owned", "completion"} {
		for _, fault := range []string{"write", "short write", "file sync", "close", "rename", "directory sync"} {
			t.Run(phase+"/"+fault, func(t *testing.T) {
				q := preparationFixture(t, true, 1)
				data, _ := os.ReadFile(q.Input)
				p := liveImport(t, q)
				original := p.intent.OriginalFile
				native := p.ops.registry
				active, fired := false, false
				p.ops.checkpoint = func(stage string) {
					if stage == "before-"+phase {
						active = true
					}
				}
				fail := errors.New("injected persistence failure")
				p.ops.registry.write = func(f *os.File, b []byte) (int, error) {
					if active && fault == "write" {
						fired = true
						return 0, fail
					}
					if active && fault == "short write" {
						fired = true
						return 0, nil
					}
					return native.write(f, b)
				}
				p.ops.registry.sync = func(fd int) error {
					if active && ((fault == "directory sync" && fd == p.view.registry.dir.fd) || (fault == "file sync" && fd != p.view.registry.dir.fd)) {
						fired = true
						return fail
					}
					return native.sync(fd)
				}
				p.ops.registry.closeFile = func(f *os.File) error {
					err := native.closeFile(f)
					if active && fault == "close" {
						fired = true
						return errors.Join(err, fail)
					}
					return err
				}
				p.ops.registry.rename = func(a int, b string, c int, d string) error {
					if active && fault == "rename" {
						fired = true
						return fail
					}
					return native.rename(a, b, c, d)
				}
				p.view.registry.ops = p.ops.registry
				result, err := p.complete(nativeImportCompleteOps())
				expected := importCompletion{Relocated: true, Linked: phase == "owned" || phase == "completion", Owned: phase == "completion"}
				if err == nil || !fired || result != expected {
					t.Fatal("fault not exercised or inaccurate report", result, err)
				}
				linked := phase == "owned" || phase == "completion"
				assertRelocated(t, q, original, data, linked)
				s := ownershipSnapshot(t, q.Ownership)
				if phase == "completion" && fault == "directory sync" {
					if s.Unfinished || len(s.Imports) != 0 {
						t.Fatal("published completion not visible")
					}
				} else {
					if !s.Unfinished || len(s.Imports) != 1 {
						t.Fatal("pending evidence lost")
					}
					assertImportBlocked(t, q)
				}
				if _, err := p.complete(nativeImportCompleteOps()); err == nil {
					t.Fatal("publication failure resumed")
				}
			})
		}
	}
}

func TestImportCompletionSyncFailures(t *testing.T) {
	for _, point := range []string{"file-sync", "input-sync", "source-sync", "link-sync"} {
		t.Run(point, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			data, _ := os.ReadFile(q.Input)
			p := liveImport(t, q)
			original := p.intent.OriginalFile
			active, fired := false, false
			p.ops.checkpoint = func(stage string) {
				if stage == "before-"+point || (point == "link-sync" && stage == "link") {
					active = true
				}
			}
			native := p.ops.registry.sync
			p.ops.registry.sync = func(fd int) error {
				if active {
					fired = true
					return unix.EIO
				}
				return native(fd)
			}
			p.view.registry.ops = p.ops.registry
			result, err := p.complete(nativeImportCompleteOps())
			if err == nil || !fired || result != (importCompletion{Relocated: true, Linked: point == "link-sync"}) {
				t.Fatal("sync failure missed or inaccurate report", result, err)
			}
			assertRelocated(t, q, original, data, point == "link-sync")
			s := ownershipSnapshot(t, q.Ownership)
			if len(s.Imports) != 1 || !s.Unfinished {
				t.Fatal("evidence lost")
			}
			if point == "link-sync" {
				if len(s.Records) != 1 || s.Records[0].Phase != "creating" {
					t.Fatal("uncertain ownership granted")
				}
			} else if s.Imports[0].Phase != "prepared" {
				t.Fatal("relocation published before sync")
			}
			assertImportBlocked(t, q)
		})
	}
}

func TestImportCompletionCrash(t *testing.T) {
	if point := os.Getenv("DOTS_COMPLETE_CRASH"); point != "" {
		var q ImportRequest
		if err := json.Unmarshal([]byte(os.Getenv("DOTS_COMPLETE_REQUEST")), &q); err != nil {
			t.Fatal(err)
		}
		p := liveImport(t, q)
		p.ops.checkpoint = func(stage string) {
			if stage == point {
				os.Exit(79)
			}
		}
		_, err := p.complete(nativeImportCompleteOps())
		t.Fatal("crash point not reached", err)
	}
	points := []string{"before-relocation", "relocation", "before-file-sync", "file-sync", "before-input-sync", "input-sync", "before-source-sync", "source-sync", "before-relocated", "relocated", "before-creating", "creating", "before-link", "link", "before-owned", "owned", "before-completion", "completion"}
	for n, point := range points {
		t.Run(point, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			data, _ := os.ReadFile(q.Input)
			var stat unix.Stat_t
			if err := unix.Lstat(q.Input, &stat); err != nil {
				t.Fatal(err)
			}
			original := importFile{fingerprintOf(stat), stat.Uid, stat.Gid, uint64(stat.Nlink)}
			encoded, _ := json.Marshal(q)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportCompletionCrash$")
			cmd.Env = append(os.Environ(), "DOTS_COMPLETE_CRASH="+point, "DOTS_COMPLETE_REQUEST="+string(encoded))
			out, err := cmd.CombinedOutput()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 79 {
				t.Fatalf("child: %v %s", err, out)
			}
			if n == 0 {
				assertPreparationInput(t, q, stat, data)
			} else {
				assertRelocated(t, q, original, data, n >= 13)
			}
			s := ownershipSnapshot(t, q.Ownership)
			if point == "completion" {
				if s.Unfinished || len(s.Imports) != 0 {
					t.Fatal("completion lost")
				}
				return
			}
			if !s.Unfinished || len(s.Imports) != 1 {
				t.Fatal("pending evidence lost")
			}
			expected := "prepared"
			if n >= 9 {
				expected = "relocated"
			}
			if s.Imports[0].Phase != expected {
				t.Fatal("phase", s.Imports[0].Phase)
			}
			if n >= 11 {
				want := "creating"
				if n >= 15 {
					want = "owned"
				}
				if len(s.Records) != 1 || s.Records[0].Phase != want {
					t.Fatal("ownership stage", s.Records)
				}
			} else if len(s.Records) != 0 {
				t.Fatal("premature claim")
			}
			assertImportBlocked(t, q)
		})
	}
}

func TestImportCompletionLateEvidence(t *testing.T) {
	for _, mode := range []string{"ctime", "uncertain rename", "late input", "late source", "late parent", "late link", "late source contents"} {
		t.Run(mode, func(t *testing.T) {
			q := preparationFixture(t, false, 1)
			data, _ := os.ReadFile(q.Input)
			p := liveImport(t, q)
			ops := nativeImportCompleteOps()
			fired := false
			if mode == "uncertain rename" {
				ops.rename = func(a int, b string, c int, d string) error {
					if err := renameImport(a, b, c, d); err != nil {
						return err
					}
					fired = true
					return unix.EIO
				}
			}
			p.ops.checkpoint = func(stage string) {
				if fired {
					return
				}
				switch {
				case mode == "ctime" && stage == "digest-read":
					fired = true
					// chmod restores the same permission bits but advances ctime.
					before := p.intent.OriginalFile.Fingerprint
					for i := 0; i < 100; i++ {
						if err := unix.Fchmod(int(p.observation.input.Fd()), uint32(before.Mode)&0o777); err != nil {
							t.Fatal(err)
						}
						var s unix.Stat_t
						if err := unix.Fstat(int(p.observation.input.Fd()), &s); err != nil {
							t.Fatal(err)
						}
						if fingerprintOf(s) != before {
							return
						}
					}
					t.Fatal("filesystem ctime did not advance")
				case mode == "late input" && stage == "before-link":
					fired = true
					if err := os.Symlink(q.Source, q.Input); err != nil {
						t.Fatal(err)
					}
				case mode == "late source" && stage == "before-creating":
					fired = true
					if err := os.Rename(q.Source, q.Source+".saved"); err != nil {
						t.Fatal(err)
					}
					fixtureFile(t, q.Source)
				case mode == "late parent" && stage == "before-relocation":
					fired = true
					if err := os.Rename(q.Root, q.Root+".saved"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(q.Root, 0o700); err != nil {
						t.Fatal(err)
					}
				case mode == "late link" && stage == "before-owned":
					fired = true
					if err := os.Rename(q.Input, q.Input+".saved"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(q.Source, q.Input); err != nil {
						t.Fatal(err)
					}
				case mode == "late source contents" && stage == "before-completion":
					fired = true
					if err := os.WriteFile(q.Source, []byte("external edit"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			result, err := p.complete(ops)
			if err == nil || !fired {
				t.Fatal("late evidence not refused", err)
			}
			if mode == "uncertain rename" {
				if result.Relocated || !strings.Contains(err.Error(), "outcome may be uncertain") {
					t.Fatal(result, err)
				}
				got, e := os.ReadFile(q.Source)
				if e != nil || !bytes.Equal(got, data) {
					t.Fatal("uncertain rename lost content", e)
				}
			}
			s := ownershipSnapshot(t, q.Ownership)
			if len(s.Imports) != 1 || !s.Unfinished {
				t.Fatal("uncertain state retired")
			}
			if mode == "late input" || mode == "late link" {
				if len(s.Records) != 1 || s.Records[0].Phase != "creating" {
					t.Fatal("external symlink adopted")
				}
			}
			if _, err := p.complete(ops); err == nil {
				t.Fatal("failed authority reused")
			}
			assertImportBlocked(t, q)
		})
	}
}

func TestImportCompletionDescriptorCleanup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			p := liveImport(t, q)
			var fds []int
			fds = append(fds, p.view.registry.lock, p.view.lock, int(p.observation.input.Fd()), p.selectedFD)
			for _, d := range p.observation.plan.order {
				fds = append(fds, d.fd)
			}
			for _, d := range p.view.registry.space.order {
				fds = append(fds, d.fd)
			}
			ops := nativeImportCompleteOps()
			if fail {
				ops.rename = func(int, string, int, string) error { return unix.ENOTSUP }
			}
			_, err := p.complete(ops)
			if (err != nil) != fail {
				t.Fatal(err)
			}
			for _, fd := range fds {
				if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
					t.Fatal("descriptor leaked", fd, err)
				}
			}
			r, err := openRegistry(q.Ownership, true, nativeRegistryOps())
			if err != nil {
				t.Fatal("lock leaked", err)
			}
			_ = r.Close()
		})
	}
}

func TestNativeImportRename(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			q := preparationFixture(t, false, 1)
			p := liveImport(t, q)
			original, _ := os.ReadFile(q.Input)
			switch kind {
			case "file":
				fixtureFile(t, q.Source)
			case "directory":
				if err := os.Mkdir(q.Source, 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("missing", q.Source); err != nil {
					t.Fatal(err)
				}
			}
			before := treeState(t, filepath.Dir(q.Root))
			err := renameImport(p.observation.inputParent.fd, filepath.Base(q.Input), p.source.fd, filepath.Base(q.Source))
			if !errors.Is(err, unix.EEXIST) && !errors.Is(err, unix.ENOTEMPTY) {
				t.Fatal("native no-replace did not refuse occupant", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
				t.Fatal("native rename overwrote occupant")
			}
			got, _ := os.ReadFile(q.Input)
			if !bytes.Equal(original, got) {
				t.Fatal("native refusal lost original")
			}
		})
	}
	t.Run("cross filesystem", func(t *testing.T) {
		q := preparationFixture(t, false, 1)
		p := liveImport(t, q)
		info, err := os.Stat("/dev/shm")
		if err != nil || !info.IsDir() {
			t.Skip("no existing writable second-filesystem fixture location")
		}
		root, err := os.MkdirTemp("/dev/shm", "dots-exclusive-rename-")
		if err != nil {
			t.Skipf("second-filesystem fixture unavailable: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(root) })
		fd, err := unix.Open(root, directoryFlags, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer unix.Close(fd)
		var s unix.Stat_t
		if err := unix.Fstat(fd, &s); err != nil {
			t.Fatal(err)
		}
		if uint64(s.Dev) == p.observation.inputParent.id.device {
			t.Skip("fixture locations share a filesystem")
		}
		if err := renameImport(p.observation.inputParent.fd, filepath.Base(q.Input), fd, "file"); !errors.Is(err, unix.EXDEV) {
			t.Fatal("native cross-device rename not refused", err)
		}
		assertPreparationInput(t, q, p.observation.inputStat, []byte("unchanged"))
	})
}
