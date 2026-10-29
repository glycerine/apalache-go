package smtcore

// Source: retained linear rational fragment of src/smt/theory_arith.h,
// src/smt/theory_arith_core.h, src/smt/theory_arith_core_def.h, and the
// retained fragments of src/smt/theory_arith_int.h and
// src/smt/theory_arith_eq.h and src/smt/theory_arith_nl.h, plus statistics
// from src/smt/theory_arith_pp.h.

import (
	"fmt"
	"io"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtLinearForm struct {
	infinity, constant, infinitesimal big.Rat
	terms                             map[uint32]*big.Rat
}

func newSMTLinearForm() smtLinearForm { return smtLinearForm{terms: make(map[uint32]*big.Rat)} }

func (f *smtLinearForm) clone() smtLinearForm {
	result := newSMTLinearForm()
	result.infinity.Set(&f.infinity)
	result.constant.Set(&f.constant)
	result.infinitesimal.Set(&f.infinitesimal)
	for variable, coefficient := range f.terms {
		result.terms[variable] = new(big.Rat).Set(coefficient)
	}
	return result
}

func (f *smtLinearForm) addTerm(variable uint32, coefficient *big.Rat) {
	if coefficient.Sign() == 0 {
		return
	}
	if old := f.terms[variable]; old != nil {
		old.Add(old, coefficient)
		if old.Sign() == 0 {
			delete(f.terms, variable)
		}
		return
	}
	f.terms[variable] = new(big.Rat).Set(coefficient)
}

func (f *smtLinearForm) add(other *smtLinearForm) {
	f.infinity.Add(&f.infinity, &other.infinity)
	f.constant.Add(&f.constant, &other.constant)
	f.infinitesimal.Add(&f.infinitesimal, &other.infinitesimal)
	for variable, coefficient := range other.terms {
		f.addTerm(variable, coefficient)
	}
}

func (f *smtLinearForm) scale(coefficient *big.Rat) {
	f.infinity.Mul(&f.infinity, coefficient)
	f.constant.Mul(&f.constant, coefficient)
	f.infinitesimal.Mul(&f.infinitesimal, coefficient)
	for variable, value := range f.terms {
		value.Mul(value, coefficient)
		if value.Sign() == 0 {
			delete(f.terms, variable)
		}
	}
}

type smtLinearArithmeticAtom struct {
	native       *smtArithmeticAtom
	nativeOffset big.Rat
	boundAxiom   *smtArithmeticBoundAxiomAtom
	variable     int32
	left         ast.ASTHandle
	right        ast.ASTHandle
	form         smtLinearForm
	op           int32
	supported    bool
	nonlinear    bool
	decided      bool
	required     bool
	truth        bool
}

type smtLinearArithmeticScope struct {
	ownEqualities, ownEqualityHead       int
	equalityBounds                       int
	pendingBounds, pendingBoundHead      int
	expressionBounds                     int
	finalCheckIndex                      uint32
	assumeEqualities, assumeEqualityHead int
	derivedBounds                        int
	nonlinearStrategy                    uint32
	nonlinearGBExhausted                 bool
	atoms, assertions                    int
	head                                 int
	unsupported                          bool
	castTerms, castPredicates            int
	divisionTerms                        int
	nonlinearTerms                       int
	nonlinearRounds                      uint32
}

type smtLinearArithmeticStatistics struct {
	euclideanCalls, euclideanBounds                      uint64
	integerEqualityBranches                              uint64
	gomoryCuts                                           uint64
	gcdTests, gcdConflicts                               uint64
	maxMin                                               uint64
	crossNested                                          uint64
	conflicts, addRows, pivots, assertLower, assertUpper uint64
	fixedEqualities, offsetEqualities, boundPropagations uint64
	branches                                             uint64
	grobner                                              uint64
	nonlinearBranches                                    uint64
}

// SMTLinearArithmeticTheory is the rational linear specialization of the
// source arithmetic theory. Comparisons of the same linear polynomial share
// a tableau row; Boolean assignments tighten its lower or upper infinitesimal
// bounds. The tableau is replayed after scope pops because math/simplex owns no
// logical-context trail.
type SMTLinearArithmeticTheory struct {
	newBoundAtoms                        []int
	ownEqualities                        []smtArithmeticOwnEquality
	ownEqualityHead                      int
	equalityBoundsEnabled                bool
	equalityBounds                       []smtArithmeticEqualityEndpoints
	propagateEqualities                  bool
	propagationThreshold                 uint32
	boundPropagation                     uint32
	skipRowsWithBigCoefficients          bool
	maxLemmaSize, smallLemmaSize         uint32
	pivotStrategy, blandThreshold        uint32
	conflictCount                        uint32
	pendingBounds                        []smtArithmeticReplayBound
	pendingBoundHead                     int
	strongerLemmas                       bool
	euclideanNumerals                    *numeral.RationalManager
	expressionBounds                     []smtArithmeticExpressionBound
	euclideanEnabled                     bool
	integerEqualityBranching             bool
	branchCutCounter, branchCutRatio     uint32
	gcdEnabled, ignoreIntegers           bool
	finalCheckIndex                      uint32
	liberalFinalCheck, changedAssignment bool
	assumeEqualityCandidates             []smtArithmeticEqualityCandidate
	assumeEqualityHead                   int
	derivedBounds                        []smtArithmeticReplayBound
	nonlinearStrategy                    uint32
	nonlinearGBPerturbate                bool
	nonlinearGBExhausted                 bool
	*SMTTheoryBase
	solver              *simplex.Solver
	simplexVariables    []uint32
	nextSimplex         uint32
	replayedNextSimplex uint32
	replayedDefinitions int
	definitions         map[SMTTheoryVar]smtLinearForm
	atoms               []smtLinearArithmeticAtom
	atomByBool          map[int32]int
	assertions          []smtIntegerDiffAssertion
	assertionHead       int
	scopes              []smtLinearArithmeticScope
	unsupported         bool
	integer             bool
	mixed               bool
	infinite            bool
	integerColumns      map[uint32]bool
	castTerms           map[ast.ASTHandle]bool
	castTermTrail       []ast.ASTHandle
	castPredicates      map[int32]bool
	castPredicateTrail  []int32
	divisionTerms       map[ast.ASTHandle]bool
	divisionTermTrail   []ast.ASTHandle
	nonlinearColumns    []uint32
	nonlinearFactors    map[uint32][]uint32
	nonlinearEnabled    bool
	nonlinearGB         bool
	nonlinearGBLimit    uint32
	nonlinearBranching  bool
	nonlinearRounds     uint32
	nonlinearRoundLimit uint32
	nonlinearRandom     z3rand.Generator
	replayStatsEpoch    uint64
	replayStatsCounted  uint64
	delta               big.Rat
	equalityAdapter     *smtArithmeticEqualityAdapter
	statistics          smtLinearArithmeticStatistics
}

func NewSMTLinearArithmeticTheory(family ast.FamilyID) (*SMTLinearArithmeticTheory, error) {
	return newSMTLinearArithmeticTheory(family, false)
}

func NewSMTLinearIntegerArithmeticTheory(family ast.FamilyID) (*SMTLinearArithmeticTheory, error) {
	return newSMTLinearArithmeticTheory(family, true)
}

// NewSMTMixedLinearArithmeticTheory tracks source is_int_expr per variable,
// retaining rational columns alongside the integer branch-and-bound columns.
func NewSMTMixedLinearArithmeticTheory(family ast.FamilyID) (*SMTLinearArithmeticTheory, error) {
	t, err := newSMTLinearArithmeticTheory(family, false)
	if err == nil {
		t.mixed = true
	}
	return t, err
}

// NewSMTInfiniteArithmeticTheory is the retained inf_ext specialization.
// It uses the same rational-coefficient tableau with full infinity/main/epsilon
// values; finite specializations continue to require zero infinity.
func NewSMTInfiniteArithmeticTheory(family ast.FamilyID) (*SMTLinearArithmeticTheory, error) {
	t, err := NewSMTMixedLinearArithmeticTheory(family)
	if err == nil {
		t.infinite = true
	}
	return t, err
}

func newSMTLinearArithmeticTheory(family ast.FamilyID, integer bool) (*SMTLinearArithmeticTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	t := &SMTLinearArithmeticTheory{
		propagateEqualities:         true,
		propagationThreshold:        ^uint32(0),
		boundPropagation:            2,
		skipRowsWithBigCoefficients: true,
		maxLemmaSize:                128,
		smallLemmaSize:              16,
		blandThreshold:              1000,
		strongerLemmas:              true,
		branchCutRatio:              2,
		gcdEnabled:                  true,
		liberalFinalCheck:           true,
		SMTTheoryBase:               base,
		solver:                      simplex.NewSolver(nil),
		atomByBool:                  make(map[int32]int),
		definitions:                 make(map[SMTTheoryVar]smtLinearForm),
		nextSimplex:                 1,
		replayedNextSimplex:         1,
		integer:                     integer,
		integerColumns:              make(map[uint32]bool),
		castTerms:                   make(map[ast.ASTHandle]bool),
		castPredicates:              make(map[int32]bool),
		divisionTerms:               make(map[ast.ASTHandle]bool),
		nonlinearFactors:            make(map[uint32][]uint32),
		nonlinearEnabled:            true,
		nonlinearGB:                 true,
		nonlinearGBPerturbate:       true,
		nonlinearGBLimit:            512,
		nonlinearBranching:          true,
		nonlinearRoundLimit:         1024,
	}
	t.equalityAdapter, err = newSMTArithmeticEqualityAdapter(base)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *SMTLinearArithmeticTheory) ConfigureArithmetic(config SMTArithState) error {
	if t == nil || t.equalityAdapter == nil {
		return ast.ErrASTHandle
	}
	t.equalityAdapter.lazy = config.LazyAdapter
	t.equalityBoundsEnabled = config.EqBounds
	t.propagateEqualities = config.PropagateEqs
	t.propagationThreshold = config.PropagationThreshold
	t.boundPropagation = config.BoundProp
	t.skipRowsWithBigCoefficients = config.SkipRowsWithBigCoeffs
	t.maxLemmaSize, t.smallLemmaSize = config.MaxLemmaSize, config.SmallLemmaSize
	t.pivotStrategy, t.blandThreshold = config.PivotStrategy, config.BlandsRuleThreshold
	t.solver.SetArithmeticPivotPolicy(t.pivotStrategy, t.blandThreshold)
	t.strongerLemmas = config.StrongerLemmas
	t.gcdEnabled = config.GCDTest
	t.ignoreIntegers = config.IgnoreInt
	t.branchCutRatio = config.BranchCutRatio
	t.integerEqualityBranching = config.IntEqBranching
	t.euclideanEnabled = config.EuclideanSolver
	t.nonlinearEnabled = config.NLArith
	t.nonlinearGB = config.NLArithGB
	t.nonlinearGBLimit = config.NLArithGBThreshold
	t.nonlinearGBPerturbate = config.NLArithGBPerturbate
	t.nonlinearBranching = config.NLArithBranching
	t.nonlinearRoundLimit = config.NLArithRounds
	t.nonlinearRandom.SetSeed(config.RandomSeed)
	return nil
}

func (t *SMTLinearArithmeticTheory) createVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	v, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	for len(t.simplexVariables) <= int(v) {
		integral, err := t.isIntegerExpression(node.owner)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if integral {
			t.integerColumns[t.nextSimplex] = true
		}
		t.simplexVariables = append(t.simplexVariables, t.nextSimplex)
		t.nextSimplex++
	}
	return v, nil
}

func (t *SMTLinearArithmeticTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() {
		return err
	}
	kind, err := state.manager.GetDeclKind(sort)
	if err != nil {
		return err
	}
	expected := int32(0)
	if t.integer {
		expected = 1
	}
	if !t.mixed && kind != expected || kind != 0 && kind != 1 {
		t.unsupported = true
		return nil
	}
	_, err = t.createVariable(node)
	return err
}

func (t *SMTLinearArithmeticTheory) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, false, err
	}
	text, _, matched, err := state.manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return nil, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (t *SMTLinearArithmeticTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	kind, err := state.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

func (t *SMTLinearArithmeticTheory) ensureVariable(expression ast.ASTHandle) (uint32, error) {
	state, err := t.State()
	if err != nil {
		return 0, err
	}
	if node := state.context.enodes[expression]; node != nil && t.IsAttachedToVariable(node) {
		return t.simplexVariables[node.TheoryVar(t.FamilyID())], nil
	}
	if err = state.Internalize(expression, false); err != nil {
		return 0, err
	}
	node := state.context.enodes[expression]
	if node == nil {
		node, err = state.MkEnode(expression, false, false, true)
		if err != nil {
			return 0, err
		}
	}
	v, err := t.createVariable(node)
	if err != nil {
		return 0, err
	}
	return t.simplexVariables[v], nil
}

func (t *SMTLinearArithmeticTheory) linearize(expression ast.ASTHandle) (smtLinearForm, bool, error) {
	if value, numeral, err := t.numeral(expression); err != nil {
		return smtLinearForm{}, false, err
	} else if numeral {
		if t.integer && !value.IsInt() {
			return smtLinearForm{}, false, nil
		}
		result := newSMTLinearForm()
		result.constant.Set(value)
		return result, true, nil
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil {
		return smtLinearForm{}, false, err
	}
	state, _ := t.State()
	if arithmetic {
		count, countErr := state.manager.GetNumArgs(expression)
		if countErr != nil {
			return smtLinearForm{}, false, countErr
		}
		switch op {
		case smtArithOpAdd, smtArithOpSub:
			result := newSMTLinearForm()
			for index := uint32(0); index < count; index++ {
				argument, argErr := state.manager.GetArg(expression, index)
				if argErr != nil {
					return smtLinearForm{}, false, argErr
				}
				part, linear, partErr := t.linearize(argument)
				if partErr != nil || !linear {
					return smtLinearForm{}, linear, partErr
				}
				if op == smtArithOpSub && index != 0 {
					part.scale(big.NewRat(-1, 1))
				}
				result.add(&part)
			}
			return result, true, nil
		case smtArithOpUMinus, smtArithOpToReal:
			if t.integer && op == smtArithOpToReal {
				return smtLinearForm{}, false, nil
			}
			if count != 1 {
				return smtLinearForm{}, false, nil
			}
			argument, argErr := state.manager.GetArg(expression, 0)
			if argErr != nil {
				return smtLinearForm{}, false, argErr
			}
			result, linear, partErr := t.linearize(argument)
			if partErr == nil && linear && op == smtArithOpUMinus {
				result.scale(big.NewRat(-1, 1))
			}
			return result, linear, partErr
		case smtArithOpMul:
			result := newSMTLinearForm()
			result.constant.SetInt64(1)
			hasTerms := false
			for index := uint32(0); index < count; index++ {
				argument, argErr := state.manager.GetArg(expression, index)
				if argErr != nil {
					return smtLinearForm{}, false, argErr
				}
				part, linear, partErr := t.linearize(argument)
				if partErr != nil || !linear {
					return smtLinearForm{}, linear, partErr
				}
				if len(part.terms) != 0 {
					if hasTerms {
						column, err := t.ensureVariable(expression)
						if err != nil {
							return smtLinearForm{}, false, err
						}
						form := newSMTLinearForm()
						form.addTerm(column, big.NewRat(1, 1))
						return form, true, nil
					}
					hasTerms = true
					part.scale(&result.constant)
					result = part
				} else {
					if t.integer && !part.constant.IsInt() {
						return smtLinearForm{}, false, nil
					}
					result.scale(&part.constant)
				}
			}
			return result, true, nil
		case smtArithOpDiv:
			if t.integer {
				return smtLinearForm{}, false, nil
			}
			if count != 2 {
				return smtLinearForm{}, false, nil
			}
			left, argErr := state.manager.GetArg(expression, 0)
			if argErr != nil {
				return smtLinearForm{}, false, argErr
			}
			right, argErr := state.manager.GetArg(expression, 1)
			if argErr != nil {
				return smtLinearForm{}, false, argErr
			}
			denominator, numeral, numeralErr := t.numeral(right)
			if numeralErr != nil {
				return smtLinearForm{}, false, numeralErr
			}
			if !numeral || denominator.Sign() == 0 {
				variable, err := t.ensureVariable(expression)
				if err != nil {
					return smtLinearForm{}, false, err
				}
				result := newSMTLinearForm()
				result.addTerm(variable, big.NewRat(1, 1))
				return result, true, nil
			}
			result, linear, partErr := t.linearize(left)
			if partErr == nil && linear {
				result.scale(new(big.Rat).Inv(denominator))
			}
			return result, linear, partErr
		case smtArithOpToInt:
			if !t.mixed {
				return smtLinearForm{}, false, nil
			}
			// Its variable is constrained by the source floor axioms below.
		case smtArithOpIDiv, smtArithOpRem, smtArithOpMod:
			// These variables are constrained by guarded division axioms.
		default:
			return smtLinearForm{}, false, nil
		}
	}
	variable, err := t.ensureVariable(expression)
	if err != nil {
		return smtLinearForm{}, false, err
	}
	result := newSMTLinearForm()
	result.addTerm(variable, big.NewRat(1, 1))
	return result, true, nil
}

func (t *SMTLinearArithmeticTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	op, own, err := t.operation(expression)
	if err != nil {
		return false, err
	}
	if own {
		switch op {
		case smtArithOpNum, smtArithOpAdd, smtArithOpSub, smtArithOpUMinus, smtArithOpMul, smtArithOpDiv, smtArithOpToReal, smtArithOpToInt, smtArithOpIDiv, smtArithOpRem, smtArithOpMod:
		default:
			t.unsupported = true
		}
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		argument, argumentErr := state.manager.GetArg(expression, index)
		if argumentErr != nil {
			return false, argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return false, argumentErr
		}
	}
	if own && op == smtArithOpMul {
		nonlinear, err := t.internalizeNonlinearMonomial(expression)
		if err != nil || nonlinear {
			return nonlinear, err
		}
	}
	division := own && (op == smtArithOpIDiv || op == smtArithOpRem || op == smtArithOpMod)
	if own && op == smtArithOpDiv {
		denominator, err := state.manager.GetArg(expression, 1)
		if err != nil {
			return false, err
		}
		value, numeral, err := t.numeral(denominator)
		if err != nil {
			return false, err
		}
		division = !numeral || value.Sign() == 0
	}
	if division || own && op == smtArithOpToInt && t.mixed {
		// Source internalize_to_int publishes its variable before creating
		// the to_real/to_int axioms, whose terms visit this cast again.
		node := state.context.enodes[expression]
		if node == nil {
			node, err = state.MkEnode(expression, false, false, true)
			if err != nil {
				return false, err
			}
		}
		if _, err = t.createVariable(node); err != nil {
			return false, err
		}
		if division {
			err = t.installDivisionAxioms(expression, op)
		} else {
			err = t.installToIntAxioms(expression)
		}
		if err != nil {
			return false, err
		}
		return true, nil
	}
	form, linear, err := t.linearize(expression)
	if err == nil && !linear {
		node := state.context.enodes[expression]
		if node == nil {
			node, err = state.MkEnode(expression, false, false, true)
		}
		if err == nil && !t.IsAttachedToVariable(node) {
			_, err = t.createVariable(node)
		}
		return err == nil, err
	}
	if err != nil {
		return false, err
	}
	node := state.context.enodes[expression]
	if node == nil {
		node, err = state.MkEnode(expression, false, false, true)
		if err != nil {
			return false, err
		}
	}
	variable, err := t.createVariable(node)
	if err != nil {
		return false, err
	}
	if _, found := t.definitions[variable]; !found {
		t.statistics.addRows++
	}
	t.definitions[variable] = form
	return true, nil
}

func (t *SMTLinearArithmeticTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil {
		return false, err
	}
	if arithmetic && op == 16 && t.mixed {
		return t.internalizeIsInt(expression)
	}
	if !arithmetic || op < smtArithOpLE || op > smtArithOpGT {
		t.unsupported = true
		return false, nil
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	decided, required, decisionErr := t.evenMonomialComparison(left, right, op)
	if decisionErr != nil {
		return false, decisionErr
	}
	form, linear, err := t.linearize(left)
	if err != nil {
		return false, err
	}
	if !linear {
		form = newSMTLinearForm()
	}
	rhs, rhsLinear, err := t.linearize(right)
	if err != nil {
		return false, err
	}
	linear = linear && rhsLinear
	if linear {
		rhs.scale(big.NewRat(-1, 1))
		form.add(&rhs)
	} else {
		form = newSMTLinearForm()
	}
	variable, err := state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	t.atomByBool[variable] = len(t.atoms)
	t.atoms = append(t.atoms, smtLinearArithmeticAtom{variable: variable, left: left, right: right, form: form, op: op, supported: linear, nonlinear: t.nonlinearForm(&form), decided: decided, required: required})
	if linear {
		t.statistics.addRows++
	}
	if err = t.registerOrdinaryBoundAxiom(len(t.atoms) - 1); err != nil {
		return false, err
	}
	return true, nil
}

func (t *SMTLinearArithmeticTheory) evenMonomialComparison(left, right ast.ASTHandle, op int32) (bool, bool, error) {
	zero := func(expression ast.ASTHandle) (bool, error) {
		value, numeral, err := t.numeral(expression)
		return numeral && value.Sign() == 0, err
	}
	evenMonomial := func(expression ast.ASTHandle) (int, bool, error) {
		operation, arithmetic, err := t.operation(expression)
		if err != nil || !arithmetic || operation != smtArithOpMul {
			return 0, false, err
		}
		state, _ := t.State()
		count, err := state.manager.GetNumArgs(expression)
		if err != nil || count < 2 {
			return 0, false, err
		}
		coefficient := new(big.Rat).SetInt64(1)
		powers := make(map[ast.ASTHandle]uint32)
		for index := uint32(0); index < count; index++ {
			argument, argErr := state.manager.GetArg(expression, index)
			if argErr != nil {
				return 0, false, argErr
			}
			if value, numeral, numeralErr := t.numeral(argument); numeralErr != nil {
				return 0, false, numeralErr
			} else if numeral {
				coefficient.Mul(coefficient, value)
			} else {
				powers[argument]++
			}
		}
		if len(powers) == 0 || coefficient.Sign() == 0 {
			return 0, false, nil
		}
		for _, power := range powers {
			if power%2 != 0 {
				return 0, false, nil
			}
		}
		return coefficient.Sign(), true, nil
	}
	leftSign, leftEven, err := evenMonomial(left)
	if err != nil {
		return false, false, err
	}
	rightZero, err := zero(right)
	if err != nil {
		return false, false, err
	}
	if leftEven && rightZero {
		if (leftSign > 0 && op == smtArithOpGE) || (leftSign < 0 && op == smtArithOpLE) {
			return true, true, nil
		}
		if (leftSign > 0 && op == smtArithOpLT) || (leftSign < 0 && op == smtArithOpGT) {
			return true, false, nil
		}
	}
	rightSign, rightEven, err := evenMonomial(right)
	if err != nil {
		return false, false, err
	}
	leftZero, err := zero(left)
	if err != nil {
		return false, false, err
	}
	if rightEven && leftZero {
		if (rightSign > 0 && op == smtArithOpLE) || (rightSign < 0 && op == smtArithOpGE) {
			return true, true, nil
		}
		if (rightSign > 0 && op == smtArithOpGT) || (rightSign < 0 && op == smtArithOpLT) {
			return true, false, nil
		}
	}
	return false, false, nil
}

func (t *SMTLinearArithmeticTheory) Assign(variable int32, truth bool) error {
	if t.castPredicates[variable] {
		return nil
	}
	index, found := t.atomByBool[variable]
	if !found {
		return ast.ErrASTDomain
	}
	t.atoms[index].truth = truth
	lower, _ := smtLinearAtomBound(&t.atoms[index], truth, t.integralAtom(&t.atoms[index]))
	if lower {
		t.statistics.assertLower++
	} else {
		t.statistics.assertUpper++
	}
	t.assertions = append(t.assertions, smtIntegerDiffAssertion{atom: index, truth: truth})
	t.replayStatsEpoch++
	return nil
}

func (t *SMTLinearArithmeticTheory) CanPropagate() bool {
	// Native internalize_numeral/linear rows immediately initialize their
	// tableau values. The Go replay must also publish new columns/definitions
	// when no Boolean arithmetic atom was asserted (for example seq.unit(7)).
	return t != nil && (len(t.newBoundAtoms) != 0 || t.replayedNextSimplex != t.nextSimplex || t.replayedDefinitions != len(t.definitions) || t.assertionHead < len(t.assertions) || t.pendingBoundHead < len(t.pendingBounds) || t.ownEqualityHead < len(t.ownEqualities))
}

func smtLinearAtomBound(atom *smtLinearArithmeticAtom, truth, integer bool) (lower bool, value simplex.Infinitesimal) {
	if atom.native != nil {
		epsilon := newSMTArithmeticValue(nil, big.NewRat(1, 1))
		if integer {
			epsilon = newSMTArithmeticValue(big.NewRat(1, 1), nil)
		}
		_ = atom.native.assign(truth, &epsilon)
		value.Infinity.Set(&atom.native.value.infinity)
		value.Main.Sub(&atom.native.value.rational, &atom.nativeOffset)
		value.Epsilon.Set(&atom.native.value.infinitesimal)
		return atom.native.kind == smtArithmeticLowerBound, value
	}
	value.Main.Neg(&atom.form.constant)
	op := smtLinearEffectiveOp(atom.op, truth)
	lower = op == smtArithOpGE || op == smtArithOpGT
	if integer {
		floor := floorSMTRational(&value.Main)
		ceil := new(big.Int).Set(floor)
		if !value.Main.IsInt() {
			ceil.Add(ceil, big.NewInt(1))
		}
		switch op {
		case smtArithOpGT:
			value.Main.SetInt(floor.Add(floor, big.NewInt(1)))
		case smtArithOpLT:
			value.Main.SetInt(ceil.Sub(ceil, big.NewInt(1)))
		case smtArithOpGE:
			value.Main.SetInt(ceil)
		case smtArithOpLE:
			value.Main.SetInt(floor)
		}
	} else if op == smtArithOpGT {
		value.Epsilon.SetInt64(1)
	} else if op == smtArithOpLT {
		value.Epsilon.SetInt64(-1)
	}
	return lower, value
}

func smtLinearEffectiveOp(op int32, truth bool) int32 {
	if truth {
		return op
	}
	switch op {
	case smtArithOpLE:
		return smtArithOpGT
	case smtArithOpLT:
		return smtArithOpGE
	case smtArithOpGE:
		return smtArithOpLT
	case smtArithOpGT:
		return smtArithOpLE
	default:
		return op
	}
}

func smtLinearFormsOpposite(first, second *smtLinearForm) bool {
	if first == nil || second == nil || len(first.terms) != len(second.terms) {
		return false
	}
	var constant big.Rat
	constant.Neg(&second.infinity)
	if first.infinity.Cmp(&constant) != 0 {
		return false
	}
	constant.Neg(&second.constant)
	if first.constant.Cmp(&constant) != 0 {
		return false
	}
	constant.Neg(&second.infinitesimal)
	if first.infinitesimal.Cmp(&constant) != 0 {
		return false
	}
	for variable, coefficient := range first.terms {
		other := second.terms[variable]
		if other == nil {
			return false
		}
		constant.Neg(other)
		if coefficient.Cmp(&constant) != 0 {
			return false
		}
	}
	return true
}

func smtLinearFormsEqual(first, second *smtLinearForm) bool {
	if first == nil || second == nil || len(first.terms) != len(second.terms) || first.infinity.Cmp(&second.infinity) != 0 || first.constant.Cmp(&second.constant) != 0 || first.infinitesimal.Cmp(&second.infinitesimal) != 0 {
		return false
	}
	for variable, coefficient := range first.terms {
		other := second.terms[variable]
		if other == nil || coefficient.Cmp(other) != 0 {
			return false
		}
	}
	return true
}

func (t *SMTLinearArithmeticTheory) theoryVariable(simplexVariable uint32) (SMTTheoryVar, bool) {
	for variable, candidate := range t.simplexVariables {
		if candidate == simplexVariable {
			return SMTTheoryVar(variable), true
		}
	}
	return SMTNullTheoryVar, false
}

func smtLinearAssertionLiteral(atom *smtLinearArithmeticAtom, assertion smtIntegerDiffAssertion) SMTLiteral {
	return NewSMTLiteral(atom.variable, !assertion.truth)
}

func (t *SMTLinearArithmeticTheory) propagateEquality(state *SMTInternalizerState, first, second uint32, literals []SMTLiteral) (bool, error) {
	return t.propagateEqualityWithParameters(state, first, second, literals, nil, nil)
}

func (t *SMTLinearArithmeticTheory) propagateEqualityWithParameters(state *SMTInternalizerState, first, second uint32, literals []SMTLiteral, parameters []ast.ASTParameter, equalities []SMTEnodePair) (bool, error) {
	leftVariable, leftOK := t.theoryVariable(first)
	rightVariable, rightOK := t.theoryVariable(second)
	if !leftOK || !rightOK {
		return false, nil
	}
	left, leftErr := t.Enode(leftVariable)
	right, rightErr := t.Enode(rightVariable)
	if leftErr != nil || rightErr != nil {
		return false, ast.ErrASTDomain
	}
	if left.Root() == right.Root() {
		return false, nil
	}
	leftSort, err := state.manager.GetSort(left.owner)
	if err != nil {
		return false, err
	}
	rightSort, err := state.manager.GetSort(right.owner)
	if err != nil {
		return false, err
	}
	// Source propagate_eq_to_core never merges Int and Real enodes.
	if leftSort != rightSort {
		if err = t.emitArithmeticEquality("arith", "propagate_eq_to_core", 334, func() bool { return true }, func(out io.Writer) error {
			if err := smtArithEqDisplayExpression(state.manager, out, left.owner); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, " = "); err != nil {
				return err
			}
			if err := smtArithEqDisplayExpression(state.manager, out, right.owner); err != nil {
				return err
			}
			return smtWriteTheoryText(out, "\n")
		}); err != nil {
			return false, err
		}
		return false, nil
	}
	justification, err := NewSMTExtTheoryEqualityPropagationJustification(
		state.manager, t.FamilyID(), literals, equalities, left, right, parameters)
	if err != nil {
		return false, err
	}
	state.ownedJustifications = append(state.ownedJustifications, justification)
	if err = t.emitArithmeticEquality("arith_eq", "propagate_eq_to_core", 351, func() bool { return true }, func(out io.Writer) error {
		leftID, err := left.OwnerID()
		if err != nil {
			return err
		}
		rightID, err := right.OwnerID()
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("detected equality: #%d = #%d\n", leftID, rightID)); err != nil {
			return err
		}
		if err = t.displayArithmeticEqualityVariable(out, first); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
		if err = t.displayArithmeticEqualityVariable(out, second); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return false, err
	}
	if err = t.emitArithmeticEquality("arith_eq_propagation", "propagate_eq_to_core", 354, func() bool { return true }, func(out io.Writer) error {
		for _, literal := range literals {
			if err := state.DisplayDetailedLiteral(out, literal); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		for _, equality := range equalities {
			if err := smtArithEqDisplayExpression(state.manager, out, equality.First.owner); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, " = "); err != nil {
				return err
			}
			if err := smtArithEqDisplayExpression(state.manager, out, equality.Second.owner); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		if err := smtWriteTheoryText(out, " ==> "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(state.manager, out, left.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " = "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(state.manager, out, right.owner); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return false, err
	}
	if err = state.assignEquality(left, right, NewSMTEqualityJustification(justification)); err != nil {
		return false, err
	}
	// The source tableau already knows the equality. This Go tableau is
	// replayed: defer its equality-bound evidence until the queued core
	// merge exists, without sending a theory event back to the origin.
	t.ownEqualities = append(t.ownEqualities, smtArithmeticOwnEquality{leftVariable, rightVariable})
	return true, nil
}

func smtLinearNormalizedLE(atom *smtLinearArithmeticAtom, assertion smtIntegerDiffAssertion) (smtLinearForm, bool) {
	if atom.native != nil {
		lower, value := smtLinearAtomBound(atom, assertion.truth, atom.boundAxiom.integral)
		form := atom.form.clone()
		form.infinity.Neg(&value.Infinity)
		form.constant.Neg(&value.Main)
		form.infinitesimal.Neg(&value.Epsilon)
		if lower {
			form.scale(big.NewRat(-1, 1))
		}
		return form, true
	}
	op := smtLinearEffectiveOp(atom.op, assertion.truth)
	if !atom.supported || op != smtArithOpLE && op != smtArithOpGE {
		return smtLinearForm{}, false
	}
	form := atom.form.clone()
	if op == smtArithOpGE {
		form.scale(big.NewRat(-1, 1))
	}
	return form, true
}

func (t *SMTLinearArithmeticTheory) propagateImpliedEqualities() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if !t.canPropagateArithmeticEqualities() {
		return false, nil
	}
	for firstIndex, firstAssertion := range t.assertions {
		firstAtom := &t.atoms[firstAssertion.atom]
		firstForm, firstOK := smtLinearNormalizedLE(firstAtom, firstAssertion)
		if !firstOK {
			continue
		}
		if err = t.emitArithmeticEquality("arith_eq_verbose", "propagate_cheap_eq", 220, func() bool { return true }, func(out io.Writer) error {
			if err := smtWriteTheoryText(out, fmt.Sprintf("checking if row %d can propagate equality.\n", firstIndex)); err != nil {
				return err
			}
			return displayArithmeticEqualityForm(out, &firstForm)
		}); err != nil {
			return false, err
		}
		if firstForm.infinity.Sign() != 0 || firstForm.constant.Sign() != 0 || firstForm.infinitesimal.Sign() != 0 || len(firstForm.terms) != 2 {
			continue
		}
		for _, secondAssertion := range t.assertions[firstIndex+1:] {
			secondAtom := &t.atoms[secondAssertion.atom]
			secondForm, secondOK := smtLinearNormalizedLE(secondAtom, secondAssertion)
			if !secondOK {
				continue
			}
			if !smtLinearFormsOpposite(&firstForm, &secondForm) {
				continue
			}
			variables := sortedSMTLinearFormVariables(&firstForm)
			coefficients := []*big.Rat{firstForm.terms[variables[0]], firstForm.terms[variables[1]]}
			if coefficients[0].Sign() == 0 || coefficients[0].Cmp(new(big.Rat).Neg(coefficients[1])) != 0 {
				continue
			}
			if err = t.emitArithmeticEquality("arith_eq", "propagate_cheap_eq", 263, func() bool { return true }, func(out io.Writer) error {
				if err := smtWriteTheoryText(out, "propagate eq using x-y=0 row:\n"); err != nil {
					return err
				}
				return displayArithmeticEqualityForm(out, &firstForm)
			}); err != nil {
				return false, err
			}
			propagated, propagateErr := t.propagateEquality(state, variables[0], variables[1], []SMTLiteral{
				smtLinearAssertionLiteral(firstAtom, firstAssertion),
				smtLinearAssertionLiteral(secondAtom, secondAssertion),
			})
			if propagated {
				t.statistics.offsetEqualities++
			}
			if propagateErr != nil || propagated {
				return propagated, propagateErr
			}
		}
	}
	type fixedVariable struct {
		variable uint32
		value    simplex.Infinitesimal
		literals []SMTLiteral
	}
	fixed := make([]fixedVariable, 0)
	for firstIndex, firstAssertion := range t.assertions {
		firstAtom := &t.atoms[firstAssertion.atom]
		firstForm, firstOK := smtLinearNormalizedLE(firstAtom, firstAssertion)
		if !firstOK || len(firstForm.terms) != 1 {
			continue
		}
		for _, secondAssertion := range t.assertions[firstIndex+1:] {
			secondAtom := &t.atoms[secondAssertion.atom]
			secondForm, secondOK := smtLinearNormalizedLE(secondAtom, secondAssertion)
			if !secondOK || !smtLinearFormsOpposite(&firstForm, &secondForm) {
				continue
			}
			for variable, coefficient := range firstForm.terms {
				value := simplex.NewInfiniteInfinitesimal(new(big.Rat).Neg(&firstForm.infinity), new(big.Rat).Neg(&firstForm.constant), new(big.Rat).Neg(&firstForm.infinitesimal))
				value.Quo(coefficient)
				fixed = append(fixed, fixedVariable{variable: variable, value: value, literals: []SMTLiteral{
					smtLinearAssertionLiteral(firstAtom, firstAssertion),
					smtLinearAssertionLiteral(secondAtom, secondAssertion),
				}})
			}
			break
		}
	}
	for first := range fixed {
		for second := first + 1; second < len(fixed); second++ {
			if fixed[first].variable == fixed[second].variable || fixed[first].value.Cmp(&fixed[second].value) != 0 {
				continue
			}
			literals := append(append([]SMTLiteral(nil), fixed[first].literals...), fixed[second].literals...)
			if err = t.emitArithmeticEquality("arith_eq_propagation", "propagate_cheap_eq", 251, func() bool { return true }, func(out io.Writer) error {
				return smtWriteTheoryText(out, "fixed\n")
			}); err != nil {
				return false, err
			}
			propagated, propagateErr := t.propagateEquality(state, fixed[first].variable, fixed[second].variable, literals)
			if propagated {
				t.statistics.fixedEqualities++
			}
			if propagateErr != nil || propagated {
				return propagated, propagateErr
			}
		}
	}
	for firstIndex, firstAssertion := range t.assertions {
		firstAtom := &t.atoms[firstAssertion.atom]
		firstForm, firstOK := smtLinearNormalizedLE(firstAtom, firstAssertion)
		if !firstOK {
			continue
		}
		for _, secondAssertion := range t.assertions[firstIndex+1:] {
			secondAtom := &t.atoms[secondAssertion.atom]
			secondForm, secondOK := smtLinearNormalizedLE(secondAtom, secondAssertion)
			if !secondOK || !smtLinearFormsOpposite(&firstForm, &secondForm) {
				continue
			}
			remaining := firstForm.clone()
			literals := []SMTLiteral{
				smtLinearAssertionLiteral(firstAtom, firstAssertion),
				smtLinearAssertionLiteral(secondAtom, secondAssertion),
			}
			for _, value := range fixed {
				coefficient := remaining.terms[value.variable]
				if coefficient == nil {
					continue
				}
				if err = arithmeticEqualityAssertion(170, "is_fixed(v)", value.variable != 0); err != nil {
					return false, err
				}
				var contribution big.Rat
				contribution.Mul(coefficient, &value.value.Infinity)
				remaining.infinity.Add(&remaining.infinity, &contribution)
				contribution.Mul(coefficient, &value.value.Main)
				remaining.constant.Add(&remaining.constant, &contribution)
				contribution.Mul(coefficient, &value.value.Epsilon)
				remaining.infinitesimal.Add(&remaining.infinitesimal, &contribution)
				delete(remaining.terms, value.variable)
				literals = append(literals, value.literals...)
			}
			if len(remaining.terms) == 1 {
				variables := sortedSMTLinearFormVariables(&remaining)
				if err = arithmeticEqualityAssertion(193, "x != null_theory_var", variables[0] != 0); err != nil {
					return false, err
				}
				coefficient := remaining.terms[variables[0]]
				if coefficient.Sign() == 0 {
					continue
				}
				value := simplex.NewInfiniteInfinitesimal(new(big.Rat).Neg(&remaining.infinity), new(big.Rat).Neg(&remaining.constant), new(big.Rat).Neg(&remaining.infinitesimal))
				value.Quo(coefficient)
				for _, candidate := range fixed {
					if candidate.variable == variables[0] || candidate.value.Cmp(&value) != 0 {
						continue
					}
					if err = t.emitArithmeticEquality("arith_eq_propagation", "propagate_cheap_eq", 251, func() bool { return true }, func(out io.Writer) error {
						return smtWriteTheoryText(out, "fixed\n")
					}); err != nil {
						return false, err
					}
					fixedLiterals := append(append([]SMTLiteral(nil), literals...), candidate.literals...)
					propagated, propagateErr := t.propagateEquality(state, variables[0], candidate.variable, fixedLiterals)
					if propagated {
						t.statistics.fixedEqualities++
					}
					if propagateErr != nil || propagated {
						return propagated, propagateErr
					}
				}
				continue
			}
			if remaining.infinity.Sign() != 0 || remaining.constant.Sign() != 0 || remaining.infinitesimal.Sign() != 0 || len(remaining.terms) != 2 {
				continue
			}
			variables := sortedSMTLinearFormVariables(&remaining)
			coefficients := []*big.Rat{remaining.terms[variables[0]], remaining.terms[variables[1]]}
			if coefficients[0].Sign() == 0 || coefficients[0].Cmp(new(big.Rat).Neg(coefficients[1])) != 0 {
				continue
			}
			if err = t.emitArithmeticEquality("arith_eq", "propagate_cheap_eq", 263, func() bool { return true }, func(out io.Writer) error {
				if err := smtWriteTheoryText(out, "propagate eq using x-y=0 row:\n"); err != nil {
					return err
				}
				return displayArithmeticEqualityForm(out, &remaining)
			}); err != nil {
				return false, err
			}
			propagated, propagateErr := t.propagateEquality(state, variables[0], variables[1], literals)
			if propagated {
				t.statistics.offsetEqualities++
			}
			if propagateErr != nil || propagated {
				return propagated, propagateErr
			}
		}
	}
	definitionVariables := make([]int, 0, len(t.definitions))
	for variable := range t.definitions {
		definitionVariables = append(definitionVariables, int(variable))
	}
	sort.Ints(definitionVariables)
	for firstIndex, rawFirst := range definitionVariables {
		first := SMTTheoryVar(rawFirst)
		firstForm := t.definitions[first]
		for _, rawSecond := range definitionVariables[firstIndex+1:] {
			second := SMTTheoryVar(rawSecond)
			secondForm := t.definitions[second]
			if !smtLinearFormsEqual(&firstForm, &secondForm) {
				continue
			}
			if err = arithmeticEqualityAssertion(300, "y == y2 && k == k2", smtLinearFormsEqual(&firstForm, &secondForm)); err != nil {
				return false, err
			}
			if err = t.emitArithmeticEquality("arith_eq", "propagate_cheap_eq", 304, func() bool { return true }, func(out io.Writer) error {
				if err := smtWriteTheoryText(out, "propagate eq two rows:\nswapped: 0\n"); err != nil {
					return err
				}
				if err := smtWriteTheoryText(out, fmt.Sprintf("x  : v%d\nx2 : v%d\n", first, second)); err != nil {
					return err
				}
				if err := displayArithmeticEqualityForm(out, &firstForm); err != nil {
					return err
				}
				return displayArithmeticEqualityForm(out, &secondForm)
			}); err != nil {
				return false, err
			}
			propagated, propagateErr := t.propagateEquality(
				state, t.simplexVariables[first], t.simplexVariables[second], nil)
			if propagated {
				t.statistics.offsetEqualities++
			}
			if propagateErr != nil || propagated {
				return propagated, propagateErr
			}
		}
	}
	return false, nil
}

func (t *SMTLinearArithmeticTheory) rebuild() (bool, error) {
	// Replaying logical rows must not discard the current assignment. In
	// particular, popping constraints leaves a feasible child assignment valid
	// in the parent. Values are seeds, never additional bounds or premises.
	seeds := make(map[uint32]simplex.Infinitesimal, len(t.simplexVariables))
	if t.solver != nil {
		for _, column := range t.simplexVariables {
			if column < t.solver.NumVariables() {
				seeds[column] = t.solver.Value(column)
			}
		}
	}
	solver, status, err := t.buildSimplexReplay(nil, seeds)
	if err == nil && status == lbool.True {
		t.solver = solver
		t.replayedNextSimplex, t.replayedDefinitions = t.nextSimplex, len(t.definitions)
	}
	if err == nil && status == lbool.False && t.state != nil {
		err = t.derivedBoundConflict(solver)
	}
	return status == lbool.True, err
}

func (t *SMTLinearArithmeticTheory) buildSimplex() (*simplex.Solver, lbool.LBool, error) {
	return t.buildSimplexWithValues(nil)
}

func (t *SMTLinearArithmeticTheory) buildSimplexWithValues(values map[uint32]*big.Rat) (*simplex.Solver, lbool.LBool, error) {
	return t.buildSimplexReplay(values, nil)
}

func (t *SMTLinearArithmeticTheory) buildSimplexReplay(values map[uint32]*big.Rat, seeds map[uint32]simplex.Infinitesimal) (*simplex.Solver, lbool.LBool, error) {
	solver := simplex.NewSolver(nil)
	solver.SetArithmeticPivotPolicy(t.pivotStrategy, t.blandThreshold)
	for variable := uint32(0); variable < t.nextSimplex; variable++ {
		solver.EnsureVariable(variable)
	}
	one := simplex.RationalInfinitesimal(big.NewRat(1, 1))
	if err := solver.SetLowerWithDependencies(0, one, nil); err != nil {
		return nil, lbool.Undef, err
	}
	if err := solver.SetUpperWithDependencies(0, one, nil); err != nil {
		return nil, lbool.Undef, err
	}
	definitionVariables := make([]int, 0, len(t.definitions))
	for variable := range t.definitions {
		definitionVariables = append(definitionVariables, int(variable))
	}
	sort.Ints(definitionVariables)
	for _, rawVariable := range definitionVariables {
		variable := SMTTheoryVar(rawVariable)
		if int(variable) >= len(t.simplexVariables) {
			continue
		}
		form := t.definitions[variable]
		base := t.simplexVariables[variable]
		variables := []uint32{base}
		coefficients := []big.Rat{*big.NewRat(1, 1)}
		ordered := make([]uint32, 0, len(form.terms))
		for term := range form.terms {
			ordered = append(ordered, term)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		for _, term := range ordered {
			variables = append(variables, term)
			coefficients = append(coefficients, *new(big.Rat).Neg(form.terms[term]))
		}
		if form.constant.Sign() != 0 {
			variables = append(variables, 0)
			coefficients = append(coefficients, *new(big.Rat).Neg(&form.constant))
		}
		if _, err := solver.AddRationalRow(base, variables, coefficients); err != nil {
			return nil, lbool.Undef, err
		}
	}
	atomBases := make([]uint32, len(t.atoms))
	coefficientBases := make(map[string]uint32)
	for index := range t.atoms {
		atom := &t.atoms[index]
		if !atom.supported {
			continue
		}
		base := t.nextSimplex + uint32(index)
		key := smtArithmeticCoefficientKey(&atom.form)
		if prior, found := coefficientBases[key]; found {
			atomBases[index] = prior
			continue
		}
		atomBases[index] = base
		coefficientBases[key] = base
		variables := make([]uint32, 0, len(atom.form.terms)+1)
		coefficients := make([]big.Rat, 0, len(atom.form.terms)+1)
		variables = append(variables, base)
		coefficients = append(coefficients, *big.NewRat(1, 1))
		ordered := make([]uint32, 0, len(atom.form.terms))
		for variable := range atom.form.terms {
			ordered = append(ordered, variable)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		for _, variable := range ordered {
			coefficient := atom.form.terms[variable]
			variables = append(variables, variable)
			coefficients = append(coefficients, *new(big.Rat).Neg(coefficient))
		}
		if _, err := solver.AddRationalRow(base, variables, coefficients); err != nil {
			return nil, lbool.Undef, err
		}
	}
	for _, assertion := range t.assertions {
		atom := &t.atoms[assertion.atom]
		if !atom.supported {
			continue
		}
		lower, bound := smtLinearAtomBound(atom, assertion.truth, t.integralAtom(atom))
		base := atomBases[assertion.atom]
		dependency := simplex.BoundDependency{Literal: smtLinearAssertionLiteral(atom, assertion).Index(), Coefficient: *big.NewRat(1, 1)}
		solver.AddArithmeticBoundAtom(base, lower, bound, []simplex.BoundDependency{dependency})
		_, err := smtTightenExplainedSimplexBound(solver, base, lower, bound, []simplex.BoundDependency{dependency})
		if err != nil {
			t.recordArithmeticBoundConflict(solver, base, lower, &smtArithmeticBoundExplanation{dependencies: []simplex.BoundDependency{dependency}})
			return solver, lbool.False, nil
		}
		// Source single-variable atoms constrain the variable directly. Keep
		// these bounds as well as the comparison row for nonlinear intervals.
		if len(atom.form.terms) == 1 {
			for variable, coefficient := range atom.form.terms {
				direct := bound.Clone()
				direct.Quo(coefficient)
				directLower := lower == (coefficient.Sign() > 0)
				directDependency := simplex.BoundDependency{Literal: dependency.Literal, Coefficient: *new(big.Rat).Inv(coefficient)}
				solver.AddArithmeticBoundAtom(variable, directLower, direct, []simplex.BoundDependency{directDependency})
				_, err = smtTightenExplainedSimplexBound(solver, variable, directLower, direct, []simplex.BoundDependency{directDependency})
				if err != nil {
					t.recordArithmeticBoundConflict(solver, variable, directLower, &smtArithmeticBoundExplanation{dependencies: []simplex.BoundDependency{directDependency}})
					return solver, lbool.False, nil
				}
			}
		}
	}
	for _, bound := range t.expressionBounds {
		base, found := coefficientBases[bound.key]
		if !found {
			return nil, lbool.Undef, ast.ErrASTDomain
		}
		if _, err := smtTightenReplaySimplexBound(solver, base, bound.lower, bound.value, bound.explanation); err != nil {
			t.recordArithmeticBoundConflict(solver, base, bound.lower, bound.explanation)
			return solver, lbool.False, nil
		}
	}
	for _, bound := range t.derivedBounds {
		if _, err := smtTightenReplaySimplexBound(solver, bound.column, bound.lower, bound.value, bound.explanation); err != nil {
			t.recordArithmeticBoundConflict(solver, bound.column, bound.lower, bound.explanation)
			return solver, lbool.False, nil
		}
	}
	for column, value := range values {
		bound := simplex.RationalInfinitesimal(value)
		if _, err := smtTightenSimplexBound(solver, column, true, bound); err != nil {
			return solver, lbool.False, nil
		}
		if _, err := smtTightenSimplexBound(solver, column, false, bound); err != nil {
			return solver, lbool.False, nil
		}
	}
	if len(t.nonlinearColumns) != 0 {
		_, conflict, err := t.propagateArithmeticDefinitionBounds(solver)
		if err != nil {
			return solver, lbool.Undef, err
		}
		if conflict {
			return solver, lbool.False, nil
		}
		if t.nonlinearEnabled {
			_, conflict, err = t.propagateNonlinearBounds(solver)
			if err != nil {
				return solver, lbool.Undef, err
			}
			if conflict {
				return solver, lbool.False, nil
			}
		}
		_, conflict, err = t.propagateArithmeticDefinitionBounds(solver)
		if err != nil {
			return solver, lbool.Undef, err
		}
		if conflict {
			return solver, lbool.False, nil
		}
		conflict, err = t.propagateLinearMonomials(solver)
		if err != nil {
			return solver, lbool.Undef, err
		}
		if conflict {
			return solver, lbool.False, nil
		}
	}
	// Set only nonbasic values: SetValue then updates every dependent basic
	// row. Clamp to logical bounds before feasibility repair, which repairs
	// basic variables rather than arbitrary nonbasic assignments.
	for column := uint32(1); column < t.nextSimplex; column++ {
		value, exists := seeds[column]
		if !exists {
			continue
		}
		if _, basic := solver.BasicRow(column); basic {
			continue
		}
		if lower, exists := solver.Lower(column); exists && value.Cmp(&lower) < 0 {
			value = lower
		}
		if upper, exists := solver.Upper(column); exists && value.Cmp(&upper) > 0 {
			value = upper
		}
		solver.SetValue(column, value)
	}
	status, err := solver.MakeFeasible()
	if err != nil {
		return nil, lbool.Undef, err
	}
	// Every Go replay reconstructs the complete current tableau. Multiple
	// source stages may inspect the same assigned state, but native arithmetic
	// keeps one tableau and does not count those reconstructions as new pivots.
	if t.replayStatsCounted != t.replayStatsEpoch {
		t.statistics.pivots += uint64(solver.Stats.NumPivots)
		t.replayStatsCounted = t.replayStatsEpoch
	}
	return solver, status, nil
}

func (t *SMTLinearArithmeticTheory) fractionalVariable(solver *simplex.Solver) (uint32, simplex.Infinitesimal, bool) {
	for _, variable := range t.simplexVariables {
		if t.mixed && !t.integerColumns[variable] {
			continue
		}
		value := solver.Value(variable)
		if !value.IsRational() || !value.Main.IsInt() {
			return variable, value, true
		}
	}
	return 0, simplex.Infinitesimal{}, false
}

func (t *SMTLinearArithmeticTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.flushBoundAxioms(); err != nil {
		return err
	}
	if err = t.replayOwnEqualities(); err != nil {
		return err
	}
	t.drainOptimizationBounds()
	t.assertionHead = len(t.assertions)
	for _, assertion := range t.assertions {
		atom := &t.atoms[assertion.atom]
		if atom.decided && assertion.truth != atom.required {
			t.recordArithmeticConflict()
			literal := smtLinearAssertionLiteral(atom, assertion)
			return state.SetTheoryConflict(t.FamilyID(), []SMTLiteral{literal})
		}
	}
	feasible, err := t.rebuild()
	if err != nil || state.inconsistent {
		return err
	}
	if feasible {
		return t.propagateArithmeticBoundAtoms()
	}
	t.recordArithmeticConflict()
	literals := make([]SMTLiteral, 0, len(t.assertions))
	for _, assertion := range t.assertions {
		literal := NewSMTLiteral(t.atoms[assertion.atom].variable, false)
		if !assertion.truth {
			literal = literal.Negated()
		}
		literals = append(literals, literal)
	}
	return t.setArithmeticFallbackConflict(literals)
}

func (t *SMTLinearArithmeticTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.equalityAdapter.PushScope(); err != nil {
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	t.scopes = append(t.scopes, smtLinearArithmeticScope{equalityBounds: len(t.equalityBounds), atoms: len(t.atoms), assertions: len(t.assertions), head: t.assertionHead, unsupported: t.unsupported, castTerms: len(t.castTermTrail), castPredicates: len(t.castPredicateTrail), divisionTerms: len(t.divisionTermTrail), nonlinearTerms: len(t.nonlinearColumns), nonlinearRounds: t.nonlinearRounds, nonlinearGBExhausted: t.nonlinearGBExhausted, derivedBounds: len(t.derivedBounds), nonlinearStrategy: t.nonlinearStrategy})
	t.scopes[len(t.scopes)-1].assumeEqualities = len(t.assumeEqualityCandidates)
	t.scopes[len(t.scopes)-1].assumeEqualityHead = t.assumeEqualityHead
	t.scopes[len(t.scopes)-1].finalCheckIndex = t.finalCheckIndex
	t.scopes[len(t.scopes)-1].expressionBounds = len(t.expressionBounds)
	t.scopes[len(t.scopes)-1].pendingBounds = len(t.pendingBounds)
	t.scopes[len(t.scopes)-1].pendingBoundHead = t.pendingBoundHead
	t.scopes[len(t.scopes)-1].ownEqualities = len(t.ownEqualities)
	t.scopes[len(t.scopes)-1].ownEqualityHead = t.ownEqualityHead
	return nil
}

func (t *SMTLinearArithmeticTheory) PopScopes(count uint32) error {
	if count == 0 || int(count) > len(t.scopes) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	clear(t.ownEqualities[scope.ownEqualities:])
	t.ownEqualities = t.ownEqualities[:scope.ownEqualities]
	t.ownEqualityHead = scope.ownEqualityHead
	clear(t.equalityBounds[scope.equalityBounds:])
	t.equalityBounds = t.equalityBounds[:scope.equalityBounds]
	clear(t.pendingBounds[scope.pendingBounds:])
	t.pendingBounds = t.pendingBounds[:scope.pendingBounds]
	t.pendingBoundHead = scope.pendingBoundHead
	clear(t.expressionBounds[scope.expressionBounds:])
	t.expressionBounds = t.expressionBounds[:scope.expressionBounds]
	t.finalCheckIndex = scope.finalCheckIndex
	clear(t.assumeEqualityCandidates[scope.assumeEqualities:])
	t.assumeEqualityCandidates = t.assumeEqualityCandidates[:scope.assumeEqualities]
	t.assumeEqualityHead = scope.assumeEqualityHead
	for index := len(t.atoms); index > scope.atoms; index-- {
		delete(t.atomByBool, t.atoms[index-1].variable)
	}
	t.atoms = t.atoms[:scope.atoms]
	t.newBoundAtoms = nil // Source pop_scope_eh clears even unflushed parent atoms.
	t.assertions = t.assertions[:scope.assertions]
	t.assertionHead = scope.head
	t.unsupported = scope.unsupported
	for _, term := range t.castTermTrail[scope.castTerms:] {
		delete(t.castTerms, term)
	}
	t.castTermTrail = t.castTermTrail[:scope.castTerms]
	for _, variable := range t.castPredicateTrail[scope.castPredicates:] {
		delete(t.castPredicates, variable)
	}
	t.castPredicateTrail = t.castPredicateTrail[:scope.castPredicates]
	for _, term := range t.divisionTermTrail[scope.divisionTerms:] {
		delete(t.divisionTerms, term)
	}
	t.divisionTermTrail = t.divisionTermTrail[:scope.divisionTerms]
	for _, column := range t.nonlinearColumns[scope.nonlinearTerms:] {
		delete(t.nonlinearFactors, column)
	}
	t.nonlinearColumns = t.nonlinearColumns[:scope.nonlinearTerms]
	t.nonlinearRounds = scope.nonlinearRounds
	t.nonlinearGBExhausted = scope.nonlinearGBExhausted
	clear(t.derivedBounds[scope.derivedBounds:])
	t.derivedBounds = t.derivedBounds[:scope.derivedBounds]
	t.nonlinearStrategy = scope.nonlinearStrategy
	t.scopes = t.scopes[:level]
	if err := t.equalityAdapter.PopScopes(count); err != nil {
		return err
	}
	if err := t.SMTTheoryBase.PopScopes(count); err != nil {
		return err
	}
	for _, column := range t.simplexVariables[t.NumVariables():] {
		delete(t.integerColumns, column)
	}
	t.simplexVariables = t.simplexVariables[:t.NumVariables()]
	for variable := range t.definitions {
		if int(variable) >= t.NumVariables() {
			delete(t.definitions, variable)
		}
	}
	return func() error { _, err := t.rebuild(); return err }()
}

func (t *SMTLinearArithmeticTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if err := t.equalityAdapter.traceFinalCheckPairs(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.CanPropagate() {
		if err := t.Propagate(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		return SMTFinalCheckContinue, nil
	}
	if progress, err := t.propagateFixedSharedValues(); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		return SMTFinalCheckContinue, nil
	}
	propagated, err := t.propagateImpliedEqualities()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if propagated {
		return SMTFinalCheckContinue, nil
	}
	if progress, err := t.delayedAssumeEqualities(); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		return SMTFinalCheckContinue, nil
	}
	t.liberalFinalCheck, t.changedAssignment = true, false
	status, err := t.finalCheckArithmeticCore()
	if err != nil || status != SMTFinalCheckDone || !t.changedAssignment {
		return status, err
	}
	t.liberalFinalCheck, t.changedAssignment = false, false
	return t.finalCheckArithmeticCore()
}

type smtFixedPolynomialValue struct {
	value    big.Rat
	literals []SMTLiteral
}

func (t *SMTLinearArithmeticTheory) fixedLinearAssignments() map[uint32]smtFixedPolynomialValue {
	fixed := make(map[uint32]smtFixedPolynomialValue)
	for firstIndex, firstAssertion := range t.assertions {
		firstAtom := &t.atoms[firstAssertion.atom]
		firstForm, firstOK := smtLinearNormalizedLE(firstAtom, firstAssertion)
		if !firstOK || firstForm.infinity.Sign() != 0 || firstForm.infinitesimal.Sign() != 0 || len(firstForm.terms) != 1 {
			continue
		}
		for _, secondAssertion := range t.assertions[firstIndex+1:] {
			secondAtom := &t.atoms[secondAssertion.atom]
			secondForm, secondOK := smtLinearNormalizedLE(secondAtom, secondAssertion)
			if !secondOK || !smtLinearFormsOpposite(&firstForm, &secondForm) {
				continue
			}
			for variable, coefficient := range firstForm.terms {
				var value big.Rat
				value.Neg(&firstForm.constant)
				value.Quo(&value, coefficient)
				fixed[variable] = smtFixedPolynomialValue{value: value, literals: []SMTLiteral{
					smtLinearAssertionLiteral(firstAtom, firstAssertion),
					smtLinearAssertionLiteral(secondAtom, secondAssertion),
				}}
			}
			break
		}
	}
	return fixed
}

func (t *SMTLinearArithmeticTheory) fixedPolynomialValue(expression ast.ASTHandle, fixed map[uint32]smtFixedPolynomialValue) (*big.Rat, []SMTLiteral, bool, error) {
	if value, numeral, err := t.numeral(expression); err != nil || numeral {
		return value, nil, numeral, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil {
		return nil, nil, false, err
	}
	state, _ := t.State()
	if arithmetic {
		count, countErr := state.manager.GetNumArgs(expression)
		if countErr != nil {
			return nil, nil, false, countErr
		}
		values := make([]*big.Rat, count)
		var literals []SMTLiteral
		for index := uint32(0); index < count; index++ {
			argument, argErr := state.manager.GetArg(expression, index)
			if argErr != nil {
				return nil, nil, false, argErr
			}
			value, dependencies, isFixed, valueErr := t.fixedPolynomialValue(argument, fixed)
			if valueErr != nil || !isFixed {
				return nil, nil, isFixed, valueErr
			}
			values[index] = value
			literals = append(literals, dependencies...)
		}
		switch op {
		case smtArithOpAdd, smtArithOpSub:
			result := new(big.Rat)
			for index, value := range values {
				if op == smtArithOpSub && index != 0 {
					result.Sub(result, value)
				} else {
					result.Add(result, value)
				}
			}
			return result, literals, true, nil
		case smtArithOpUMinus:
			if count == 1 {
				return new(big.Rat).Neg(values[0]), literals, true, nil
			}
		case smtArithOpToReal:
			if count == 1 {
				return new(big.Rat).Set(values[0]), literals, true, nil
			}
		case smtArithOpMul:
			result := new(big.Rat).SetInt64(1)
			for _, value := range values {
				result.Mul(result, value)
			}
			return result, literals, true, nil
		case smtArithOpDiv:
			if count == 2 && values[1].Sign() != 0 {
				return new(big.Rat).Quo(values[0], values[1]), literals, true, nil
			}
		}
		return nil, nil, false, nil
	}
	node := state.context.enodes[expression]
	if node == nil || !t.IsAttachedToVariable(node) {
		return nil, nil, false, nil
	}
	theoryVariable := node.TheoryVar(t.FamilyID())
	if theoryVariable < 0 || int(theoryVariable) >= len(t.simplexVariables) {
		return nil, nil, false, nil
	}
	variable := t.simplexVariables[theoryVariable]
	value, found := fixed[variable]
	if !found {
		return nil, nil, false, nil
	}
	return new(big.Rat).Set(&value.value), append([]SMTLiteral(nil), value.literals...), true, nil
}

type smtPolynomialIntervals struct {
	manager   *smtArithmeticBoundDependencyManager
	variables map[uint32]*smtOldInterval
}

func (t *SMTLinearArithmeticTheory) linearVariableIntervals() (*smtPolynomialIntervals, error) {
	result := &smtPolynomialIntervals{
		manager:   &smtArithmeticBoundDependencyManager{},
		variables: make(map[uint32]*smtOldInterval),
	}
	type endpoints struct {
		lower, upper         *big.Rat
		lowerOpen, upperOpen bool
	}
	bounds := make(map[uint32]*endpoints)
	for _, assertion := range t.assertions {
		atom := &t.atoms[assertion.atom]
		if !atom.supported || len(atom.form.terms) != 1 {
			continue
		}
		if atom.native != nil {
			lower, value := smtLinearAtomBound(atom, assertion.truth, t.integralAtom(atom))
			if value.Infinity.Sign() != 0 {
				continue
			}
			for variable, coefficient := range atom.form.terms {
				value.Quo(coefficient)
				lower = lower == (coefficient.Sign() > 0)
				// A rational interval can represent ordinary strict endpoints,
				// but cannot project a lower r-epsilon or upper r+epsilon
				// onto r without strengthening it. Keep those endpoints in
				// the exact tableau and omit this rational-only shortcut.
				if lower && value.Epsilon.Sign() < 0 || !lower && value.Epsilon.Sign() > 0 {
					continue
				}
				open := value.Epsilon.Sign() != 0
				entry := bounds[variable]
				if entry == nil {
					entry = &endpoints{}
					bounds[variable] = entry
				}
				if lower {
					if entry.lower == nil || value.Main.Cmp(entry.lower) > 0 || value.Main.Cmp(entry.lower) == 0 && open {
						entry.lower = new(big.Rat).Set(&value.Main)
						entry.lowerOpen = open
					}
				} else if entry.upper == nil || value.Main.Cmp(entry.upper) < 0 || value.Main.Cmp(entry.upper) == 0 && open {
					entry.upper = new(big.Rat).Set(&value.Main)
					entry.upperOpen = open
				}
			}
			continue
		}
		op := smtLinearEffectiveOp(atom.op, assertion.truth)
		if op != smtArithOpLE && op != smtArithOpLT && op != smtArithOpGE && op != smtArithOpGT {
			continue
		}
		form := atom.form.clone()
		if op == smtArithOpGE || op == smtArithOpGT {
			form.scale(big.NewRat(-1, 1))
		}
		open := op == smtArithOpLT || op == smtArithOpGT
		for variable, coefficient := range form.terms {
			if coefficient.Sign() == 0 {
				continue
			}
			value := new(big.Rat).Neg(&form.constant)
			value.Quo(value, coefficient)
			entry := bounds[variable]
			if entry == nil {
				entry = &endpoints{}
				bounds[variable] = entry
			}
			if coefficient.Sign() > 0 {
				if entry.upper == nil || value.Cmp(entry.upper) < 0 || value.Cmp(entry.upper) == 0 && open {
					entry.upper, entry.upperOpen = value, open
				}
			} else if entry.lower == nil || value.Cmp(entry.lower) > 0 || value.Cmp(entry.lower) == 0 && open {
				entry.lower, entry.lowerOpen = value, open
			}
		}
	}
	for variable, entry := range bounds {
		var interval *smtOldInterval
		var err error
		switch {
		case entry.lower != nil && entry.upper != nil:
			interval, err = newSMTOldIntervalBounds(result.manager, entry.lower, entry.lowerOpen, nil, entry.upper, entry.upperOpen, nil)
		case entry.lower != nil:
			interval, err = newSMTOldIntervalHalfBound(result.manager, entry.lower, entry.lowerOpen, true, nil)
		case entry.upper != nil:
			interval, err = newSMTOldIntervalHalfBound(result.manager, entry.upper, entry.upperOpen, false, nil)
		}
		if err != nil {
			return nil, err
		}
		result.variables[variable] = interval
	}
	return result, nil
}

func (t *SMTLinearArithmeticTheory) polynomialInterval(expression ast.ASTHandle, intervals *smtPolynomialIntervals) (*smtOldInterval, bool, error) {
	if value, numeral, err := t.numeral(expression); err != nil || numeral {
		if err != nil || !numeral {
			return nil, false, err
		}
		result, pointErr := newSMTOldIntervalPoint(intervals.manager, value, nil, nil)
		return result, pointErr == nil, pointErr
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil {
		return nil, false, err
	}
	state, _ := t.State()
	if arithmetic {
		count, countErr := state.manager.GetNumArgs(expression)
		if countErr != nil {
			return nil, false, countErr
		}
		values := make([]*smtOldInterval, count)
		for index := uint32(0); index < count; index++ {
			argument, argErr := state.manager.GetArg(expression, index)
			if argErr != nil {
				return nil, false, argErr
			}
			value, evaluated, valueErr := t.polynomialInterval(argument, intervals)
			if valueErr != nil || !evaluated {
				return nil, evaluated, valueErr
			}
			values[index] = value
		}
		var result *smtOldInterval
		switch op {
		case smtArithOpAdd, smtArithOpSub:
			result, err = newSMTOldIntervalPoint(intervals.manager, new(big.Rat), nil, nil)
			for index, value := range values {
				if err != nil {
					break
				}
				if op == smtArithOpSub && index != 0 {
					err = result.sub(value)
				} else {
					err = result.add(value)
				}
			}
		case smtArithOpUMinus:
			if count == 1 {
				result = values[0].clone()
				result.neg()
			}
		case smtArithOpToReal:
			if count == 1 {
				result = values[0].clone()
			}
		case smtArithOpMul:
			result, err = newSMTOldIntervalPoint(intervals.manager, big.NewRat(1, 1), nil, nil)
			for _, value := range values {
				if err = result.mul(value); err != nil {
					break
				}
			}
		case smtArithOpDiv:
			if count == 2 {
				result = values[0].clone()
				err = result.div(values[1])
			}
		}
		if err != nil {
			return nil, false, nil
		}
		return result, result != nil, nil
	}
	node := state.context.enodes[expression]
	if node == nil || !t.IsAttachedToVariable(node) {
		return nil, false, nil
	}
	theoryVariable := node.TheoryVar(t.FamilyID())
	if theoryVariable < 0 || int(theoryVariable) >= len(t.simplexVariables) {
		return nil, false, nil
	}
	variable := t.simplexVariables[theoryVariable]
	if interval := intervals.variables[variable]; interval != nil {
		return interval.clone(), true, nil
	}
	result, err := newSMTOldInterval(intervals.manager)
	return result, err == nil, err
}

func (t *SMTLinearArithmeticTheory) checkNonlinearAssertions(validated bool) (bool, bool, error) {
	_, err := t.State()
	if err != nil {
		return false, false, err
	}
	fixed := t.fixedLinearAssignments()
	intervals, intervalErr := t.linearVariableIntervals()
	if intervalErr != nil {
		return false, false, intervalErr
	}
	for _, assertion := range t.assertions {
		atom := &t.atoms[assertion.atom]
		if atom.supported && (!atom.nonlinear || validated) || atom.decided {
			continue
		}
		left, leftLiterals, leftFixed, valueErr := t.fixedPolynomialValue(atom.left, fixed)
		if valueErr != nil {
			return false, false, valueErr
		}
		right, rightLiterals, rightFixed, valueErr := t.fixedPolynomialValue(atom.right, fixed)
		if valueErr != nil {
			return false, false, valueErr
		}
		if leftFixed && rightFixed {
			comparison := left.Cmp(right)
			truth := (atom.op == smtArithOpLE && comparison <= 0) ||
				(atom.op == smtArithOpGE && comparison >= 0) ||
				(atom.op == smtArithOpLT && comparison < 0) ||
				(atom.op == smtArithOpGT && comparison > 0)
			if truth == assertion.truth {
				continue
			}
			t.recordArithmeticConflict()
			literals := []SMTLiteral{smtLinearAssertionLiteral(atom, assertion)}
			literals = append(literals, leftLiterals...)
			literals = append(literals, rightLiterals...)
			if err = t.setArithmeticFallbackConflict(literals); err != nil {
				return false, false, err
			}
			return true, true, nil
		}
		difference, bounded, intervalErr := t.polynomialInterval(atom.left, intervals)
		if intervalErr != nil {
			return false, false, intervalErr
		}
		rightInterval, rightBounded, intervalErr := t.polynomialInterval(atom.right, intervals)
		if intervalErr != nil {
			return false, false, intervalErr
		}
		if !bounded || !rightBounded || difference.sub(rightInterval) != nil {
			return false, false, nil
		}
		op := smtLinearEffectiveOp(atom.op, assertion.truth)
		proved := (op == smtArithOpLE && difference.negative()) ||
			(op == smtArithOpLT && difference.negativeOne()) ||
			(op == smtArithOpGE && difference.positive()) ||
			(op == smtArithOpGT && difference.positiveOne())
		if proved {
			continue
		}
		refuted := (op == smtArithOpLE && difference.positiveOne()) ||
			(op == smtArithOpLT && difference.positive()) ||
			(op == smtArithOpGE && difference.negativeOne()) ||
			(op == smtArithOpGT && difference.negative())
		if !refuted {
			return false, false, nil
		}
		t.recordArithmeticConflict()
		literals := []SMTLiteral{smtLinearAssertionLiteral(atom, assertion)}
		for _, linearAssertion := range t.assertions {
			linearAtom := &t.atoms[linearAssertion.atom]
			if linearAtom.supported {
				literals = append(literals, smtLinearAssertionLiteral(linearAtom, linearAssertion))
			}
		}
		if err = t.setArithmeticFallbackConflict(literals); err != nil {
			return false, false, err
		}
		return true, true, nil
	}
	return true, false, nil
}

func (t *SMTLinearArithmeticTheory) InitializeSearch() error {
	t.conflictCount = 0
	t.finalCheckIndex = 0
	t.nonlinearStrategy = 0
	t.nonlinearGBExhausted = false
	return t.equalityAdapter.InitializeSearch()
}

// Source m_num_conflicts is per search and is not restored by scope pops.
// Lifetime statistics remain cumulative across searches.
func (t *SMTLinearArithmeticTheory) recordArithmeticConflict() {
	t.statistics.conflicts++
	t.conflictCount++
}

func (t *SMTLinearArithmeticTheory) canPropagateArithmeticEqualities() bool {
	return t.propagateEqualities && t.conflictCount < t.propagationThreshold
}
func (t *SMTLinearArithmeticTheory) Restart() error { return t.equalityAdapter.Restart() }

func (t *SMTLinearArithmeticTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	lower, err := state.manager.MkFamilyApp(t.FamilyID(), smtArithOpLE, nil, []ast.ASTHandle{left, right})
	if err != nil {
		return err
	}
	upper, err := state.manager.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{left, right})
	if err != nil {
		return err
	}
	if err = state.Internalize(lower, true); err != nil {
		return err
	}
	if err = state.Internalize(upper, true); err != nil {
		return err
	}
	lowerLiteral, err := state.Literal(lower)
	if err != nil {
		return err
	}
	upperLiteral, err := state.Literal(upper)
	if err != nil {
		return err
	}
	data, err := state.BooleanData(variable)
	if err != nil {
		return err
	}
	if data.TryTrueFirst() {
		if _, err = state.setScopedTrueFirst(lowerLiteral.Var()); err != nil {
			return err
		}
		if _, err = state.setScopedTrueFirst(upperLiteral.Var()); err != nil {
			return err
		}
	}
	equality := NewSMTLiteral(variable, false)
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality.Negated(), lowerLiteral}); err != nil {
		return err
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality.Negated(), upperLiteral}); err != nil {
		return err
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality, lowerLiteral.Negated(), upperLiteral.Negated()}); err != nil {
		return err
	}
	return t.equalityAdapter.AddRelevancy(left, right, expression, lower, upper)
}

func (t *SMTLinearArithmeticTheory) NewEquality(left, right SMTTheoryVar) error {
	if t.equalityBoundsEnabled {
		return t.queueCongruenceEqualityBounds(left, right)
	}
	return t.equalityAdapter.NewEquality(left, right)
}
func (t *SMTLinearArithmeticTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewDisequality(left, right)
}
func (*SMTLinearArithmeticTheory) Name() string { return "arithmetic" }

func (t *SMTLinearArithmeticTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint64("arith conflicts", t.statistics.conflicts)
	target.UpdateUint64("arith add rows", t.statistics.addRows)
	target.UpdateUint64("arith pivots", t.statistics.pivots)
	target.UpdateUint64("arith assert lower", t.statistics.assertLower)
	target.UpdateUint64("arith assert upper", t.statistics.assertUpper)
	target.UpdateUint64("arith fixed eqs", t.statistics.fixedEqualities)
	target.UpdateUint64("arith offset eqs", t.statistics.offsetEqualities)
	target.UpdateUint64("arith bound prop", t.statistics.boundPropagations)
	target.UpdateUint64("arith ineq splits", t.statistics.branches)
	target.UpdateUint64("arith eq adapter", t.equalityAdapter.NumAxioms())
	target.UpdateUint64("arith grobner", t.statistics.grobner)
	target.UpdateUint64("arith nonlinear horner", t.statistics.crossNested)
	target.UpdateUint64("arith max-min", t.statistics.maxMin)
	target.UpdateUint64("arith gcd tests", t.statistics.gcdTests)
	target.UpdateUint64("arith gcd conflicts", t.statistics.gcdConflicts)
	target.UpdateUint64("arith gomory cuts", t.statistics.gomoryCuts)
	target.UpdateUint64("arith eq branches", t.statistics.integerEqualityBranches)
	target.UpdateUint64("arith euclidean calls", t.statistics.euclideanCalls)
	target.UpdateUint64("arith euclidean bounds", t.statistics.euclideanBounds)
	return nil
}

func (t *SMTLinearArithmeticTheory) ResetStatistics() {
	if t != nil {
		t.statistics = smtLinearArithmeticStatistics{}
		t.equalityAdapter.ResetStatistics()
	}
}

func (t *SMTLinearArithmeticTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if err = t.computeModelDelta(); err != nil {
		return err
	}
	factory, err := protomodel.NewArithmeticFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func (t *SMTLinearArithmeticTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil || generator.manager != state.manager {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	theoryVariable := node.TheoryVar(t.FamilyID())
	rational := t.arithmeticColumnValue(t.simplexVariables[theoryVariable])
	sort, err := state.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}
	kind, err := state.manager.GetDeclKind(sort)
	if err != nil {
		return nil, err
	}
	// Native mk_value floors fractional Int candidates when incomplete
	// arithmetic (including arith.ignore_int) leaves a nonintegral assignment.
	if kind == 1 && !rational.IsInt() {
		rational.SetInt(smtRatFloor(rational))
	}
	numeral, err := state.manager.ArithmeticNumeralRat(rational, kind == 1)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(numeral)
}

func (t *SMTLinearArithmeticTheory) computeModelDelta() error {
	t.delta.SetInt64(1)
	for variable := uint32(0); variable < t.solver.NumVariables(); variable++ {
		value := t.solver.Value(variable)
		for _, item := range []struct {
			bound simplex.Infinitesimal
			valid bool
			lower bool
		}{
			func() struct {
				bound        simplex.Infinitesimal
				valid, lower bool
			} {
				b, ok := t.solver.Lower(variable)
				return struct {
					bound        simplex.Infinitesimal
					valid, lower bool
				}{b, ok, true}
			}(),
			func() struct {
				bound        simplex.Infinitesimal
				valid, lower bool
			} {
				b, ok := t.solver.Upper(variable)
				return struct {
					bound        simplex.Infinitesimal
					valid, lower bool
				}{b, ok, false}
			}(),
		} {
			if !item.valid {
				continue
			}
			var main, epsilon big.Rat
			main.Sub(&value.Main, &item.bound.Main)
			epsilon.Sub(&value.Epsilon, &item.bound.Epsilon)
			if !item.lower {
				main.Neg(&main)
				epsilon.Neg(&epsilon)
			}
			if main.Sign() > 0 && epsilon.Sign() < 0 {
				candidate := new(big.Rat).Quo(&main, new(big.Rat).Neg(&epsilon))
				if candidate.Cmp(&t.delta) < 0 {
					t.delta.Set(candidate)
				}
			}
		}
	}
	return t.refineModelDelta()
}
