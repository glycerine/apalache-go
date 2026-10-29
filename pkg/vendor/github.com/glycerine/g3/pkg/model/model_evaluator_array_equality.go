package model

// Source: src/model/model_evaluator.cpp:292-420.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type arrayEquality struct {
	active             bool
	left, right        arrayExpansion
	conj, args1, args2 []ast.ASTHandle
	select1, select2   ast.ASTRef
	table1, table2     arrayTupleTable
}

func (f *arrayEquality) close(m *ast.ASTManager) error {
	if !f.active {
		return nil
	}
	f.table2.close()
	f.table1.close()
	if e := f.right.closeLexical(m); e != nil {
		return e
	}
	if e := f.left.closeLexical(m); e != nil {
		return e
	}
	for _, r := range []*ast.ASTRef{&f.select2, &f.select1} {
		if *r != (ast.ASTRef{}) {
			if e := m.DelRef(r); e != nil {
				return e
			}
		}
	}
	for _, roots := range []*[]ast.ASTHandle{&f.args2, &f.args1, &f.conj} {
		if e := releaseArrayRoots(m, roots); e != nil {
			return e
		}
		*roots = nil
	}
	if e := f.right.closeElse(m); e != nil {
		return e
	}
	if e := f.left.closeElse(m); e != nil {
		return e
	}
	if e := f.right.closeStores(m); e != nil {
		return e
	}
	if e := f.left.closeStores(m); e != nil {
		return e
	}
	*f = arrayEquality{}
	return nil
}
func (c *evaluatorConfiguration) arrayCompare(a, b ast.ASTHandle) (int, error) {
	m := c.owner.model.manager
	same, e := m.AreEqual(a, b)
	if e != nil || same {
		return 1, e
	}
	different, e := m.AreDistinct(a, b)
	if e != nil {
		return 0, e
	}
	if different {
		return -1, nil
	}
	return 0, nil
}
func (c *evaluatorConfiguration) arrayConjPush(f *arrayEquality, a, b ast.ASTHandle) error {
	m := c.owner.model.manager
	eq, e := m.MkEq(a, b)
	if e != nil {
		return e
	}
	return appendArrayRoot(m, &f.conj, eq)
}
func (c *evaluatorConfiguration) arrayConjunction(f *arrayEquality, out ast.ASTRef, shortcut bool) (rewriter.RewriteStatus, error) {
	m := c.owner.model.manager
	args := ast.SliceArguments(f.conj)
	if shortcut {
		if args.Count() == 0 {
			yes, e := m.MkTrue()
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			return rewriter.RewriteFull, m.SetRef(out, yes)
		}
		if args.Count() == 1 {
			h, e := args.At(0)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			return rewriter.RewriteFull, m.SetRef(out, h)
		}
	}
	d, e := m.MkAndDecl()
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	h, e := m.RewriterMakeApp(d, args)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	return rewriter.RewriteFull, m.SetRef(out, h)
}
func (c *evaluatorConfiguration) arrayEq(a, b ast.ASTHandle, out ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	m := c.owner.model.manager
	if a == b {
		yes, e := m.MkTrue()
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		return rewriter.RewriteDone, m.SetRef(out, yes)
	}
	if !c.arrayEqualities {
		return rewriter.RewriteFailed, nil
	}
	f := &c.equality
	if f.active {
		return rewriter.RewriteFailed, uparams.ErrParamsPrecondition
	}
	f.active = true
	f.left.active = true
	f.right.active = true
	defer func() { err = errors.Join(err, f.close(m)) }()
	var e error
	f.left.elseCase, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	f.right.elseCase, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	leftOK, leftUnique, e := c.extractArray(a, &f.left)
	if e != nil || !leftOK {
		return rewriter.RewriteFailed, e
	}
	rightOK, rightUnique, e := c.extractArray(b, &f.right)
	if e != nil || !rightOK {
		return rewriter.RewriteFailed, e
	}
	else1, e := m.RefGet(f.left.elseCase)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	else2, e := m.RefGet(f.right.elseCase)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	comparison, e := c.arrayCompare(else1, else2)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if comparison != 1 {
		if comparison == -1 {
			sort, e := m.GetSort(else1)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			size, e := m.GetSortSize(sort)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			if !size.IsFinite() {
				no, e := m.MkFalse()
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				return rewriter.RewriteDone, m.SetRef(out, no)
			}
		}
		if e = c.arrayConjPush(f, else1, else2); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	if leftUnique && rightUnique && len(f.left.stores) != 0 {
		return c.arrayEqCore(f, else1, else2, out)
	}
	if e = appendArrayRoot(m, &f.args1, a); e != nil {
		return rewriter.RewriteFailed, e
	}
	if e = appendArrayRoot(m, &f.args2, b); e != nil {
		return rewriter.RewriteFailed, e
	}
	for _, row := range f.right.stores {
		if e = f.left.appendRowCopy(m, row); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	for _, row := range f.left.stores {
		indices := ast.SliceArguments(row[:len(row)-1])
		if e = releaseArrayTail(m, &f.args1); e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = appendArrayArguments(m, &f.args1, indices); e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = releaseArrayTail(m, &f.args2); e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = appendArrayArguments(m, &f.args2, indices); e != nil {
			return rewriter.RewriteFailed, e
		}
		args := ast.SliceArguments(f.args1)
		s1, e := m.MkArrayAppArguments(c.arrayFamily, ast.ASTArrayOpSelect, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		f.select1, e = m.NewRef(s1)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		args = ast.SliceArguments(f.args2)
		s2, e := m.MkArrayAppArguments(c.arrayFamily, ast.ASTArrayOpSelect, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		f.select2, e = m.NewRef(s2)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = c.arrayConjPush(f, s1, s2); e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.DelRef(&f.select2); e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.DelRef(&f.select1); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	return c.arrayConjunction(f, out, false)
}
func (c *evaluatorConfiguration) arrayEqCore(f *arrayEquality, else1, else2 ast.ASTHandle, out ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.owner.model.manager
	arity := len(f.left.stores[0]) - 1
	f.table1.arity = arity
	f.table2.arity = arity
	for i := len(f.left.stores); i > 0; {
		i--
		if e := f.table1.insert(f.left.stores[i]); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	for _, row := range f.right.stores {
		_, duplicate, e := f.table2.find(row)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if duplicate {
			continue
		}
		if e = f.table2.insert(row); e != nil {
			return rewriter.RewriteFailed, e
		}
		other, found, e := f.table1.find(row)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		val := row[arity]
		if found {
			leftVal := other[arity]
			r, e := c.arrayCompare(leftVal, val)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			switch r {
			case 1:
				if e = f.table1.remove(other); e != nil {
					return rewriter.RewriteFailed, e
				}
			case -1:
				no, e := m.MkFalse()
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				return rewriter.RewriteDone, m.SetRef(out, no)
			default:
				if e = c.arrayConjPush(f, val, leftVal); e != nil {
					return rewriter.RewriteFailed, e
				}
			}
		} else {
			r, e := c.arrayCompare(else1, val)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			switch r {
			case -1:
				no, e := m.MkFalse()
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				return rewriter.RewriteDone, m.SetRef(out, no)
			case 0:
				if e = c.arrayConjPush(f, else1, val); e != nil {
					return rewriter.RewriteFailed, e
				}
			}
		}
	}
	for _, row := range f.table1.rows.All() {
		value := row[arity]
		r, e := c.arrayCompare(value, else2)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		switch r {
		case -1:
			no, e := m.MkFalse()
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			return rewriter.RewriteDone, m.SetRef(out, no)
		case 0:
			if e = c.arrayConjPush(f, value, else2); e != nil {
				return rewriter.RewriteFailed, e
			}
		}
	}
	return c.arrayConjunction(f, out, true)
}
