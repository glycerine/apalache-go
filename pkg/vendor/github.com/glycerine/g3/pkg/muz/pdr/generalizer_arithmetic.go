package pdr

// Source: core_arith_inductive_generalizer in
// src/muz/pdr/pdr_generalizers.cpp and pdr_generalizers.h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

const arithmeticInductiveMod int32 = 13

type arithmeticInductiveBound struct {
	term  ast.ASTHandle
	value big.Rat
	index int
}

type arithmeticInductiveMatch struct {
	term         ast.ASTHandle
	value        big.Rat
	lower, upper int
}

type arithmeticInductiveBounds struct {
	transformer *PredicateTransformer
	lower       map[string]arithmeticInductiveBound
	upper       map[string]arithmeticInductiveBound
	lowerOrder  []string
	references  []ast.ASTRef
}

func newArithmeticInductiveBounds(transformer *PredicateTransformer) (*arithmeticInductiveBounds, error) {
	if transformer == nil {
		return nil, ErrPredicateTransformerOwner
	}
	if err := transformer.open(); err != nil {
		return nil, err
	}
	return &arithmeticInductiveBounds{
		transformer: transformer,
		lower:       make(map[string]arithmeticInductiveBound),
		upper:       make(map[string]arithmeticInductiveBound),
	}, nil
}

func (b *arithmeticInductiveBounds) pin(handle ast.ASTHandle) error {
	reference, err := b.transformer.manager.NewRef(handle)
	if err != nil {
		return err
	}
	b.references = append(b.references, reference)
	return nil
}

func (b *arithmeticInductiveBounds) close() error {
	if b == nil || b.transformer == nil {
		return nil
	}
	result := closeASTRefs(b.transformer.manager, b.references)
	b.references = nil
	return result
}

func arithmeticInductiveRelation(manager *ast.ASTManager, expression ast.ASTHandle) (operator int32, left, right ast.ASTHandle, matched bool, err error) {
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil || family != arithmetic {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	operator, err = manager.GetDeclKind(expression)
	if err != nil || operator != closureArithLE && operator != closureArithGE {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	arguments, err := manager.AppArgumentView(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if arguments.Count() != 2 {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	left, err = arguments.At(0)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err = arguments.At(1)
	return operator, left, right, err == nil, err
}

func arithmeticInductiveNumeral(manager *ast.ASTManager, expression ast.ASTHandle) (big.Rat, bool, error) {
	text, _, matched, err := manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return big.Rat{}, matched, err
	}
	value := new(big.Rat)
	if _, ok := value.SetString(text); !ok {
		return big.Rat{}, false, ast.ErrASTDomain
	}
	return *value, true, nil
}

func arithmeticInductiveInteger(manager *ast.ASTManager, expression ast.ASTHandle) (bool, error) {
	sort, err := manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	family, err := manager.GetFamilyID(sort)
	if err != nil || family != arithmetic {
		return false, err
	}
	kind, err := manager.GetDeclKind(sort)
	return kind == 1, err
}

func (b *arithmeticInductiveBounds) insert(lower bool, term ast.ASTHandle, value *big.Rat, index int) error {
	manager := b.transformer.manager
	normalized := new(big.Rat).Set(value)
	if normalized.Sign() < 0 {
		negated, err := manager.MkArithmeticNeg(term)
		if err != nil {
			return err
		}
		if err = b.pin(negated); err != nil {
			return err
		}
		term = negated
		lower = !lower
		normalized.Neg(normalized)
	}
	key := normalized.RatString()
	bound := arithmeticInductiveBound{term: term, value: *normalized, index: index}
	if lower {
		if _, found := b.lower[key]; !found {
			b.lowerOrder = append(b.lowerOrder, key)
		}
		// Source map::insert overwrites the vector at an existing magnitude.
		b.lower[key] = bound
	} else {
		b.upper[key] = bound
	}
	return nil
}

func (b *arithmeticInductiveBounds) collect(core []ast.ASTHandle) error {
	manager := b.transformer.manager
	one := big.NewRat(1, 1)
	for index, expression := range core {
		inner := expression
		negated, child, err := manager.IsNot(expression)
		if err != nil {
			return err
		}
		if negated {
			inner = child
		}
		operator, term, numeral, relation, err := arithmeticInductiveRelation(manager, inner)
		if err != nil {
			return err
		}
		if !relation {
			continue
		}
		value, found, err := arithmeticInductiveNumeral(manager, numeral)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if negated {
			integer, err := arithmeticInductiveInteger(manager, term)
			if err != nil {
				return err
			}
			if !integer {
				continue
			}
			switch operator {
			case closureArithLE:
				value.Add(&value, one)
				if err = b.insert(true, term, &value, index); err != nil {
					return err
				}
			case closureArithGE:
				value.Sub(&value, one)
				if err = b.insert(false, term, &value, index); err != nil {
					return err
				}
			}
			continue
		}
		if err = b.insert(operator == closureArithGE, term, &value, index); err != nil {
			return err
		}
	}
	return nil
}

func (b *arithmeticInductiveBounds) equivalent(left, right ast.ASTHandle) (_ bool, resultErr error) {
	if left == right {
		return true, nil
	}
	manager := b.transformer.manager
	equality, err := manager.MkEq(left, right)
	if err != nil {
		return false, err
	}
	equalityRef, err := manager.NewRef(equality)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&equalityRef)) }()
	config := b.transformer.owner.contexts.config
	theory, err := rewriter.NewTheoryRewriter(manager, config.Params, config.Globals, nil)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, theory.Close()) }()
	rewritten, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&rewritten)) }()
	if err = theory.Rewrite(equality, rewritten); err != nil {
		return false, err
	}
	result, err := manager.RefGet(rewritten)
	if err != nil {
		return false, err
	}
	return manager.IsTrue(result), nil
}

func (b *arithmeticInductiveBounds) matches() ([]arithmeticInductiveMatch, error) {
	two := big.NewRat(2, 1)
	result := make([]arithmeticInductiveMatch, 0, len(b.lowerOrder))
	// The source hash-table exposes no semantic ordering. Encounter order makes
	// the Go result deterministic without porting its hashing implementation.
	for _, key := range b.lowerOrder {
		lower := b.lower[key]
		if lower.value.Cmp(two) < 0 {
			continue
		}
		upper, found := b.upper[key]
		if !found {
			continue
		}
		equal, err := b.equivalent(lower.term, upper.term)
		if err != nil {
			return nil, err
		}
		if equal {
			result = append(result, arithmeticInductiveMatch{term: lower.term, value: lower.value, lower: lower.index, upper: upper.index})
		}
	}
	return result, nil
}

func pinArithmeticInductive(manager *ast.ASTManager, references *[]ast.ASTRef, handle ast.ASTHandle) error {
	reference, err := manager.NewRef(handle)
	if err != nil {
		return err
	}
	*references = append(*references, reference)
	return nil
}

func arithmeticInductiveOne(manager *ast.ASTManager, term ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := arithmeticInductiveInteger(manager, term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.ArithmeticNumeralRat(big.NewRat(1, 1), integer)
}

func substituteArithmeticInductiveAlias(manager *ast.ASTManager, matchedValue *big.Rat, matchedTerm, expression ast.ASTHandle, references *[]ast.ASTRef) (ast.ASTHandle, bool, error) {
	negated, child, err := manager.IsNot(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if negated {
		rewritten, changed, err := substituteArithmeticInductiveAlias(manager, matchedValue, matchedTerm, child, references)
		if err != nil || !changed {
			return ast.ASTHandle{}, false, err
		}
		result, err := manager.MkNot(rewritten)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if err = pinArithmeticInductive(manager, references, result); err != nil {
			return ast.ASTHandle{}, false, err
		}
		return result, true, nil
	}
	operator, term, numeral, relation, err := arithmeticInductiveRelation(manager, expression)
	if err != nil || !relation {
		return ast.ASTHandle{}, false, err
	}
	value, found, err := arithmeticInductiveNumeral(manager, numeral)
	if err != nil || !found {
		return ast.ASTHandle{}, false, err
	}
	comparison := value.Cmp(matchedValue)
	var result ast.ASTHandle
	switch operator {
	case closureArithLE:
		switch comparison {
		case 0:
			result, err = manager.MkArithmeticLE(term, matchedTerm)
		case -1:
			family, x := manager.ArithmeticFamilyID()
			if x != nil {
				return ast.ASTHandle{}, false, x
			}
			result, err = manager.MkFamilyApp(family, closureArithLT, nil, []ast.ASTHandle{term, matchedTerm})
		case 1:
			one, x := arithmeticInductiveOne(manager, matchedTerm)
			if x != nil {
				return ast.ASTHandle{}, false, x
			}
			right, x := manager.MkArithmeticSub(matchedTerm, one)
			if x != nil {
				return ast.ASTHandle{}, false, x
			}
			if x = pinArithmeticInductive(manager, references, right); x != nil {
				return ast.ASTHandle{}, false, x
			}
			result, err = manager.MkArithmeticLE(term, right)
		default:
			return ast.ASTHandle{}, false, nil
		}
	case closureArithGE:
		switch comparison {
		case 0:
			result, err = manager.MkArithmeticGE(term, matchedTerm)
		case 1:
			family, x := manager.ArithmeticFamilyID()
			if x != nil {
				return ast.ASTHandle{}, false, x
			}
			result, err = manager.MkFamilyApp(family, closureArithGT, nil, []ast.ASTHandle{term, matchedTerm})
		case -1:
			one, x := arithmeticInductiveOne(manager, matchedTerm)
			if x != nil {
				return ast.ASTHandle{}, false, x
			}
			right, x := manager.MkArithmeticSub(matchedTerm, one)
			if x != nil {
				return ast.ASTHandle{}, false, x
			}
			if x = pinArithmeticInductive(manager, references, right); x != nil {
				return ast.ASTHandle{}, false, x
			}
			result, err = manager.MkArithmeticGE(term, right)
		default:
			return ast.ASTHandle{}, false, nil
		}
	}
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = pinArithmeticInductive(manager, references, result); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return result, true, nil
}

// arithmeticInductiveCandidate returns a candidate plus references owning all
// synthesized terms until the caller completes its inductiveness check.
func arithmeticInductiveCandidate(transformer *PredicateTransformer, input []ast.ASTHandle) (_ []ast.ASTHandle, resultReferences []ast.ASTRef, _ bool, resultErr error) {
	bounds, err := newArithmeticInductiveBounds(transformer)
	if err != nil {
		return nil, nil, false, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, bounds.close())
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(transformer.manager, resultReferences))
			resultReferences = nil
		}
	}()
	if len(input) <= 1 {
		return append([]ast.ASTHandle(nil), input...), nil, false, nil
	}
	if err = bounds.collect(input); err != nil {
		return nil, nil, false, err
	}
	matches, err := bounds.matches()
	if err != nil {
		return nil, nil, false, err
	}
	if len(matches) == 0 {
		return append([]ast.ASTHandle(nil), input...), nil, false, nil
	}
	manager := transformer.manager
	result := append([]ast.ASTHandle(nil), input...)
	truth, err := manager.MkTrue()
	if err != nil {
		return nil, nil, false, err
	}
	for _, match := range matches {
		result[match.upper] = truth
		result[match.lower] = truth
		for index, expression := range result {
			rewritten, changed, x := substituteArithmeticInductiveAlias(manager, &match.value, match.term, expression, &resultReferences)
			if x != nil {
				return nil, nil, false, x
			}
			if changed {
				result[index] = rewritten
			}
		}
		integer, err := arithmeticInductiveInteger(manager, match.term)
		if err != nil {
			return nil, nil, false, err
		}
		if integer {
			two, x := manager.ArithmeticIntegerNumeralText("2")
			if x != nil {
				return nil, nil, false, x
			}
			zero, x := manager.ArithmeticIntegerNumeralText("0")
			if x != nil {
				return nil, nil, false, x
			}
			family, x := manager.ArithmeticFamilyID()
			if x != nil {
				return nil, nil, false, x
			}
			modulo, x := manager.MkFamilyApp(family, arithmeticInductiveMod, nil, []ast.ASTHandle{match.term, two})
			if x != nil {
				return nil, nil, false, x
			}
			if x = pinArithmeticInductive(manager, &resultReferences, modulo); x != nil {
				return nil, nil, false, x
			}
			parity, x := manager.MkEq(modulo, zero)
			if x != nil {
				return nil, nil, false, x
			}
			if x = pinArithmeticInductive(manager, &resultReferences, parity); x != nil {
				return nil, nil, false, x
			}
			nonpositive, x := manager.MkArithmeticLE(match.term, zero)
			if x != nil {
				return nil, nil, false, x
			}
			if x = pinArithmeticInductive(manager, &resultReferences, nonpositive); x != nil {
				return nil, nil, false, x
			}
			result[match.lower] = parity
			result[match.upper] = nonpositive
		}
	}
	return result, resultReferences, true, nil
}

// generalizeArithmeticInductiveCore implements the optional source arithmetic
// lattice heuristic. Its result owns every literal independently of input.
func generalizeArithmeticInductiveCore(transformer *PredicateTransformer, level uint32, input generalizedCore) (_ generalizedCore, resultErr error) {
	if transformer == nil {
		return generalizedCore{}, ErrPredicateTransformerOwner
	}
	candidate, workspace, changed, err := arithmeticInductiveCandidate(transformer, input.literals)
	if err != nil {
		return generalizedCore{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(transformer.manager, workspace)) }()
	if !changed {
		return pinGeneralizedCore(transformer.manager, input.literals, input.usesLevel)
	}
	inductive, assumesLevel, reduced, err := transformer.CheckInductive(level, candidate)
	if err != nil {
		return generalizedCore{}, err
	}
	if !inductive {
		return pinGeneralizedCore(transformer.manager, input.literals, input.usesLevel)
	}
	return pinGeneralizedCore(transformer.manager, reduced, assumesLevel)
}
