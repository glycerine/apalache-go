package ast

// Source: src/ast/ast.cpp:2114-2128,2259-2261,2368-2373;
// src/util/symbol.h:97-110.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) traceApp(r astNodeRef) error {
	if m.TraceStream == nil {
		return nil
	}
	out := m.TraceStream
	if e := out.AppendCString("[mk-app] #"); e != nil {
		return e
	}
	if e := out.AppendUint32(m.nodeAt(r.slot).id); e != nil {
		return e
	}
	if e := out.AppendCString(" "); e != nil {
		return e
	}
	a := m.appAt(r.slot)
	d := m.declAt(a.decl.slot)
	special := false
	if a.count == 0 {
		// Actual strcmp against the literal name reads borrowed spelling here,
		// before the source else-if label predicate and low-level printer.
		if !d.name.IsNumerical() && !d.name.IsMarked() && !d.name.IsNull() {
			view, e := uparams.SymbolCStringView(m.Symbols, d.name)
			if e != nil {
				return e
			}
			special = true
			for i, want := range []byte{'i', 'n', 't', 0} {
				ch, e := view.Byte(uint64(i))
				if e != nil {
					return e
				}
				if ch != want {
					special = false
					break
				}
			}
		}
	}
	if !special && d.info != (ASTFuncDeclInfo{}) {
		info, e := m.funcInfo(d.info)
		if e != nil {
			return e
		}
		special = info.family == 1 && info.kind == 1
	}
	if special {
		return m.lowLevelPrint(out, r, true, true)
	}
	if e := appendASTSymbol(out, m, d.name); e != nil {
		return e
	}
	for i := uint32(0); i < a.count; i++ {
		if e := out.AppendCString(" #"); e != nil {
			return e
		}
		child, e := m.readInternal(a.args, i)
		if e != nil {
			return e
		}
		if e = m.validNode(child); e != nil {
			return e
		}
		if e = out.AppendUint32(m.nodeAt(child.slot).id); e != nil {
			return e
		}
	}
	return out.AppendCString("\n")
}
func (m *ASTManager) traceVar(r astNodeRef) error {
	if m.TraceStream == nil {
		return nil
	}
	if e := m.TraceStream.AppendCString("[mk-var] #"); e != nil {
		return e
	}
	if e := m.TraceStream.AppendUint32(m.nodeAt(r.slot).id); e != nil {
		return e
	}
	return m.TraceStream.AppendCString("\n")
}
func (m *ASTManager) traceQuantifier(r astNodeRef) error {
	if m.TraceStream == nil {
		return nil
	}
	out := m.TraceStream
	if e := out.AppendCString("[mk-quant] #"); e != nil {
		return e
	}
	if e := out.AppendUint32(m.nodeAt(r.slot).id); e != nil {
		return e
	}
	if e := out.AppendCString(" "); e != nil {
		return e
	}
	q := m.quantifierAt(r.slot)
	if e := appendASTSymbol(out, m, q.qid); e != nil {
		return e
	}
	for i := uint32(0); i < q.patternCount; i++ {
		if e := out.AppendCString(" #"); e != nil {
			return e
		}
		child, e := m.readInternal(q.patterns, i)
		if e != nil {
			return e
		}
		if e = m.validNode(child); e != nil {
			return e
		}
		if e = out.AppendUint32(m.nodeAt(child.slot).id); e != nil {
			return e
		}
	}
	if e := out.AppendCString(" #"); e != nil {
		return e
	}
	if e := m.validNode(q.body); e != nil {
		return e
	}
	if e := out.AppendUint32(m.nodeAt(q.body.slot).id); e != nil {
		return e
	}
	return out.AppendCString("\n")
}
