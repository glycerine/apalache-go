package ast

// Source: src/ast/pp.cpp:22-132; src/ast/pp_params.pyg:1-23.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type astFormatPPPair struct {
	format astNodeRef
	indent uint32
}
type astFormatPPFrame struct {
	next                                                      *astFormatPPFrame
	owner                                                     *ASTManager
	parameters                                                uparams.ModuleParamsOwner
	helper                                                    *uparams.ModuleParamsFrame
	todo                                                      []astFormatPPPair
	size                                                      uint32
	space                                                     astNodeRef
	spaceAcquired, spaceConsumed, sourceDone, astContextEnded bool
	header                                                    uint64
	invocation                                                uint64
	bodyCause, sourceCause, ownCause                          error
}

func (f *astFormatPPFrame) bytes() uint64 {
	n := f.header
	for h := f.parameters.Helpers; h != nil; h = h.Next {
		n += h.Header
	}
	return n
}
func (f *astFormatPPFrame) push(r astNodeRef, indent uint32) error {
	if f.size == math.MaxUint32 {
		return arena.ErrBounds
	}
	if f.size == uint32(len(f.todo)) {
		f.todo = append(f.todo, astFormatPPPair{r, indent})
	} else {
		f.todo[f.size] = astFormatPPPair{r, indent}
	}
	f.size++
	return nil
}
func (m *ASTManager) ppFramesBytes() uint64 {
	var n uint64
	for f := m.ppFrames; f != nil; f = f.next {
		n += f.bytes()
	}
	return n
}
func (m *ASTManager) ppAssertFormat(r astNodeRef, g *uparams.GlobalParamsService, line int32) error {
	// SASSERT switch is reached before either actual expression operand.
	if !ASTSourceDebug {
		return nil
	}
	var enabled bool
	var e error
	if g == nil {
		enabled, e = diagnostic.BorrowProcessDebugContext().AssertionsEnabled()
	} else {
		enabled, e = g.NamespaceAssertionsEnabled()
	}
	if e != nil {
		return e
	}
	if !enabled {
		return nil
	}
	if e = m.validNode(r); e != nil {
		return e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return ErrASTDomain
	}
	id, _, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil {
		return e
	}
	symbol, e := m.Symbols.Intern("format")
	if e != nil {
		return e
	}
	expected, e := m.families.GetID(symbol)
	if e != nil {
		return e
	}
	if id != expected {
		if g == nil {
			return diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/ast/pp.cpp", line, "f->get_family_id() == fm(m).get_family_id(\"format\")")
		}
		return g.NamespaceAssertionFailure("src/ast/pp.cpp", line, "f->get_family_id() == fm(m).get_family_id(\"format\")")
	}
	return nil
}
func (m *ASTManager) formatKind(r astNodeRef) (int32, error) {
	if e := m.validNode(r); e != nil {
		return 0, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return 0, ErrASTDomain
	}
	_, kind, e := m.declIdentity(m.appAt(r.slot).decl)
	return kind, e
}
func (m *ASTManager) formatParameter(r astNodeRef) (*astParameterRecord, error) {
	if e := m.validNode(r); e != nil {
		return nil, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return nil, ErrASTDomain
	}
	d := m.appAt(r.slot).decl
	info, e := m.funcInfo(m.declAt(d.slot).info)
	if e != nil {
		return nil, e
	}
	token, e := m.infoParameter(info, 0)
	if e != nil {
		return nil, e
	}
	return m.parameter(token)
}
func (m *ASTManager) formatSymbolView(s usymbol.Symbol) (uparams.ParamsStringView, error) {
	return uparams.SymbolCStringView(m.Symbols, s)
}
func (m *ASTManager) formatSpaceToBreak(r astNodeRef, g *uparams.GlobalParamsService) (uint32, bool, error) {
	if e := m.ppAssertFormat(r, g, 25); e != nil {
		return 0, false, e
	}
	kind, e := m.formatKind(r)
	if e != nil {
		return 0, false, e
	}
	a := m.appAt(r.slot)
	switch kind {
	case ASTFormatString:
		p, e := m.formatParameter(r)
		if e != nil {
			return 0, false, e
		}
		if p.Kind != ASTParameterSymbol {
			return 0, false, ErrASTDomain
		}
		view, e := m.formatSymbolView(p.symbol)
		if e != nil {
			return 0, false, e
		}
		n, e := view.Length()
		return uint32(n), false, e
	case ASTFormatChoice, ASTFormatIndent:
		child, e := m.readInternal(a.args, 0)
		if e != nil {
			return 0, false, e
		}
		return m.formatSpaceToBreak(child, g)
	case ASTFormatCompose:
		var n uint32
		for i := uint32(0); i < a.count; i++ {
			child, e := m.readInternal(a.args, i)
			if e != nil {
				return 0, false, e
			}
			size, line, e := m.formatSpaceToBreak(child, g)
			if e != nil {
				return 0, false, e
			}
			n += size
			if line {
				return n, true, nil
			}
		}
		return n, false, nil
	case ASTFormatLineBreak, ASTFormatLineBreakExt:
		return 0, true, nil
	default:
		return 0, false, nil
	}
}
func (f *astFormatPPFrame) sourceDestroy(body error) error {
	if f.bodyCause == nil {
		f.bodyCause = body
	}
	if f.sourceCause != nil {
		return errors.Join(body, f.sourceCause)
	}
	m := f.owner
	if f.spaceAcquired && !f.spaceConsumed {
		consumed, e := m.decNode(f.space)
		if consumed {
			f.spaceConsumed = true
		}
		if e != nil {
			f.sourceCause = e
			return errors.Join(body, e)
		}
	}
	f.todo = nil
	if f.helper != nil {
		f.parameters.Running = true
		e := f.parameters.FinishHelper(f.helper, body)
		f.parameters.Running = false
		if e != nil && f.helper.DestructorCause != nil {
			f.sourceCause = f.helper.DestructorCause
			return e
		}
	}
	f.sourceDone = true
	return body
}
func (m *ASTManager) releasePPFrames() error {
	for f := m.ppFrames; f != nil; f = f.next {
		if f.spaceAcquired && !f.spaceConsumed && !f.astContextEnded {
			consumed, e := m.decNode(f.space)
			if consumed {
				f.spaceConsumed = true
			}
			if e != nil {
				if f.ownCause == nil {
					f.ownCause = e
				}
				return e
			}
		}
		// This owning path is invoked only after the actual containing AST table
		// has ended: an already-consumed source child cannot be DEC'd again.
		f.todo = nil
		if f.parameters.Self != nil {
			if e := f.parameters.CloseHelpers(); e != nil {
				if f.ownCause == nil {
					f.ownCause = e
				}
				return e
			}
		}
	}
	for f := m.ppFrames; f != nil; f = f.next {
		m.BudgetField.Release(f.header)
		f.header = 0
	}
	m.ppFrames = nil
	return nil
}

// A nil parameter-owner trio uses the source pp defaults for standalone AST
// warning and trace callers. Bound owners keep their module-parameter scope.
func (m *ASTManager) PrintFormat(out *utext.NumericTextStream, h ASTHandle, c *uparams.ParamsContext, p *uparams.ParamsRef, g *uparams.GlobalParamsService) (result error) {
	f, e := m.lockFormat()
	if e != nil {
		return e
	}
	defer f.unlockFormat()
	defaults := c == nil && p == nil && g == nil
	if out == nil || !defaults && (c == nil || c.CheckServices(f.Symbols, nil) != nil || g == nil || g.CheckOwner(c, f.BudgetField) != nil) {
		return uparams.ErrParamsOwner
	}
	r, e := f.requireExpr(h)
	if e != nil {
		return e
	}
	if f.ppFrameSerial == math.MaxUint64 {
		return arena.ErrBounds
	}
	n := uint64(reflect.TypeOf(astFormatPPFrame{}).Size())
	if e = f.BudgetField.Reserve(n); e != nil {
		return e
	}
	f.ppFrameSerial++
	frame := &astFormatPPFrame{owner: f, header: n, invocation: f.ppFrameSerial, next: f.ppFrames}
	f.ppFrames = frame
	defer func() {
		result = frame.sourceDestroy(result)
		if result == nil && frame.sourceDone {
			f.ppFrames = frame.next
			f.BudgetField.Release(frame.header)
			frame.header = 0
		}
	}()
	maxWidth, maxRibbon, maxLines, maxIndent := uint32(80), uint32(80), uint32(math.MaxUint32), uint32(math.MaxUint32)
	bounded, singleLine := false, false
	if !defaults {
		frame.parameters = uparams.ModuleParamsOwner{Context: c, Globals: g, Budget: f.BudgetField}
		frame.parameters.Self = &frame.parameters
		frame.parameters.Running = true
		frame.helper, e = frame.parameters.BeginHelper(p, "pp")
		frame.parameters.Running = false
		if e != nil {
			return e
		}
		frame.parameters.Running = true
		maxWidth, e = frame.parameters.FrameUInt(frame.helper, "max_width", 80)
		if e == nil {
			maxRibbon, e = frame.parameters.FrameUInt(frame.helper, "max_ribbon", 80)
		}
		if e == nil {
			maxLines, e = frame.parameters.FrameUInt(frame.helper, "max_num_lines", math.MaxUint32)
		}
		if e == nil {
			maxIndent, e = frame.parameters.FrameUInt(frame.helper, "max_indent", math.MaxUint32)
		}
		if e == nil {
			bounded, e = frame.parameters.FrameBool(frame.helper, "bounded", false)
		}
		if e == nil {
			singleLine, e = frame.parameters.FrameBool(frame.helper, "single_line", false)
		}
		frame.parameters.Running = false
		if e != nil {
			return e
		}
	}
	var pos, ribbonPos, line uint32
	if e = frame.push(r, 0); e != nil {
		return e
	}
	symbol, e := f.Symbols.Intern(" ")
	if e != nil {
		return e
	}
	parameter := astParameterRecord{Kind: ASTParameterSymbol, symbol: symbol}
	frame.space, e = f.formatApp(ASTFormatString, &parameter, nil)
	if e != nil {
		return e
	}
	if e = f.incNode(frame.space); e != nil {
		return e
	}
	frame.spaceAcquired = true
	for frame.size != 0 {
		if line >= maxLines {
			return nil
		}
		pair := frame.todo[frame.size-1]
		r = pair.format
		indent := pair.indent
		frame.size--
		if e = f.ppAssertFormat(r, g, 84); e != nil {
			return e
		}
		kind, e := f.formatKind(r)
		if e != nil {
			return e
		}
		a := f.appAt(r.slot)
		switch kind {
		case ASTFormatString:
			if bounded && pos > maxWidth {
				break
			}
			parameter, e := f.formatParameter(r)
			if e != nil {
				return e
			}
			if parameter.Kind != ASTParameterSymbol {
				return ErrASTDomain
			}
			view, e := f.formatSymbolView(parameter.symbol)
			if e != nil {
				return e
			}
			wide, e := view.Length()
			if e != nil {
				return e
			}
			length := uint32(wide)
			if bounded && pos+length > maxWidth {
				if e = out.AppendCString("..."); e != nil {
					return e
				}
				break
			}
			pos += length
			ribbonPos += length
			// Symbol insertion is separate from its earlier strlen and inherits the
			// actual caller stream flags, including width consumption and badbit.
			if e = appendASTSymbol(out, f, parameter.symbol); e != nil {
				return e
			}
		case ASTFormatIndent:
			child, e := f.readInternal(a.args, 0)
			if e != nil {
				return e
			}
			parameter, e := f.formatParameter(r)
			if e != nil {
				return e
			}
			if parameter.Kind != ASTParameterInt {
				return ErrASTDomain
			}
			next := indent + uint32(parameter.integer)
			if maxIndent < next {
				next = maxIndent
			}
			if e = frame.push(child, next); e != nil {
				return e
			}
		case ASTFormatCompose:
			i := a.count
			for i > 0 {
				i--
				child, e := f.readInternal(a.args, i)
				if e != nil {
					return e
				}
				if e = frame.push(child, indent); e != nil {
					return e
				}
			}
		case ASTFormatChoice:
			left := maxWidth - pos
			if maxRibbon-pos < left {
				left = maxRibbon - pos
			}
			spaceLeft := int32(left)
			var child astNodeRef
			if spaceLeft > 0 {
				first, e := f.readInternal(a.args, 0)
				if e != nil {
					return e
				}
				size, _, e := f.formatSpaceToBreak(first, g)
				if e != nil {
					return e
				}
				if size <= uint32(spaceLeft) {
					child = first
				} else {
					child, e = f.readInternal(a.args, 1)
					if e != nil {
						return e
					}
				}
			} else {
				child, e = f.readInternal(a.args, 1)
				if e != nil {
					return e
				}
			}
			if e = frame.push(child, indent); e != nil {
				return e
			}
		case ASTFormatLineBreak, ASTFormatLineBreakExt:
			if singleLine {
				if e = frame.push(frame.space, indent); e != nil {
					return e
				}
				break
			}
			pos = indent
			ribbonPos = 0
			line++
			if line < maxLines {
				if e = out.AppendCString("\n"); e != nil {
					return e
				}
				for i := uint32(0); i < indent; i++ {
					if e = out.AppendCString(" "); e != nil {
						return e
					}
				}
			} else {
				if e = out.AppendCString("...\n"); e != nil {
					return e
				}
			}
		}
	}
	_ = ribbonPos // actual source writes it but CHOICE reads pos for both limits.
	return nil
}

// Stamp the actual allocator child end at its reached Close, not by table size,
// root counts or a scan. Every recorded space is non-owning after this exact
// containing placement context ends; later owning metadata never DEC's it.
func (m *ASTManager) ppContextEnded() {
	for f := m.ppFrames; f != nil; f = f.next {
		f.astContextEnded = true
	}
}
