package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp sliced-predicate forwarding
// and src/muz/transforms/dl_mk_slice.cpp result-variable expansion.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

func (e *PropositionalEngine) spacerPredicate(declaration ast.ASTHandle) ast.ASTHandle {
	return hornDerivedPredicate(declaration, e.spacerQuantified, e.spacerScaled, e.spacerMagic, e.spacerBitBlasted, e.spacerInstantiated, e.spacerPredicates)
}

func hornDerivedPredicate(declaration ast.ASTHandle, mappings ...map[ast.ASTHandle]ast.ASTHandle) ast.ASTHandle {
	for _, mapping := range mappings {
		if derived, found := mapping[declaration]; found {
			declaration = derived
		}
	}
	return declaration
}

func slicedPredicateHasRemovedColumns(slices map[ast.ASTHandle][]bool, declaration ast.ASTHandle) bool {
	for _, removed := range slices[declaration] {
		if removed {
			return true
		}
	}
	return false
}

func expandSlicedFormula(manager *ast.ASTManager, slices map[ast.ASTHandle][]bool, declaration ast.ASTHandle, source ast.ASTRef) (result ast.ASTRef, resultErr error) {
	removed, found := slices[declaration]
	if !found {
		return source, nil
	}
	attempt := &slicedFormulaAttempt{manager: manager, source: source}
	defer func() { result, resultErr = attempt.finish(resultErr) }()
	formula, err := manager.RefGet(source)
	if err != nil {
		return ast.ASTRef{}, err
	}
	bindings := make([]ast.ASTHandle, 0, len(removed))
	for index, sliced := range removed {
		if sliced {
			continue
		}
		sort, err := manager.GetDomain(declaration, uint32(index))
		if err != nil {
			return ast.ASTRef{}, err
		}
		// Source subst is an expr_ref_vector. Own each variable before the
		// next binding or substitution construction can fail.
		pin, err := manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTRef{}, err
		}
		attempt.bindings = append(attempt.bindings, pin)
		variable, err := manager.MkVar(uint32(index), sort)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = manager.SetRef(pin, variable); err != nil {
			return ast.ASTRef{}, err
		}
		bindings = append(bindings, variable)
	}
	attempt.substitute, err = astrewriter.NewVarSubstitution(manager, false)
	if err != nil {
		return ast.ASTRef{}, err
	}
	attempt.result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	err = attempt.substitute.Substitute(formula, bindings, attempt.result)
	return ast.ASTRef{}, err
}

func rejectSlicedPredicateProperty(slices map[ast.ASTHandle][]bool, declaration ast.ASTHandle) error {
	if slicedPredicateHasRemovedColumns(slices, declaration) {
		return fmt.Errorf("muz/fp: covers and invariants are incompatible with sliced predicate columns")
	}
	return nil
}

func expandSlicedHandle(manager *ast.ASTManager, slices map[ast.ASTHandle][]bool, declaration, source ast.ASTHandle) (ast.ASTRef, error) {
	reference, err := manager.NewRef(source)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return expandSlicedFormula(manager, slices, declaration, reference)
}

func expandSlicedFunction(manager *ast.ASTManager, slices map[ast.ASTHandle][]bool, destination *model.ModelCore, original, derived ast.ASTHandle) (resultErr error) {
	source, err := destination.FunctionInterpretation(derived)
	if err != nil {
		return err
	}
	if source == nil {
		return destination.UnregisterDeclaration(derived)
	}
	oldArity, err := manager.GetArity(original)
	if err != nil {
		return err
	}
	interpretation, err := model.NewFunctionInterpretation(manager, oldArity)
	if err != nil {
		return err
	}
	transferred := false
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, interpretation.Close())
		}
	}()
	var temporary []ast.ASTRef
	defer func() {
		for i := len(temporary) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&temporary[i]))
		}
	}()
	expandTerm := func(term ast.ASTHandle) (ast.ASTHandle, error) {
		reference, expandErr := expandSlicedHandle(manager, slices, original, term)
		if expandErr != nil {
			return ast.ASTHandle{}, expandErr
		}
		temporary = append(temporary, reference)
		return manager.RefGet(reference)
	}
	partial, err := source.IsPartial()
	if err != nil {
		return err
	}
	if !partial {
		fallback, getErr := source.Else()
		if getErr != nil {
			return getErr
		}
		fallback, getErr = expandTerm(fallback)
		if getErr != nil {
			return getErr
		}
		if err = interpretation.SetElse(fallback); err != nil {
			return err
		}
	}
	// The source converter copies graph entries independently of the fallback.
	// In particular, partial interpretations have no ITE expression to flatten,
	// so rebuilding only Interpretation() would silently discard their entries.
	removed := slices[original]
	compactIndex := uint32(0)
	for entryIndex := uint32(0); entryIndex < source.NumEntries(); entryIndex++ {
		entry, getErr := source.Entry(entryIndex)
		if getErr != nil {
			return getErr
		}
		arguments := make([]ast.ASTHandle, 0, oldArity)
		for column := uint32(0); column < oldArity; column++ {
			if int(column) < len(removed) && removed[column] {
				sort, sortErr := manager.GetDomain(original, column)
				if sortErr != nil {
					return sortErr
				}
				pin, pinErr := manager.NewRef(ast.ASTHandle{})
				if pinErr != nil {
					return pinErr
				}
				temporary = append(temporary, pin)
				variable, varErr := manager.MkVar(column, sort)
				if varErr != nil {
					return varErr
				}
				if pinErr = manager.SetRef(pin, variable); pinErr != nil {
					return pinErr
				}
				arguments = append(arguments, variable)
				continue
			}
			argument, argErr := entry.Argument(compactIndex)
			if argErr != nil {
				return argErr
			}
			argument, argErr = expandTerm(argument)
			if argErr != nil {
				return argErr
			}
			arguments = append(arguments, argument)
			compactIndex++
		}
		compactIndex = 0
		value, getErr := entry.Result()
		if getErr != nil {
			return getErr
		}
		value, getErr = expandTerm(value)
		if getErr != nil {
			return getErr
		}
		if err = interpretation.InsertNewEntry(arguments, value); err != nil {
			return err
		}
	}
	if err = destination.RegisterFunction(original, interpretation); err != nil {
		return err
	}
	transferred = true
	return destination.UnregisterDeclaration(derived)
}

func expandSlicedModel(manager *ast.ASTManager, predicates map[ast.ASTHandle]ast.ASTHandle, slices map[ast.ASTHandle][]bool, source *model.Model) (_ *model.Model, resultErr error) {
	if len(predicates) == 0 {
		return source, nil
	}
	if source == nil {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && source != nil {
			resultErr = errors.Join(resultErr, source.Close())
			source = nil
		}
	}()
	core := source.Core()
	for original, derived := range predicates {
		arity, err := manager.GetArity(derived)
		if err != nil {
			return nil, err
		}
		if arity != 0 {
			if err = expandSlicedFunction(manager, slices, core, original, derived); err != nil {
				return nil, err
			}
			continue
		}
		present, err := core.HasConstantInterpretation(derived)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		value, err := core.ConstantInterpretation(derived)
		if err != nil {
			return nil, err
		}
		oldArity, err := manager.GetArity(original)
		if err != nil {
			return nil, err
		}
		interpretation, err := model.NewFunctionInterpretation(manager, oldArity)
		if err != nil {
			return nil, err
		}
		if !value.IsNull() {
			err = interpretation.SetElse(value)
		}
		if err == nil {
			err = core.RegisterFunction(original, interpretation)
		}
		if err != nil {
			return nil, errors.Join(err, interpretation.Close())
		}
		if err = core.UnregisterDeclaration(derived); err != nil {
			return nil, err
		}
	}
	return source, nil
}

func (e *PropositionalEngine) spacerHasRemovedColumns(declaration ast.ASTHandle) bool {
	return slicedPredicateHasRemovedColumns(e.spacerSlices, declaration)
}

func (e *PropositionalEngine) expandSpacerFormula(declaration ast.ASTHandle, source ast.ASTRef) (ast.ASTRef, error) {
	return expandSlicedFormula(e.manager, e.spacerSlices, declaration, source)
}

func (e *PropositionalEngine) rejectSlicedProperty(declaration ast.ASTHandle) error {
	return rejectSlicedPredicateProperty(e.spacerSlices, declaration)
}

func (e *PropositionalEngine) expandSpacerHandle(declaration, source ast.ASTHandle) (ast.ASTRef, error) {
	return expandSlicedHandle(e.manager, e.spacerSlices, declaration, source)
}

func (e *PropositionalEngine) expandSpacerFunction(destination *model.ModelCore, original, derived ast.ASTHandle) error {
	return expandSlicedFunction(e.manager, e.spacerSlices, destination, original, derived)
}

func (e *PropositionalEngine) expandSpacerModel(source *model.Model) (_ *model.Model, resultErr error) {
	conversions := e.spacerConversions
	if len(conversions) == 0 {
		if len(e.spacerQuantified) != 0 {
			conversions = append(conversions, hornModelConversion{kind: hornConvertQuantifier, predicates: e.spacerQuantified})
		}
		if len(e.spacerScaled) != 0 {
			conversions = append(conversions, hornModelConversion{kind: hornConvertScale, predicates: e.spacerScaled})
		}
		if len(e.spacerKarr) != 0 {
			conversions = append(conversions, hornModelConversion{kind: hornConvertKarr, karr: e.spacerKarr})
		}
		if len(e.spacerBitBlasted) != 0 {
			conversions = append(conversions, hornModelConversion{kind: hornConvertBitBlast, predicates: e.spacerBitBlasted})
		}
		if len(e.spacerPredicates) != 0 {
			conversions = append(conversions, hornModelConversion{kind: hornConvertSlice, predicates: e.spacerPredicates, columns: e.spacerSlices})
		}
	}
	return e.expandHornModel(source, conversions)
}

func (e *PropositionalEngine) applyHornConeModelStep(destination *model.ModelCore, step transforms.ConeModelStep) (resultErr error) {
	if len(step) == 0 {
		return nil
	}
	cooperation, err := cooperate.NewCooperation(e.manager.Budget())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
	evaluator, err := model.NewModelEvaluator(destination, model.ModelEvaluatorConfig{
		Context: e.params, Globals: e.globals,
		Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: cooperation,
	})
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	if err = evaluator.SetExpandArrayEqualities(false); err != nil {
		return err
	}
	value, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&value)) }()
	for j := len(step) - 1; j >= 0; j-- {
		definition := step[j]
		formula, err := e.manager.RefGet(definition.Formula)
		if err != nil {
			return err
		}
		// extension_model_converter::operator() evaluates definitions before
		// registering them, including ground Boolean simplification.
		if err = evaluator.Evaluate(formula, value); err != nil {
			return err
		}
		formula, err = e.manager.RefGet(value)
		if err != nil {
			return err
		}
		arity, err := e.manager.GetArity(definition.Declaration)
		if err != nil {
			return err
		}
		if arity == 0 {
			if err = destination.RegisterConstant(definition.Declaration, formula); err != nil {
				return err
			}
			continue
		}
		interpretation, err := model.NewFunctionInterpretation(e.manager, arity)
		if err != nil {
			return err
		}
		if err = interpretation.SetElse(formula); err != nil {
			return errors.Join(err, interpretation.Close())
		}
		if err = destination.RegisterFunction(definition.Declaration, interpretation); err != nil {
			return errors.Join(err, interpretation.Close())
		}
	}
	return nil
}

func (e *PropositionalEngine) expandHornModel(source *model.Model, conversions []hornModelConversion) (_ *model.Model, resultErr error) {
	if len(conversions) == 0 {
		return source, nil
	}
	if source == nil {
		return nil, ast.ErrASTHandle
	}
	// Converters consume and may clear their local source on failure. Keep
	// the incoming owner independently until all conversion stages succeed.
	owner := &hornModelConversionOwner{manager: e.manager, model: source}
	defer func() {
		if resultErr != nil {
			resultErr = owner.fail(resultErr)
		}
	}()
	core := source.Core()
	for i := len(conversions) - 1; i >= 0; i-- {
		conversion := conversions[i]
		switch conversion.kind {
		case hornConvertSlice:
			converted, err := expandSlicedModel(e.manager, conversion.predicates, conversion.columns, source)
			if err != nil {
				source = nil
				return nil, err
			}
			source = converted
			core = source.Core()
		case hornConvertBitBlast:
			for original, derived := range conversion.predicates {
				if err := e.expandSpacerBitBlastedFunction(core, original, derived); err != nil {
					return nil, err
				}
			}
		case hornConvertScale:
			for original, derived := range conversion.predicates {
				if err := e.expandSpacerScaledFunction(core, original, derived); err != nil {
					return nil, err
				}
			}
		case hornConvertQuantifier:
			for original, derived := range conversion.predicates {
				if err := e.expandSpacerQuantifiedFunction(core, original, derived); err != nil {
					return nil, err
				}
			}
		case hornConvertKarr:
			if err := e.applyHornKarrInvariants(core, conversion.karr); err != nil {
				return nil, err
			}
		case hornConvertCone:
			if err := e.applyHornConeModelStep(core, conversion.cone); err != nil {
				return nil, err
			}
		case hornConvertInline:
			if err := e.applyHornInlineModelRules(source, conversion.inline); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("muz/fp: unknown Horn model conversion %d", conversion.kind)
		}
	}
	for declaration := range e.auxiliary {
		if err := core.UnregisterDeclaration(declaration); err != nil {
			return nil, err
		}
	}
	return source, nil
}

func (e *PropositionalEngine) expandHornCover(original, derived ast.ASTHandle, source ast.ASTRef, includeDerived bool, conversions []hornModelConversion) (result ast.ASTRef, resultErr error) {
	attempt := &hornCoverAttempt{manager: e.manager, source: source}
	defer func() { result, resultErr = attempt.finish(resultErr) }()
	formula, err := e.manager.RefGet(source)
	if err != nil {
		return ast.ASTRef{}, err
	}
	attempt.seed, err = model.NewModel(e.manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if includeDerived {
		arity, err := e.manager.GetArity(derived)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if arity == 0 {
			if err = attempt.seed.Core().RegisterConstant(derived, formula); err != nil {
				return ast.ASTRef{}, err
			}
		} else {
			attempt.interpretation, err = model.NewFunctionInterpretation(e.manager, arity)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if err = attempt.interpretation.SetElse(formula); err != nil {
				return ast.ASTRef{}, err
			}
			if err = attempt.seed.Core().RegisterFunction(derived, attempt.interpretation); err != nil {
				return ast.ASTRef{}, err
			}
			attempt.interpretation = nil
		}
	}
	// Converters currently mutate this model in place and consume it on errors.
	// Keep the original pointer as a cleanup root even if their cleanup refuses.
	attempt.expanded, err = e.expandHornModel(attempt.seed, conversions)
	if err != nil {
		return ast.ASTRef{}, err
	}
	originalArity, err := e.manager.GetArity(original)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if originalArity == 0 {
		present, err := attempt.expanded.Core().HasConstantInterpretation(original)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if present {
			formula, err = attempt.expanded.Core().ConstantInterpretation(original)
		} else if includeDerived {
			return ast.ASTRef{}, ast.ErrASTDomain
		} else {
			formula, err = e.manager.MkTrue()
		}
	} else {
		interpretation, getErr := attempt.expanded.Core().FunctionInterpretation(original)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		if interpretation == nil {
			if includeDerived {
				return ast.ASTRef{}, ast.ErrASTDomain
			}
			formula, err = e.manager.MkTrue()
		} else {
			formula, err = interpretation.Else()
			if err == nil && formula.IsNull() {
				formula, err = e.manager.MkFalse()
			}
		}
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	attempt.result, err = e.manager.NewRef(formula)
	return ast.ASTRef{}, err
}
