package smtcore

// Source: src/smt/asserted_formulas.cpp reduce/invoke and the concrete
// should_apply methods in src/smt/asserted_formulas.h.

import "fmt"

type assertedPass uint8

const (
	assertedPropagate assertedPass = iota
	assertedMacros
	assertedNNF
	assertedReduce
	assertedPullQuantifiers
	assertedLiftITE
	assertedLiftNonGroundITE
	assertedEliminateITE
	assertedInjectivity
	assertedDistribute
	assertedQuasiMacros
	assertedBit2Int
	assertedBounds
	assertedPatterns
	assertedBVSharing
)

func (k *SMTKernel) shouldApplyAssertedPass(pass assertedPass) bool {
	p := k.preprocessState
	switch pass {
	case assertedPropagate:
		return p.PropagateValues
	case assertedMacros:
		return p.MacroFinder && k.assertedHasQuantifiers
	case assertedNNF:
		return p.NNFCNF || (k.assertedHasQuantifiers && k.internalizer.quantifierManager != nil && k.internalizer.quantifierManager.mbqi)
	case assertedReduce:
		return true
	case assertedPullQuantifiers:
		return p.PullNestedQuantifiers && k.assertedHasQuantifiers
	case assertedLiftITE:
		return p.LiftITE != smtLiftITENone
	case assertedLiftNonGroundITE:
		// Source setup disables a redundant non-ground lifting pass.
		return p.NonGroundLiftITE != smtLiftITENone && p.LiftITE != smtLiftITEFull &&
			!(p.LiftITE == smtLiftITEConservative && p.NonGroundLiftITE == smtLiftITEConservative)
	case assertedEliminateITE:
		return p.EliminateTermITE && p.LiftITE != smtLiftITEFull
	case assertedInjectivity:
		return p.RefineInjectivityAxiom && k.assertedHasQuantifiers
	case assertedDistribute:
		return p.DistributeForall && k.assertedHasQuantifiers
	case assertedQuasiMacros:
		return p.QuasiMacros && k.assertedHasQuantifiers
	case assertedBit2Int:
		return p.SimplifyBitToInt
	case assertedBounds:
		return p.EliminateBounds && k.assertedHasQuantifiers
	case assertedPatterns:
		return k.smtState.Ematching && k.assertedHasQuantifiers
	case assertedBVSharing:
		return p.MaxBVSharing
	}
	return false
}

// invokeAssertedPass leaves skipped passes free of resource-count increments.
// An inconsistent result still needs internalization; cancellation does not.
func (k *SMTKernel) invokeAssertedPass(pass assertedPass) (proceed bool, canceled bool, err error) {
	if !k.shouldApplyAssertedPass(pass) {
		return true, false, nil
	}
	if err := assertedVerbose(10, "(smt."+pass.sourceName()+")\n"); err != nil {
		return false, false, err
	}
	switch pass {
	case assertedPropagate:
		err = k.propagatePendingAssertions()
	case assertedMacros:
		err = k.findPendingMacros()
	case assertedNNF:
		err = k.normalizePendingAssertions()
	case assertedReduce:
		err = k.reducePendingAssertions()
	case assertedPullQuantifiers:
		err = k.pullPendingNestedQuantifiers()
	case assertedLiftITE:
		err = k.liftPendingITEs(false)
	case assertedLiftNonGroundITE:
		err = k.liftPendingITEs(true)
	case assertedEliminateITE:
		err = k.eliminatePendingTermITEs()
	case assertedInjectivity:
		err = k.refinePendingInjectivityAxioms()
	case assertedDistribute:
		err = k.distributePendingUniversals()
	case assertedQuasiMacros:
		err = k.applyPendingQuasiMacros()
	case assertedBit2Int:
		err = k.applyPendingBit2Int()
	case assertedBounds:
		err = k.eliminatePendingBounds()
	case assertedPatterns:
		err = k.inferPendingPatterns()
	case assertedBVSharing:
		err = k.maximizePendingBVSharing()
	}
	if err != nil {
		return false, false, fmt.Errorf("asserted preprocessing %s: %w", pass.sourceName(), err)
	}
	if err := k.assertedVerboseSize(); err != nil {
		return false, false, err
	}
	if err := k.traceAssertedState("reduce_step_ll", "invoke", 285, true); err != nil {
		return false, false, err
	}
	if err := k.checkAssertedWellSorted(286); err != nil {
		return false, false, err
	}
	if k.assertedFormulasInconsistent() {
		return false, false, k.traceAssertedAfterReduce("invoke", 288, 289)
	}
	canceled, err = k.manager.Canceled()
	if err == nil && canceled {
		err = k.traceAssertedAfterReduce("invoke", 288, 289)
	}
	return !canceled && err == nil, canceled, err
}

// reduceAssertedPending returns false only when cancellation prevents committing
// the pending assertions. The source's unwritten, false bb_quantifiers flag has
// no retained consumers and does not introduce a pass here.
func (k *SMTKernel) reduceAssertedPending() (bool, error) {
	if k.assertedFormulasInconsistent() {
		return true, nil
	}
	if canceled, err := k.manager.Canceled(); canceled || err != nil {
		return false, err
	}
	if k.assertedHead == len(k.assertions) {
		return true, nil
	}
	// Definitions removed by earlier macro discovery remain active even if
	// discovery or preprocessing is subsequently disabled.
	if k.assertedMacros.HasMacros() {
		if err := k.expandPendingMacros(); err != nil {
			return false, err
		}
	}
	if !k.preprocess {
		return true, nil
	}
	if k.assertedMacros.HasMacros() {
		// reduce deliberately ignores the first invoke's boolean result.
		if _, _, err := k.invokeAssertedPass(assertedMacros); err != nil {
			return false, err
		}
	}
	if err := k.traceAssertedState("before_reduce", "reduce", 238, false); err != nil {
		return false, err
	}
	if err := k.checkAssertedWellSorted(239); err != nil {
		return false, err
	}
	if err := k.setAssertedEliminateAnd(false); err != nil {
		return false, err
	}
	for index, pass := range [...]assertedPass{
		assertedPropagate, assertedMacros, assertedNNF, assertedReduce,
		assertedPullQuantifiers, assertedLiftITE, assertedLiftNonGroundITE,
		assertedEliminateITE, assertedInjectivity, assertedDistribute,
		assertedMacros, assertedQuasiMacros, assertedBit2Int, assertedBounds,
		assertedPatterns, assertedBVSharing, assertedReduce,
	} {
		if index == 3 {
			if err := k.setAssertedEliminateAnd(true); err != nil {
				return false, err
			}
		}
		proceed, canceled, err := k.invokeAssertedPass(pass)
		if err != nil || !proceed {
			return !canceled && err == nil, err
		}
	}
	if err := assertedVerbose(10, "(smt.simplifier-done)\n"); err != nil {
		return false, err
	}
	if err := k.traceAssertedAfterReduce("reduce", 263, 264); err != nil {
		return false, err
	}
	if err := k.traceAssertedMacros(); err != nil {
		return false, err
	}
	if k.preprocessor != nil {
		if err := k.flushAssertedCache(); err != nil {
			return false, err
		}
	}
	if err := k.checkAssertedWellSorted(267); err != nil {
		return false, err
	}
	return true, nil
}
