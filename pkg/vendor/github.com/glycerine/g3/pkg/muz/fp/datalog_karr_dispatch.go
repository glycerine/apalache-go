package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
)

func (e *PropositionalEngine) ensureKarrDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	checkName, checkOn, checkErr := e.datalogCheckRelationName()
	if checkErr != nil {
		return false, checkErr
	}
	selected := e.datalogRelationFavourite == "karr_relation" || (checkOn && checkName == "karr_relation") || e.karrDatalog != nil
	for _, state := range e.predicates {
		for _, name := range state.requestedRepresentations {
			selected = selected || name == "karr_relation"
		}
	}
	if !selected {
		return false, nil
	}
	if e.udocDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil || e.boundIntervalDatalog != nil || e.externalIntervalDatalog != nil {
		return true, ErrKarrDatalogFragment
	}
	enabled := false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		enabled, err = view.Bool("xform.karr", false)
		err = errors.Join(err, view.Close())
		if err != nil {
			return true, err
		}
	}
	if !enabled {
		return true, ErrKarrDatalogFragment
	}
	for declaration, state := range e.predicates {
		karrSelectedByChecker := checkOn && checkName == "karr_relation"
		if e.datalogRelationFavourite != "karr_relation" && !karrSelectedByChecker && !e.auxiliary[declaration] && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != "karr_relation") {
			return true, ErrKarrDatalogFragment
		}
		if state.external != nil || state.explicitFiniteRepresentation {
			return true, ErrKarrDatalogFragment
		}
		if len(state.requestedRepresentations) > 0 && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != "karr_relation") {
			return true, ErrKarrDatalogFragment
		}
		if state.table != nil {
			n, err := state.table.Len()
			if err != nil {
				return true, err
			}
			if n != 0 {
				return true, ErrKarrDatalogFragment
			}
		}
	}
	if on, err := e.datalogExplanationsEnabled(); err != nil || on {
		return true, errors.Join(err, ErrKarrDatalogFragment)
	}
	if checkOn && checkName != "karr_relation" {
		return true, ErrKarrDatalogFragment
	}
	if _, on, err := e.datalogDefaultTableCheck(); err != nil || on {
		return true, errors.Join(err, ErrKarrDatalogFragment)
	}
	var formulaCheck *smtcore.SMTSolverConfig
	if checkOn {
		config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
		formulaCheck = &config
	}
	if e.karrDatalog != nil {
		if (e.karrDatalog.formulaChecker != nil) != checkOn {
			return true, ErrKarrDatalogFragment
		}
		return true, e.karrDatalog.syncPredicates(e.predicateOrder)
	}
	owner, err := newKarrDatalogActivation(e.manager, e.set, e.predicateOrder, formulaCheck)
	if err != nil {
		return true, err
	}
	e.karrDatalog = owner
	return true, nil
}

func (e *PropositionalEngine) queryKarrDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	e.compiled = false
	e.datalogStats.Reset()
	enabled, threshold := true, uint32(11)
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		enabled, err = view.Bool("datalog.similarity_compressor", true)
		if err == nil && enabled {
			threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	if enabled {
		return e.karrDatalog.queryWithSimilarity(declarations, threshold)
	}
	return e.karrDatalog.query(declarations)
}

func (e *PropositionalEngine) karrDatalogAnswer(declaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	a := e.karrDatalog
	if e.queryClone != nil && e.queryClone.karrDatalog != nil {
		a = e.queryClone.karrDatalog
	}
	if a == nil || a.results == nil || a.results[declaration] == nil {
		return result, ErrKarrDatalogFragment
	}
	if err := a.checkpoint(); err != nil {
		return result, err
	}
	result, resultErr = e.manager.NewRef(ast.ASTHandle{})
	if resultErr != nil {
		return result, resultErr
	}
	refs := []ast.ASTRef{}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&refs[i]))
		}
		if resultErr != nil {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	// Each output is rooted before the next AST operation can fail.
	makeNode := func(build func() (ast.ASTHandle, error)) (ast.ASTHandle, error) {
		ref, err := e.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		h, err := build()
		if !h.IsNull() {
			err = errors.Join(err, e.manager.SetRef(ref, h))
		}
		return h, err
	}
	// Conversion lazily changes the relation's representation. Work on a
	// private clone so an interrupted Hilbert conversion cannot poison the
	// published query snapshot.
	r := a.results[declaration].Clone()
	var formula ast.ASTHandle
	if r.Empty() {
		formula, resultErr = makeNode(e.manager.MkFalse)
	} else {
		matrix, err := r.Inequalities()
		if err != nil {
			return result, err
		}
		if err = a.checkpoint(); err != nil {
			return result, err
		}
		var zero ast.ASTHandle
		zero, resultErr = makeNode(func() (ast.ASTHandle, error) { return e.manager.ArithmeticIntegerNumeralText("0") })
		if resultErr != nil {
			return result, resultErr
		}
		conjuncts := make([]ast.ASTHandle, 0, matrix.Len())
		for i, row := range matrix.A {
			terms := make([]ast.ASTHandle, 0, len(row)+1)
			for column := range row {
				coefficient := &row[column]
				if coefficient.Sign() == 0 {
					continue
				}
				sort, err := e.manager.GetDomain(declaration, uint32(column))
				if err != nil {
					return result, err
				}
				term, err := makeNode(func() (ast.ASTHandle, error) { return e.manager.MkVar(uint32(column), sort) })
				if err != nil {
					return result, err
				}
				if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
					num, err := makeNode(func() (ast.ASTHandle, error) { return e.manager.ArithmeticNumeralRat(coefficient, true) })
					if err != nil {
						return result, err
					}
					old := term
					term, err = makeNode(func() (ast.ASTHandle, error) { return e.manager.MkArithmeticMul(num, old) })
					if err != nil {
						return result, err
					}
				}
				terms = append(terms, term)
			}
			if matrix.B[i].Sign() != 0 {
				term, err := makeNode(func() (ast.ASTHandle, error) { return e.manager.ArithmeticNumeralRat(&matrix.B[i], true) })
				if err != nil {
					return result, err
				}
				terms = append(terms, term)
			}
			lhs := zero
			for _, term := range terms {
				if lhs == zero && len(terms) > 0 {
					lhs = term
					continue
				}
				old := lhs
				lhs, err = makeNode(func() (ast.ASTHandle, error) { return e.manager.MkArithmeticAdd(old, term) })
				if err != nil {
					return result, err
				}
			}
			var cond ast.ASTHandle
			if matrix.Eq[i] {
				cond, err = makeNode(func() (ast.ASTHandle, error) { return e.manager.MkEq(lhs, zero) })
			} else {
				cond, err = makeNode(func() (ast.ASTHandle, error) { return e.manager.MkArithmeticGE(lhs, zero) })
			}
			if err != nil {
				return result, err
			}
			conjuncts = append(conjuncts, cond)
		}
		if len(conjuncts) == 1 {
			formula = conjuncts[0]
		} else {
			formula, resultErr = makeNode(func() (ast.ASTHandle, error) { return e.manager.MkAndArray(conjuncts) })
		}
	}
	if !formula.IsNull() {
		resultErr = errors.Join(resultErr, e.manager.SetRef(result, formula))
	}
	if resultErr == nil {
		resultErr = a.checkpoint()
	}
	return result, resultErr
}
