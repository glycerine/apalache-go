package ast

// Source: src/ast/ast_ll_pp.cpp:274-328.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DisplayBoundedLowLevel is the retained mk_bounded_pp path used by source
// diagnostics. The caller supplies the source depth (three by default).
func (m *ASTManager) DisplayBoundedLowLevel(out *utext.NumericTextStream, h ASTHandle, depth uint32) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	return m.boundedLowLevelPrint(out, r, depth)
}

func (m *ASTManager) boundedLowLevelPrint(out *utext.NumericTextStream, r astNodeRef, depth uint32) (result error) {
	f := &m.LowLevel
	if f.Active {
		return ErrASTPending
	}
	if f.invocation == math.MaxUint64 {
		return arena.ErrBounds
	}
	*f = astLowLevelFrame{invocation: f.invocation + 1, Active: true}
	m.traceExecuting = true
	defer func() { m.traceExecuting = false }()
	symbol, err := m.Symbols.Intern("arith")
	if err != nil {
		f.Active = false
		return err
	}
	family, err := m.families.MkID(symbol)
	if err != nil {
		f.Active = false
		return err
	}
	p := astLowLevelPrinter{manager: m, out: out, root: astNodeRef{}, arithFamily: family, frame: f}
	defer func() {
		if result != nil && f.bodyCause == nil {
			f.bodyCause = result
		}
		result = errors.Join(result, m.llFrameDestroy(false))
	}()
	return p.displayBounded(r, depth)
}

func (p *astLowLevelPrinter) displayBounded(r astNodeRef, depth uint32) error {
	m := p.manager
	if err := m.validNode(r); err != nil {
		return err
	}
	switch m.nodeAt(r.slot).kind {
	case ASTVar:
		if err := p.out.AppendCString("(:var "); err != nil {
			return err
		}
		if err := p.out.AppendUint32(m.varAt(r.slot).index); err != nil {
			return err
		}
		return p.out.AppendCString(")")
	case ASTApp:
		a := m.appAt(r.slot)
		if depth == 0 || a.count == 0 || a.count > depth && a.count > 16 {
			return p.child(r)
		}
		if err := p.out.AppendCString("("); err != nil {
			return err
		}
		if err := p.name(a.decl); err != nil {
			return err
		}
		if err := p.displayParams(a.decl); err != nil {
			return err
		}
		for i := uint32(0); i < a.count; i++ {
			if err := p.out.AppendCString(" "); err != nil {
				return err
			}
			child, err := m.readInternal(a.args, i)
			if err != nil {
				return err
			}
			if err = p.displayBounded(child, depth-1); err != nil {
				return err
			}
		}
		return p.out.AppendCString(")")
	default:
		return p.ref(r)
	}
}
