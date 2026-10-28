package filesystem

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// Evidence belongs to the uninterrupted locked invocation, never loaded state.
// The digest covers revision, import intent and all unrelated ownership history.
type registryPublication struct {
	entry    importStateEntry
	revision uint64
	digest   [sha256.Size]byte
}

func publicationFor(state snapshot, entry importStateEntry) (registryPublication, error) {
	data, err := snapshotBytes(state, 0)
	return registryPublication{entry, state.Revision, sha256.Sum256(data)}, err
}

func (s registryPublication) check(r *registry) error {
	if err := s.entry.revalidate(); err != nil {
		return fmt.Errorf("registry snapshot changed since live transaction publication: %w", err)
	}
	if !s.entry.exists { // Only initial admission may have no snapshot yet.
		return nil
	}
	// Reuse strict bounded loading without replacing proposed in-memory state
	// or confusing loaded evidence with authority to advance a pending import.
	probe := *r
	probe.data, probe.publication = snapshot{}, nil
	if err := probe.load(); err != nil {
		return fmt.Errorf("validate expected registry snapshot: %w", err)
	}
	data, err := snapshotBytes(probe.data, 0)
	if err == nil && (probe.data.Revision != s.revision || sha256.Sum256(data) != s.digest) {
		err = fmt.Errorf("registry revision or transaction contents differ from last live publication")
	}
	return errors.Join(err, s.entry.revalidate())
}

func (r *registry) validatePublication() error {
	if r.publication == nil { // Ordinary M3 operations retain their existing path.
		return nil
	}
	return r.publication.check(r)
}
