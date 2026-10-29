package numeral

// Actual accepted MPBQ debug children, shared by retained numeric callers.

type binaryDebugChildren struct {
	owned, integer, arithmetic uint64
	snapshots                  *OwnedRational
}

func (s *binaryDebugChildren) capture(b *BinaryRationalManager) {
	*s = binaryDebugChildren{}
	if b.toOwnedRationalScope.pending {
		s.owned = b.toOwnedInvocation
	}
	if b.debugIntegerPending {
		s.integer = b.debugIntegerInvocation
	}
	if b.debugArithmeticPending {
		s.arithmetic = b.debugArithmeticInvocation
	}
	if n := len(b.debugSnapshotQueue); n != 0 {
		s.snapshots = &b.debugSnapshotQueue[n-1]
	}
}
func (s *binaryDebugChildren) ownsSnapshots(b *BinaryRationalManager) bool {
	if s.snapshots != nil {
		for i := range b.debugSnapshotQueue {
			if &b.debugSnapshotQueue[i] == s.snapshots {
				return true
			}
		}
	}
	return false
}
func (s *binaryDebugChildren) pending(b *BinaryRationalManager) bool {
	return s.owned != 0 && s.owned == b.toOwnedInvocation && b.toOwnedRationalScope.pending || s.integer != 0 && s.integer == b.debugIntegerInvocation && b.debugIntegerPending || s.arithmetic != 0 && s.arithmetic == b.debugArithmeticInvocation && b.debugArithmeticPending || s.ownsSnapshots(b)
}

func (s *binaryDebugChildren) sourceCause(b *BinaryRationalManager, body error) error {
	if s.owned != 0 && s.owned == b.toOwnedInvocation && b.toOwnedRationalScope.sourceCause != nil {
		return b.toOwnedRationalScope.sourceCause
	}
	if s.arithmetic != 0 && s.arithmetic == b.debugArithmeticInvocation && b.debugArithmeticPending {
		if e := b.debugArithmeticCall.SourceReady(); e != nil {
			return e
		}
	}
	if s.integer != 0 && s.integer == b.debugIntegerInvocation && b.debugIntegerPending || s.ownsSnapshots(b) {
		return body
	}
	return nil
}
func (s *binaryDebugChildren) cleanup(b *BinaryRationalManager) error {
	if s.owned != 0 && s.owned == b.toOwnedInvocation && b.toOwnedRationalScope.pending {
		if e := b.CleanupToOwnedRational(); e != nil {
			return e
		}
	}
	if s.integer != 0 && s.integer == b.debugIntegerInvocation && b.debugIntegerPending {
		if e := b.CleanupDebugInteger(); e != nil {
			return e
		}
	}
	if s.arithmetic != 0 && s.arithmetic == b.debugArithmeticInvocation && b.debugArithmeticPending || s.ownsSnapshots(b) {
		if e := b.CleanupDebugSnapshots(); e != nil {
			return e
		}
	}
	return nil
}
