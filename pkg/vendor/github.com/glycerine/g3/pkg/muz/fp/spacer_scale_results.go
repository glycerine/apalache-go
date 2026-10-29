package fp

// Source: src/muz/transforms/dl_mk_scale.cpp scale_model_converter.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (e *PropositionalEngine) expandSpacerScaledFunction(destination *model.ModelCore, original, derived ast.ASTHandle) (resultErr error) {
	source, err := destination.FunctionInterpretation(derived)
	if err != nil {
		return err
	}
	if source == nil {
		return destination.UnregisterDeclaration(derived)
	}
	if source.NumEntries() != 0 {
		return fmt.Errorf("muz/fp: partial scaled predicate interpretation")
	}
	body, err := source.Else()
	if err != nil {
		return err
	}
	if body.IsNull() {
		return fmt.Errorf("muz/fp: scaled predicate interpretation has no fallback")
	}
	oldArity, err := e.manager.GetArity(original)
	if err != nil {
		return err
	}
	bindings := make([]ast.ASTHandle, 0, oldArity+1)
	for i := uint32(0); i < oldArity; i++ {
		sort, getErr := e.manager.GetDomain(original, i)
		if getErr != nil {
			return getErr
		}
		argument, getErr := e.manager.MkVar(i, sort)
		if getErr != nil {
			return getErr
		}
		bindings = append(bindings, argument)
	}
	one, err := e.manager.ArithmeticRealNumeralText("1")
	if err != nil {
		return err
	}
	oneRef, err := e.manager.NewRef(one)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&oneRef)) }()
	bindings = append(bindings, one)
	substitute, err := astrewriter.NewVarSubstitution(e.manager, false)
	if substitute != nil {
		defer func() { resultErr = errors.Join(resultErr, substitute.Close()) }()
	}
	if err != nil {
		return err
	}
	rewritten, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&rewritten)) }()
	if err = substitute.Substitute(body, bindings, rewritten); err != nil {
		return err
	}
	rewrittenBody, err := e.manager.RefGet(rewritten)
	if err != nil {
		return err
	}
	if oldArity == 0 {
		if err = destination.RegisterConstant(original, rewrittenBody); err != nil {
			return err
		}
		return destination.UnregisterDeclaration(derived)
	}
	interpretation, err := model.NewFunctionInterpretation(e.manager, oldArity)
	if err != nil {
		return err
	}
	transferred := false
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, interpretation.Close())
		}
	}()
	if err = interpretation.SetElse(rewrittenBody); err != nil {
		return err
	}
	if err = destination.RegisterFunction(original, interpretation); err != nil {
		return err
	}
	transferred = true
	return destination.UnregisterDeclaration(derived)
}
