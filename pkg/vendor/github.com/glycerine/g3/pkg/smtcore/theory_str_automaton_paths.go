package smtcore

// Source: src/smt/theory_str.cpp aut_path_add_next,
// aut_path_rewrite_constraint and generate_regex_path_constraints.

import (
	"errors"
	"sort"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type smtStringAutomatonPath struct {
	theory *SMTStringTheory
	locals smtStringLengthPremises
}

func (p *smtStringAutomatonPath) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	return value, p.locals.add(value)
}

func (p *smtStringAutomatonPath) byteValue(value ast.ASTHandle) (byte, bool, error) {
	text, numeral, err := p.theory.state.manager.BitVectorNumeralText(value)
	if err != nil || !numeral {
		return 0, numeral, err
	}
	n, err := strconv.ParseUint(text, 10, 8)
	if err != nil {
		return 0, true, ast.ErrASTDomain // Native byte-range assertion.
	}
	return byte(n), true, nil
}

func (p *smtStringAutomatonPath) rewrite(condition, character ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.theory.state.manager
	value, numeral, err := p.byteValue(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if numeral {
		return p.pin(m.MkString(string([]byte{value})))
	}
	kind, err := m.GetKind(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTVar {
		return character, nil // Native substitution ignores the variable index.
	}
	op, basic, err := p.theory.state.basicOperation(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if basic && op == ast.ASTOpEq {
		left, err := m.GetArg(condition, 0)
		if err != nil {
			return left, err
		}
		right, err := m.GetArg(condition, 1)
		if err != nil {
			return right, err
		}
		left, err = p.rewrite(left, character)
		if err != nil {
			return left, err
		}
		right, err = p.rewrite(right, character)
		if err != nil {
			return right, err
		}
		if left.IsNull() || right.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return p.pin(m.MkEq(left, right)) // Sort changes from BV to String.
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := m.GetSort(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort != boolean {
		return ast.ASTHandle{}, nil // Native unrecognized constraint returns NULL.
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	count, err := m.GetNumArgs(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var arguments []ast.ASTHandle
	for i := uint32(0); i < count; i++ {
		argument, err := m.GetArg(condition, i)
		if err != nil {
			return argument, err
		}
		argument, err = p.rewrite(argument, character)
		if err != nil || argument.IsNull() {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return argument, err
		}
		arguments = append(arguments, argument)
	}
	decl, err := m.GetDecl(condition)
	if err != nil {
		return decl, err
	}
	// Preserve the declaration. Do not invent a String comparison adapter
	// for Boolean BV operators that cannot accept the translated arguments.
	return p.pin(m.MkApp(decl, arguments))
}

func (t *SMTStringTheory) rewriteStringAutomatonConstraint(condition, character ast.ASTHandle) (root ast.ASTRef, result error) {
	p := smtStringAutomatonPath{theory: t, locals: smtStringLengthPremises{theory: t}}
	defer func() { result = errors.Join(result, p.locals.close()) }()
	value, err := p.rewrite(condition, character)
	if err != nil {
		return root, err
	}
	return t.state.manager.NewRef(value)
}

func (p *smtStringAutomatonPath) addNext(next map[uint32]ast.ASTHandle, state uint32, condition ast.ASTHandle) error {
	m := p.theory.state.manager
	if previous, found := next[state]; found && !m.IsTrue(condition) {
		var err error
		condition, err = p.pin(m.MkOrArray([]ast.ASTHandle{condition, previous}))
		if err != nil {
			return err
		}
	}
	if err := p.locals.add(condition); err != nil {
		return err
	}
	next[state] = condition
	return nil
}

func stringPathStates(frontier map[uint32]ast.ASTHandle) []uint32 {
	var states []uint32
	for state := range frontier {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i] < states[j] })
	return states
}

// characterConstraints is a caller-owned output root. The only retained C++
// caller handles zero length separately and invokes this helper in its else
// arm, so this concrete specialization requires a nonzero length.
func (t *SMTStringTheory) generateStringRegexPathConstraints(word ast.ASTHandle, owner *rewriter.StringRegexAutomaton, length uint32, characterConstraints ast.ASTRef) (root ast.ASTRef, result error) {
	graph, err := owner.Graph()
	if err != nil {
		return root, err
	}
	if length == 0 {
		return root, ast.ErrASTDomain
	}
	m := t.state.manager
	if _, err := m.RefGet(characterConstraints); err != nil {
		return root, err
	}
	p := smtStringAutomatonPath{theory: t, locals: smtStringLengthPremises{theory: t}}
	defer func() { result = errors.Join(result, p.locals.close()) }()
	characters, reused := t.stringChars[word]
	characters = append([]ast.ASTHandle(nil), characters...)
	for uint64(len(characters)) < uint64(length) {
		character, err := t.freshStringVariable("ch" + strconv.Itoa(len(characters)))
		if err != nil {
			return root, err
		}
		if reused {
			// Native existing-cache growth explicitly trails the new variable
			// again in addition to mk_str_var's lifetime ownership.
			if err := t.retain(character); err != nil {
				return root, err
			}
		}
		characters = append(characters, character)
	}
	t.stringChars[word] = characters
	characters = characters[:length]
	var lengthConstraints []ast.ASTHandle
	one, err := p.pin(m.ArithmeticIntegerNumeralText("1"))
	if err != nil {
		return root, err
	}
	for _, character := range characters {
		if reused {
			if err := t.refreshStringVariable(character); err != nil {
				return root, err
			}
		}
		characterLength, err := p.pin(m.MkSequenceLength(character))
		if err != nil {
			return root, err
		}
		constraint, err := p.pin(m.MkEq(characterLength, one))
		if err != nil {
			return root, err
		}
		lengthConstraints = append(lengthConstraints, constraint)
	}
	truth, err := m.MkTrue()
	if err != nil {
		return root, err
	}
	frontier := map[uint32]ast.ASTHandle{graph.Initial(): truth}
	for _, character := range characters {
		next := make(map[uint32]ast.ASTHandle)
		for _, state := range stringPathStates(frontier) {
			accumulator := frontier[state]
			for _, move := range graph.ExpandedMovesFrom(state, false) {
				label, err := owner.Condition(move.Condition)
				if err != nil {
					return root, err
				}
				var condition ast.ASTHandle
				switch label.Kind {
				case rewriter.StringRegexCharacter:
					value, err := m.IsValue(label.Character)
					if err != nil {
						return root, err
					}
					if !value {
						continue // Native non-value character labels have no path case.
					}
					rhs, err := p.rewrite(label.Character, character)
					if err != nil || rhs.IsNull() {
						if err == nil {
							err = ast.ErrASTDomain
						}
						return root, err
					}
					condition, err = p.pin(m.MkEq(character, rhs))
					if err != nil {
						return root, err
					}
				case rewriter.StringRegexRange:
					lower, lc, err := p.byteValue(label.Lower)
					if err != nil {
						return root, err
					}
					upper, uc, err := p.byteValue(label.Upper)
					if err != nil {
						return root, err
					}
					if !lc || !uc {
						return root, ast.ErrASTDomain
					}
					if upper < lower {
						lower, upper = upper, lower
					}
					var choices []ast.ASTHandle
					for value := uint32(lower); value <= uint32(upper); value++ {
						literal, err := p.pin(m.MkString(string([]byte{byte(value)})))
						if err != nil {
							return root, err
						}
						equality, err := p.pin(m.MkEq(character, literal))
						if err != nil {
							return root, err
						}
						choices = append(choices, equality)
					}
					condition, err = p.pin(m.MkOrArray(choices))
					if err != nil {
						return root, err
					}
				case rewriter.StringRegexPredicate:
					condition, err = p.rewrite(label.Predicate, character)
					if err != nil || condition.IsNull() {
						if err == nil {
							err = ast.ErrASTDomain
						}
						return root, err
					}
					if m.IsFalse(condition) {
						continue
					}
					if m.IsTrue(condition) {
						if err := p.addNext(next, move.Destination, accumulator); err != nil {
							return root, err
						}
						continue
					}
				default:
					return root, ast.ErrASTDomain
				}
				condition, err = p.pin(m.MkAndArray([]ast.ASTHandle{condition, accumulator}))
				if err != nil {
					return root, err
				}
				if err := p.addNext(next, move.Destination, condition); err != nil {
					return root, err
				}
			}
		}
		frontier = next
	}
	var alternatives []ast.ASTHandle
	for _, state := range stringPathStates(frontier) {
		if stringAutomatonFrontierAccepts(graph, graph.EpsilonClosure(state)) {
			alternatives = append(alternatives, frontier[state])
		}
	}
	path, err := p.pin(m.MkOrArray(alternatives))
	if err != nil {
		return root, err
	}
	concat := characters[0]
	for _, character := range characters[1:] {
		concat, err = p.pin(t.concatTerm(concat, character))
		if err != nil {
			return root, err
		}
	}
	decomposition, err := p.pin(m.MkEq(word, concat))
	if err != nil {
		return root, err
	}
	lengths, err := p.pin(m.MkAndArray(lengthConstraints))
	if err != nil {
		return root, err
	}
	constraints, err := p.pin(m.MkAnd(lengths, decomposition))
	if err != nil {
		return root, err
	}
	if err := m.SetRef(characterConstraints, constraints); err != nil {
		return root, err
	}
	return m.NewRef(path)
}
