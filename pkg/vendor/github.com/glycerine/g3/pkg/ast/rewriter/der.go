package rewriter

// Source: src/ast/rewriter/der.cpp:27-98,208-315;
// src/ast/rewriter/der.h:27-157.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type derCall struct {
	owner *DER
	refs  []ast.ASTRef
}

func (c *derCall) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return h, err
}

func (c *derCall) close() (result error) {
	for i := len(c.refs); i > 0; {
		i--
		result = errors.Join(result, c.owner.manager.DelRef(&c.refs[i]))
	}
	return result
}

// DER is the destructive multi-equality resolution core. The substitution
// owner is reused across quantifier reductions, as in the source object.
type DER struct {
	self       *DER
	manager    *ast.ASTManager
	substitute *VarSubstitution
	closed     bool
}

func NewDER(m *ast.ASTManager) (*DER, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	d := &DER{manager: m}
	d.self = d
	var err error
	d.substitute, err = NewVarSubstitution(m, true)
	if err != nil {
		if d.substitute != nil {
			err = errors.Join(err, d.substitute.Close())
		}
		return nil, err
	}
	return d, nil
}

func (d *DER) open() error {
	if d == nil || d.self != d || d.closed || d.manager == nil || d.substitute == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (d *DER) isBasic(h ast.ASTHandle, kind int32) (bool, error) {
	k, err := d.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	family, err := d.manager.GetFamilyID(h)
	if err != nil || family != d.manager.BasicFamilyID() {
		return false, err
	}
	op, err := d.manager.GetDeclKind(h)
	return op == kind, err
}

func (d *DER) boundVariable(h ast.ASTHandle, declarations uint32) (uint32, bool, error) {
	kind, err := d.manager.GetKind(h)
	if err != nil || kind != ast.ASTVar {
		return 0, false, err
	}
	index, err := d.manager.GetVarIndex(h)
	return index, err == nil && index < declarations, err
}

func (d *DER) negatedBoundVariable(h ast.ASTHandle, declarations uint32) (ast.ASTHandle, uint32, bool, error) {
	isNot, err := d.isBasic(h, ast.ASTOpNot)
	if err != nil || !isNot {
		return ast.ASTHandle{}, 0, false, err
	}
	if n, err := d.manager.GetNumArgs(h); err != nil {
		return ast.ASTHandle{}, 0, false, err
	} else if n != 1 {
		return ast.ASTHandle{}, 0, false, nil
	}
	v, err := d.manager.GetArg(h, 0)
	if err != nil {
		return ast.ASTHandle{}, 0, false, err
	}
	index, matched, err := d.boundVariable(v, declarations)
	return v, index, matched, err
}

// isVarDiseq recognizes exactly the source's Boolean disequality encodings.
func (d *DER) isVarDiseq(c *derCall, h ast.ASTHandle, declarations uint32) (variable ast.ASTHandle, index uint32, definition ast.ASTHandle, matched bool, err error) {
	if err = d.open(); err != nil {
		return
	}
	isNot, err := d.isBasic(h, ast.ASTOpNot)
	if err != nil {
		return variable, index, definition, false, err
	}
	if isNot {
		inner, x := d.manager.GetArg(h, 0)
		if x != nil {
			return variable, index, definition, false, x
		}
		isEq, x := d.isBasic(inner, ast.ASTOpEq)
		if x != nil {
			return variable, index, definition, false, x
		}
		isIff, x := d.isBasic(inner, ast.ASTOpIff)
		if x != nil {
			return variable, index, definition, false, x
		}
		if isEq || isIff {
			if n, x := d.manager.GetNumArgs(inner); x != nil {
				return variable, index, definition, false, x
			} else if n != 2 {
				return variable, index, definition, false, nil
			}
			lhs, x := d.manager.GetArg(inner, 0)
			if x != nil {
				return variable, index, definition, false, x
			}
			rhs, x := d.manager.GetArg(inner, 1)
			if x != nil {
				return variable, index, definition, false, x
			}
			idx, left, x := d.boundVariable(lhs, declarations)
			if x != nil {
				return variable, index, definition, false, x
			}
			if !left {
				idx, left, x = d.boundVariable(rhs, declarations)
				if x != nil {
					return variable, index, definition, false, x
				}
				if !left {
					return variable, index, definition, false, nil
				}
				lhs, rhs = rhs, lhs
			}
			return lhs, idx, rhs, true, nil
		}
	}

	isIff, err := d.isBasic(h, ast.ASTOpIff)
	if err != nil {
		return variable, index, definition, false, err
	}
	if isIff {
		lhs, x := d.manager.GetArg(h, 0)
		if x != nil {
			return variable, index, definition, false, x
		}
		rhs, x := d.manager.GetArg(h, 1)
		if x != nil {
			return variable, index, definition, false, x
		}
		idx, left, x := d.boundVariable(lhs, declarations)
		if x != nil {
			return variable, index, definition, false, x
		}
		if !left {
			idx, left, x = d.boundVariable(rhs, declarations)
			if x != nil {
				return variable, index, definition, false, x
			}
			if left {
				lhs, rhs = rhs, lhs
			}
		}
		if left {
			negated, x := d.manager.MkNot(rhs)
			if x != nil {
				return variable, index, definition, false, x
			}
			negated, x = c.hold(negated)
			return lhs, idx, negated, x == nil, x
		}
		lhsVar, idx, leftNegated, x := d.negatedBoundVariable(lhs, declarations)
		if x != nil {
			return variable, index, definition, false, x
		}
		if !leftNegated {
			lhsVar, idx, leftNegated, x = d.negatedBoundVariable(rhs, declarations)
			if x != nil {
				return variable, index, definition, false, x
			}
			if leftNegated {
				rhs, lhs = lhs, rhs
			}
		}
		if leftNegated {
			return lhsVar, idx, rhs, true, nil
		}
		return variable, index, definition, false, nil
	}

	idx, isVar, err := d.boundVariable(h, declarations)
	if err != nil {
		return variable, index, definition, false, err
	}
	if isVar {
		value, x := d.manager.MkFalse()
		if x != nil {
			return variable, index, definition, false, x
		}
		return h, idx, value, true, nil
	}
	variable, idx, negated, err := d.negatedBoundVariable(h, declarations)
	if err != nil || !negated {
		return ast.ASTHandle{}, 0, ast.ASTHandle{}, false, err
	}
	value, err := d.manager.MkTrue()
	return variable, idx, value, err == nil, err
}

type derSortFrame struct {
	term ast.ASTHandle
	next uint32
}

// sortVariables performs the source cycle-breaking DFS and mutates definitions
// by clearing invalid or cyclic candidates.
func (d *DER) sortVariables(variables, definitions []ast.ASTHandle) ([]uint32, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	if len(variables) != len(definitions) {
		return nil, ast.ErrASTDomain
	}
	found := false
	for i, definition := range definitions {
		if definition == (ast.ASTHandle{}) {
			continue
		}
		hasQuantifiers, err := d.manager.HasQuantifiers(definition)
		if err != nil {
			return nil, err
		}
		occurs, err := d.manager.OccursExpr(variables[i], definition)
		if err != nil {
			return nil, err
		}
		if hasQuantifiers || occurs {
			definitions[i] = ast.ASTHandle{}
		} else {
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	visiting := make(map[ast.ASTHandle]bool)
	done := make(map[ast.ASTHandle]bool)
	order := make([]uint32, 0, len(definitions))
	for i, definition := range definitions {
		if definition == (ast.ASTHandle{}) || done[variables[i]] {
			continue
		}
		stack := []derSortFrame{{term: variables[i]}}
		for len(stack) != 0 {
			frame := &stack[len(stack)-1]
			if done[frame.term] {
				stack = stack[:len(stack)-1]
				continue
			}
			kind, err := d.manager.GetKind(frame.term)
			if err != nil {
				return nil, err
			}
			switch kind {
			case ast.ASTVar:
				index, err := d.manager.GetVarIndex(frame.term)
				if err != nil {
					return nil, err
				}
				candidate := uint64(index) < uint64(len(definitions)) && definitions[index] != (ast.ASTHandle{})
				if frame.next == 0 && candidate {
					if visiting[frame.term] {
						visiting[frame.term] = false
						definitions[index] = ast.ASTHandle{}
					} else {
						visiting[frame.term] = true
						frame.next = 1
						stack = append(stack, derSortFrame{term: definitions[index]})
						continue
					}
				} else if frame.next == 1 && candidate {
					visiting[frame.term] = false
					order = append(order, index)
				}
				done[frame.term] = true
				stack = stack[:len(stack)-1]
			case ast.ASTApp:
				count, err := d.manager.GetNumArgs(frame.term)
				if err != nil {
					return nil, err
				}
				if frame.next < count {
					arg, err := d.manager.GetArg(frame.term, frame.next)
					if err != nil {
						return nil, err
					}
					frame.next++
					if !done[arg] {
						stack = append(stack, derSortFrame{term: arg})
					}
					continue
				}
				done[frame.term] = true
				stack = stack[:len(stack)-1]
			default:
				return nil, ast.ErrASTDomain
			}
		}
	}
	return order, nil
}

func (d *DER) Close() (result error) {
	if d == nil || d.self != d {
		return ast.ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if d.substitute != nil {
		result = errors.Join(result, d.substitute.Close())
	}
	d.closed = true
	return result
}
