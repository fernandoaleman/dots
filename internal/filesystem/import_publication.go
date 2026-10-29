package filesystem

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
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
	digest := sha256.New()
	encoder := json.NewEncoder(digest)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(probe.data) // Same canonical bytes and newline as snapshotBytes.
	if err == nil && (probe.data.Revision != s.revision || !bytes.Equal(digest.Sum(nil), s.digest[:])) {
		// Matching contents already passed recovery-capacity checks when the
		// expected evidence was established. Changed contents retain the checked
		// path so capacity failures keep their existing diagnostic precedence.
		_, err = snapshotBytes(probe.data, 0)
		if err == nil {
			err = fmt.Errorf("registry revision or transaction contents differ from last live publication")
		}
	}
	return errors.Join(err, s.entry.revalidate())
}

func (r *registry) validatePublication() error {
	if r.publication == nil { // Ordinary M3 operations retain their existing path.
		return nil
	}
	return r.publication.check(r)
}
