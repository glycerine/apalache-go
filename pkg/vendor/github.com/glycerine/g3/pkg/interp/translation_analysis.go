package interp

// Source checkpoint: src/interp/iz3translate.cpp proof-DAG analysis,
// clause extraction, and resolution helpers.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

type translationEquivalence struct {
	local ast.ASTHandle
	proof ast.ASTHandle
}

// FullTranslation is the retained IZ3_TRANSLATE_FULL implementation state.
// It borrows Base and owns its translator-specific declarations.
type FullTranslation struct {
	self                *FullTranslation
	base                *Base
	frames              int
	commute             ast.ASTHandle
	owned               []ast.ASTRef
	hypotheses          map[ast.ASTHandle]map[ast.ASTHandle]struct{}
	equivalences        map[ast.ASTHandle]translationEquivalence
	equivalencesVisited map[ast.ASTHandle]bool
	locality            map[ast.ASTHandle]int
	closed              bool
}

func NewFullTranslation(base *Base) (result *FullTranslation, resultErr error) {
	if base == nil || base.open() != nil {
		return nil, ast.ErrASTHandle
	}
	t := &FullTranslation{
		base:                base,
		frames:              len(base.constraints),
		hypotheses:          make(map[ast.ASTHandle]map[ast.ASTHandle]struct{}),
		equivalences:        make(map[ast.ASTHandle]translationEquivalence),
		equivalencesVisited: make(map[ast.ASTHandle]bool),
		locality:            make(map[ast.ASTHandle]int),
	}
	t.self = t
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, t.Close())
		}
	}()
	boolean, err := base.manager.BoolType()
	if err != nil {
		return nil, err
	}
	t.commute, err = base.manager.Function("@commute", []ast.ASTHandle{boolean, boolean}, boolean)
	if err != nil {
		return nil, err
	}
	ref, err := base.manager.ast.NewRef(t.commute)
	if err != nil {
		return nil, err
	}
	t.owned = append(t.owned, ref)
	return t, nil
}

func (t *FullTranslation) open() error {
	if t == nil || t.self != t || t.closed || t.base == nil {
		return ast.ErrASTHandle
	}
	return t.base.open()
}

func (t *FullTranslation) Close() (result error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	for i := len(t.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, t.base.manager.ast.DelRef(&t.owned[i]))
	}
	t.owned = nil
	t.hypotheses = nil
	t.equivalences = nil
	t.equivalencesVisited = nil
	t.locality = nil
	t.closed = true
	return result
}

func (t *FullTranslation) scanSkolemsRecursive(proof ast.ASTHandle, frame int, memo map[ast.ASTHandle]int) (int, error) {
	if result, found := memo[proof]; found {
		return result, nil
	}
	result := math.MaxInt
	memo[proof] = result
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil {
		return 0, err
	}
	switch rule {
	case ast.ASTPrAsserted:
		conclusion, e := t.base.manager.Conclusion(proof)
		if e != nil {
			return 0, e
		}
		result, err = t.base.FrameOfAssertion(conclusion)
	case ast.ASTPrSkolemize:
		conclusion, e := t.base.manager.Conclusion(proof)
		if e != nil {
			return 0, e
		}
		quantified, e := t.base.manager.Arg(conclusion, 0)
		if e != nil {
			return 0, e
		}
		op, e := t.base.manager.Operator(quantified)
		if e != nil {
			return 0, e
		}
		if op == OperatorNot {
			quantified, e = t.base.manager.Arg(quantified, 0)
			if e != nil {
				return 0, e
			}
		}
		scope, e := t.base.ASTScope(quantified)
		if e != nil {
			return 0, e
		}
		if scope.IsEmpty() {
			return 0, NewError("can't skolemize")
		}
		inside, e := t.base.scopes.InRange(frame, scope)
		if e != nil {
			return 0, e
		}
		if frame == math.MaxInt || !inside {
			frame = scope.Max()
		}
		if frame >= t.frames {
			frame = t.frames - 1
		}
		skolem, e := t.base.manager.Arg(conclusion, 1)
		if e != nil {
			return 0, e
		}
		err = t.base.addFrameRange(frame, skolem)
	case ast.ASTPrModusPonensOEq:
		first, e := t.base.manager.Premise(proof, 0)
		if e != nil {
			return 0, e
		}
		frame, e = t.scanSkolemsRecursive(first, frame, memo)
		if e != nil {
			return 0, e
		}
		second, e := t.base.manager.Premise(proof, 1)
		if e != nil {
			return 0, e
		}
		_, err = t.scanSkolemsRecursive(second, frame, memo)
	default:
		count, e := t.base.manager.NumPremises(proof)
		if e != nil {
			return 0, e
		}
		for i := uint32(0); i < count; i++ {
			premise, e := t.base.manager.Premise(proof, i)
			if e != nil {
				return 0, e
			}
			value, e := t.scanSkolemsRecursive(premise, frame, memo)
			if e != nil {
				return 0, e
			}
			if result == math.MaxInt || result == value {
				result = value
			} else if value != math.MaxInt {
				result = -1
			}
		}
	}
	memo[proof] = result
	return result, err
}

func (t *FullTranslation) ScanSkolems(proof ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	_, err := t.scanSkolemsRecursive(proof, math.MaxInt, make(map[ast.ASTHandle]int))
	return err
}

// Quantify preserves the retained iz3translation default. The alternative
// localization-variable quantifier machinery in iz3translate.cpp is disabled
// by the selected source's #if 0 block.
func (t *FullTranslation) Quantify(term ast.ASTHandle, _ ScopeRange) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return term, nil
}

func (t *FullTranslation) localityRecursive(proof ast.ASTHandle) (int, error) {
	if result, found := t.locality[proof]; found {
		return result, nil
	}
	result := math.MaxInt
	// Match the source's insert-before-recursion memo behavior.
	t.locality[proof] = result
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil {
		return 0, err
	}
	if rule == ast.ASTPrAsserted {
		conclusion, e := t.base.manager.Conclusion(proof)
		if e != nil {
			return 0, e
		}
		result, err = t.base.FrameOfAssertion(conclusion)
	} else if rule != ast.ASTPrQuantInst {
		count, e := t.base.manager.NumPremises(proof)
		if e != nil {
			return 0, e
		}
		for i := uint32(0); i < count; i++ {
			premise, e := t.base.manager.Premise(proof, i)
			if e != nil {
				return 0, e
			}
			value, e := t.localityRecursive(premise)
			if e != nil {
				return 0, e
			}
			if result == math.MaxInt || result == value {
				result = value
			} else if value != math.MaxInt {
				result = -1
			}
		}
	}
	t.locality[proof] = result
	return result, err
}

func (t *FullTranslation) Locality(proof ast.ASTHandle) (int, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	result, err := t.localityRecursive(proof)
	if err != nil || result == -1 {
		return result, err
	}
	conclusion, err := t.base.manager.Conclusion(proof)
	if err != nil {
		return 0, err
	}
	scope, err := t.base.ASTScope(conclusion)
	if err != nil {
		return 0, err
	}
	if !scope.IsEmpty() {
		hypotheses, e := t.proofHypotheses(proof)
		if e != nil {
			return 0, e
		}
		for hypothesis := range hypotheses {
			hypothesisScope, e := t.base.ASTScope(hypothesis)
			if e != nil {
				return 0, e
			}
			scope, e = t.base.scopes.RangeGLB(scope, hypothesisScope)
			if e != nil {
				return 0, e
			}
		}
	}
	if result == math.MaxInt {
		if scope.IsEmpty() {
			return -1, nil
		}
		return scope.Max(), nil
	}
	inside, err := t.base.scopes.InRange(result, scope)
	if err != nil {
		return 0, err
	}
	if !inside {
		return -1, nil
	}
	return result, nil
}

func cloneHandleSet(source map[ast.ASTHandle]struct{}) map[ast.ASTHandle]struct{} {
	result := make(map[ast.ASTHandle]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}

func (t *FullTranslation) proofHypotheses(proof ast.ASTHandle) (map[ast.ASTHandle]struct{}, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if result, found := t.hypotheses[proof]; found {
		return result, nil
	}
	result := make(map[ast.ASTHandle]struct{})
	// Publish before recursion, matching the source memo insertion and making
	// shared proof DAGs linear in the number of proof nodes.
	t.hypotheses[proof] = result
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil {
		return nil, err
	}
	if rule == ast.ASTPrHypothesis {
		conclusion, e := t.base.manager.Conclusion(proof)
		if e != nil {
			return nil, e
		}
		result[conclusion] = struct{}{}
		return result, nil
	}
	count, err := t.base.manager.NumPremises(proof)
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < count; i++ {
		premise, e := t.base.manager.Premise(proof, i)
		if e != nil {
			return nil, e
		}
		values, e := t.proofHypotheses(premise)
		if e != nil {
			return nil, e
		}
		for value := range values {
			result[value] = struct{}{}
		}
	}
	if rule == ast.ASTPrLemma {
		conclusion, e := t.base.manager.Conclusion(proof)
		if e != nil {
			return nil, e
		}
		negated, e := t.base.manager.MkNot(conclusion)
		if e != nil {
			return nil, e
		}
		delete(result, negated)
		op, e := t.base.manager.Operator(conclusion)
		if e != nil {
			return nil, e
		}
		if op == OperatorOr {
			arguments, e := t.base.manager.Args(conclusion)
			if e != nil {
				return nil, e
			}
			for _, argument := range arguments {
				negated, e = t.base.manager.MkNot(argument)
				if e != nil {
					return nil, e
				}
				delete(result, negated)
			}
		}
	}
	return result, nil
}

// ProofHypotheses returns a copy of the source hypothesis set for proof.
func (t *FullTranslation) ProofHypotheses(proof ast.ASTHandle) (map[ast.ASTHandle]struct{}, error) {
	values, err := t.proofHypotheses(proof)
	if err != nil {
		return nil, err
	}
	return cloneHandleSet(values), nil
}

func (t *FullTranslation) findEquivalences(proof ast.ASTHandle) error {
	if t.equivalencesVisited[proof] {
		return nil
	}
	t.equivalencesVisited[proof] = true
	count, err := t.base.manager.NumPremises(proof)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		premise, e := t.base.manager.Premise(proof, i)
		if e != nil {
			return e
		}
		if e = t.findEquivalences(premise); e != nil {
			return e
		}
	}
	conclusion, err := t.base.manager.Conclusion(proof)
	if err != nil {
		return err
	}
	op, err := t.base.manager.Operator(conclusion)
	if err != nil || op != OperatorIff {
		return err
	}
	arguments, err := t.base.manager.Args(conclusion)
	if err != nil || len(arguments) != 2 {
		return err
	}
	for i := 0; i < 2; i++ {
		remoteLocal, e := t.base.IsLocal(arguments[i])
		if e != nil {
			return e
		}
		local, e := t.base.IsLocal(arguments[1-i])
		if e != nil {
			return e
		}
		if !remoteLocal && local {
			if _, exists := t.equivalences[arguments[i]]; !exists {
				t.equivalences[arguments[i]] = translationEquivalence{local: arguments[1-i], proof: proof}
			}
		}
	}
	return nil
}

func (t *FullTranslation) FindEquivalences(proof ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.findEquivalences(proof)
}

func (t *FullTranslation) EquivalentLocal(term ast.ASTHandle) (local, proof ast.ASTHandle, found bool, err error) {
	if err = t.open(); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	entry, found := t.equivalences[term]
	return entry.local, entry.proof, found, nil
}

func (t *FullTranslation) clauseLiterals(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	op, err := t.base.manager.Operator(term)
	if err != nil {
		return nil, err
	}
	if op == OperatorFalse {
		return nil, nil
	}
	if op == OperatorOr {
		return t.base.manager.Args(term)
	}
	return []ast.ASTHandle{term}, nil
}

// ClauseLiterals returns the source vector representation of a clause.
func (t *FullTranslation) ClauseLiterals(term ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.clauseLiterals(term)
}

func (t *FullTranslation) resolveClause(pivot ast.ASTHandle, first, second []ast.ASTHandle) ([]ast.ASTHandle, error) {
	negated, err := t.base.manager.MkNot(pivot)
	if err != nil {
		return nil, err
	}
	result := append([]ast.ASTHandle(nil), first...)
	for i, literal := range result {
		if literal != pivot {
			continue
		}
		result[i] = result[len(result)-1]
		result = result[:len(result)-1]
		found := false
		for _, candidate := range second {
			if candidate == negated {
				found = true
			} else {
				result = append(result, candidate)
			}
		}
		if !found {
			return nil, ProofError{}
		}
		return result, nil
	}
	return nil, ProofError{}
}

func (t *FullTranslation) UnitResolutionLiterals(proof ast.ASTHandle, position int) ([]ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	count, err := t.base.manager.NumPremises(proof)
	if err != nil || position < 0 || uint32(position) >= count {
		return nil, ast.ErrASTDomain
	}
	first, err := t.base.manager.Premise(proof, 0)
	if err != nil {
		return nil, err
	}
	conclusion, err := t.base.manager.Conclusion(first)
	if err != nil {
		return nil, err
	}
	literals, err := t.clauseLiterals(conclusion)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= position; i++ {
		premise, e := t.base.manager.Premise(proof, uint32(i))
		if e != nil {
			return nil, e
		}
		unit, e := t.base.manager.Conclusion(premise)
		if e != nil {
			return nil, e
		}
		negated, e := t.base.manager.MkNot(unit)
		if e != nil {
			return nil, e
		}
		literals, e = t.resolveClause(negated, literals, []ast.ASTHandle{unit})
		if e != nil {
			return nil, e
		}
	}
	return literals, nil
}
