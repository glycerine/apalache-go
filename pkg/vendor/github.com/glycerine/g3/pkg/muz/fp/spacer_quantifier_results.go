package fp

// Source: src/muz/transforms/dl_mk_quantifier_abstraction.cpp
// qa_model_converter reconstruction of abstracted array predicates.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (e *PropositionalEngine) spacerArraySort(sort ast.ASTHandle) (bool, error) {
	family, err := e.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	array, err := e.manager.ArrayFamilyID()
	return family == array, err
}

func (e *PropositionalEngine) spacerSelect(array ast.ASTHandle, indices []ast.ASTHandle) (ast.ASTHandle, error) {
	family, err := e.manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 1, len(indices)+1)
	arguments[0] = array
	arguments = append(arguments, indices...)
	return e.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, arguments)
}

func (e *PropositionalEngine) expandSpacerQuantifiedFunction(destination *model.ModelCore, original, derived ast.ASTHandle) (resultErr error) {
	oldArity, err := e.manager.GetArity(original)
	if err != nil {
		return err
	}
	if oldArity == 0 {
		return ast.ErrASTDomain
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

	var pinned []ast.ASTRef
	defer func() {
		for index := len(pinned) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&pinned[index]))
		}
	}()
	pin := func(handle ast.ASTHandle) (ast.ASTHandle, error) {
		reference, pinErr := e.manager.NewRef(handle)
		if pinErr != nil {
			return ast.ASTHandle{}, pinErr
		}
		pinned = append(pinned, reference)
		return handle, nil
	}

	originalArguments := make([]ast.ASTHandle, 0, oldArity)
	expandedArguments := make([]ast.ASTHandle, 0)
	boundIndices := make([]ast.ASTHandle, 0)
	for i := uint32(0); i < oldArity; i++ {
		sort, getErr := e.manager.GetDomain(original, i)
		if getErr != nil {
			return getErr
		}
		current, getErr := e.manager.MkFreshConst("qa-model-argument", sort)
		if getErr != nil {
			return getErr
		}
		if current, getErr = pin(current); getErr != nil {
			return getErr
		}
		originalArguments = append(originalArguments, current)
		for {
			array, arrayErr := e.spacerArraySort(sort)
			if arrayErr != nil {
				return arrayErr
			}
			if !array {
				break
			}
			arity, arrayErr := e.manager.ArrayArity(sort)
			if arrayErr != nil {
				return arrayErr
			}
			indices := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				indexSort, indexErr := e.manager.ArrayDomain(sort, j)
				if indexErr != nil {
					return indexErr
				}
				indices[j], indexErr = e.manager.MkFreshConst("qa-model-index", indexSort)
				if indexErr != nil {
					return indexErr
				}
				if indices[j], indexErr = pin(indices[j]); indexErr != nil {
					return indexErr
				}
				boundIndices = append(boundIndices, indices[j])
				expandedArguments = append(expandedArguments, indices[j])
			}
			current, arrayErr = e.spacerSelect(current, indices)
			if arrayErr != nil {
				return arrayErr
			}
			if current, arrayErr = pin(current); arrayErr != nil {
				return arrayErr
			}
			sort, arrayErr = e.manager.ArrayRange(sort)
			if arrayErr != nil {
				return arrayErr
			}
		}
		expandedArguments = append(expandedArguments, current)
	}
	newArity, err := e.manager.GetArity(derived)
	if err != nil {
		return err
	}
	if uint64(len(expandedArguments)) != uint64(newArity) {
		return ast.ErrASTDomain
	}

	source, err := destination.FunctionInterpretation(derived)
	if err != nil {
		return err
	}
	var body ast.ASTHandle
	if source == nil {
		body, err = e.manager.MkFalse()
	} else {
		body, err = source.Interpretation()
		if err == nil && body.IsNull() {
			err = fmt.Errorf("muz/fp: partial abstracted predicate interpretation")
		}
	}
	if err != nil {
		return err
	}
	if body, err = pin(body); err != nil {
		return err
	}

	substitute, err := astrewriter.NewVarSubstitution(e.manager, false)
	if substitute != nil {
		defer func() { resultErr = errors.Join(resultErr, substitute.Close()) }()
	}
	if err != nil {
		return err
	}
	substituted, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	pinned = append(pinned, substituted)
	if err = substitute.Substitute(body, expandedArguments, substituted); err != nil {
		return err
	}
	substitutedBody, err := e.manager.RefGet(substituted)
	if err != nil {
		return err
	}

	abstractor, err := ast.NewExprAbstractor(e.manager)
	if abstractor != nil {
		defer func() { resultErr = errors.Join(resultErr, abstractor.Close()) }()
	}
	if err != nil {
		return err
	}
	quantified, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	pinned = append(pinned, quantified)
	if err = abstractor.MkForall(boundIndices, substitutedBody, quantified); err != nil {
		return err
	}
	quantifiedBody, err := e.manager.RefGet(quantified)
	if err != nil {
		return err
	}

	replace, err := astrewriter.NewExprSafeReplace(e.manager)
	if replace != nil {
		defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	}
	if err != nil {
		return err
	}
	for i, argument := range originalArguments {
		sort, getErr := e.manager.GetSort(argument)
		if getErr != nil {
			return getErr
		}
		variable, getErr := e.manager.MkVar(uint32(i), sort)
		if getErr != nil {
			return getErr
		}
		if getErr = replace.Insert(argument, variable); getErr != nil {
			return getErr
		}
	}
	reconstructed, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	pinned = append(pinned, reconstructed)
	if err = replace.Apply(quantifiedBody, reconstructed); err != nil {
		return err
	}
	reconstructedBody, err := e.manager.RefGet(reconstructed)
	if err != nil {
		return err
	}
	if err = interpretation.SetElse(reconstructedBody); err != nil {
		return err
	}
	if err = destination.RegisterFunction(original, interpretation); err != nil {
		return err
	}
	transferred = true
	return destination.UnregisterDeclaration(derived)
}
