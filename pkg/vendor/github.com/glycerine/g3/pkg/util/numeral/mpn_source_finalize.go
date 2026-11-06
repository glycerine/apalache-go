package numeral

// Source mpn.h manager member destruction: t_ab,t_ms,v,u.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// SourceFinalize performs only the actual completed manager's source member
// destructors. Native division workspace is released without Runtime events.

func (m *NaturalManager) SourceFinalize() error {
	if m.numericCallerExecuting() {
		return arena.ErrInvalidSlot
	}
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.ClosedField || m.Closing || m.runtime == nil || m.originalCause != nil {
		return arena.ErrClosed
	}
	return m.sourceFinalizeMembers()
}

// Owning Close resumes an explicitly begun source cursor without reopening
// the public half-destroyed owner.
func (m *NaturalManager) sourceFinalizeMembers() error {
	if m.sourceFinalized {
		return nil
	}
	if m.Formatting.Active || m.CharFormatting.active || m.characterCalls != nil {
		return arena.ErrInvalidSlot
	}
	m.SourceFinalizing = true
	m.Division = [4][]uint32{}
	m.verificationScratch = nil
	m.SourceStage = 4
	m.sourceFinalized = true
	return nil
}
