package model

// Source: src/model/model2expr.cpp:79-155.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Model2ExprBuilder owns the source's top-level expr_ref_vector and temporary
// expression. A caller retains the builder until Close succeeds, including
// after an interrupted AST destructor.
type Model2ExprBuilder struct {
	model   *ModelCore
	manager *ast.ASTManager
	conjs   []ast.ASTHandle
	tmp     ast.ASTRef
	current *model2ExprFunctionFrame
	started bool
	closed  bool
}

type model2ExprFunctionFrame struct {
	manager  *ast.ASTManager
	function ast.ASTRef
	cond     ast.ASTRef
	args     []ast.ASTHandle
	revVars  []ast.ASTHandle
	entry    *modelEntryCondition
	subst    *rewriter.VarSubstitution
}

func NewModel2ExprBuilder(md *ModelCore) (*Model2ExprBuilder, error) {
	if md == nil {
		return nil, ast.ErrASTHandle
	}
	if e := md.open(); e != nil {
		return nil, e
	}
	b := &Model2ExprBuilder{model: md, manager: md.manager}
	var e error
	b.tmp, e = b.manager.NewRef(ast.ASTHandle{})
	return b, e
}

func (f *model2ExprFunctionFrame) close() error {
	if f.entry != nil {
		if e := f.entry.Close(); e != nil {
			return e
		}
		f.entry = nil
	}
	if f.subst != nil {
		if e := f.subst.Close(); e != nil {
			return e
		}
		f.subst = nil
	}
	if e := releaseArrayRoots(f.manager, &f.revVars); e != nil {
		return e
	}
	if e := releaseArrayRoots(f.manager, &f.args); e != nil {
		return e
	}
	if f.cond != (ast.ASTRef{}) {
		if e := f.manager.DelRef(&f.cond); e != nil {
			return e
		}
	}
	if f.function != (ast.ASTRef{}) {
		if e := f.manager.DelRef(&f.function); e != nil {
			return e
		}
	}
	return nil
}

func (b *Model2ExprBuilder) Close() error {
	if b == nil {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	if b.current != nil {
		if e := b.current.close(); e != nil {
			return e
		}
		b.current = nil
	}
	if b.tmp != (ast.ASTRef{}) {
		if e := b.manager.DelRef(&b.tmp); e != nil {
			return e
		}
	}
	if e := releaseArrayRoots(b.manager, &b.conjs); e != nil {
		return e
	}
	b.closed = true
	return nil
}

func (b *Model2ExprBuilder) Build(out ast.ASTRef) (err error) {
	if b == nil {
		return ast.ErrASTHandle
	}
	if b.closed {
		return arena.ErrClosed
	}
	if b.started {
		return ast.ErrASTPending
	}
	if e := b.model.open(); e != nil {
		return e
	}
	b.started = true
	m := b.manager
	n, e := b.model.NumConstants()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		d, e := b.model.Constant(i)
		if e != nil {
			return e
		}
		v, e := b.model.ConstantInterpretation(d)
		if e != nil {
			return e
		}
		constant, e := m.MkApp(d, nil)
		if e != nil {
			return e
		}
		eq, e := m.MkEq(constant, v)
		if e != nil {
			return e
		}
		if e = appendArrayRoot(m, &b.conjs, eq); e != nil {
			return e
		}
	}
	functionCount := b.model.NumFunctionDeclarations()
	for i := uint32(0); i < functionCount; i++ {
		if e = b.buildFunction(i); e != nil {
			return e
		}
	}
	if e = rewriter.DefaultAnd(m, b.conjs, out); e != nil {
		return e
	}
	return b.Close()
}

func (b *Model2ExprBuilder) buildFunction(i uint32) (err error) {
	m := b.manager
	d, e := b.model.Function(i)
	if e != nil {
		return e
	}
	fi, e := b.model.FunctionInterpretation(d)
	if e != nil {
		return e
	}
	if fi == nil {
		return ast.ErrASTHandle
	}
	arity, e := m.GetArity(d)
	if e != nil {
		return e
	}
	fresh := NewFreshName(m)
	if e = fresh.AddAST(d); e != nil {
		return e
	}
	count := fi.NumEntries()
	for j := uint32(0); j < count; j++ {
		entry := fi.entries[j]
		value, e := entry.Result()
		if e != nil {
			return e
		}
		if e = fresh.AddAST(value); e != nil {
			return e
		}
		for k := uint32(0); k < arity; k++ {
			arg, e := entry.Argument(k)
			if e != nil {
				return e
			}
			if e = fresh.AddAST(arg); e != nil {
				return e
			}
		}
	}
	f := &model2ExprFunctionFrame{manager: m}
	b.current = f
	defer func() {
		if f != nil {
			closed := f.close()
			err = errors.Join(err, closed)
			if closed == nil {
				b.current = nil
			}
		}
	}()
	f.function, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	f.cond, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	for j := uint32(0); j < arity; j++ {
		sort, e := m.GetDomain(d, j)
		if e != nil {
			return e
		}
		v, e := m.MkVar(j, sort)
		if e != nil {
			return e
		}
		if e = appendArrayRoot(m, &f.args, v); e != nil {
			return e
		}
	}
	app, e := m.MkApp(d, f.args)
	if e != nil {
		return e
	}
	if e = m.SetRef(f.function, app); e != nil {
		return e
	}
	partial, e := fi.IsPartial()
	if e != nil {
		return e
	}
	if partial && count == 0 {
		return nil
	}
	if partial {
		last := fi.entries[count-1]
		if e = b.entryCondition(f, arity, last); e != nil {
			return e
		}
		cond, e := m.RefGet(f.cond)
		if e != nil {
			return e
		}
		value, e := last.Result()
		if e != nil {
			return e
		}
		eq, e := m.MkEq(app, value)
		if e != nil {
			return e
		}
		term, e := m.MkImplies(cond, eq)
		if e != nil {
			return e
		}
		if e = m.SetRef(b.tmp, term); e != nil {
			return e
		}
		for j := count - 1; j > 0; {
			j--
			if e = b.entryCondition(f, arity, fi.entries[j]); e != nil {
				return e
			}
			cond, e := m.RefGet(f.cond)
			if e != nil {
				return e
			}
			value, e := fi.entries[j].Result()
			if e != nil {
				return e
			}
			eq, e := m.MkEq(app, value)
			if e != nil {
				return e
			}
			previous, e := m.RefGet(b.tmp)
			if e != nil {
				return e
			}
			term, e := m.MkIte(cond, eq, previous)
			if e != nil {
				return e
			}
			if e = m.SetRef(b.tmp, term); e != nil {
				return e
			}
		}
	} else {
		fallback, e := fi.Else()
		if e != nil {
			return e
		}
		if e = fresh.AddAST(fallback); e != nil {
			return e
		}
		if e = m.SetRef(b.tmp, fallback); e != nil {
			return e
		}
		for j := count; j > 0; {
			j--
			if e = b.entryCondition(f, arity, fi.entries[j]); e != nil {
				return e
			}
			cond, e := m.RefGet(f.cond)
			if e != nil {
				return e
			}
			value, e := fi.entries[j].Result()
			if e != nil {
				return e
			}
			previous, e := m.RefGet(b.tmp)
			if e != nil {
				return e
			}
			term, e := m.MkIte(cond, value, previous)
			if e != nil {
				return e
			}
			if e = m.SetRef(b.tmp, term); e != nil {
				return e
			}
		}
		body, e := m.RefGet(b.tmp)
		if e != nil {
			return e
		}
		eq, e := m.MkEq(app, body)
		if e != nil {
			return e
		}
		if e = m.SetRef(b.tmp, eq); e != nil {
			return e
		}
	}
	sorts := make([]ast.ASTHandle, 0, arity)
	names := make([]usymbol.Symbol, 0, arity)
	for j := uint32(0); j < arity; j++ {
		sort, e := m.GetDomain(d, j)
		if e != nil {
			return e
		}
		sorts = append(sorts, sort)
		v, e := m.MkVar(arity-j-1, sort)
		if e != nil {
			return e
		}
		if e = appendArrayRoot(m, &f.revVars, v); e != nil {
			return e
		}
		name, e := fresh.Next()
		if e != nil {
			return e
		}
		names = append(names, name)
	}
	if arity != 0 {
		f.subst, e = rewriter.NewVarSubstitution(m, false)
		if e != nil {
			return e
		}
		body, e := m.RefGet(b.tmp)
		if e != nil {
			return e
		}
		if e = f.subst.Substitute(body, f.revVars, b.tmp); e != nil {
			return e
		}
		if e = f.subst.Close(); e != nil {
			return e
		}
		f.subst = nil
		body, e = m.RefGet(b.tmp)
		if e != nil {
			return e
		}
		quant, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
		if e != nil {
			return e
		}
		if e = m.SetRef(b.tmp, quant); e != nil {
			return e
		}
	}
	term, e := m.RefGet(b.tmp)
	if e != nil {
		return e
	}
	return appendArrayRoot(m, &b.conjs, term)
}

func (b *Model2ExprBuilder) entryCondition(f *model2ExprFunctionFrame, arity uint32, entry *FunctionEntry) error {
	var e error
	f.entry, e = newModelEntryCondition(b.manager, arity, entry, f.cond)
	if e != nil {
		return e
	}
	if e = f.entry.Close(); e != nil {
		return e
	}
	f.entry = nil
	return nil
}
