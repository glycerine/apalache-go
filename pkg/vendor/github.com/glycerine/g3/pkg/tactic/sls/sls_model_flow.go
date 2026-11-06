package sls

// Source: src/tactic/sls/sls_tracker.h:530-570 and
// src/tactic/sls/sls_evaluator.h:500-522.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrEvaluatorCheck = errors.New("sls: evaluator result disagrees with model evaluator")

// ValueAST ports mpz2value for the retained Boolean and BV sorts.
func (t *Tracker) ValueAST(sort ast.ASTHandle, value *numeral.Int) (node ast.ASTHandle, result error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if value == nil {
		return ast.ASTHandle{}, ErrTracker
	}
	boolSort, err := t.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort == boolSort {
		one, err := t.numbers.IsOne(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if one {
			return t.manager.MkTrue()
		}
		zero, err := t.numbers.IsZero(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !zero {
			return ast.ASTHandle{}, ErrTracker
		}
		return t.manager.MkFalse()
	}
	isBV, err := t.manager.IsBitVectorSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !isBV {
		return ast.ASTHandle{}, ErrEvaluatorUnsupported
	}
	bits, err := t.manager.BitVectorSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	service := t.manager.NumeralService()
	if service == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	rational, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var call numeral.RationalServiceInvocation
	defer func() {
		result = errors.Join(result, call.Cleanup(false))
		result = errors.Join(result, service.CleanupValue(&rational))
		result = errors.Join(result, call.Cleanup(true))
		result = errors.Join(result, call.EndContext())
		if result != nil {
			node = ast.ASTHandle{}
		}
	}()
	if err = service.SetIntegerFromOwnerWithInvocation(&rational, t.numbers, value, &call); err != nil {
		return ast.ASTHandle{}, err
	}
	return t.manager.BitVectorNumeral(&rational, bits)
}

// GetModel materializes the current values of the source constant list.
func (t *Tracker) GetModel() (answer *model.Model, result error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	answer, err := model.NewModel(t.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil && answer != nil {
			result = errors.Join(result, answer.Close())
			answer = nil
		}
	}()
	for _, decl := range t.constants {
		sort, err := t.manager.GetRange(decl)
		if err != nil {
			return nil, err
		}
		value, err := t.DeclValue(decl)
		if err != nil {
			return nil, err
		}
		node, err := t.ValueAST(sort, value)
		if err != nil {
			return nil, err
		}
		if err = answer.Core().RegisterConstant(decl, node); err != nil {
			return nil, err
		}
	}
	return answer, nil
}

// SetModel imports retained Boolean/BV constant interpretations that already
// have entry points in this tracker.
func (t *Tracker) SetModel(source *model.Model) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.Core().Manager() != t.manager {
		return ErrTracker
	}
	count, err := source.Core().NumConstants()
	if err != nil {
		return err
	}
	value := numeral.NewInt(0)
	defer func() { result = errors.Join(result, t.numbers.Del(&value)) }()
	for i := uint32(0); i < count; i++ {
		decl, err := source.Core().Constant(i)
		if err != nil {
			return err
		}
		if _, found := t.entryPoints[decl]; !found {
			continue
		}
		interpretation, err := source.Core().ConstantInterpretation(decl)
		if err != nil {
			return err
		}
		if interpretation.IsNull() {
			continue
		}
		if err = t.valueToInteger(interpretation, &value); err != nil {
			return err
		}
		if err = t.SetDeclValue(decl, &value); err != nil {
			return err
		}
	}
	return nil
}

// EvalChecked runs the SLS evaluator, substitutes each current child value,
// and compares against the already-ported model evaluator. Process services
// stay explicit because the Go port does not use C++ process globals.
func (e *Evaluator) EvalChecked(node ast.ASTHandle, out *numeral.Int, services model.ModelEvaluatorConfig) (result error) {
	if err := e.Evaluate(node, out); err != nil {
		return err
	}
	n, err := e.manager.GetNumArgs(node)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 0, n)
	refs := make([]ast.ASTRef, 0, n+2)
	defer func() {
		for i := len(refs); i > 0; i-- {
			result = errors.Join(result, e.manager.DelRef(&refs[i-1]))
		}
	}()
	for i := uint32(0); i < n; i++ {
		arg, x := e.manager.GetArg(node, i)
		if x != nil {
			return x
		}
		sort, x := e.manager.GetSort(arg)
		if x != nil {
			return x
		}
		value, x := e.tracker.GetValue(arg)
		if x != nil {
			return x
		}
		literal, x := e.tracker.ValueAST(sort, value)
		if x != nil {
			return x
		}
		ref, x := e.manager.NewRef(literal)
		if x != nil {
			return x
		}
		refs = append(refs, ref)
		args = append(args, literal)
	}
	decl, err := e.manager.GetDecl(node)
	if err != nil {
		return err
	}
	query, err := e.manager.MkApp(decl, args)
	if err != nil {
		return err
	}
	queryRef, err := e.manager.NewRef(query)
	if err != nil {
		return err
	}
	refs = append(refs, queryRef)
	dummy, err := model.NewModel(e.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, dummy.Close()) }()
	valueRef, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	refs = append(refs, valueRef)
	if err = dummy.Evaluate(query, valueRef, false, services); err != nil {
		return err
	}
	checkedNode, err := e.manager.RefGet(valueRef)
	if err != nil {
		return err
	}
	checked := numeral.NewInt(0)
	defer func() { result = errors.Join(result, e.numbers.Del(&checked)) }()
	if err = e.tracker.valueToInteger(checkedNode, &checked); err != nil {
		return err
	}
	equal, err := e.numbers.Equal(out, &checked)
	if err != nil {
		return err
	}
	if !equal {
		return ErrEvaluatorCheck
	}
	return nil
}
