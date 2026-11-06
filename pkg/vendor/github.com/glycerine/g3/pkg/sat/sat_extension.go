package sat

// Source: src/sat/sat_extension.h.

import "github.com/glycerine/g3/pkg/util/lbool"

type SATExtensionCheckResult uint8

const (
	SATExtensionDone SATExtensionCheckResult = iota
	SATExtensionContinue
	SATExtensionGiveUp
)

type SATExternalConstraintIndex uint32
type SATExternalJustificationIndex uint32

// SATExtension is the concrete typed counterpart of sat::extension. A solver
// borrows the implementation and never closes it.
type SATExtension interface {
	Propagate(Literal, SATExternalConstraintIndex, bool) (bool, error)
	GetAntecedents(Literal, SATExternalJustificationIndex, *[]Literal) error
	Asserted(Literal) error
	Check() (SATExtensionCheckResult, error)
	Push() error
	Pop(uint32) error
	Simplify() error
	ClausesModified() error
	GetPhase(BoolVar) (lbool.LBool, error)
}
