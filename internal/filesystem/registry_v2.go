package filesystem

import (
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/sys/unix"
)

// These are recovery observations, never a capability to advance an import.
// Stage 1 has no intent producer, upgrade path, or administrative recovery.
type importBinding struct {
	Parent entryID         `json:"parent"`
	Routes []recordedRoute `json:"routes"`
}

type importAnchor struct {
	importBinding
	Missing []pathBytes `json:"missing"`
}

type importFile struct {
	Fingerprint fingerprint `json:"fingerprint"`
	UID         uint32      `json:"uid"`
	GID         uint32      `json:"gid"`
	Nlink       uint64      `json:"nlink"`
}

type importDigest struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type importIntent struct {
	ID            string         `json:"id"`
	Phase         string         `json:"phase"`
	Profile       pathBytes      `json:"profile"`
	Target        pathBytes      `json:"target"`
	Root          pathBytes      `json:"root"`
	Input         pathBytes      `json:"input"`
	Source        pathBytes      `json:"source"`
	InputBinding  importBinding  `json:"input_binding"`
	SourceAnchor  importAnchor   `json:"source_anchor"`
	OriginalFile  importFile     `json:"original_file"`
	Digest        importDigest   `json:"digest"`
	SourceBinding *importBinding `json:"source_binding,omitempty"`
	RelocatedFile *importFile    `json:"relocated_file,omitempty"`
}

func (s snapshot) allowMutation() error {
	if len(s.Imports) != 0 {
		i := s.Imports[0]
		return fmt.Errorf("pending import %s blocks mutation: original %q, source %q; preserve both locations and ownership state; inspect with dots recover before explicitly selecting recovery", i.ID, i.Input, i.Source)
	}
	return nil
}

func (r *registry) allowMutation() error {
	if r.blocked != nil {
		return r.blocked
	}
	return r.data.allowMutation()
}

func (r *registry) reserveRevisions(steps uint64) error {
	if err := r.allowMutation(); err != nil {
		return err
	}
	return revisionCapacity(r.data.Revision, steps)
}

func revisionCapacity(revision, steps uint64) error {
	if steps > math.MaxUint64-revision {
		return fmt.Errorf("ownership revision counter exhausted")
	}
	return nil
}

func validateImports(s snapshot) error {
	if len(s.Imports) > 1 || (len(s.Imports) != 0 && !s.Unfinished) {
		return fmt.Errorf("invalid pending import count or unfinished barrier")
	}
	for _, i := range s.Imports {
		if !lowerHex(i.ID, 32) || !validAbsolute(string(i.Profile)) ||
			!importChild(i.Input, i.Target) || !importChild(i.Source, i.Root) || (i.Input == i.Source || strings.HasPrefix(string(i.Input), string(i.Source)+"/") || strings.HasPrefix(string(i.Source), string(i.Input)+"/")) ||
			i.Digest.Algorithm != "sha256-v1" || !lowerHex(i.Digest.Value, 64) {
			return fmt.Errorf("invalid import identifier, paths, or digest")
		}
		if !validImportBinding(i.InputBinding) || !validImportBinding(i.SourceAnchor.importBinding) || i.SourceAnchor.Missing == nil || !validImportFile(i.OriginalFile) {
			return fmt.Errorf("incomplete import file or ancestry evidence")
		}
		relative := strings.TrimPrefix(string(i.Source), strings.TrimRight(string(i.Root), "/")+"/")
		if len(i.SourceAnchor.Missing) > len(strings.Split(relative, "/"))-1 {
			return fmt.Errorf("missing import parents extend above selected root")
		}
		if len(i.SourceAnchor.Missing) != 0 {
			names := make([]string, len(i.SourceAnchor.Missing))
			for n, name := range i.SourceAnchor.Missing {
				names[n] = string(name)
			}
			if !strings.HasSuffix(parentPath(string(i.Source)), "/"+strings.Join(names, "/")) {
				return fmt.Errorf("import source suffix does not match missing parents")
			}
		}
		for _, name := range i.SourceAnchor.Missing {
			if !validBasename(string(name)) {
				return fmt.Errorf("invalid missing import parent")
			}
		}
		if len(i.SourceAnchor.Missing) == 0 && (i.SourceBinding == nil || !reflect.DeepEqual(*i.SourceBinding, i.SourceAnchor.importBinding)) {
			return fmt.Errorf("existing import source parent requires matching binding")
		}
		if i.SourceBinding != nil && !validSourceBinding(i) {
			return fmt.Errorf("invalid import source binding")
		}
		if importEndpointConflict(i) {
			return fmt.Errorf("import endpoints identify overlapping or ambiguous physical slots")
		}
		switch i.Phase {
		case "prepared":
			if i.RelocatedFile != nil {
				return fmt.Errorf("prepared import has relocated evidence")
			}
		case "relocated":
			if i.SourceBinding == nil || i.RelocatedFile == nil || !validImportFile(*i.RelocatedFile) || !sameImportFile(i.OriginalFile, *i.RelocatedFile) {
				return fmt.Errorf("relocated import requires matching complete file and parent evidence")
			}
		default:
			return fmt.Errorf("unsupported import phase %q", i.Phase)
		}
		for _, r := range s.Records {
			// The associated M3 creation may coexist with the intent, including
			// later invalidation. Loading that relationship never grants authority.
			associated := i.Phase == "relocated" && r.Profile == i.Profile && r.Target == i.Target &&
				r.Destination == i.Input && r.Text == i.Source && r.Parent == i.InputBinding.Parent &&
				reflect.DeepEqual(r.Routes, i.InputBinding.Routes) && r.Phase != "removing"
			if associated {
				continue
			}
			if importRecordOverlap(i.Input, i.InputBinding, r) ||
				importRecordOverlap(i.sourceAnchorPath(), i.SourceAnchor.importBinding, r) ||
				(i.SourceBinding != nil && importRecordOverlap(i.Source, *i.SourceBinding, r)) {
				return fmt.Errorf("import overlaps recorded ownership claim %q", r.Destination)
			}
		}
	}
	return nil
}

func importEndpointConflict(i importIntent) bool {
	inRoutes := func(parent entryID, name string, routes []recordedRoute) bool {
		for _, route := range routes {
			if sameImportSlot(parent, name, route.Parent, string(route.Name)) {
				return true
			}
		}
		return false
	}
	inputName := filepath.Base(string(i.Input))
	source, sourceName := i.SourceAnchor.importBinding, filepath.Base(string(i.sourceAnchorPath()))
	if i.SourceBinding != nil {
		// A validated binding includes the anchor routes and realized suffix.
		source, sourceName = *i.SourceBinding, filepath.Base(string(i.Source))
	}
	return sameImportSlot(i.InputBinding.Parent, inputName, source.Parent, sourceName) ||
		inRoutes(i.InputBinding.Parent, inputName, source.Routes) ||
		inRoutes(source.Parent, sourceName, i.InputBinding.Routes)
}

func lowerHex(s string, length int) bool {
	return len(s) == length && strings.Trim(s, "0123456789abcdef") == ""
}

func importChild(path, root pathBytes) bool {
	if !validAbsolute(string(root)) || !validAbsolute(string(path)) {
		return false
	}
	prefix := strings.TrimRight(string(root), "/") + "/"
	if !strings.HasPrefix(string(path), prefix) {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(string(path), prefix), "/") {
		if !validBasename(part) {
			return false
		}
	}
	return true
}

func validImportBinding(b importBinding) bool {
	if b.Parent.Inode == 0 || b.Routes == nil {
		return false
	}
	for n, r := range b.Routes {
		if (!validBasename(string(r.Name)) && r.Name != "..") || r.Parent.Inode == 0 || r.Entry.Inode == 0 || r.Target.Inode == 0 ||
			strings.ContainsRune(string(r.Text), 0) || (n > 0 && b.Routes[n-1].Target != r.Parent) ||
			(r.Type != unix.S_IFDIR && r.Type != unix.S_IFLNK) ||
			(r.Type == unix.S_IFLNK && r.Text == "") || (r.Type == unix.S_IFDIR && (r.Entry != r.Target || r.Text != "")) {
			return false
		}
	}
	return len(b.Routes) == 0 || b.Routes[len(b.Routes)-1].Target == b.Parent
}

func validSourceBinding(i importIntent) bool {
	b, a := *i.SourceBinding, i.SourceAnchor
	if !validImportBinding(b) || len(b.Routes) != len(a.Routes)+len(a.Missing) {
		return false
	}
	if !reflect.DeepEqual(b.Routes[:len(a.Routes)], a.Routes) {
		return false
	}
	parent := a.Parent
	for n, name := range a.Missing {
		route := b.Routes[len(a.Routes)+n]
		if route.Parent != parent || route.Name != name || route.Type != unix.S_IFDIR {
			return false
		}
		parent = route.Target
	}
	return parent == b.Parent
}

func validImportFile(f importFile) bool {
	p := f.Fingerprint
	return p.ID.Inode != 0 && p.Mode&unix.S_IFMT == unix.S_IFREG && p.Mode&0o7000 == 0 &&
		p.Size >= 0 && p.ModNsec >= 0 && p.ModNsec < 1e9 && p.ChgNsec >= 0 && p.ChgNsec < 1e9 && f.Nlink == 1
}

// Rename may change ctime. Preserve both observations without requiring ctime
// equality across the rename; recovery must verify stability during observation.
func sameImportFile(a, b importFile) bool {
	a.Fingerprint.Changed, a.Fingerprint.ChgNsec = 0, 0
	b.Fingerprint.Changed, b.Fingerprint.ChgNsec = 0, 0
	return a == b
}

func importRecordOverlap(path pathBytes, b importBinding, r ownershipRecord) bool {
	a, other := string(path), string(r.Destination)
	if a == other || strings.HasPrefix(a, other+"/") || strings.HasPrefix(other, a+"/") ||
		sameImportSlot(b.Parent, filepath.Base(a), r.Parent, filepath.Base(other)) {
		return true
	}
	for _, route := range b.Routes {
		if sameImportSlot(route.Parent, string(route.Name), r.Parent, filepath.Base(other)) {
			return true
		}
	}
	for _, route := range r.Routes {
		if sameImportSlot(route.Parent, string(route.Name), b.Parent, filepath.Base(a)) {
			return true
		}
	}
	return false
}

// Pair the existing anchor identity with its immediate prospective child, not
// a final basename beneath missing parents. Preserve raw symlink/.. traversal.
func (i importIntent) sourceAnchorPath() pathBytes {
	path := i.Source
	for range i.SourceAnchor.Missing {
		path = pathBytes(parentPath(string(path)))
	}
	return path
}

func sameImportSlot(a entryID, x string, b entryID, y string) bool {
	// Do not traverse historical paths for native capabilities. Ambiguous names
	// in a shared recorded parent conservatively reserve the same slot.
	return a == b && potentialNameKey(x) == potentialNameKey(y)
}
