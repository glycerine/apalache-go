package smtcore

// Source: src/smt/theory_str.cpp add_theory_assumptions and
// validate_unsat_core, used by loop-avoiding concat arrangements.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTStringTheory) AddTheoryAssumptions(assumptions []ast.ASTHandle) ([]ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil || t.closed {
		if err == nil {
			err = ast.ErrASTHandle
		}
		return assumptions, err
	}
	m := state.manager
	sort, err := m.MkBoolSort()
	if err != nil {
		return assumptions, err
	}
	name := "!!TheoryStrOverlapAssumption!!!tmp" + strconv.FormatUint(uint64(t.freshID), 10)
	t.freshID++
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return assumptions, err
	}
	term, err := m.MkSequenceSkolem(symbol, nil, sort)
	if err != nil {
		return assumptions, err
	}
	if t.overlapAssumption == (ast.ASTRef{}) {
		t.overlapAssumption, err = m.NewRef(term)
	} else {
		err = m.SetRef(t.overlapAssumption, term)
	}
	if err != nil {
		return assumptions, err
	}
	negative, err := m.MkNot(term)
	if err != nil {
		return assumptions, err
	}
	return append(assumptions, negative), nil
}

func (t *SMTStringTheory) ValidateUnsatCore(core []ast.ASTHandle) (lbool.LBool, error) {
	state, err := t.State()
	if err != nil || t.closed || t.overlapAssumption == (ast.ASTRef{}) {
		if err == nil {
			err = ast.ErrASTHandle
		}
		return lbool.Undef, err
	}
	term, err := state.manager.RefGet(t.overlapAssumption)
	if err != nil {
		return lbool.Undef, err
	}
	target, err := state.manager.MkNot(term)
	if err != nil {
		return lbool.Undef, err
	}
	if err := state.Internalize(target, false); err != nil {
		return lbool.Undef, err
	}
	node := state.context.enodes[target]
	if node == nil {
		return lbool.Undef, ast.ErrASTDomain
	}
	for _, assumption := range core {
		// Native compares enode identity, not congruence roots. Core
		// expressions were internalized by the assumption search.
		if node == state.context.enodes[assumption] {
			return lbool.Undef, nil
		}
	}
	return lbool.False, nil
}
