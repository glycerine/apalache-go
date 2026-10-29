package smtcore

// Source: src/smt/theory_lra.cpp get_value/init_variable_values and model hooks.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lp"
)

// The generator's proto model owns the factory. The theory only borrows it.
type smtLRAModelOwner struct {
	generator *SMTModelGenerator
	factory   *protomodel.ArithmeticFactory
}

var _ SMTModelValueTheory = (*SMTLRATheory)(nil)
var _ SMTTheoryDirectValue = (*SMTLRATheory)(nil)

// GetTheoryValue preserves the source cache eligibility test; it does not
// initialize a model or read fresh LP values on behalf of the quick checker.
func (t *SMTLRATheory) GetTheoryValue(node *SMTEnode) (ast.ASTHandle, bool, error) {
	s, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if node == nil || node.manager != s.manager {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	if err = t.numeric.ready(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !t.IsAttachedToVariable(node) || !t.canGetModelValue(node.TheoryVar(t.FamilyID())) {
		return ast.ASTHandle{}, false, nil
	}
	value, err := t.cachedModelValue(node.TheoryVar(t.FamilyID()))
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sort, err := s.manager.GetSort(node.owner)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := s.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	expression, err := s.manager.ArithmeticNumeralRat(value, kind == 1)
	return expression, err == nil, err
}

func (t *SMTLRATheory) canGetModelValue(v SMTTheoryVar) bool {
	index, ok := t.numeric.index(v)
	return ok && (t.numeric.solver.IsTerm(index) || t.modelValues[index] != nil)
}

func (t *SMTLRATheory) cachedModelValue(v SMTTheoryVar) (value *big.Rat, err error) {
	if err = t.numeric.ready(); err != nil {
		return nil, err
	}
	if !t.canGetModelValue(v) {
		return new(big.Rat), nil
	}
	index, _ := t.numeric.index(v)
	if value := t.modelValues[index]; value != nil {
		return new(big.Rat).Set(value), nil
	}
	defer func() {
		if err != nil {
			t.numeric.stopped = true
		}
	}()
	term := t.numeric.solver.Terms[index-lp.LARTermStartIndex]
	value, err = t.numeric.rational(&term.Value)
	if err != nil {
		return nil, err
	}
	if t.modelValues == nil {
		t.modelValues = make(map[uint32]*big.Rat)
	}
	for column, coefficient := range term.Coefficients().All() {
		c, err := t.numeric.rational(coefficient)
		if err != nil {
			return nil, err
		}
		x := t.modelValues[column]
		if x == nil {
			x = new(big.Rat)
			t.modelValues[column] = x
		}
		value.Add(value, new(big.Rat).Mul(c, x))
	}
	t.modelValues[index] = new(big.Rat).Set(value)
	return value, nil
}

func (t *SMTLRATheory) initializeVariableValues() error {
	if err := t.numeric.ready(); err != nil {
		return err
	}
	if t.NumVariables() == 0 {
		return nil
	}
	columns, err := t.numeric.modelColumnValues()
	if err != nil {
		return err
	}
	if t.modelValues == nil {
		t.modelValues = make(map[uint32]*big.Rat)
	}
	// Native GetModel updates the current extent without erasing older entries.
	for column, value := range columns {
		t.modelValues[column] = value
	}
	return nil
}

func (t *SMTLRATheory) InitializeModel(g *SMTModelGenerator) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if g == nil || g.manager != s.manager {
		return ast.ErrASTDomain
	}
	if err = g.open(); err != nil {
		return err
	}
	if g.model == nil {
		return ast.ErrASTDomain
	}
	if err = t.initializeVariableValues(); err != nil {
		return err
	}
	factory, err := protomodel.NewArithmeticFactory(s.manager)
	if err != nil {
		return err
	}
	if err = g.RegisterFactory(factory); err != nil {
		return errors.Join(err, factory.Close())
	}
	t.modelOwner = &smtLRAModelOwner{g, factory}
	return t.emitLRA("init_model", 2258, t.Display)
}

func (t *SMTLRATheory) ModelValue(node *SMTEnode, g *SMTModelGenerator) (SMTModelValueProcedure, error) {
	s, err := t.State()
	if err != nil {
		return nil, err
	}
	if node == nil || g == nil || g.manager != s.manager || !t.IsAttachedToVariable(node) || t.modelOwner == nil || t.modelOwner.generator != g {
		return nil, ast.ErrASTDomain
	}
	if err = g.open(); err != nil {
		return nil, err
	}
	if g.model == nil || g.model.Factory(t.FamilyID()) != t.modelOwner.factory {
		return nil, ast.ErrASTDomain
	}
	value, err := t.cachedModelValue(node.TheoryVar(t.FamilyID()))
	if err != nil {
		return nil, err
	}
	sort, err := s.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}
	kind, err := s.manager.GetDeclKind(sort)
	if err != nil {
		return nil, err
	}
	// The native LRA factory delegates directly to arith mk_numeral, which
	// rejects a fractional Int. It does not floor an incomplete assignment.
	expression, err := t.modelOwner.factory.MkNumValue(value, kind == 1)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(expression)
}
