package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"

	"golang.org/x/sys/unix"
)

type importCompleteOps struct {
	rename  func(int, string, int, string) error
	symlink func(string, int, string) error
}

func nativeImportCompleteOps() importCompleteOps {
	return importCompleteOps{renameImport, nativeInstallOps().symlinkat}
}

// Counts describe successful calls/publications even if a later step fails.
type importCompletion struct{ Relocated, Linked, Owned bool }

// complete consumes only this uninterrupted preparation's authority, including
// on failure. Loading an intent cannot produce this capability. No rollback.
func (p *preparedImport) complete(ops importCompleteOps) (report importCompletion, err error) {
	if p.closed || p.used {
		return report, fmt.Errorf("import completion authority already consumed or closed")
	}
	p.used = true
	renameAttempted := false
	defer func() {
		if err != nil {
			err = errors.Join(err, p.view.registry.validatePublication())
			location := "original file was not relocated by this invocation"
			if report.Relocated {
				location = "file was relocated; original pathname may be absent; do not retry relocation"
			} else if renameAttempted {
				location = "rename outcome may be uncertain; original pathname may be absent; do not retry relocation"
			}
			err = fmt.Errorf("import stopped: %s; preserve original %q, source %q and ownership state; inspect with dots recover before explicitly selecting recovery: %w", location, p.request.Input, p.request.Source, err)
		}
		err = errors.Join(err, p.Close())
	}()
	if err := p.check(); err != nil {
		return report, err
	}
	// Reopen the selected input route: deduplicated inode aliases can have
	// different mount flags. Rename and symlink creation use this descriptor.
	fd, err := unix.Open(parentPath(p.request.Input), directoryFlags, 0)
	if err != nil {
		return report, err
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	var parent unix.Stat_t
	if err := unix.Fstat(fd, &parent); err != nil {
		return report, err
	}
	if identity(parent) != p.observation.inputParent.id {
		return report, fmt.Errorf("selected input-parent route changed")
	}
	inputParent := &directory{fd: fd, id: identity(parent)}
	r := p.view.registry
	var link *fingerprint
	check := func() error {
		if err := errors.Join(r.validatePublication(), r.validate(), privateFD(r.dir.fd, true), privateFD(r.lock, false), p.observation.bindings()); err != nil {
			return err
		}
		if p.closed || r.blocked != nil || r.lock < 0 || r.data.Version != 2 || !r.data.Unfinished ||
			len(r.data.Imports) != 1 || !reflect.DeepEqual(r.data.Imports[0], p.intent) || p.intent.SourceBinding == nil {
			return fmt.Errorf("no live import completion authority")
		}
		if err := validateSnapshot(r.data); err != nil {
			return err
		}
		for _, route := range []struct {
			path string
			dir  *directory
		}{{parentPath(p.request.Input), inputParent}, {parentPath(p.request.Source), p.source}} {
			if err := importCheckRoute(route.path, route.dir.id, p.observation.inputStat, p.ops.inspect); err != nil {
				return err
			}
		}
		if !report.Relocated {
			if err := p.check(); err != nil {
				return err
			}
			if err := p.checkInputAt(fd); err != nil {
				return err
			}
		} else {
			if err := p.checkRelocated(inputParent, link); err != nil {
				return err
			}
		}
		if err := p.checkDigest(); err != nil {
			return err
		}
		if err := errors.Join(r.validatePublication(), p.observation.bindings()); err != nil {
			return err
		}
		if report.Relocated {
			return p.checkRelocated(inputParent, link)
		}
		return errors.Join(p.check(), p.checkInputAt(fd))
	}
	// Every publication consumes Stage 2's reservation and verifies actual size.
	publish := func(phase string, completion bool) error {
		p.checkpoint("before-" + phase)
		if err := check(); err != nil {
			return err
		}
		if p.steps == 0 {
			return fmt.Errorf("import revision reservation exhausted")
		}
		if completion {
			r.data.Imports, r.data.Unfinished = []importIntent{}, false
		}
		encoded, err := snapshotBytes(r.data, 0)
		if err != nil || len(encoded) > p.bound {
			return errors.Join(fmt.Errorf("import snapshot exceeds reserved transaction capacity"), err)
		}
		p.steps--
		if err := r.publishSnapshot(); err != nil {
			return err
		}
		p.checkpoint(phase)
		return nil
	}
	p.checkpoint("before-relocation")
	if err := check(); err != nil {
		return report, err
	}
	renameAttempted = true
	if err := ops.rename(inputParent.fd, filepath.Base(p.request.Input), p.source.fd, filepath.Base(p.request.Source)); err != nil {
		return report, fmt.Errorf("exclusive same-filesystem import rename (no fallback): %w", err)
	}
	report.Relocated = true
	p.checkpoint("relocation")
	if err := check(); err != nil {
		return report, err
	}
	for _, step := range []struct {
		name string
		fd   int
	}{{"file-sync", int(p.observation.input.Fd())}, {"input-sync", inputParent.fd}, {"source-sync", p.source.fd}} {
		p.checkpoint("before-" + step.name)
		if err := check(); err != nil {
			return report, err
		}
		if err := p.ops.registry.sync(step.fd); err != nil {
			return report, fmt.Errorf("sync relocated import %s: %w", step.name, err)
		}
		p.checkpoint(step.name)
	}
	var moved unix.Stat_t
	if err := unix.Fstat(int(p.observation.input.Fd()), &moved); err != nil {
		return report, err
	}
	evidence := importFile{fingerprintOf(moved), moved.Uid, moved.Gid, uint64(moved.Nlink)}
	p.intent.Phase, p.intent.RelocatedFile = "relocated", &evidence
	r.data.Imports = []importIntent{p.intent}
	if err := publish("relocated", false); err != nil {
		return report, err
	}
	entry := preparedEntry{destination: Destination{Path: p.request.Input, Source: p.request.Source}, routes: p.observation.inputRoutes}
	r.data.Records = append(r.data.Records, newRecord(r.options, entry, inputParent))
	if err := publish("creating", false); err != nil {
		return report, err
	}
	p.checkpoint("before-link")
	if err := check(); err != nil {
		return report, err
	}
	if err := ops.symlink(p.request.Source, inputParent.fd, filepath.Base(p.request.Input)); err != nil {
		return report, fmt.Errorf("create import symlink without replacement: %w", err)
	}
	report.Linked = true
	p.checkpoint("link")
	leaf, err := r.captureCreated(entry, inputParent)
	if err != nil {
		return report, err
	}
	link = &leaf
	for n := range r.data.Records {
		if r.data.Records[n].Destination == pathBytes(p.request.Input) {
			r.data.Records[n].Leaf, r.data.Records[n].Phase = link, "owned"
		}
	}
	if err := publish("owned", false); err != nil {
		return report, err
	}
	report.Owned = true
	if err := publish("completion", true); err != nil {
		return report, err
	}
	return report, nil
}

func (p *preparedImport) checkInputAt(fd int) error {
	var selected unix.Stat_t
	original := p.observation.inputStat
	if err := unix.Fstatat(fd, filepath.Base(p.request.Input), &selected, unix.AT_SYMLINK_NOFOLLOW); err != nil || fingerprintOf(selected) != fingerprintOf(original) || selected.Uid != original.Uid || selected.Gid != original.Gid || selected.Nlink != original.Nlink {
		return errors.Join(fmt.Errorf("selected input slot changed"), err)
	}
	return nil
}

func (p *preparedImport) checkDigest() error {
	var before unix.Stat_t
	if err := unix.Fstat(int(p.observation.input.Fd()), &before); err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(p.observation.input, 0, p.intent.OriginalFile.Fingerprint.Size)); err != nil {
		return err
	}
	p.checkpoint("digest-read")
	var stat unix.Stat_t
	if err := unix.Fstat(int(p.observation.input.Fd()), &stat); err != nil {
		return err
	}
	file := importFile{fingerprintOf(stat), stat.Uid, stat.Gid, uint64(stat.Nlink)}
	if fingerprintOf(before) != fingerprintOf(stat) || before.Uid != stat.Uid || before.Gid != stat.Gid || before.Nlink != stat.Nlink || !sameImportFile(file, p.intent.OriginalFile) || hex.EncodeToString(h.Sum(nil)) != p.intent.Digest.Value {
		return fmt.Errorf("import file content or metadata changed")
	}
	return nil
}

func (p *preparedImport) checkRelocated(inputParent *directory, link *fingerprint) error {
	var opened, named unix.Stat_t
	if err := unix.Fstat(int(p.observation.input.Fd()), &opened); err != nil {
		return err
	}
	if err := unix.Fstatat(p.source.fd, filepath.Base(p.request.Source), &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	file := importFile{fingerprintOf(named), named.Uid, named.Gid, uint64(named.Nlink)}
	if fingerprintOf(named) != fingerprintOf(opened) || !sameImportFile(file, p.intent.OriginalFile) ||
		(p.intent.RelocatedFile != nil && file != *p.intent.RelocatedFile) {
		return fmt.Errorf("relocated source identity or metadata changed")
	}
	err := unix.Fstatat(inputParent.fd, filepath.Base(p.request.Input), &named, unix.AT_SYMLINK_NOFOLLOW)
	if link == nil {
		if !errors.Is(err, unix.ENOENT) {
			return errors.Join(fmt.Errorf("original import slot became occupied"), err)
		}
		return nil
	}
	if err != nil || fingerprintOf(named) != *link {
		return errors.Join(fmt.Errorf("live import symlink identity changed"), err)
	}
	text, err := readlinkAt(inputParent.fd, filepath.Base(p.request.Input))
	if err != nil || text != p.request.Source {
		return errors.Join(fmt.Errorf("live import symlink contents changed"), err)
	}
	return nil
}
