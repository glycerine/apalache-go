package smtcore

// Source: src/smt/theory_seq.h stats and
// src/smt/theory_seq.cpp collect_statistics.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtSequenceStatistics struct {
	numSplits, numReductions, propagateAutomata uint32
	checkLengthCoherence, branchVariable        uint32
	solveDisequalities, solveEqualities         uint32
	addAxiom, extensionality, fixedLength       uint32
	propagateContains, integerString            uint32
}

func (t *SMTSequenceTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || t.axioms == nil || target == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	s := &t.statistics
	target.UpdateUint("seq num splits", s.numSplits)
	target.UpdateUint("seq num reductions", s.numReductions)
	target.UpdateUint("seq unfold def", s.propagateAutomata)
	target.UpdateUint("seq length coherence", s.checkLengthCoherence)
	target.UpdateUint("seq branch", s.branchVariable)
	target.UpdateUint("seq solve !=", s.solveDisequalities)
	target.UpdateUint("seq solve =", s.solveEqualities)
	target.UpdateUint("seq add axiom", s.addAxiom)
	target.UpdateUint("seq extensionality", s.extensionality)
	target.UpdateUint("seq fixed length", s.fixedLength)
	target.UpdateUint("seq int.to.str", s.integerString)
	return nil
}

func (t *SMTSequenceTheory) ResetStatistics() {
	if t != nil {
		t.statistics = smtSequenceStatistics{}
	}
}

func (t *SMTSequenceTheory) recordSequenceBranch(progress bool, err error) (bool, error) {
	if progress && err == nil {
		t.statistics.branchVariable++
		t.branchProgress = true
	}
	return progress, err
}
