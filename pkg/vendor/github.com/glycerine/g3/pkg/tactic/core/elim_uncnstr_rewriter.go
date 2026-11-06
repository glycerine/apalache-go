package core

// Source: src/tactic/core/elim_uncnstr_tactic.cpp rw_cfg and rw.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

type elimUnconstrainedRewriteConfig struct {
	rewriter.DefaultConfiguration
	owner     *EliminateUnconstrainedTactic
	proofs    bool
	converter *tactic.ExtensionModelConverter
	fresh     []ast.ASTHandle
	cache     map[ast.ASTHandle]ast.ASTHandle
	owned     solveEquationFrame
	frame     *solveEquationFrame
}

func (c *elimUnconstrainedRewriteConfig) MaxStepsExceeded(n uint32) (bool, error) {
	if err := c.owner.services.Cooperation.Cooperate("elim-uncnstr-vars"); err != nil {
		return false, err
	}
	allocated, err := c.owner.services.Memory.AllocationSize()
	if err != nil {
		return false, err
	}
	if allocated > c.owner.maxMemory {
		return false, tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return n > c.owner.maxSteps, nil
}

func (c *elimUnconstrainedRewriteConfig) all(args []ast.ASTHandle) bool {
	for _, a := range args {
		if !c.owner.variables[a] {
			return false
		}
	}
	return true
}

// keep models the scoped expr_refs needed while assembling a reduction. Fresh
// names/cache domains and published definitions acquire their own references.
func (c *elimUnconstrainedRewriteConfig) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return h, err
	}
	return h, c.frame.pin(h)
}
func (c *elimUnconstrainedRewriteConfig) app(fid ast.FamilyID, op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.keep(c.owner.manager.MkFamilyApp(fid, op, nil, args))
}
func (c *elimUnconstrainedRewriteConfig) apply(d ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	return c.keep(c.owner.manager.MkApp(d, args))
}
func (c *elimUnconstrainedRewriteConfig) freshFor(d ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	t, err := c.apply(d, args)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if v, found := c.cache[t]; found {
		return v, false, nil
	}
	m := c.owner.manager
	s, err := m.GetSort(t)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	v, err := c.keep(m.MkFreshConstNullable(nil, s))
	if err != nil {
		return v, false, err
	}
	if err = c.owner.traceFresh(t); err != nil {
		return v, false, err
	}
	if err = c.owned.pin(v); err != nil {
		return v, false, err
	}
	if err = c.owned.pin(t); err != nil {
		return v, false, err
	}
	c.fresh = append(c.fresh, v)
	c.cache[t] = v
	return v, true, nil
}
func (c *elimUnconstrainedRewriteConfig) addDef(v, def ast.ASTHandle) error {
	if !c.owner.variables[v] {
		return ast.ErrASTDomain
	}
	if c.converter == nil {
		return nil
	}
	d, err := c.owner.manager.GetDecl(v)
	if err != nil {
		return err
	}
	return c.converter.Insert(d, def)
}
func (c *elimUnconstrainedRewriteConfig) addDefs(args []ast.ASTHandle, value, identity ast.ASTHandle) error {
	if c.converter == nil {
		return nil
	}
	if len(args) == 0 {
		return ast.ErrASTDomain
	}
	if err := c.addDef(args[0], value); err != nil {
		return err
	}
	for _, a := range args[1:] {
		if err := c.addDef(a, identity); err != nil {
			return err
		}
	}
	return nil
}

func (c *elimUnconstrainedRewriteConfig) ReduceApp(d ast.ASTHandle, arguments ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	status = rewriter.RewriteFailed
	if arguments.Count() == 0 {
		return
	}
	m := c.owner.manager
	fid, err := m.GetFamilyID(d)
	if err != nil || fid == ast.NullFamilyID {
		return status, err
	}
	args := make([]ast.ASTHandle, arguments.Count())
	for i := range args {
		args[i], err = arguments.At(uint32(i))
		if err != nil {
			return status, err
		}
		ground, e := m.IsGround(args[i])
		if e != nil || !ground {
			return status, e
		}
	}
	frame := solveEquationFrame{manager: m}
	c.frame = &frame
	defer func() { c.frame = nil; err = errors.Join(err, frame.Close()) }()
	kind, err := m.GetDeclKind(d)
	if err != nil {
		return status, err
	}
	var value ast.ASTHandle
	switch fid {
	case m.BasicFamilyID():
		value, err = c.basic(d, kind, args)
	case c.owner.arithmetic:
		value, err = c.arith(d, kind, args)
	case c.owner.bitVector:
		value, err = c.bv(d, kind, args)
	case c.owner.array:
		value, err = c.arrayApp(d, kind, args)
	case c.owner.datatype:
		value, err = c.datatypeApp(d, kind, args)
	}
	if err != nil || value.IsNull() {
		return status, err
	}
	if err = m.SetRef(out, value); err != nil {
		return status, err
	}
	if c.proofs {
		original, e := c.apply(d, args)
		if e != nil {
			return status, e
		}
		equality, e := c.keep(m.MkEq(original, value))
		if e != nil {
			return status, e
		}
		intro, e := c.keep(m.MkProofDefIntro(equality))
		if e != nil {
			return status, e
		}
		pr, e := c.keep(m.MkProofApplyDef(original, value, intro))
		if e != nil {
			return status, e
		}
		if err = m.SetRef(proof, pr); err != nil {
			return status, err
		}
	}
	return rewriter.RewriteDone, nil
}
