package fp

// Source: src/muz/transforms/dl_mk_bit_blast.cpp
// bit_blast_model_converter predicate reconstruction.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (e *PropositionalEngine) expandSpacerBitBlastedFunction(destination *model.ModelCore, original, derived ast.ASTHandle) (resultErr error) {
	source, err := destination.FunctionInterpretation(derived)
	if err != nil {
		return err
	}
	if source == nil {
		return destination.UnregisterDeclaration(derived)
	}
	body, err := source.Interpretation()
	if err != nil {
		return err
	}
	if body.IsNull() {
		return fmt.Errorf("muz/fp: partial bit-blasted predicate interpretation")
	}
	oldArity, err := e.manager.GetArity(original)
	if err != nil {
		return err
	}
	bindings := make([]ast.ASTHandle, 0)
	pins := make([]ast.ASTRef, 0)
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&pins[i]))
		}
	}()
	pin := func(handle ast.ASTHandle) error {
		reference, pinErr := e.manager.NewRef(handle)
		if pinErr == nil {
			pins = append(pins, reference)
		}
		return pinErr
	}
	for i := uint32(0); i < oldArity; i++ {
		sort, getErr := e.manager.GetDomain(original, i)
		if getErr != nil {
			return getErr
		}
		argument, getErr := e.manager.MkVar(i, sort)
		if getErr != nil {
			return getErr
		}
		bitVector, getErr := e.manager.IsBitVectorSort(sort)
		if getErr != nil {
			return getErr
		}
		if !bitVector {
			bindings = append(bindings, argument)
			continue
		}
		width, getErr := e.manager.BitVectorSize(sort)
		if getErr != nil {
			return getErr
		}
		for bit := uint32(0); bit < width; bit++ {
			projection, projectionErr := e.manager.MkBitVectorBit2Bool(bit, argument)
			if projectionErr != nil {
				return projectionErr
			}
			if projectionErr = pin(projection); projectionErr != nil {
				return projectionErr
			}
			bindings = append(bindings, projection)
		}
	}
	newArity, err := e.manager.GetArity(derived)
	if err != nil {
		return err
	}
	if uint64(len(bindings)) != uint64(newArity) {
		return ast.ErrASTDomain
	}
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
	pins = append(pins, rewritten)
	if err = substitute.Substitute(body, bindings, rewritten); err != nil {
		return err
	}
	rewrittenBody, err := e.manager.RefGet(rewritten)
	if err != nil {
		return err
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
