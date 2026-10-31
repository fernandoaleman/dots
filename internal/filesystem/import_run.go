package filesystem

import "fmt"

// ImportCompletion reports confirmed steps, even if a later publication fails.
// False means unconfirmed, not proof that an uncertain operation had no effect.
type ImportCompletion = importCompletion

// RunImport prepares and completes one fresh transaction under the same lock.
// Refresh must reload configuration and resolve again; a preview grants no
// authority. No handle, retry, rollback or recovery capability is returned.
func RunImport(request ImportRequest, refresh func() (ImportRequest, error)) (ImportCompletion, error) {
	p, err := prepareImport(request, refresh, nativeImportPrepareOps())
	if err != nil {
		return ImportCompletion{}, fmt.Errorf("input was not relocated by this invocation; preserve input %q, proposed source %q and ownership evidence; inspect with dots recover before retrying: %w", request.Input, request.Source, err)
	}
	return p.complete(nativeImportCompleteOps())
}
