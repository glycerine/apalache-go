package qe

// Source: src/qe/qsat.cpp pred_abs scoped assumptions and concretization.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *PredicateAbstraction) Push() error {
	if err := p.open(); err != nil {
		return err
	}
	p.limits = append(p.limits, len(p.assumptions))
	return nil
}
func (p *PredicateAbstraction) Pop(count uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	// The source indexes size-count; zero/out-of-range are invalid callers.
	if count == 0 || uint64(count) > uint64(len(p.limits)) {
		return arena.ErrBounds
	}
	i := len(p.limits) - int(count)
	limit := p.limits[i]
	for len(p.assumptions) > limit {
		j := len(p.assumptions) - 1
		if err := p.manager.DelRef(&p.assumptions[j]); err != nil {
			return err
		}
		p.assumptions = p.assumptions[:j]
	}
	p.limits = p.limits[:i]
	return nil
}
func (c *predicateCall) startEvaluator(md *model.ModelCore, complete bool) error {
	p := c.owner
	if md == nil || md.Manager() != p.manager {
		return ast.ErrASTHandle
	}
	var err error
	c.evaluator, err = model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	if err != nil {
		return err
	}
	return c.evaluator.SetModelCompletion(complete)
}
func (c *predicateCall) eval(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, r)
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(r)
}
func (p *PredicateAbstraction) polarity(c *predicateCall, predicate ast.ASTHandle) (ast.ASTHandle, error) {
	value, err := c.eval(predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if p.manager.IsFalse(value) {
		return c.hold(p.manager.MkNot(predicate))
	}
	if !p.manager.IsTrue(value) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return predicate, nil
}

func (p *PredicateAbstraction) GetAssumptions(md *model.ModelCore, out *[]ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if out == nil {
		return ast.ErrASTHandle
	}
	appendAssumptions := func() error {
		for _, r := range p.assumptions {
			h, e := p.manager.RefGet(r)
			if e != nil {
				return e
			}
			if e = p.appendRef(out, h); e != nil {
				return e
			}
		}
		return nil
	}
	if md == nil {
		return appendAssumptions()
	}
	level := len(p.limits)
	if level > len(p.predicates) {
		level = len(p.predicates)
	}
	if level == 0 {
		return nil
	}
	if err = c.startEvaluator(md, true); err != nil {
		return err
	}
	if err = p.emit("get_assumptions", 162, func(out io.Writer) error { return md.WriteV2(out, p.config.Context, p.config.Globals, false) }); err != nil {
		return err
	}
	for _, r := range p.predicates[level-1] {
		h, e := p.manager.RefGet(r)
		if e != nil {
			return e
		}
		if e = p.emit("get_assumptions", 167, func(out io.Writer) error {
			_, _ = fmt.Fprintf(out, "process level: %d: ", level-1)
			if e := writeExpression(p.manager, p.config, out, h); e != nil {
				return e
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		}); e != nil {
			return e
		}
		lit, e := p.polarity(c, h)
		if e != nil {
			return e
		}
		if e = p.appendRef(&p.assumptions, lit); e != nil {
			return e
		}
	}
	if err = appendAssumptions(); err != nil {
		return err
	}
	for i := level + 1; i < len(p.predicates); i += 2 {
		for _, r := range p.predicates[i] {
			h, e := p.manager.RefGet(r)
			if e != nil {
				return e
			}
			l, ok := p.exprLevels[h]
			if !ok {
				return ast.ErrASTDomain
			}
			use := (l.Universal == uint32(i) && (l.Existential == NoQuantifierLevel || l.Existential < uint32(level))) || (l.Existential == uint32(i) && (l.Universal == NoQuantifierLevel || l.Universal < uint32(level)))
			if use {
				lit, e := p.polarity(c, h)
				if e != nil {
					return e
				}
				if e = p.appendRef(out, lit); e != nil {
					return e
				}
			}
		}
	}
	return p.emit("get_assumptions", 199, func(w io.Writer) error {
		_, _ = fmt.Fprintf(w, "level: %d\n", level)
		if e := md.WriteV2(w, p.config.Context, p.config.Globals, false); e != nil {
			return e
		}
		return p.DisplayAssumptions(w, *out)
	})
}

func (p *PredicateAbstraction) MakeAssumptionLiteral(formula ast.ASTHandle, md *model.ModelCore, level MaxLevel, defs *[]ast.ASTRef, out ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if defs == nil || (md != nil && md.Manager() != p.manager) {
		return ast.ErrASTHandle
	}
	hs, err := p.substitute(c, []ast.ASTHandle{formula}, p.pred2asm)
	if err != nil {
		return err
	}
	a := hs[0]
	if err = p.emit("mk_assumption_literal", 348, func(w io.Writer) error {
		if e := writeExpression(p.manager, p.config, w, a); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " ")
		if e := level.Display(w); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	negative, inner, err := p.manager.IsNot(a)
	if err != nil {
		return err
	}
	if !negative {
		inner = a
	}
	if b, ok := p.asm2pred[a]; ok {
		return p.manager.SetRef(out, b)
	}
	if negative {
		if b, ok := p.asm2pred[inner]; ok {
			r, e := c.hold(p.manager.MkNot(b))
			if e != nil {
				return e
			}
			return p.manager.SetRef(out, r)
		}
	}
	if _, ok := p.pred2asm[a]; ok {
		return p.manager.SetRef(out, a)
	}
	if negative {
		if _, ok := p.pred2asm[inner]; ok {
			return p.manager.SetRef(out, a)
		}
	}
	b, err := p.fresh(c, "def")
	if err != nil {
		return err
	}
	r := b
	if negative {
		r, err = c.hold(p.manager.MkNot(b))
		if err != nil {
			return err
		}
	}
	if md != nil {
		d, e := p.manager.GetDecl(b)
		if e != nil {
			return e
		}
		var value ast.ASTHandle
		if negative {
			value, e = p.manager.MkFalse()
		} else {
			value, e = p.manager.MkTrue()
		}
		if e != nil {
			return e
		}
		if e = md.RegisterConstant(d, value); e != nil {
			return e
		}
	}
	p.exprLevels[b] = level
	if err = p.Insert(b, level); err != nil {
		return err
	}
	merged := EmptyMaxLevel()
	if err = p.abstractAtoms(c, inner, &merged, defs); err != nil {
		return err
	}
	concrete, err := p.substitute(c, []ast.ASTHandle{inner}, p.lit2pred)
	if err != nil {
		return err
	}
	eq, err := c.hold(p.manager.MkEq(b, concrete[0]))
	if err != nil {
		return err
	}
	if err = p.appendRef(defs, eq); err != nil {
		return err
	}
	if _, ok := p.asm2pred[inner]; ok {
		return ast.ErrASTDomain
	}
	if err = p.appendRef(&p.defKeys, b); err != nil {
		return err
	}
	if err = p.appendRef(&p.asmKeys, inner); err != nil {
		return err
	}
	p.asm2pred[inner] = b
	p.pred2asm[b] = inner
	if err = p.emit("mk_assumption_literal", 380, func(w io.Writer) error {
		if e := writeExpression(p.manager, p.config, w, inner); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " |-> ")
		if e := writeExpression(p.manager, p.config, w, b); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	return p.manager.SetRef(out, r)
}

func (p *PredicateAbstraction) GetFreeVariables(formula ast.ASTHandle, out *[]ast.ASTRef) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	seen := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{formula}
	for len(todo) != 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[h] {
			continue
		}
		kind, err := p.manager.GetKind(h)
		if err != nil {
			return err
		}
		if kind == ast.ASTVar {
			continue
		}
		seen[h] = true
		if kind == ast.ASTQuantifier {
			body, e := p.manager.GetQuantifierBody(h)
			if e != nil {
				return e
			}
			todo = append(todo, body)
			continue
		}
		d, err := p.manager.GetDecl(h)
		if err != nil {
			return err
		}
		n, err := p.manager.GetNumArgs(h)
		if err != nil {
			return err
		}
		fid, err := p.manager.GetFamilyID(d)
		if err != nil {
			return err
		}
		if n == 0 && fid == ast.NullFamilyID {
			if err = p.appendRef(out, h); err != nil {
				return err
			}
		}
		for i := uint32(0); i < n; i++ {
			a, e := p.manager.GetArg(h, i)
			if e != nil {
				return e
			}
			todo = append(todo, a)
		}
	}
	return nil
}
func (p *PredicateAbstraction) CollectStatistics() (map[string]float64, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return map[string]float64{"qsat num predicates": float64(len(p.pred2lit))}, nil
}
func (p *PredicateAbstraction) ValidateDefinitions(md *model.ModelCore) (valid bool, err error) {
	c, err := p.begin()
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if err = c.startEvaluator(md, false); err != nil {
		return false, err
	}
	valid = true
	for a, b := range p.pred2lit {
		x, e := c.eval(a)
		if e != nil {
			return false, e
		}
		y, e := c.eval(b)
		if e != nil {
			return false, e
		}
		if x != y {
			if e = p.emit("validate_defs", 522, func(w io.Writer) error {
				for i, h := range []ast.ASTHandle{a, x, b, y} {
					if e := writeExpression(p.manager, p.config, w, h); e != nil {
						return e
					}
					if i%2 == 0 {
						_, _ = io.WriteString(w, " := ")
					} else {
						_, _ = io.WriteString(w, "\n")
					}
				}
				level, ok := p.exprLevels[a]
				if !ok {
					return ast.ErrASTDomain
				}
				if e := level.Display(w); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); e != nil {
				return false, e
			}
			valid = false
		}
	}
	return valid, nil
}

func (p *PredicateAbstraction) Display(out io.Writer) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(out, "pred2lit:\n"); err != nil {
		return err
	}
	for a, b := range p.pred2lit {
		if err := writeExpression(p.manager, p.config, out, a); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " |-> "); err != nil {
			return err
		}
		if err := writeExpression(p.manager, p.config, out, b); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	for i, preds := range p.predicates {
		if _, err := fmt.Fprintf(out, "level %d\n", i); err != nil {
			return err
		}
		for _, r := range preds {
			a, err := p.manager.RefGet(r)
			if err != nil {
				return err
			}
			if err = writeExpression(p.manager, p.config, out, a); err != nil {
				return err
			}
			if b, ok := p.pred2lit[a]; ok {
				if _, err = io.WriteString(out, " := "); err != nil {
					return err
				}
				if err = writeExpression(p.manager, p.config, out, b); err != nil {
					return err
				}
			}
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
func (p *PredicateAbstraction) DisplayAssumptions(out io.Writer, asms []ast.ASTRef) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	for _, r := range asms {
		a, err := p.manager.RefGet(r)
		if err != nil {
			return err
		}
		if err = writeExpression(p.manager, p.config, out, a); err != nil {
			return err
		}
		negative, inner, err := p.manager.IsNot(a)
		if err != nil {
			return err
		}
		if negative {
			a = inner
		}
		if level, ok := p.exprLevels[a]; ok {
			if _, err = io.WriteString(out, " - "); err != nil {
				return err
			}
			if err = level.Display(out); err != nil {
				return err
			}
		}
		if b, ok := p.pred2lit[a]; ok {
			prefix := " : "
			if negative {
				prefix += "!"
			}
			if _, err = io.WriteString(out, prefix); err != nil {
				return err
			}
			if err = writeExpression(p.manager, p.config, out, b); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
