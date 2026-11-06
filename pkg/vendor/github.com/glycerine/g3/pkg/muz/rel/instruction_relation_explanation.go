package rel

// Source: dl_instruction.cpp delegates each concrete operation to the
// registered product/sieve plugins used for relation-level explanations.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
)

type relationExplanationInstruction struct{ Instruction }

func relationExplanationInstructionFor(instruction Instruction) (Instruction, error) {
	switch instruction.(type) {
	case *registerInstruction, *relationIOInstruction, *explanationFiniteLoadInstruction, *constructInstruction, *tableInstruction, *interpretedFilterInstruction, *explanationNegationInstruction:
		return &relationExplanationInstruction{instruction}, nil
	default:
		return nil, ErrCompilerUnsupported
	}
}

func setRelationExplanationResult(context *ExecutionContext, target uint32, product *RelationExplanationProduct) (bool, error) {
	empty, err := product.Empty()
	if err != nil {
		return false, errors.Join(err, product.Close())
	}
	if empty {
		return true, errors.Join(product.Close(), context.MakeEmpty(target))
	}
	if err := context.SetRelationExplanationRegister(target, product); err != nil {
		return false, errors.Join(err, product.Close())
	}
	return true, nil
}

func finishRelationExplanationMutation(context *ExecutionContext, target uint32, product *RelationExplanationProduct, resultErr error) (bool, error) {
	if resultErr != nil {
		return false, resultErr
	}
	empty, err := product.Empty()
	if err != nil {
		return false, err
	}
	if empty {
		return true, context.MakeEmpty(target)
	}
	return true, nil
}

func (i *relationExplanationInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := context.open(); err != nil {
		return false, err
	}
	switch instruction := i.Instruction.(type) {
	case *relationIOInstruction:
		return performRelationExplanationIO(instruction, context)
	case *explanationFiniteLoadInstruction:
		return performRelationExplanationFiniteLoad(instruction, context)
	case *constructInstruction:
		complete, err := performRelationExplanationConstruct(instruction, context)
		if err != nil {
			err = fmt.Errorf("relation explanation construct %s: %w", instruction.kind, err)
		}
		return complete, err
	case *tableInstruction:
		complete, err := performRelationExplanationTable(instruction, context)
		if err != nil {
			err = fmt.Errorf("relation explanation table %s: %w", instruction.kind, err)
		}
		return complete, err
	case *interpretedFilterInstruction:
		if err := instruction.open(); err != nil {
			return false, err
		}
		product, err := context.RelationExplanationRegister(instruction.register)
		if err != nil || product == nil {
			return err == nil, err
		}
		conditions, err := instruction.conditionHandles()
		if err != nil {
			return false, err
		}
		context.stats.Filter++
		err = product.FilterInterpreted(instruction.rewriter, conditions, instruction.variableColumns, instruction.variableSorts)
		if err != nil {
			err = fmt.Errorf("relation explanation interpreted filter: %w", err)
		}
		return finishRelationExplanationMutation(context, instruction.register, product, err)
	case *explanationNegationInstruction:
		if err := instruction.open(); err != nil {
			return false, err
		}
		product, err := context.RelationExplanationRegister(instruction.first)
		if err != nil || product == nil {
			return err == nil, err
		}
		negative, err := context.RelationExplanationRegister(instruction.second)
		if err != nil || negative == nil {
			return err == nil, err
		}
		context.stats.FilterByNegation++
		err = product.FilterByNegation(negative, instruction.left, instruction.right)
		return finishRelationExplanationMutation(context, instruction.first, product, err)
	case *registerInstruction:
		if err := instruction.open(); err != nil {
			return false, err
		}
		if instruction.kind == "dealloc" {
			return true, context.MakeEmpty(instruction.source)
		}
		// Relation-level programs also keep proof-free magic and auxiliary
		// predicates in ordinary finite registers. Their register moves retain
		// the concrete finite representation and must use the base instruction.
		if !hasRelationExplanationRegister(context, instruction.source) {
			return instruction.Perform(context)
		}
		product, err := context.RelationExplanationRegister(instruction.source)
		if err != nil {
			return false, fmt.Errorf("relation explanation register %s source %d: %w", instruction.kind, instruction.source, err)
		}
		if product == nil {
			return true, context.MakeEmpty(instruction.target)
		}
		if instruction.kind == "clone" {
			clone, err := product.Clone()
			if err != nil {
				return false, err
			}
			return setRelationExplanationResult(context, instruction.target, clone)
		}
		if instruction.kind == "move" {
			product, err = context.ReleaseRelationExplanationRegister(instruction.source)
			if err != nil {
				return false, fmt.Errorf("relation explanation register move release %d: %w", instruction.source, err)
			}
			if err := context.SetRelationExplanationRegister(instruction.target, product); err != nil {
				return false, errors.Join(err, context.SetRelationExplanationRegister(instruction.source, product))
			}
			return true, nil
		}
	}
	return false, ErrInstruction
}

func performRelationExplanationIO(i *relationIOInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if !i.explanation {
		return i.Perform(context)
	}
	relations, err := i.contextRelations(context)
	if err != nil {
		return false, err
	}
	if !i.store {
		product, err := relations.GetRelationExplanation(i.predicate)
		if err != nil {
			return false, err
		}
		empty, err := product.Empty()
		if err != nil {
			return false, err
		}
		if empty {
			checked, checkErr := product.FormulaChecked()
			if checkErr != nil {
				return false, checkErr
			}
			if checked {
				clone, cloneErr := product.Clone()
				if cloneErr != nil {
					return false, cloneErr
				}
				if setErr := context.SetRelationExplanationRegister(i.register, clone); setErr != nil {
					return false, errors.Join(setErr, clone.Close())
				}
				return true, nil
			}
			return true, context.MakeEmpty(i.register)
		}
		clone, err := product.Clone()
		if err != nil {
			return false, err
		}
		return setRelationExplanationResult(context, i.register, clone)
	}
	product, err := context.RelationExplanationRegister(i.register)
	if err != nil {
		return false, err
	}
	transferred := product != nil
	if transferred {
		product, err = context.ReleaseRelationExplanationRegister(i.register)
	} else {
		sorts, _, _, signatureErr := relations.relationExplanationPredicateSignature(i.predicate)
		if signatureErr != nil {
			return false, signatureErr
		}
		product, err = NewRelationExplanationProduct(relations, sorts)
	}
	if err != nil {
		return false, err
	}
	if err = relations.StoreRelationExplanation(i.predicate, product); err != nil {
		if transferred {
			return false, errors.Join(err, context.SetRelationExplanationRegister(i.register, product))
		}
		return false, errors.Join(err, product.Close())
	}
	return true, nil
}

func performRelationExplanationFiniteLoad(i *explanationFiniteLoadInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	relations, err := i.contextRelations(context)
	if err != nil {
		return false, err
	}
	sorts, _, err := relations.PredicateSignature(i.predicate)
	if err != nil {
		return false, err
	}
	var table FiniteTable
	if i.source == VoidRegister {
		table, err = relations.GetFiniteRelation(i.predicate)
	} else {
		table, err = context.FiniteRegister(i.source)
	}
	if err != nil {
		return false, err
	}
	if table == nil {
		return true, context.MakeEmpty(i.register)
	}
	rows, err := table.Len()
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return true, context.MakeEmpty(i.register)
	}
	product, err := RelationExplanationFromFinite(relations, sorts, table)
	if err != nil {
		return false, err
	}
	return setRelationExplanationResult(context, i.register, product)
}

func performRelationExplanationConstruct(i *constructInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if context.relations != nil && context.relations != i.relations {
		return false, ErrInstruction
	}
	if i.kind == "mk_explanation_total" {
		context.stats.Total++
		product, err := NewRelationExplanationTotal(i.relations, i.explanations)
		if err != nil {
			return false, err
		}
		return setRelationExplanationResult(context, i.target, product)
	}
	if i.kind == "mk_explanation_numeric_column" {
		sort, err := i.finiteManager.RefGet(i.finiteSort)
		if err != nil {
			return false, err
		}
		product, err := newNumericRelationExplanationProductWithProofColumns(i.relations, []ast.ASTHandle{sort}, i.relations.explanationNumericKind, i.relations.explanationNumericCheck, 0)
		if err != nil {
			return false, err
		}
		seed := RelationExplanationNumericSeed{Kind: product.numeric.kind}
		switch seed.Kind {
		case "bound_relation":
			seed.Bound = NewBoundRelation(1, false)
		case "interval_relation":
			seed.Interval = NewIntervalFull(1)
		case "karr_relation":
			seed.Karr = NewKarrFull(1, nil, product.numeric.karr.formulaChecker)
		default:
			return false, errors.Join(ErrInstruction, product.Close())
		}
		if err = product.UnionNumericSeed(seed); err == nil && !i.astValue.IsNull() {
			err = product.FilterEqualAST(i.astValue, 0)
		}
		if err != nil {
			return false, errors.Join(fmt.Errorf("numeric explanation column construction: %w", err), product.Close())
		}
		if err = product.explanation.AddFact(nil); err != nil {
			return false, errors.Join(err, product.Close())
		}
		context.stats.Total++
		return setRelationExplanationResult(context, i.target, product)
	}
	if i.kind != "mk_explanation_finite_total" && i.kind != "mk_explanation_finite_singleton" {
		return false, ErrCompilerUnsupported
	}
	sort, err := i.finiteManager.RefGet(i.finiteSort)
	if err != nil {
		return false, err
	}
	table := NewHashTable(i.signature)
	if i.kind == "mk_explanation_finite_total" {
		context.stats.Total++
		err = fillTotalTable(table, i.signature)
	} else {
		context.stats.UnarySingleton++
		_, err = table.Add(Fact{i.value})
	}
	if err != nil {
		return false, errors.Join(err, table.Close())
	}
	product, err := RelationExplanationFromFinite(i.relations, []ast.ASTHandle{sort}, table)
	err = errors.Join(err, table.Close())
	if err != nil {
		if product != nil {
			err = errors.Join(err, product.Close())
		}
		return false, err
	}
	return setRelationExplanationResult(context, i.target, product)
}

func performRelationExplanationTable(i *tableInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	source, err := context.RelationExplanationRegister(i.first)
	if err != nil {
		return false, err
	}
	if source == nil {
		switch i.kind {
		case "join", "project", "rename":
			return true, context.MakeEmpty(i.result)
		}
		return true, nil
	}
	switch i.kind {
	case "filter_equal":
		context.stats.FilterEqual++
		if source.numeric != nil {
			err = source.FilterEqualAST(i.astValue, i.column)
			if err != nil {
				err = fmt.Errorf("numeric relation explanation equality filter: %w", err)
			}
		} else {
			err = source.FilterEqual(i.value, i.column)
		}
		return finishRelationExplanationMutation(context, i.first, source, err)
	case "filter_identical":
		context.stats.FilterIdentical++
		err = source.FilterIdentical(i.left)
		return finishRelationExplanationMutation(context, i.first, source, err)
	case "join":
		context.stats.Join++
		right, err := context.RelationExplanationRegister(i.second)
		if err != nil {
			return false, err
		}
		if right == nil {
			return true, context.MakeEmpty(i.result)
		}
		product, err := source.Join(right, i.left, i.right)
		if err != nil {
			return false, fmt.Errorf("relation explanation join: %w", err)
		}
		return setRelationExplanationResult(context, i.result, product)
	case "project", "rename":
		context.stats.ProjectRename++
		var product *RelationExplanationProduct
		if i.kind == "project" {
			product, err = source.ProjectColumns(i.removed)
		} else {
			product, err = source.RenameFinite(i.left)
		}
		if err != nil {
			return false, fmt.Errorf("relation explanation %s: %w", i.kind, err)
		}
		return setRelationExplanationResult(context, i.result, product)
	case "union", "widen":
		context.stats.Union++
		proofSorts, err := source.explanation.signature()
		if err != nil {
			return false, err
		}
		if len(proofSorts) == 0 {
			return unionRelationExplanationIntoFinite(i, context, source)
		}
		target, err := context.RelationExplanationRegister(i.second)
		if err != nil {
			return false, err
		}
		if target == nil {
			target, err = source.EmptyLike()
			if err != nil {
				return false, err
			}
			if err = context.SetRelationExplanationRegister(i.second, target); err != nil {
				return false, errors.Join(err, target.Close())
			}
		}
		var delta *RelationExplanationProduct
		if i.result != VoidRegister {
			delta, err = context.RelationExplanationRegister(i.result)
			if err != nil {
				return false, err
			}
			if delta == nil {
				delta, err = source.EmptyLike()
				if err != nil {
					return false, err
				}
				if err = context.SetRelationExplanationRegister(i.result, delta); err != nil {
					return false, errors.Join(err, delta.Close())
				}
			}
		}
		err = target.Union(source, delta)
		if err != nil {
			err = fmt.Errorf("relation explanation union: %w", err)
		}
		return err == nil, err
	}
	return false, ErrCompilerUnsupported
}

func unionRelationExplanationIntoFinite(i *tableInstruction, context *ExecutionContext, source *RelationExplanationProduct) (bool, error) {
	target, err := context.FiniteRegister(i.second)
	if err != nil {
		return false, err
	}
	signature, err := source.table.Signature()
	if err != nil {
		return false, err
	}
	if target == nil {
		target, err = source.manager.NewDefaultFiniteTable(signature)
		if err != nil {
			return false, err
		}
		if err = context.SetFiniteRegister(i.second, target); err != nil {
			return false, errors.Join(err, target.Close())
		}
	}
	var delta FiniteTable
	if i.result != VoidRegister {
		delta, err = context.FiniteRegister(i.result)
		if err != nil {
			return false, err
		}
		if delta == nil {
			delta, err = source.manager.NewDefaultFiniteTable(signature)
			if err != nil {
				return false, err
			}
			if err = context.SetFiniteRegister(i.result, delta); err != nil {
				return false, errors.Join(err, delta.Close())
			}
		}
	}
	err = finiteTableUnion(target, source.table, delta)
	return err == nil, err
}
