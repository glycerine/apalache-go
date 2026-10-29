package qe

import "github.com/glycerine/g3/pkg/ast"

func (d *EqualityDER) appendConjunction(conjs *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := d.manager.NewRef(h)
	if err != nil {
		return err
	}
	*conjs = append(*conjs, r)
	return nil
}
func (d *EqualityDER) truncateConjunctions(conjs *[]ast.ASTRef, n int) error {
	for len(*conjs) > n {
		last := len(*conjs) - 1
		if (*conjs)[last] != (ast.ASTRef{}) {
			if err := d.manager.DelRef(&(*conjs)[last]); err != nil {
				return err
			}
		}
		*conjs = (*conjs)[:last]
	}
	return nil
}
func (d *EqualityDER) expressions(conjs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(conjs))
	for i, r := range conjs {
		h, err := d.manager.RefGet(r)
		if err != nil {
			return nil, err
		}
		result[i] = h
	}
	return result, nil
}
func (d *EqualityDER) flattenConstructor(conjs *[]ast.ASTRef, constructor, rhs ast.ASTHandle) error {
	decl, matched, err := d.manager.DatatypeConstructorApp(constructor)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	other, isConstructor, err := d.manager.DatatypeConstructorApp(rhs)
	if err != nil {
		return err
	}
	if isConstructor {
		if decl != other {
			no, err := d.manager.MkFalse()
			if err != nil {
				return err
			}
			return d.appendConjunction(conjs, no)
		}
		args, err := d.children(constructor)
		if err != nil {
			return err
		}
		for i, arg := range args {
			value, err := d.manager.GetArg(rhs, uint32(i))
			if err != nil {
				return err
			}
			eq, err := d.manager.MkEq(arg, value)
			if err != nil {
				return err
			}
			if err = d.appendConjunction(conjs, eq); err != nil {
				return err
			}
		}
		return nil
	}
	recognizer, err := d.manager.DatatypeConstructorRecognizer(decl)
	if err != nil {
		return err
	}
	test, err := d.manager.MkApp(recognizer, []ast.ASTHandle{rhs})
	if err != nil {
		return err
	}
	if err = d.appendConjunction(conjs, test); err != nil {
		return err
	}
	accessors, err := d.manager.DatatypeConstructorAccessors(decl)
	if err != nil {
		return err
	}
	for i, accessor := range accessors {
		field, err := d.manager.MkApp(accessor, []ast.ASTHandle{rhs})
		if err != nil {
			return err
		}
		value, err := d.manager.GetArg(constructor, uint32(i))
		if err != nil {
			return err
		}
		eq, err := d.manager.MkEq(value, field)
		if err != nil {
			return err
		}
		if err = d.appendConjunction(conjs, eq); err != nil {
			return err
		}
	}
	return nil
}
func (d *EqualityDER) flattenDefinitions(c *equalityDERCall, conjs *[]ast.ASTRef) error {
	if err := d.traceConjunctions("flatten_definitions", 636, "", *conjs); err != nil {
		return err
	}
	for i := 0; i < len(*conjs); i++ {
		h, err := d.manager.RefGet((*conjs)[i])
		if err != nil {
			return err
		}
		if d.manager.IsFalse(h) {
			if err = d.manager.SetRef((*conjs)[0], h); err != nil {
				return err
			}
			if err = d.truncateConjunctions(conjs, 1); err != nil {
				return err
			}
			break
		}
		ground, err := d.manager.IsGround(h)
		if err != nil {
			return err
		}
		if ground {
			continue
		}
		eq, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return err
		}
		if !eq {
			continue
		}
		left, err := d.manager.GetArg(h, 0)
		if err != nil {
			return err
		}
		right, err := d.manager.GetArg(h, 1)
		if err != nil {
			return err
		}
		lk, err := d.manager.GetKind(left)
		if err != nil {
			return err
		}
		rk, err := d.manager.GetKind(right)
		if err != nil {
			return err
		}
		if lk != ast.ASTApp || rk != ast.ASTApp {
			continue
		}
		_, constructor, err := d.manager.DatatypeConstructorApp(left)
		if err != nil {
			return err
		}
		if !constructor {
			_, constructor, err = d.manager.DatatypeConstructorApp(right)
			if err != nil {
				return err
			}
			if constructor {
				left, right = right, left
			}
		}
		if !constructor {
			continue
		}
		if _, err = c.hold(h); err != nil {
			return err
		}
		if err = d.flattenConstructor(conjs, left, right); err != nil {
			return err
		}
		last := len(*conjs) - 1
		replacement, err := d.manager.RefGet((*conjs)[last])
		if err != nil {
			return err
		}
		if err = d.manager.SetRef((*conjs)[i], replacement); err != nil {
			return err
		}
		if err = d.truncateConjunctions(conjs, last); err != nil {
			return err
		}
		i--
	}
	return d.traceConjunctions("flatten_definitions", 672, "after flatten\n", *conjs)
}
func (d *EqualityDER) unconstrained(variable, term ast.ASTHandle, index int, conjs []ast.ASTRef) (bool, error) {
	idx, err := d.manager.GetVarIndex(variable)
	if err != nil {
		return false, err
	}
	occurs, err := OccursVariable(d.manager, idx, term)
	if err != nil || occurs {
		return false, err
	}
	for i, ref := range conjs {
		if i == index {
			continue
		}
		h, err := d.manager.RefGet(ref)
		if err != nil {
			return false, err
		}
		occurs, err = OccursVariable(d.manager, idx, h)
		if err != nil || occurs {
			return false, err
		}
	}
	return true, nil
}
func (d *EqualityDER) removeUnconstrained(conjs []ast.ASTRef) (reduced bool, err error) {
	for change := true; change; {
		change = false
		for i, ref := range conjs {
			h, err := d.manager.RefGet(ref)
			if err != nil {
				return reduced, err
			}
			neg, err := d.operation(h, d.manager.BasicFamilyID(), ast.ASTOpNot)
			if err != nil {
				return reduced, err
			}
			if !neg {
				continue
			}
			inner, err := d.manager.GetArg(h, 0)
			if err != nil {
				return reduced, err
			}
			eq, err := d.operation(inner, d.manager.BasicFamilyID(), ast.ASTOpEq)
			if err != nil {
				return reduced, err
			}
			if !eq {
				continue
			}
			left, err := d.manager.GetArg(inner, 0)
			if err != nil {
				return reduced, err
			}
			right, err := d.manager.GetArg(inner, 1)
			if err != nil {
				return reduced, err
			}
			if err = d.traceUnconstrained(h, left, right); err != nil {
				return reduced, err
			}
			for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
				selected, err := d.selection.IsVariable(pair[0])
				if err != nil {
					return reduced, err
				}
				if !selected {
					continue
				}
				free, err := d.unconstrained(pair[0], pair[1], i, conjs)
				if err != nil {
					return reduced, err
				}
				if !free {
					continue
				}
				value, err := d.manager.MkTrue()
				if err != nil {
					return reduced, err
				}
				if err = d.manager.SetRef(ref, value); err != nil {
					return reduced, err
				}
				reduced = true
				change = true
				break
			}
		}
	}
	return reduced, nil
}
func (d *EqualityDER) substitute(c *equalityDERCall, h ast.ASTHandle, bindings []ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = d.substitution.Substitute(h, bindings, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return d.manager.RefGet(out)
}
func (d *EqualityDER) createSubstitution(c *equalityDERCall) ([]ast.ASTHandle, error) {
	bindings := make([]ast.ASTHandle, len(c.definitions))
	for _, index := range c.order {
		h, err := d.substitute(c, c.definitions[index].term, bindings)
		if err != nil {
			return nil, err
		}
		bindings[len(bindings)-int(index)-1] = h
	}
	return bindings, nil
}
func (d *EqualityDER) reduceVariableSet(c *equalityDERCall, conjs *[]ast.ASTRef) (bool, error) {
	if err := d.flattenDefinitions(c, conjs); err != nil {
		return false, err
	}
	args, err := d.expressions(*conjs)
	if err != nil {
		return false, err
	}
	count, err := d.findDefinitions(c, args, true)
	if err != nil {
		return false, err
	}
	reduced := false
	if count > 0 {
		if err = d.eliminationOrder(c); err != nil {
			return false, err
		}
		if len(c.order) != 0 {
			root, err := d.manager.MkAndArray(args)
			if err != nil {
				return false, err
			}
			if root, err = c.hold(root); err != nil {
				return false, err
			}
			bindings, err := d.createSubstitution(c)
			if err != nil {
				return false, err
			}
			substituted, err := d.substitute(c, root, bindings)
			if err != nil {
				return false, err
			}
			out, err := c.ref(ast.ASTHandle{})
			if err != nil {
				return false, err
			}
			if err = d.rewriter.Rewrite(substituted, out); err != nil {
				return false, err
			}
			result, err := d.manager.RefGet(out)
			if err != nil {
				return false, err
			}
			if err = d.truncateConjunctions(conjs, 0); err != nil {
				return false, err
			}
			parts, err := d.manager.FlattenAnd([]ast.ASTHandle{result})
			if err != nil {
				return false, err
			}
			for _, part := range parts {
				if err = d.appendConjunction(conjs, part); err != nil {
					return false, err
				}
			}
			reduced = true
		}
	}
	removed, err := d.removeUnconstrained(*conjs)
	return reduced || removed, err
}

// EliminateConjunctions is eq::der::operator()(expr_ref_vector&). The caller
// owns the current slice and all its references, including prefixes on error.
func (d *EqualityDER) EliminateConjunctions(conjs *[]ast.ASTRef) (err error) {
	if err = d.check(); err != nil {
		return err
	}
	if conjs == nil {
		return ast.ErrASTHandle
	}
	if err = d.selection.check(); err != nil {
		return err
	}
	if err = d.drain(); err != nil {
		return err
	}
	c := &equalityDERCall{owner: d}
	d.pending = append(d.pending, c)
	defer d.finish(c, &err)
	for {
		changed, e := d.reduceVariableSet(c, conjs)
		if e != nil {
			return e
		}
		if !changed {
			return nil
		}
	}
}
