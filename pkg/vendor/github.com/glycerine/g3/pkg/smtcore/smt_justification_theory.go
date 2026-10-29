package smtcore

// Source: ownership and dependency portions of the theory justifications in
// src/smt/smt_justification.h and src/smt/smt_justification.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type SMTJustificationProofWrapper struct {
	SMTJustificationBase
	manager  *ast.ASTManager
	proof    ast.ASTRef
	released bool
}

func NewSMTJustificationProofWrapper(manager *ast.ASTManager, proof ast.ASTHandle, region bool) (*SMTJustificationProofWrapper, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	ref, err := manager.NewRef(proof)
	if err != nil {
		return nil, err
	}
	return &SMTJustificationProofWrapper{SMTJustificationBase: NewSMTJustificationBase(region), manager: manager, proof: ref}, nil
}
func (*SMTJustificationProofWrapper) Name() string { return "proof-wrapper" }
func (j *SMTJustificationProofWrapper) Proof() (ast.ASTHandle, error) {
	if j == nil || j.manager == nil || j.released {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return j.manager.RefGet(j.proof)
}
func (j *SMTJustificationProofWrapper) Release(manager *ast.ASTManager) error {
	if j == nil || j.manager == nil || manager != j.manager {
		return ast.ErrASTHandle
	}
	if j.released {
		return nil
	}
	if err := manager.DelRef(&j.proof); err != nil {
		return err
	}
	j.released = true
	return nil
}

type smtTheoryParameters struct {
	manager *ast.ASTManager
	values  []ast.ASTParameter
}

func newSMTTheoryParameters(manager *ast.ASTManager, parameters []ast.ASTParameter) (smtTheoryParameters, error) {
	if manager == nil {
		return smtTheoryParameters{}, ast.ErrASTDomain
	}
	result := smtTheoryParameters{manager: manager}
	for _, parameter := range parameters {
		copy, err := manager.CopyParameter(parameter)
		if err != nil {
			return smtTheoryParameters{}, errors.Join(err, result.release())
		}
		result.values = append(result.values, copy)
	}
	return result, nil
}
func (p *smtTheoryParameters) release() error {
	if p == nil || p.manager == nil {
		return nil
	}
	for len(p.values) != 0 {
		i := len(p.values) - 1
		if err := p.manager.DelParameter(&p.values[i]); err != nil {
			return err
		}
		p.values = p.values[:i]
	}
	return nil
}
func (p *smtTheoryParameters) Parameters() []ast.ASTParameter {
	if p == nil {
		return nil
	}
	return append([]ast.ASTParameter(nil), p.values...)
}

type SMTSimpleTheoryJustification struct {
	SMTSimpleJustification
	theory     SMTTheoryID
	parameters smtTheoryParameters
}

func NewSMTSimpleTheoryJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, parameters []ast.ASTParameter) (*SMTSimpleTheoryJustification, error) {
	simple, err := NewSMTSimpleJustification(true, literals)
	if err != nil {
		return nil, err
	}
	owned, err := newSMTTheoryParameters(manager, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTSimpleTheoryJustification{SMTSimpleJustification: *simple, theory: theory, parameters: owned}, nil
}
func (j *SMTSimpleTheoryJustification) FromTheory() SMTTheoryID { return j.theory }
func (j *SMTSimpleTheoryJustification) Parameters() []ast.ASTParameter {
	return j.parameters.Parameters()
}
func (j *SMTSimpleTheoryJustification) Release(manager *ast.ASTManager) error {
	if manager != j.parameters.manager {
		return ast.ErrASTHandle
	}
	return j.parameters.release()
}

type SMTTheoryAxiomJustification struct{ SMTSimpleTheoryJustification }

func NewSMTTheoryAxiomJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, parameters []ast.ASTParameter) (*SMTTheoryAxiomJustification, error) {
	base, err := NewSMTSimpleTheoryJustification(manager, theory, literals, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTTheoryAxiomJustification{*base}, nil
}
func (*SMTTheoryAxiomJustification) Name() string                              { return "theory-axiom" }
func (*SMTTheoryAxiomJustification) MarkAntecedents(SMTAntecedentMarker) error { return nil }

type SMTTheoryPropagationJustification struct {
	SMTSimpleTheoryJustification
	Consequent SMTLiteral
}

func NewSMTTheoryPropagationJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, consequent SMTLiteral, parameters []ast.ASTParameter) (*SMTTheoryPropagationJustification, error) {
	base, err := NewSMTSimpleTheoryJustification(manager, theory, literals, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTTheoryPropagationJustification{*base, consequent}, nil
}
func (*SMTTheoryPropagationJustification) Name() string { return "theory-propagation" }

type SMTTheoryConflictJustification struct{ SMTSimpleTheoryJustification }

func NewSMTTheoryConflictJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, parameters []ast.ASTParameter) (*SMTTheoryConflictJustification, error) {
	base, err := NewSMTSimpleTheoryJustification(manager, theory, literals, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTTheoryConflictJustification{*base}, nil
}
func (*SMTTheoryConflictJustification) Name() string { return "theory-conflict" }

type SMTExtTheoryJustification struct {
	SMTExtSimpleJustification
	theory     SMTTheoryID
	parameters smtTheoryParameters
}

func NewSMTExtTheoryJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, equalities []SMTEnodePair, parameters []ast.ASTParameter) (*SMTExtTheoryJustification, error) {
	extended, err := NewSMTExtSimpleJustification(true, literals, equalities)
	if err != nil {
		return nil, err
	}
	owned, err := newSMTTheoryParameters(manager, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTExtTheoryJustification{SMTExtSimpleJustification: *extended, theory: theory, parameters: owned}, nil
}
func (j *SMTExtTheoryJustification) FromTheory() SMTTheoryID        { return j.theory }
func (j *SMTExtTheoryJustification) Parameters() []ast.ASTParameter { return j.parameters.Parameters() }
func (j *SMTExtTheoryJustification) Release(manager *ast.ASTManager) error {
	if manager != j.parameters.manager {
		return ast.ErrASTHandle
	}
	return j.parameters.release()
}

type SMTExtTheoryPropagationJustification struct {
	SMTExtTheoryJustification
	Consequent    SMTLiteral
	returnToOwner bool
}

func NewSMTExtTheoryPropagationJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, equalities []SMTEnodePair, consequent SMTLiteral, parameters []ast.ASTParameter) (*SMTExtTheoryPropagationJustification, error) {
	base, err := NewSMTExtTheoryJustification(manager, theory, literals, equalities, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTExtTheoryPropagationJustification{SMTExtTheoryJustification: *base, Consequent: consequent}, nil
}

func (j *SMTExtTheoryPropagationJustification) FromTheory() SMTTheoryID {
	if j.returnToOwner {
		return SMTNullTheoryID
	}
	return j.theory
}
func (j *SMTExtTheoryPropagationJustification) Name() string { return "ext-theory-propagation" }

type SMTExtTheoryConflictJustification struct{ SMTExtTheoryJustification }

func NewSMTExtTheoryConflictJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, equalities []SMTEnodePair, parameters []ast.ASTParameter) (*SMTExtTheoryConflictJustification, error) {
	base, err := NewSMTExtTheoryJustification(manager, theory, literals, equalities, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTExtTheoryConflictJustification{*base}, nil
}
func (j *SMTExtTheoryConflictJustification) Name() string { return "ext-theory-conflict" }

type SMTExtTheoryEqualityPropagationJustification struct {
	SMTExtTheoryJustification
	Left, Right   *SMTEnode
	returnToOwner bool
}

func NewSMTExtTheoryEqualityPropagationJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, equalities []SMTEnodePair, left, right *SMTEnode, parameters []ast.ASTParameter) (*SMTExtTheoryEqualityPropagationJustification, error) {
	if left == nil || right == nil {
		return nil, ast.ErrASTDomain
	}
	base, err := NewSMTExtTheoryJustification(manager, theory, literals, equalities, parameters)
	if err != nil {
		return nil, err
	}
	return &SMTExtTheoryEqualityPropagationJustification{
		SMTExtTheoryJustification: *base, Left: left, Right: right,
	}, nil
}
func (j *SMTExtTheoryEqualityPropagationJustification) FromTheory() SMTTheoryID {
	if j.returnToOwner {
		return SMTNullTheoryID
	}
	return j.theory
}
func (j *SMTExtTheoryEqualityPropagationJustification) Name() string {
	return "ext-theory-eq-propagation"
}

type SMTTheoryLemmaJustification struct {
	SMTJustificationBase
	theory      SMTTheoryID
	parameters  smtTheoryParameters
	expressions []ast.ASTRef
	signs       []bool
}

func NewSMTTheoryLemmaJustification(manager *ast.ASTManager, theory SMTTheoryID, literals []SMTLiteral, expressions []ast.ASTHandle, parameters []ast.ASTParameter) (*SMTTheoryLemmaJustification, error) {
	owned, err := newSMTTheoryParameters(manager, parameters)
	if err != nil {
		return nil, err
	}
	j := &SMTTheoryLemmaJustification{SMTJustificationBase: NewSMTJustificationBase(false), theory: theory, parameters: owned}
	for _, literal := range literals {
		expression, e := smtLiteralExpression(literal, expressions)
		if e != nil {
			return nil, errors.Join(e, j.Release(manager))
		}
		ref, e := manager.NewRef(expression)
		if e != nil {
			return nil, errors.Join(e, j.Release(manager))
		}
		j.expressions = append(j.expressions, ref)
		j.signs = append(j.signs, literal.Sign())
	}
	return j, nil
}
func (*SMTTheoryLemmaJustification) Name() string                              { return "theory-lemma" }
func (j *SMTTheoryLemmaJustification) FromTheory() SMTTheoryID                 { return j.theory }
func (*SMTTheoryLemmaJustification) MarkAntecedents(SMTAntecedentMarker) error { return nil }
func (j *SMTTheoryLemmaJustification) Release(manager *ast.ASTManager) error {
	if manager != j.parameters.manager {
		return ast.ErrASTHandle
	}
	for len(j.expressions) != 0 {
		i := len(j.expressions) - 1
		if err := manager.DelRef(&j.expressions[i]); err != nil {
			return err
		}
		j.expressions = j.expressions[:i]
		j.signs = j.signs[:i]
	}
	return j.parameters.release()
}
func (j *SMTTheoryLemmaJustification) Expressions() ([]ast.ASTHandle, []bool, error) {
	result := make([]ast.ASTHandle, len(j.expressions))
	for i := range result {
		value, err := j.parameters.manager.RefGet(j.expressions[i])
		if err != nil {
			return nil, nil, err
		}
		result[i] = value
	}
	return result, append([]bool(nil), j.signs...), nil
}
