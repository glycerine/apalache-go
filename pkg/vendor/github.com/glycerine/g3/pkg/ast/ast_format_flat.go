package ast

// Source: src/ast/format.cpp:119-145; src/ast/recurse_expr.h:28-60;
// src/ast/recurse_expr_def.h:25-107.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Formatting recursion borrows node references. The cache is lookup-only;
// traversal order comes exclusively from todo and each application's arguments.
type astFormatFlatFrame struct {
	next               *astFormatFlatFrame
	owner              *ASTManager
	cache              map[astNodeRef]astNodeRef
	todo               []astNodeRef
	results            []ASTHandle
	family             FamilyID
	header             uint64
	invocation         uint64
	bodyCause          error
	sourceDone, active bool
}

func (f *astFormatFlatFrame) bytes() uint64 { return f.header }
func (f *astFormatFlatFrame) find(key astNodeRef) (astNodeRef, bool, error) {
	if e := f.owner.validNode(key); e != nil {
		return astNodeRef{}, false, e
	}
	value, found := f.cache[key]
	return value, found, nil
}
func (f *astFormatFlatFrame) insert(key, value astNodeRef) error {
	if e := f.owner.validNode(key); e != nil {
		return e
	}
	f.cache[key] = value
	return nil
}
func (f *astFormatFlatFrame) sourceDestroy() {
	if f.sourceDone {
		return
	}
	// Borrowed pointer elements have no AST INC/DEC ownership.
	f.results = nil
	f.todo = nil
	f.cache = nil
	f.sourceDone = true
	f.active = false
}
func (m *ASTManager) formatFramesBytes() uint64 {
	var n uint64
	for f := m.formatFrames; f != nil; f = f.next {
		n += f.bytes()
	}
	return n
}
func (m *ASTManager) releaseFormatFrames() {
	for f := m.formatFrames; f != nil; {
		next := f.next
		f.sourceDestroy()
		m.BudgetField.Release(f.header)
		f.header = 0
		f = next
	}
	m.formatFrames = nil
}
func (m *ASTManager) flatVisit(f *astFormatFlatFrame, n astNodeRef) (astNodeRef, error) {
	if m.nodeAt(n.slot).kind != ASTApp {
		return astNodeRef{}, ErrASTDomain
	} // selected source UNREACHABLE
	a := m.appAt(n.slot)
	id, kind, e := m.declIdentity(a.decl)
	if e != nil {
		return astNodeRef{}, e
	}
	if id == f.family && kind == ASTFormatLineBreak {
		name, e := m.Symbols.Intern(" ")
		if e != nil {
			return astNodeRef{}, e
		}
		p := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
		return m.formatApp(ASTFormatString, &p, nil)
	}
	if id == f.family && kind == ASTFormatLineBreakExt {
		info, e := m.funcInfo(m.declAt(a.decl.slot).info)
		if e != nil {
			return astNodeRef{}, e
		}
		p, e := m.infoParameter(info, 0)
		if e != nil {
			return astNodeRef{}, e
		}
		value, e := m.parameter(p)
		if e != nil {
			return astNodeRef{}, e
		}
		if value.Kind != ASTParameterSymbol {
			return astNodeRef{}, ErrASTDomain
		}
		// symbol bare_str access is performed before actual mk_string symbol ctor.
		name, e := m.Symbols.InternBorrowedSymbolCString(value.symbol)
		if e != nil {
			return astNodeRef{}, e
		}
		parameter := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
		return m.formatApp(ASTFormatString, &parameter, nil)
	}
	if id == f.family && kind == ASTFormatChoice {
		return m.readInternal(a.args, 0)
	}
	return m.mkAppArguments(a.decl, SliceArguments(f.results))
}
func (m *ASTManager) formatFlat(root astNodeRef) (result astNodeRef, err error) {
	family, e := m.formatFamilyID()
	if e != nil {
		return astNodeRef{}, e
	}
	if m.formatFrameSerial == math.MaxUint64 {
		return astNodeRef{}, arena.ErrBounds
	}
	header := uint64(reflect.TypeOf(astFormatFlatFrame{}).Size())
	if e = m.BudgetField.Reserve(header); e != nil {
		return astNodeRef{}, e
	}
	m.formatFrameSerial++
	f := &astFormatFlatFrame{owner: m, family: family, header: header, invocation: m.formatFrameSerial, active: true, next: m.formatFrames}
	m.formatFrames = f
	defer func() {
		if err != nil && f.bodyCause == nil {
			f.bodyCause = err
		}
		f.sourceDestroy()
		if err == nil {
			m.formatFrames = f.next
			m.BudgetField.Release(f.header)
			f.header = 0
		}
	}()
	f.cache = make(map[astNodeRef]astNodeRef)
	f.todo = append(f.todo, root)
	for len(f.todo) != 0 {
		n := f.todo[len(f.todo)-1]
		_, found, e := f.find(n)
		if e != nil {
			return astNodeRef{}, e
		}
		if found {
			f.todo = f.todo[:len(f.todo)-1]
			continue
		}
		if e = m.validNode(n); e != nil {
			return astNodeRef{}, e
		}
		visited := true
		switch m.nodeAt(n.slot).kind {
		case ASTApp:
			a := m.appAt(n.slot)
			for j := uint32(0); j < a.count; j++ {
				child, e := m.readInternal(a.args, j)
				if e != nil {
					return astNodeRef{}, e
				}
				_, found, e = f.find(child)
				if e != nil {
					return astNodeRef{}, e
				}
				if !found {
					f.todo = append(f.todo, child)
					visited = false
				}
			}
		case ASTQuantifier:
			child := m.quantifierAt(n.slot).body
			_, found, e = f.find(child)
			if e != nil {
				return astNodeRef{}, e
			}
			if !found {
				f.todo = append(f.todo, child)
				visited = false
			}
		}
		if !visited {
			continue
		}
		f.todo = f.todo[:len(f.todo)-1]
		if m.nodeAt(n.slot).kind != ASTApp {
			return astNodeRef{}, ErrASTDomain
		}
		f.results = f.results[:0]
		a := m.appAt(n.slot)
		for j := uint32(0); j < a.count; j++ {
			child, e := m.readInternal(a.args, j)
			if e != nil {
				return astNodeRef{}, e
			}
			value, found, e := f.find(child)
			if e != nil {
				return astNodeRef{}, e
			}
			if !found {
				return astNodeRef{}, ErrASTDomain
			}
			f.results = append(f.results, m.external(value))
		}
		result, e = m.flatVisit(f, n)
		if e != nil {
			return astNodeRef{}, e
		}
		if e = f.insert(n, result); e != nil {
			return astNodeRef{}, e
		}
	}
	result, found, e := f.find(root)
	if e != nil {
		return astNodeRef{}, e
	}
	if !found {
		return astNodeRef{}, ErrASTDomain
	}
	return result, nil
}
func (m *ASTManager) FormatFlat(root ASTHandle) (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	r, e := f.requireExpr(root)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e = f.formatFlat(r)
	return f.external(r), e
}
func (m *ASTManager) FormatGroup(root ASTHandle) (ASTHandle, error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	defer f.unlockFormat()
	r, e := f.requireExpr(root)
	if e != nil {
		return ASTHandle{}, e
	}
	flat, e := f.formatFlat(r)
	if e != nil {
		return ASTHandle{}, e
	}
	args := [2]ASTHandle{f.external(flat), root}
	r, e = f.formatApp(ASTFormatChoice, nil, args[:])
	return f.external(r), e
}
