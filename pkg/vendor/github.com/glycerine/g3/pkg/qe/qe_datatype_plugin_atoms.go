package qe

// Source: src/qe/qe_datatype_plugin.cpp datatype_atoms and update_eqs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type datatypeEqualityAtom struct{ atom, value, condition ast.ASTHandle }
type datatypeEqualityAtoms struct {
	refs               []ast.ASTRef
	positive, negative []datatypeEqualityAtom
	recognizers, unsat []ast.ASTHandle
}

func (a *datatypeEqualityAtoms) close(m *ast.ASTManager) error {
	for len(a.refs) != 0 {
		i := len(a.refs) - 1
		if err := m.DelRef(&a.refs[i]); err != nil {
			return err
		}
		a.refs = a.refs[:i]
	}
	return nil
}

// constructorOccurs follows only datatype constructor arguments, as in the
// source occurs check and disequality traversal.
func (p *DatatypeSolverPlugin) constructorOccurs(term, variable ast.ASTHandle) (bool, error) {
	todo := []ast.ASTHandle{term}
	seen := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		if h == variable {
			return true, nil
		}
		_, constructor, err := p.manager.DatatypeConstructorApp(h)
		if err != nil {
			return false, err
		}
		if !constructor {
			continue
		}
		n, err := p.manager.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < n; j++ {
			arg, e := p.manager.GetArg(h, j)
			if e != nil {
				return false, e
			}
			todo = append(todo, arg)
		}
	}
	return false, nil
}

// PrepareAtoms records equality and constructor-disequality cases used when a
// recursive target already has a recognizer.
func (p *DatatypeSolverPlugin) PrepareAtoms(variable *ast.ContainsApp, formula ast.ASTHandle, atoms *ArithmeticAtomSet) (supported bool, err error) {
	if err = p.open(); err != nil {
		return false, err
	}
	if variable == nil || atoms == nil || atoms.manager != p.manager {
		return false, ast.ErrASTHandle
	}
	positive, err := atoms.Positive()
	if err != nil {
		return false, err
	}
	negative, err := atoms.Negative()
	if err != nil {
		return false, err
	}
	return p.Prepare(variable, formula, positive, negative)
}

// Prepare collects the current solver atom sets for recursive equality branches.
func (p *DatatypeSolverPlugin) Prepare(variable *ast.ContainsApp, formula ast.ASTHandle, positive, negative []ast.ASTHandle) (supported bool, err error) {
	if err = p.open(); err != nil {
		return false, err
	}
	if variable == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	key := arithmeticPluginKey{variable: target, formula: formula}
	if p.equalities[key] != nil {
		return true, nil
	}
	entry := &datatypeEqualityAtoms{}
	defer func() {
		if err != nil || !supported {
			err = errors.Join(err, entry.close(p.manager))
		}
	}()
	hold := func(h ast.ASTHandle) error {
		ref, e := p.manager.NewRef(h)
		if e == nil {
			entry.refs = append(entry.refs, ref)
		}
		return e
	}
	if err = hold(target); err != nil {
		return false, err
	}
	if err = hold(formula); err != nil {
		return false, err
	}
	for pass, list := range [][]ast.ASTHandle{positive, negative} {
		seen := make(map[ast.ASTHandle]bool)
		for _, atom := range list {
			contains, e := variable.Check(atom)
			if e != nil {
				return false, e
			}
			if !contains || seen[atom] {
				continue
			}
			seen[atom] = true
			kind, e := p.manager.GetKind(atom)
			if e != nil {
				return false, e
			}
			if kind != ast.ASTApp {
				return false, nil
			}
			family, e := p.manager.GetFamilyID(atom)
			if e != nil {
				return false, e
			}
			if family == p.family {
				decl, e := p.manager.GetDecl(atom)
				if e != nil {
					return false, e
				}
				_, recognizer, e := p.manager.DatatypeRecognizerConstructor(decl)
				if e != nil {
					return false, e
				}
				if recognizer {
					arg, e := p.manager.GetArg(atom, 0)
					if e != nil {
						return false, e
					}
					if arg == target {
						if e = hold(atom); e != nil {
							return false, e
						}
						entry.recognizers = append(entry.recognizers, atom)
						continue
					}
				}
			}
			op, e := p.manager.GetDeclKind(atom)
			if e != nil {
				return false, e
			}
			if family != p.manager.BasicFamilyID() || op != ast.ASTOpEq {
				return false, nil
			}
			left, e := p.manager.GetArg(atom, 0)
			if e != nil {
				return false, e
			}
			right, e := p.manager.GetArg(atom, 1)
			if e != nil {
				return false, e
			}
			if right == target {
				left, right = right, left
			}
			leftContains, e := variable.Check(left)
			if e != nil {
				return false, e
			}
			rightContains, e := variable.Check(right)
			if e != nil {
				return false, e
			}
			if !leftContains && rightContains {
				left, right = right, left
				leftContains, rightContains = rightContains, leftContains
			}
			if left == target {
				rightContains, e := variable.Check(right)
				if e != nil {
					return false, e
				}
				if rightContains {
					occurs, e := p.constructorOccurs(right, target)
					if e != nil {
						return false, e
					}
					if !occurs {
						return false, nil
					}
					if e = hold(atom); e != nil {
						return false, e
					}
					entry.unsat = append(entry.unsat, atom)
					continue
				}
				if e = hold(atom); e != nil {
					return false, e
				}
				if pass == 0 {
					entry.positive = append(entry.positive, datatypeEqualityAtom{atom: atom, value: right})
				} else {
					entry.negative = append(entry.negative, datatypeEqualityAtom{atom: atom, value: right})
				}
				continue
			}
			if pass == 0 && leftContains && !rightContains {
				truth, e := p.manager.MkTrue()
				if e != nil {
					return false, e
				}
				value, condition, matched, e := p.solveConstructorEquality(variable, left, right, truth, entry)
				if e != nil {
					return false, e
				}
				if matched {
					if e = hold(atom); e != nil {
						return false, e
					}
					entry.positive = append(entry.positive, datatypeEqualityAtom{atom: atom, value: value, condition: condition})
					continue
				}
			}
			if pass == 1 {
				rightContains, e := variable.Check(right)
				if e != nil {
					return false, e
				}
				occurs, e := p.constructorOccurs(left, target)
				if e != nil {
					return false, e
				}
				if !rightContains && occurs {
					if e = hold(atom); e != nil {
						return false, e
					}
					entry.negative = append(entry.negative, datatypeEqualityAtom{atom: atom, value: right})
					continue
				}
			}
			return false, nil
		}
	}
	p.equalities[key] = entry
	return true, nil
}

func (p *DatatypeSolverPlugin) assignEqualityBranch(target, formula ast.ASTHandle, branch uint32) error {
	entry := p.equalities[arithmeticPluginKey{variable: target, formula: formula}]
	if entry == nil || uint64(branch) > uint64(len(entry.positive)) {
		return ast.ErrASTDomain
	}
	add := func(h ast.ASTHandle) error {
		ref, err := p.manager.NewRef(h)
		if err == nil {
			p.constraints = append(p.constraints, ArithmeticConstraint{UseVariable: true, Formula: ref})
		}
		return err
	}
	if int(branch) < len(entry.positive) {
		eq, err := p.manager.MkEq(target, entry.positive[branch].value)
		if err != nil {
			return err
		}
		return add(eq)
	}
	for _, a := range entry.positive {
		eq, err := p.manager.MkEq(target, a.value)
		if err != nil {
			return err
		}
		neg, err := p.manager.MkNot(eq)
		if err != nil {
			return err
		}
		if err = add(neg); err != nil {
			return err
		}
	}
	return nil
}
