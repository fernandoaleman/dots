package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"golang.org/x/sys/unix"
)

type importObservation struct {
	plan                      *PreparedPlan
	input                     *os.File
	inputParent, sourceParent *directory
	inputStat                 unix.Stat_t
	inputRoutes, sourceRoutes []binding
	missing                   []string
	sourceParentName          string
	check                     func() error
	bindings                  func() error
}

func (o *importObservation) Close() error {
	var err error
	if o.input != nil {
		err = o.input.Close()
		o.input = nil
	}
	if o.plan != nil {
		err = errors.Join(err, o.plan.Close())
	}
	return err
}

type importPrepareOps struct {
	registry   registryOps
	inspect    importInspectionOps
	mkdir      func(int, string, uint32) error
	checkpoint func(string)
}

func nativeImportPrepareOps() importPrepareOps {
	return importPrepareOps{registry: nativeRegistryOps(), inspect: nativeImportInspectionOps(), mkdir: unix.Mkdirat}
}

// A live capability is created only from clean state under the retained lock.
// Close releases resources, never completes, retires or rolls back an import.
type preparedImport struct {
	view                     *importRegistryInspection
	observation              importObservation
	request                  ImportRequest
	intent                   importIntent
	source                   *directory
	selectedFD               int
	remaining                []string
	steps                    uint64
	bound                    int
	closed, barrierAttempted bool
	used                     bool
	ops                      importPrepareOps
}

// refresh must reload configuration and run the resolver, not replay a preview.
// A callback keeps resolution in its owning package; no public caller is wired.
// This private stage requires existing state/lock and never bootstraps them.
func prepareImport(preview ImportRequest, refresh func() (ImportRequest, error), ops importPrepareOps) (_ *preparedImport, err error) {
	preview.Ownership.Sources = append([]string(nil), preview.Ownership.Sources...)
	view, err := openImportRegistry(preview.Ownership, nil)
	if err != nil {
		return nil, err
	}
	p := &preparedImport{view: view, selectedFD: -1, ops: ops}
	defer func() {
		if err != nil {
			err = errors.Join(err, view.registry.validatePublication())
			if p.barrierAttempted {
				err = fmt.Errorf("import preparation stopped; preserve original %q, proposed source %q and ownership state: %w", preview.Input, preview.Source, err)
			}
			err = errors.Join(err, p.Close())
		}
	}()
	r := view.registry
	if r.dir == nil || view.lock < 0 {
		return nil, fmt.Errorf("private import preparation requires an existing private state directory and stable lock; bootstrap is not implemented")
	}
	r.ops = ops.registry
	if err := r.acquireExisting(); err != nil {
		return nil, err
	}
	r.data = snapshot{Version: 1, Records: []ownershipRecord{}}
	if err := r.load(); err != nil {
		return nil, err
	}
	if err := view.revalidate(); err != nil {
		return nil, err
	}
	expected, err := publicationFor(r.data, view.entries[0])
	if err != nil {
		return nil, err
	}
	r.publication = &expected
	if err := r.validatePublication(); err != nil {
		return nil, err
	}
	if err := r.allowMutation(); err != nil {
		return nil, err
	}
	if r.data.Unfinished {
		return nil, fmt.Errorf("unfinished ownership transaction blocks private import preparation; no recovery performed")
	}
	for _, record := range r.data.Records {
		if record.Phase == "creating" || record.Phase == "removing" {
			return nil, fmt.Errorf("unresolved ownership operation blocks private import preparation")
		}
	}
	if refresh == nil {
		return nil, fmt.Errorf("import preparation requires fresh resolution under the lock")
	}
	fresh, err := refresh()
	if err != nil {
		return nil, err
	}
	fresh.Ownership.Sources = append([]string(nil), fresh.Ownership.Sources...)
	fresh.Entries = append([]Destination(nil), fresh.Entries...)
	if fresh.Input != preview.Input || fresh.Source != preview.Source || fresh.Root != preview.Root || !reflect.DeepEqual(fresh.Ownership, preview.Ownership) {
		return nil, fmt.Errorf("import selection or ownership scope changed since preview")
	}
	p.request = fresh
	ops.inspect.retain = &p.observation
	if _, err := inspectImport(fresh, ops.inspect); err != nil {
		return nil, err
	}
	o := &p.observation
	p.remaining = append([]string(nil), o.missing...)
	// Retain the selected mount route rather than an inode-deduplicated alias.
	p.selectedFD, err = unix.Open(o.sourceParentName, directoryFlags, 0)
	if err != nil {
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(p.selectedFD, &stat); err != nil {
		return nil, err
	}
	if identity(stat) != o.sourceParent.id {
		return nil, fmt.Errorf("selected source-parent route changed")
	}
	p.source = &directory{fd: p.selectedFD, id: identity(stat)}
	hash := sha256.New()
	if _, err := io.Copy(hash, o.input); err != nil {
		return nil, fmt.Errorf("hash import input: %w", err)
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	id, err := randomName()
	if err != nil {
		return nil, err
	}
	anchor := importBinding{idOfDirectory(o.sourceParent), recordRoutes(o.sourceRoutes)}
	i := importIntent{ID: id, Phase: "prepared", Profile: pathBytes(fresh.Ownership.Profile), Target: pathBytes(fresh.Ownership.Target),
		Root: pathBytes(fresh.Root), Input: pathBytes(fresh.Input), Source: pathBytes(fresh.Source),
		InputBinding: importBinding{idOfDirectory(o.inputParent), recordRoutes(o.inputRoutes)},
		SourceAnchor: importAnchor{anchor, []pathBytes{}},
		OriginalFile: importFile{fingerprintOf(o.inputStat), o.inputStat.Uid, o.inputStat.Gid, uint64(o.inputStat.Nlink)},
		Digest:       importDigest{"sha256-v1", hex.EncodeToString(hash.Sum(nil))}}
	for _, name := range o.missing {
		i.SourceAnchor.Missing = append(i.SourceAnchor.Missing, pathBytes(name))
	}
	if len(o.missing) == 0 {
		i.SourceBinding = &anchor
	}
	p.intent = i
	p.bound, p.steps, err = reserveImport(r.data, i)
	if err != nil {
		return nil, err
	}
	if err := view.revalidate(); err != nil {
		return nil, err
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	if err := ops.registry.sync(int(o.input.Fd())); err != nil {
		return nil, fmt.Errorf("sync original import file: %w", err)
	}
	if err := ops.registry.sync(r.dir.fd); err != nil {
		return nil, fmt.Errorf("sync existing state directory: %w", err)
	}
	p.checkpoint("before-barrier")
	if err := view.revalidate(); err != nil {
		return nil, err
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	p.barrierAttempted = true
	p.steps--
	if err := r.begin(); err != nil {
		return nil, err
	}
	p.checkpoint("barrier")
	if err := p.publishPrepared(); err != nil {
		return nil, err
	}
	p.checkpoint("intent")
	for n, name := range o.missing {
		p.checkpoint(fmt.Sprintf("before-mkdir:%d", n))
		if err := p.check(); err != nil {
			return nil, err
		}
		if err := ops.mkdir(p.source.fd, name, 0o700); err != nil {
			return nil, fmt.Errorf("create import parent %q without replacement: %w", name, err)
		}
		p.checkpoint(fmt.Sprintf("mkdir:%d", n))
		next, err := o.plan.openDirectory(p.source.fd, name, unix.O_NOFOLLOW)
		if err != nil {
			return nil, err
		}
		var created unix.Stat_t
		if err := unix.Fstatat(p.source.fd, name, &created, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, err
		}
		route := binding{parent: p.source, name: name, entry: created, target: next}
		if err := errors.Join(validateBinding(route), privateFD(next.fd, true), importExactParent(p.source.fd, name)); err != nil {
			return nil, err
		}
		if err := ops.registry.sync(next.fd); err != nil {
			return nil, fmt.Errorf("sync new import directory: %w", err)
		}
		if err := ops.registry.sync(p.source.fd); err != nil {
			return nil, fmt.Errorf("sync import parent binding: %w", err)
		}
		o.sourceRoutes = append(o.sourceRoutes, route)
		o.plan.routes = append(o.plan.routes, route)
		p.source, p.remaining = next, p.remaining[1:]
		p.checkpoint(fmt.Sprintf("parent:%d", n))
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	if len(o.missing) != 0 {
		p.intent.SourceBinding = &importBinding{idOfDirectory(p.source), recordRoutes(o.sourceRoutes)}
		if err := p.publishPrepared(); err != nil {
			return nil, err
		}
	}
	p.checkpoint("binding")
	if err := p.check(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *preparedImport) checkpoint(name string) {
	if p.ops.checkpoint != nil {
		p.ops.checkpoint(name)
	}
}

func (p *preparedImport) check() error {
	if p.closed || p.view.registry.lock < 0 || p.view.registry.publication == nil {
		return fmt.Errorf("import preparation is closed or registry continuity evidence is missing")
	}
	o, r := &p.observation, p.view.registry
	if err := errors.Join(r.validate(), r.validatePublication(), privateFD(r.dir.fd, true), privateFD(r.lock, false), o.check()); err != nil {
		return err
	}
	var current unix.Stat_t
	if err := unix.Fstat(int(o.input.Fd()), &current); err != nil {
		return err
	}
	if fingerprintOf(current) != fingerprintOf(o.inputStat) || current.Uid != o.inputStat.Uid || current.Gid != o.inputStat.Gid || current.Nlink != o.inputStat.Nlink {
		return fmt.Errorf("opened import input changed")
	}
	name := filepath.Base(p.request.Source)
	if len(p.remaining) != 0 {
		name = p.remaining[0]
	}
	if err := unix.Fstatat(p.source.fd, name, &current, unix.AT_SYMLINK_NOFOLLOW); !errors.Is(err, unix.ENOENT) {
		return errors.Join(fmt.Errorf("prospective source slot became occupied"), err)
	}
	parentName := parentPath(p.request.Source)
	for range p.remaining {
		parentName = parentPath(parentName)
	}
	return errors.Join(p.ops.inspect.writable(p.source.fd), importCheckRoute(parentName, p.source.id, o.inputStat, p.ops.inspect))
}

// Only this uninterrupted preparation may upgrade and publish its own intent.
// Ordinary save/begin/finish gates and loaded pending-import latches stay intact.
func (p *preparedImport) publishPrepared() error {
	r := p.view.registry
	if p.closed || !p.barrierAttempted || p.steps == 0 || r.blocked != nil || r.lock < 0 || !r.data.Unfinished || p.intent.Phase != "prepared" {
		return fmt.Errorf("no live prepared-import publication authority")
	}
	if len(r.data.Imports) != 0 && (len(r.data.Imports) != 1 || r.data.Imports[0].ID != p.intent.ID) {
		return fmt.Errorf("unrelated import intent")
	}
	if err := p.check(); err != nil {
		return err
	}
	r.data.Version, r.data.Imports = 2, []importIntent{p.intent}
	encoded, err := snapshotBytes(r.data, 0)
	if err != nil {
		return err
	}
	if len(encoded) > p.bound {
		return fmt.Errorf("prepared snapshot exceeds reserved transaction capacity")
	}
	p.steps--
	if err := r.publishSnapshot(); err != nil {
		return err
	}
	r.version = 2
	return nil
}

func (p *preparedImport) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	err := p.observation.Close()
	if p.selectedFD >= 0 {
		err = errors.Join(err, unix.Close(p.selectedFD))
		p.selectedFD = -1
	}
	return errors.Join(err, p.view.Close())
}
