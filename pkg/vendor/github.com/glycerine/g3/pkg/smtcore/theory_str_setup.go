package smtcore

// Source: src/smt/theory_str.cpp set_up_axioms and init_search_eh.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTStringTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	family, err := t.state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	kind, err := t.state.manager.GetDeclKind(expression)
	return kind, err == nil, err
}

func (t *SMTStringTheory) ensureEnode(expression ast.ASTHandle) (*SMTEnode, error) {
	state := t.state
	if err := state.Internalize(expression, false); err != nil {
		return nil, err
	}
	if variable, boolean := state.BoolVar(expression); boolean && state.context.enodes[expression] == nil {
		// Go Boolean gates need an explicit enode flag to realize the
		// source ensure_enode contract used by this theory's traversal.
		if err := state.ensureBooleanEnode(expression, variable, false, true, true); err != nil {
			return nil, err
		}
	}
	if err := state.markRelevant(expression); err != nil {
		return nil, err
	}
	return state.context.enodes[expression], nil
}

func (t *SMTStringTheory) setUpAxioms(expression ast.ASTHandle) error {
	m := t.state.manager
	sort, err := m.GetSort(expression)
	if err != nil {
		return err
	}
	isString, err := m.IsStringSort(sort)
	if err != nil {
		return err
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return err
	}
	if isString {
		node := t.state.context.enodes[expression]
		if node == nil {
			return ast.ErrASTDomain
		}
		t.basicTodo = append(t.basicTodo, node)
		if kind == ast.ASTApp {
			op, own, err := t.operation(expression)
			if err != nil {
				return err
			}
			count, err := m.GetNumArgs(expression)
			if err != nil {
				return err
			}
			if own && op == ast.ASTSequenceOpConcat {
				t.concatTodo = append(t.concatTodo, node)
				t.concatEvalTodo = append(t.concatEvalTodo, node)
			} else if own && (op == 6 || op == 7 || op == 8 || op == 27) {
				t.libraryTodo = append(t.libraryTodo, node)
				if op == 27 {
					if err := t.recordStringIntConversion(expression); err != nil {
						return err
					}
				}
			} else if _, constant, err := m.SequenceStringValue(expression); err != nil {
				return err
			} else if count == 0 && !constant {
				t.variableSet[expression] = struct{}{}
				if err = t.state.markRelevant(expression); err != nil {
					return err
				}
				if _, err = t.mkVar(node); err != nil {
					return err
				}
			}
		}
	} else {
		booleanSort, err := m.MkBoolSort()
		if err != nil {
			return err
		}
		family, err := m.GetFamilyID(sort)
		if err != nil {
			return err
		}
		arithmeticFamily, err := m.ArithmeticFamilyID()
		if err != nil {
			return err
		}
		sortKind, err := m.GetDeclKind(sort)
		if err != nil {
			return err
		}
		isBoolean := sort == booleanSort && kind != ast.ASTQuantifier
		isInteger := family == arithmeticFamily && sortKind == 1
		if isBoolean || isInteger {
			node, err := t.ensureEnode(expression)
			if err != nil {
				return err
			}
			if node == nil {
				if t.searchStarted {
					return ast.ErrASTDomain
				}
				if err = t.retain(expression); err != nil {
					return err
				}
				t.delayedSetup = append(t.delayedSetup, expression)
				return nil
			}
			if kind == ast.ASTApp {
				op, own, err := t.operation(expression)
				if err != nil {
					return err
				}
				if own && ((isBoolean && (op == 3 || op == 4 || op == 5 || op == 12)) || (isInteger && (op == 10 || op == 28))) {
					t.libraryTodo = append(t.libraryTodo, node)
					if op == 28 {
						if err := t.recordStringIntConversion(expression); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if kind == ast.ASTApp {
		count, err := m.GetNumArgs(expression)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			argument, err := m.GetArg(expression, i)
			if err != nil {
				return err
			}
			if err = t.setUpAxioms(argument); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) InitializeSearch() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if state.assertedFormulas != nil {
		for _, assertion := range *state.assertedFormulas {
			if err = t.setUpAxioms(assertion.Formula); err != nil {
				return err
			}
		}
	}
	if err = t.Propagate(); err != nil {
		return err
	}
	t.searchStarted = true
	return nil
}
