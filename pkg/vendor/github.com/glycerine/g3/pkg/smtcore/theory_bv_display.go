package smtcore

// Source: src/smt/theory_bv.cpp:492-505,1603-1661.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTBitVectorTheory) fixedLiteralValue(variable SMTTheoryVar) (*big.Int, bool, error) {
	if variable < 0 || int(variable) >= len(t.bitLiterals) {
		return nil, false, ast.ErrASTDomain
	}
	value := new(big.Int)
	for index, bit := range t.bitLiterals[variable] {
		assigned, err := t.state.Assignment(bit)
		if err != nil {
			return nil, false, err
		}
		if assigned == lbool.Undef {
			return value, false, nil
		}
		if assigned == lbool.True {
			value.SetBit(value, index, 1)
		}
	}
	return value, true, nil
}

func (t *SMTBitVectorTheory) DisplayVariable(out io.Writer, variable SMTTheoryVar) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	root, err := t.privateRoot(variable)
	if err != nil {
		return err
	}
	rootNode, err := t.Enode(root)
	if err != nil {
		return err
	}
	id, err := node.OwnerID()
	if err != nil {
		return err
	}
	rootID, err := rootNode.OwnerID()
	if err != nil {
		return err
	}
	if err = smtWriteTheoryText(out, fmt.Sprintf("v%-4d #%-4d -> #%-4d, bits:", variable, id, rootID)); err != nil {
		return err
	}
	for _, literal := range t.bitLiterals[variable] {
		if err = smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err = literal.DisplayCompact(smtContextDisplayWriter{out}, s.manager, s.boolExpr); err != nil {
			return err
		}
	}
	value, fixed, err := t.fixedLiteralValue(variable)
	if err != nil {
		return err
	}
	if fixed {
		if err = smtWriteTheoryText(out, ", value: "+value.String()); err != nil {
			return err
		}
	}
	return smtWriteTheoryText(out, "\n")
}

func (t *SMTBitVectorTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if t.NumVariables() == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "Theory bv:\n"); err != nil {
		return err
	}
	for v := range t.variables {
		if err := t.DisplayVariable(out, SMTTheoryVar(v)); err != nil {
			return err
		}
	}
	return t.displayBitAtoms(out)
}
