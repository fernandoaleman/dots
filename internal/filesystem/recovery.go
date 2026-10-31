package filesystem

// RecoverySelection identifies one stored intent at an exact registry revision.
type RecoverySelection = recoverySelection

// RecoveryInspection contains copied observations, never mutation authority.
type RecoveryInspection = recoveryInspection

// RecoveryRetirement reports confirmed publications and the surviving file.
type RecoveryRetirement = recoveryRetirement

// BarrierSettlement reports a registry-only settlement or unchanged-state no-op.
type BarrierSettlement = barrierSettlement

// InspectRecovery observes existing state without locking, writing or syncing.
// With no selection, a pending summary supplies an exact selection for a second
// detailed observation. Concurrent changes refuse rather than authorize action.
func InspectRecovery(path string, selection *RecoverySelection) (RecoveryInspection, error) {
	result, err := inspectRecovery(path, selection, nil)
	if err == nil && selection == nil && result.Intent != nil {
		selected := RecoverySelection{ID: result.Intent.ID, Revision: result.Revision}
		return inspectRecovery(path, &selected, nil)
	}
	return result, err
}

// RetireImport verifies the selected intent under the existing exclusive lock,
// invalidates prior authority and retires only the intent, retaining the barrier.
// User files are never modified. Inspection results cannot authorize this call.
func RetireImport(path string, selection RecoverySelection) (RecoveryRetirement, error) {
	return retireImport(path, selection, nativeRecoveryRetireOps())
}

// SettleBarrier independently verifies an exact revision under the existing
// exclusive lock before clearing an eligible barrier. It never repairs user paths.
func SettleBarrier(path string, revision uint64) (BarrierSettlement, error) {
	return settleBarrier(path, revision, nativeBarrierSettlementOps())
}
