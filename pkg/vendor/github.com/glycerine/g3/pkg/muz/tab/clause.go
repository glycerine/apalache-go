package tab

// Source: src/muz/tab/tab_context.cpp, tb::clause initialization and reduction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
)

// Clause owns its expressions and borrows its manager. The variable count is
// measured before equality reduction, as in the source tabulation engine.
type Clause struct {
	manager          *ast.ASTManager
	head, constraint ast.ASTRef
	predicates       []ast.ASTRef
	numVars          uint32
	index            uint32
	sequence         uint32
	nextRule         uint32
	predicateIndex   uint32
	parentIndex      uint32
	parentRule       uint32
	closed           bool
}

// NewClause initializes and simplifies a tabulation clause. The theory rewriter
// is borrowed and must use the same manager.
func NewClause(m *ast.ASTManager, rw *rewriter.TheoryRewriter, head ast.ASTHandle, predicates []ast.ASTHandle, constraint ast.ASTHandle) (built *Clause, result error) {
	if m == nil || rw == nil || rw.Manager() != m {
		return nil, ast.ErrASTHandle
	}
	c := &Clause{manager: m, nextRule: ^uint32(0)}
	attempt := &clauseConstructionAttempt{manager: m, clause: c}
	defer func() { built, result = finishClauseConstruction(attempt, result) }()
	for _, h := range append([]ast.ASTHandle{head}, predicates...) {
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			return nil, ast.ErrASTDomain
		}
	}
	var err error
	c.head, err = m.NewRef(head)
	if err != nil {
		return nil, err
	}
	for _, h := range predicates {
		r, e := m.NewRef(h)
		if e != nil {
			return nil, e
		}
		c.predicates = append(c.predicates, r)
	}
	c.constraint, err = m.NewRef(constraint)
	if err != nil {
		return nil, err
	}
	fv, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return nil, err
	}
	for _, h := range append(append([]ast.ASTHandle{head}, predicates...), constraint) {
		if err = fv.Accumulate(h); err != nil {
			return nil, err
		}
	}
	c.numVars = fv.Size()
	if err = c.reduceEqualities(rw); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Clause) Head() (ast.ASTHandle, error) {
	if c == nil || c.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return c.manager.RefGet(c.head)
}
func (c *Clause) Constraint() (ast.ASTHandle, error) {
	if c == nil || c.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return c.manager.RefGet(c.constraint)
}
func (c *Clause) NumVars() uint32 {
	if c == nil || c.closed {
		return 0
	}
	return c.numVars
}
func (c *Clause) Predicates() ([]ast.ASTHandle, error) {
	if c == nil || c.closed {
		return nil, ast.ErrASTHandle
	}
	out := make([]ast.ASTHandle, 0, len(c.predicates))
	for _, r := range c.predicates {
		h, err := c.manager.RefGet(r)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}
func (c *Clause) Close() error {
	if c == nil || c.closed {
		return nil
	}
	for i := range c.predicates {
		if c.predicates[i] != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&c.predicates[i]); err != nil {
				return err
			}
		}
	}
	if c.head != (ast.ASTRef{}) {
		if err := c.manager.DelRef(&c.head); err != nil {
			return err
		}
	}
	if c.constraint != (ast.ASTRef{}) {
		if err := c.manager.DelRef(&c.constraint); err != nil {
			return err
		}
	}
	c.closed = true
	return nil
}

// Constructor terms and values are the only legal substitution targets.
func clauseSubstitutable(m *ast.ASTManager, root ast.ASTHandle) (bool, error) {
	todo := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTVar {
			continue
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		value, err := m.IsValue(h)
		if err != nil {
			return false, err
		}
		_, constructor, err := m.DatatypeConstructorApp(h)
		if err != nil {
			return false, err
		}
		if !value && !constructor {
			return false, nil
		}
		n, err := m.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < n; j++ {
			a, e := m.GetArg(h, j)
			if e != nil {
				return false, e
			}
			todo = append(todo, a)
		}
	}
	return true, nil
}

func (c *Clause) reduceEqualities(rw *rewriter.TheoryRewriter) (result error) {
	m := c.manager
	s, err := substitution.New(m)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.Close()) }()
	if err = s.Reserve(1, c.numVars); err != nil {
		return err
	}
	constraint, err := m.RefGet(c.constraint)
	if err != nil {
		return err
	}
	forms, err := m.FlattenAnd([]ast.ASTHandle{constraint})
	if err != nil {
		return err
	}
	refs := make([]ast.ASTRef, 0, len(forms))
	defer func() {
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	for _, h := range forms {
		r, e := m.NewRef(h)
		if e != nil {
			return e
		}
		refs = append(refs, r)
	}
	for i := range refs {
		h, e := m.RefGet(refs[i])
		if e != nil {
			return e
		}
		if e = s.ApplyExpression(h, refs[i]); e != nil {
			return e
		}
		h, e = m.RefGet(refs[i])
		if e != nil {
			return e
		}
		if e = rw.Rewrite(h, refs[i]); e != nil {
			return e
		}
		h, e = m.RefGet(refs[i])
		if e != nil {
			return e
		}
		kind, e := m.GetKind(h)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp {
			continue
		}
		family, e := m.GetFamilyID(h)
		if e != nil {
			return e
		}
		if family != m.BasicFamilyID() {
			continue
		}
		op, e := m.GetDeclKind(h)
		if e != nil {
			return e
		}
		if op != ast.ASTOpEq {
			continue
		}
		v, e := m.GetArg(h, 0)
		if e != nil {
			return e
		}
		t, e := m.GetArg(h, 1)
		if e != nil {
			return e
		}
		vk, e := m.GetKind(v)
		if e != nil {
			return e
		}
		if vk != ast.ASTVar {
			v, t = t, v
			vk, e = m.GetKind(v)
			if e != nil {
				return e
			}
		}
		if vk != ast.ASTVar {
			continue
		}
		yes, e := clauseSubstitutable(m, t)
		if e != nil {
			return e
		}
		if !yes {
			continue
		}
		idx, e := m.GetVarIndex(v)
		if e != nil {
			return e
		}
		if e = s.PushScope(); e != nil {
			return e
		}
		if e = s.Insert(idx, 0, substitution.NewExprOffset(t, 0)); e != nil {
			return e
		}
		acyclic, e := s.Acyclic()
		if e != nil {
			return e
		}
		if !acyclic {
			if e = s.PopScope(1); e != nil {
				return e
			}
			continue
		}
		truth, e := m.MkTrue()
		if e != nil {
			return e
		}
		if e = m.SetRef(refs[i], truth); e != nil {
			return e
		}
	}
	all := append([]ast.ASTRef{c.head}, c.predicates...)
	for _, r := range all {
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		if e = s.ApplyExpression(h, r); e != nil {
			return e
		}
	}
	forms = forms[:0]
	for _, r := range refs {
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		forms = append(forms, h)
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, forms, c.constraint); err != nil {
		return err
	}
	h, err := m.RefGet(c.constraint)
	if err != nil {
		return err
	}
	if err = s.ApplyExpression(h, c.constraint); err != nil {
		return err
	}
	h, err = m.RefGet(c.constraint)
	if err != nil {
		return err
	}
	return rw.Rewrite(h, c.constraint)
}
