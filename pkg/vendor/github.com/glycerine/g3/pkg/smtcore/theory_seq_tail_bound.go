package smtcore

// Source: src/smt/theory_seq.cpp propagate_max_length.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTSequenceTheory) sequenceTailArguments(expression ast.ASTHandle) (source, index ast.ASTHandle, matched bool, result error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own || op != 42 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	decl, err := state.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	name, err := state.manager.GetASTName(decl)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	text, err := state.manager.Symbols.String(name)
	if err != nil || text != "seq.tail" {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	source, err = state.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	index, err = state.manager.GetArg(expression, 1)
	return source, index, err == nil, err
}

func (t *SMTSequenceTheory) addTailEmptyMaximum(left, right []ast.ASTHandle) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) == 1 && len(right) == 0 {
			source, index, tail, err := t.sequenceTailArguments(left[0])
			if err != nil {
				return false, err
			}
			if tail {
				position, numeral, valueErr := t.groundInteger(index)
				if valueErr != nil {
					return false, valueErr
				}
				if numeral && position.Sign() >= 0 {
					state, stateErr := t.State()
					if stateErr != nil {
						return false, stateErr
					}
					sort, sortErr := state.manager.GetSort(left[0])
					if sortErr != nil {
						return false, sortErr
					}
					empty, makeErr := state.manager.MkSequenceEmpty(sort)
					if makeErr != nil {
						return false, makeErr
					}
					tailEmpty, makeErr := state.manager.MkEq(left[0], empty)
					if makeErr != nil {
						return false, makeErr
					}
					length, makeErr := state.manager.MkSequenceLength(source)
					if makeErr != nil {
						return false, makeErr
					}
					arithmeticFamily, makeErr := state.manager.ArithmeticFamilyID()
					if makeErr != nil {
						return false, makeErr
					}
					if arithmetic, ok := state.Theory(arithmeticFamily).(*SMTLinearArithmeticTheory); ok {
						intervals, intervalErr := arithmetic.linearVariableIntervals()
						if intervalErr != nil {
							return false, intervalErr
						}
						interval, evaluated, intervalErr := arithmetic.polynomialInterval(length, intervals)
						if intervalErr != nil {
							return false, intervalErr
						}
						if evaluated && !interval.plusInfinity() {
							return false, nil
						}
					}
					maximum := new(big.Int).Add(position, big.NewInt(1))
					bound, makeErr := state.manager.ArithmeticIntegerNumeralText(maximum.String())
					if makeErr != nil {
						return false, makeErr
					}
					atMost, makeErr := state.manager.MkArithmeticLE(length, bound)
					if makeErr != nil {
						return false, makeErr
					}
					constraint, makeErr := state.manager.MkImplies(tailEmpty, atMost)
					if makeErr != nil {
						return false, makeErr
					}
					if _, found := t.axioms.set[constraint]; found {
						return false, nil
					}
					t.trackInternalTerms(left[0])
					return true, t.axioms.Push(constraint)
				}
			}
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) addSolvedTailEmptyMaximum() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	seen := make(map[ast.ASTHandle]struct{})
	for _, node := range t.variables {
		if node == nil {
			continue
		}
		tail := node.Owner()
		if _, found := seen[tail]; found {
			continue
		}
		seen[tail] = struct{}{}
		if _, internallyDefined := t.internalTerms[tail]; internallyDefined {
			continue
		}
		if _, _, matched, matchErr := t.sequenceTailArguments(tail); matchErr != nil {
			return false, matchErr
		} else if !matched {
			continue
		}
		representative, _, findErr := t.solutions.Find(tail)
		if findErr != nil {
			return false, findErr
		}
		empty, emptyErr := state.manager.IsSequenceEmpty(representative)
		if emptyErr != nil {
			return false, emptyErr
		}
		if !empty {
			continue
		}
		progress, addErr := t.addTailEmptyMaximum([]ast.ASTHandle{tail}, nil)
		if addErr != nil || progress {
			return progress, addErr
		}
	}
	return false, nil
}
