package smtcore

// Source: src/smt/asserted_formulas.cpp push_assertion().

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

// DisplayAssertedFormulas renders the asserted-formula manager view, including
// the first formula that has not yet been committed to the internalizer.
func (k *SMTKernel) DisplayAssertedFormulas(out io.Writer) error {
	if err := k.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	printer, err := ast.NewLegacySMTPrinter(k.manager)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "asserted formulas:\n"); err != nil {
		return err
	}
	for i, assertion := range k.assertions {
		if i == k.assertedHead {
			if _, err = io.WriteString(out, "[HEAD] ==>\n"); err != nil {
				return err
			}
		}
		if err = printer.DisplayAST(out, assertion.Formula, 0, nil); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	inconsistent := 0
	if k.assertedFormulasInconsistent() {
		inconsistent = 1
	}
	_, err = fmt.Fprintf(out, "inconsistent: %d\n", inconsistent)
	return err
}

func (k *SMTKernel) assertedFormulasInconsistent() bool {
	return k != nil && k.assertedInconsistent
}

// AssertionInconsistencyProof returns the borrowed proof attached to the first
// false asserted formula, matching asserted_formulas::get_inconsistency_proof.
func (k *SMTKernel) AssertionInconsistencyProof() (ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	for _, assertion := range k.assertions {
		if k.manager.IsFalse(assertion.Formula) {
			return assertion.Proof, nil
		}
	}
	if k.assertedInconsistent {
		if err := k.assertedSourceAssertion(610, "UNREACHABLE", func() (bool, error) { return false, nil }); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return ast.ASTHandle{}, nil
}

func (k *SMTKernel) appendAssertion(formula, proof ast.ASTHandle) error {
	if err := k.manager.IncRef(formula); err != nil {
		return err
	}
	if !proof.IsNull() {
		if err := k.manager.IncRef(proof); err != nil {
			return errors.Join(err, k.manager.DecRef(formula))
		}
	}
	k.assertions = append(k.assertions, SMTKernelAssertion{Formula: formula, Proof: proof})
	return nil
}

func (k *SMTKernel) pushAssertion(formula, proof ast.ASTHandle) error {
	return k.pushAssertionInto(formula, proof, nil)
}

// A nil batch is the public assertion vector. Simplification uses a separate
// owning vector until the source cancellation check permits its swap.
func (k *SMTKernel) pushAssertionInto(formula, proof ast.ASTHandle, batch *macros.FormulaBatch) error {
	if k.assertedFormulasInconsistent() {
		return nil
	}
	if k.manager.IsTrue(formula) {
		return nil
	}
	op, basic, err := k.internalizer.basicOperation(formula)
	if err != nil {
		return err
	}
	if basic && op == ast.ASTOpAnd {
		count, err := k.manager.GetNumArgs(formula)
		if err != nil {
			return err
		}
		for index := uint32(0); index < count; index++ {
			child, err := k.manager.GetArg(formula, index)
			if err != nil {
				return err
			}
			childProof := ast.ASTHandle{}
			if !proof.IsNull() {
				childProof, err = k.manager.MkProofAndElim(proof, index)
				if err != nil {
					return err
				}
			}
			if err = k.pushAssertionInto(child, childProof, batch); err != nil {
				return fmt.Errorf("push conjunction child %d: %w", index, err)
			}
		}
		return nil
	}
	if basic && op == ast.ASTOpNot {
		child, err := k.manager.GetArg(formula, 0)
		if err != nil {
			return err
		}
		childOp, childBasic, err := k.internalizer.basicOperation(child)
		if err != nil {
			return err
		}
		if childBasic && childOp == ast.ASTOpOr {
			count, err := k.manager.GetNumArgs(child)
			if err != nil {
				return err
			}
			for index := uint32(0); index < count; index++ {
				argument, err := k.manager.GetArg(child, index)
				if err != nil {
					return err
				}
				childProof := ast.ASTHandle{}
				if !proof.IsNull() {
					childProof, err = k.manager.MkProofNotOrElim(proof, index)
					if err != nil {
						return err
					}
				}
				negated, err := k.manager.MkNotSimplified(argument)
				if err != nil {
					return err
				}
				if err = k.pushAssertionInto(negated, childProof, batch); err != nil {
					return fmt.Errorf("push negated disjunction child %d: %w", index, err)
				}
			}
			return nil
		}
	}
	if batch == nil {
		err = k.appendAssertion(formula, proof)
	} else {
		err = batch.Insert(macros.Formula{Expression: formula, Proof: proof})
	}
	if err == nil && k.manager.IsFalse(formula) {
		k.assertedInconsistent = true
	}
	return err
}

func (k *SMTKernel) preprocessAssertion(formula, proof ast.ASTHandle) (result, resultProof ast.ASTHandle, err error) {
	if !k.preprocess || k.preprocessor == nil {
		return formula, proof, nil
	}
	resultRef, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, k.manager.DelRef(&resultRef)) }()
	proofRef, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, k.manager.DelRef(&proofRef)) }()
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if proofs {
		err = k.preprocessor.RewriteWithProof(formula, resultRef, proofRef)
	} else {
		err = k.preprocessor.Rewrite(formula, resultRef)
	}
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	result, err = k.manager.RefGet(resultRef)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	resultProof = proof
	if proofs && result != formula {
		rewriteProof, getErr := k.manager.RefGet(proofRef)
		if getErr != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, getErr
		}
		resultProof, err = k.manager.MkProofModusPonens(proof, rewriteProof)
	}
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = k.manager.IncRef(result); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if !resultProof.IsNull() {
		if err = k.manager.IncRef(resultProof); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, errors.Join(err, k.manager.DecRef(result))
		}
	}
	return result, resultProof, nil
}
