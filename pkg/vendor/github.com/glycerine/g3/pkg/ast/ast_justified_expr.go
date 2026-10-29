package ast

// Source: src/ast/justified_expr.h.

import "errors"

type justifiedExprAssignment struct {
	formula ASTRef
	proof   ASTRef
	stage   uint8
	cleanup bool
}

// JustifiedExpr owns a formula and its optional proof. It is the concrete Go
// counterpart used by asserted-formula and macro pipelines.
type JustifiedExpr struct {
	self       *JustifiedExpr
	manager    *ASTManager
	formula    ASTRef
	proof      ASTRef
	assignment *justifiedExprAssignment
	closed     bool
}

func NewJustifiedExpr(m *ASTManager, formula, proof ASTHandle) (*JustifiedExpr, error) {
	if m == nil || formula.IsNull() {
		return nil, ErrASTHandle
	}
	kind, err := m.GetKind(formula)
	if err != nil {
		return nil, err
	}
	if kind != ASTApp && kind != ASTVar && kind != ASTQuantifier {
		return nil, ErrASTDomain
	}
	if !proof.IsNull() {
		yes, err := m.IsProof(proof)
		if err != nil {
			return nil, err
		}
		if !yes {
			return nil, ErrASTDomain
		}
	}
	f, err := m.NewRef(formula)
	if err != nil {
		return nil, err
	}
	p, err := m.NewRef(proof)
	if err != nil {
		cleanup := m.DelRef(&f)
		return nil, errors.Join(err, cleanup)
	}
	j := &JustifiedExpr{manager: m, formula: f, proof: p}
	j.self = j
	return j, nil
}

func (j *JustifiedExpr) open() error {
	if j == nil || j.self != j || j.manager == nil || j.closed {
		return ErrASTHandle
	}
	_, err := j.manager.ProofsEnabled()
	return err
}

func (j *JustifiedExpr) Formula() (ASTHandle, error) {
	if err := j.open(); err != nil {
		return ASTHandle{}, err
	}
	return j.manager.RefGet(j.formula)
}

func (j *JustifiedExpr) Proof() (ASTHandle, error) {
	if err := j.open(); err != nil {
		return ASTHandle{}, err
	}
	return j.manager.RefGet(j.proof)
}

func (j *JustifiedExpr) Copy() (*JustifiedExpr, error) {
	formula, err := j.Formula()
	if err != nil {
		return nil, err
	}
	proof, err := j.Proof()
	if err != nil {
		return nil, err
	}
	return NewJustifiedExpr(j.manager, formula, proof)
}

func (j *JustifiedExpr) resumeAssignment() error {
	a := j.assignment
	if a == nil {
		return nil
	}
	if a.cleanup {
		if a.proof != (ASTRef{}) {
			if err := j.manager.DelRef(&a.proof); err != nil {
				return err
			}
		}
		if a.formula != (ASTRef{}) {
			if err := j.manager.DelRef(&a.formula); err != nil {
				return err
			}
		}
		j.assignment = nil
		return nil
	}
	if a.stage == 0 {
		if err := j.manager.DelRef(&j.formula); err != nil {
			return err
		}
		a.stage = 1
	}
	if a.stage == 1 {
		if err := j.manager.DelRef(&j.proof); err != nil {
			return err
		}
		a.stage = 2
	}
	j.formula, a.formula = a.formula, ASTRef{}
	j.proof, a.proof = a.proof, ASTRef{}
	j.assignment = nil
	return nil
}

// Assign follows the source order: acquire both incoming references, release
// the old formula and proof, then publish the new pair. A stopped release can
// be resumed by calling Assign again.
func (j *JustifiedExpr) Assign(other *JustifiedExpr) error {
	if err := j.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if j.manager != other.manager {
		return ErrASTDomain
	}
	if j == other {
		return nil
	}
	if j.assignment != nil {
		return j.resumeAssignment()
	}
	formula, err := other.Formula()
	if err != nil {
		return err
	}
	proof, err := other.Proof()
	if err != nil {
		return err
	}
	f, err := j.manager.NewRef(formula)
	if err != nil {
		return err
	}
	p, err := j.manager.NewRef(proof)
	if err != nil {
		a := &justifiedExprAssignment{formula: f, cleanup: true}
		j.assignment = a
		cleanup := j.resumeAssignment()
		return errors.Join(err, cleanup)
	}
	j.assignment = &justifiedExprAssignment{formula: f, proof: p}
	return j.resumeAssignment()
}

func (j *JustifiedExpr) Close() error {
	if j == nil || j.self != j {
		return ErrASTHandle
	}
	if j.closed {
		return nil
	}
	if a := j.assignment; a != nil {
		a.cleanup = true
		if err := j.resumeAssignment(); err != nil {
			return err
		}
	}
	if j.formula != (ASTRef{}) {
		if err := j.manager.DelRef(&j.formula); err != nil {
			return err
		}
	}
	if j.proof != (ASTRef{}) {
		if err := j.manager.DelRef(&j.proof); err != nil {
			return err
		}
	}
	j.closed = true
	return nil
}
