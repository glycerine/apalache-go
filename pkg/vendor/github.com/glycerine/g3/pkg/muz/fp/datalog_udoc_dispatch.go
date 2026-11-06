package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) ensureUDocDatalog() (selected bool, resultErr error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	checkerName, checkerEnabled, err := e.datalogCheckRelationName()
	if err != nil {
		return false, err
	}
	checkedDoc := checkerEnabled && checkerName == "doc"
	docFavourite := e.datalogRelationFavourite == "doc" || checkedDoc
	if e.udocDatalog != nil {
		// Source relation_manager keeps the existing concrete relation owner
		// when favourite/checker parameters change. Its migrated facts must
		// not fall back to the finite table whose contents it already adopted.
		docFavourite = true
		checkedDoc = e.udocDatalog.factory.FormulaChecked()
	}
	requested := false
	for _, state := range e.predicates {
		requested = requested || len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
	}
	if !docFavourite && !requested {
		return false, nil
	}
	owner := e.udocDatalog
	if owner == nil {
		var err error
		owner, err = e.newUDocDatalogOwner(checkedDoc)
		if err != nil {
			return false, err
		}
		defer func() {
			if owner != nil && e.udocDatalog != owner {
				resultErr = errors.Join(resultErr, owner.close())
			}
		}()
	}
	if selected, err := e.selectUDocProducts(owner, docFavourite, checkerEnabled); err != nil || selected {
		return selected, err
	}
	any, other, finiteGuards := false, false, false
	finiteInputs := make(map[ast.ASTHandle]*udocFiniteInput)
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		supported, err := owner.factory.CanHandleSignature(state.sorts)
		if err != nil {
			return false, err
		}
		hasDoc := len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
		candidate := supported && !state.explicitFiniteRepresentation && state.external == nil && (hasDoc || docFavourite && len(state.requestedRepresentations) == 0 || e.auxiliary[d])
		any = any || candidate
		guard := udocFiniteNullary(state) && owner.seeds[d] == nil
		forward := !checkedDoc && supported && udocFiniteForwardingInput(state) && owner.seeds[d] == nil
		if forward {
			finiteInputs[d] = &udocFiniteInput{table: state.table, sorts: state.sorts, auxiliary: e.auxiliary[d]}
		}
		finiteGuards = finiteGuards || guard
		// The existing finite setter rejects an adopted UDoc owner. Keep that
		// boundary rather than allowing a second owner to mask its facts.
		changedOwner := udocFiniteNullary(state) && owner.seeds[d] != nil
		other = other || !candidate && !guard && !forward || changedOwner
	}
	if !any {
		return false, nil
	}
	if finiteGuards {
		allowed, err := e.udocNullaryGuardsOnly()
		if err != nil {
			return false, err
		}
		other = other || !allowed
	}
	if len(finiteInputs) != 0 {
		allowed, err := e.udocFiniteForwardingOnly(finiteInputs)
		if err != nil {
			return false, err
		}
		other = other || !allowed
	}
	if other {
		if checkedDoc {
			// Preserve existing mixed finite execution until the generic
			// checked doc/table product compiler is integrated. Pure doc
			// programs take the concrete checked factory above; mixed Bool
			// signatures with only doc nullaries use the zero-column DOC
			// specialization in the finite scope.
			if e.udocDatalog == nil {
				return false, nil
			}
			return true, ErrUDocDatalogFragment
		}
		// The source generic compiler may mix the favourite doc plugin with a
		// fallback relation plugin. The specialized Go UDoc executor cannot,
		// so leave an initially mixed program to the ordinary finite executor.
		// Once UDoc has adopted facts, falling back would silently abandon its
		// seed owners, so preserve the explicit unsupported-fragment result.
		if e.udocDatalog != nil {
			return true, ErrUDocDatalogFragment
		}
		return false, nil
	}
	explanations, err := e.datalogExplanationsEnabled()
	if err != nil {
		return true, err
	}
	if explanations {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		relationLevel, err := view.Bool("datalog.explanations_on_relation_level", false)
		err = errors.Join(err, view.Close())
		if err != nil || !relationLevel {
			return true, errors.Join(err, ErrUDocDatalogFragment)
		}
	}
	if checkerEnabled && !checkedDoc {
		return true, ErrUDocDatalogFragment
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrUDocDatalogFragment)
	}
	if explanations || checkerEnabled {
		if owner.borrowed {
			err = e.relations.BorrowRelationExplanationUDocFactory(owner.factory)
		} else {
			err = e.relations.SetRelationExplanationUDocFactory(owner.factory)
		}
		if err != nil {
			return true, err
		}
		owner.borrowed = true
	}
	e.udocDatalog = owner
	return true, owner.sync(e)
}

func (e *PropositionalEngine) closeUDocDatalog() error {
	if e.udocDatalog == nil {
		return nil
	}
	if err := e.udocDatalog.close(); err != nil {
		return err
	}
	e.udocDatalog = nil
	return nil
}

func (e *PropositionalEngine) udocAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if e.queryClone != nil && e.queryClone.udocDatalog != nil {
		return e.queryClone.udocDatalog.answer(declaration)
	}
	if e.udocDatalog == nil {
		return ast.ASTRef{}, ErrUDocDatalogFragment
	}
	return e.udocDatalog.answer(declaration)
}

// Query preparation and status are separate from positive-rule evaluation so
// errors never expose an earlier result through the current source snapshot.
func (e *PropositionalEngine) prepareUDocQuery() error {
	if err := e.udocDatalog.snapshot(); err != nil {
		return err
	}
	e.compiled = false
	e.datalogStats.Reset()
	return nil
}

func (e *PropositionalEngine) finishUDocQuery(declarations []ast.ASTHandle) (lbool.LBool, error) {
	return e.udocDatalog.status(declarations)
}

func (e *PropositionalEngine) queryUDocDatalog(declarations []ast.ASTHandle) (status lbool.LBool, resultErr error) {
	owner := e.udocDatalog
	if owner == nil {
		return lbool.Undef, ErrUDocDatalogFragment
	}
	if err := e.relations.ResetSaturatedMarks(); err != nil {
		return lbool.Undef, err
	}
	set := e.set
	var similarityScope *udocSimilarityQueryScope
	if enabled, threshold, err := e.udocSimilaritySettings(); err != nil {
		return lbool.Undef, err
	} else if enabled {
		outputs, err := e.set.OutputPredicates()
		if err != nil {
			return lbool.Undef, err
		}
		outputs = append(outputs, declarations...)
		productHeads := make(map[ast.ASTHandle]bool)
		if owner.products != nil && owner.products.program != nil {
			for declaration := range owner.products.program.groundProducts {
				productHeads[declaration] = true
			}
			for declaration, plan := range owner.products.program.rules {
				if plan.inputKind == 0 {
					productHeads[declaration] = true
				}
			}
		}
		program, err := rel.NewUDocSimilarityProgram(e.manager, e.relations, productHeads, e.rules, owner.factory, e.set, threshold, outputs)
		if err != nil {
			return lbool.Undef, err
		}
		if program != nil {
			set, err = program.RuleSet()
			if err != nil {
				return lbool.Undef, errors.Join(err, program.Close())
			}
			var productPlan *udocProductProgram
			if owner.products != nil {
				productPlan, err = e.planUDocProductsForSet(owner.factory, true, owner.products.program.sorts, set, program.SeedSorts(), similarityProductSeedSorts(program))
				if err != nil && !errors.Is(err, ErrUDocDatalogFragment) {
					return lbool.Undef, errors.Join(err, program.Close())
				}
				if err != nil || productPlan == nil {
					if closeErr := program.Close(); closeErr != nil {
						return lbool.Undef, errors.Join(err, closeErr)
					}
					program = nil
					set = e.set
				}
			}
			if program != nil {
				similarityScope, err = installUDocSimilaritySeeds(owner, program)
				if err != nil {
					return lbool.Undef, errors.Join(err, program.Close())
				}
				if productPlan != nil {
					similarityScope.previousProducts = owner.products.program
					owner.products.program = productPlan
				}
				defer func() { resultErr = errors.Join(resultErr, similarityScope.Close()) }()
			}
		}
	}
	if err := e.udocDatalog.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	if err := e.prepareUDocQuery(); err != nil {
		return lbool.Undef, err
	}
	if err := e.udocDatalog.saturate(set); err != nil {
		return lbool.Undef, err
	}
	return e.finishUDocQuery(declarations)
}
