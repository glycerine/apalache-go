package ast

// Source: src/ast/ast_ll_pp.cpp:21-324; src/ast/for_each_ast.h:27-125;
// src/ast/for_each_ast.cpp:37-48. Native Go traversal collections.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// The lexical frame lives inline in the charged manager. Its native stack and
// identity marks borrow AST nodes; only the acquired numeral owns a value.
type astLowLevelFrame struct {
	stack                                []astNodeRef
	marks                                map[astNodeRef]struct{}
	visited                              *ASTMark
	invocation                           uint64
	Active, sourceDone                   bool
	bodyCause, destructorCause, ownCause error
	Numeral                              unumeral.OwnedRational
	NumeralAcquired                      bool
	numeralInsertion                     unumeral.NumeralInsertionReceipt
}
type astLowLevelPrinter struct {
	manager            *ASTManager
	out                *utext.NumericTextStream
	root               astNodeRef
	onlyExprs, compact bool
	arithFamily        FamilyID
	frame              *astLowLevelFrame
}

func (m *ASTManager) llFrameDestroy(owning bool) error {
	f := &m.LowLevel
	if !f.Active {
		return nil
	}
	if !owning && f.destructorCause != nil {
		return f.destructorCause
	}
	stop := func(e error) error {
		if e != nil {
			if owning {
				if f.ownCause == nil {
					f.ownCause = e
				}
			} else if f.destructorCause == nil {
				f.destructorCause = e
			}
		}
		return e
	}
	if owning && f.numeralInsertion.Active() {
		if e := m.numerals.CompleteNumeralInsertion(&f.numeralInsertion, false); e != nil {
			return stop(e)
		}
	}
	if f.NumeralAcquired {
		if e := m.numerals.Del(&f.Numeral); e != nil {
			return stop(e)
		}
		f.NumeralAcquired = false
	}
	f.stack = nil
	f.marks = nil
	f.visited = nil
	f.sourceDone = true
	if owning && f.numeralInsertion.Active() {
		if e := m.numerals.CompleteNumeralInsertion(&f.numeralInsertion, true); e != nil {
			return stop(e)
		}
	}
	if !f.numeralInsertion.Active() {
		f.Active = false
	}
	return nil
}
func (m *ASTManager) lowLevelPrint(out *utext.NumericTextStream, r astNodeRef, onlyExprs, compact bool) (result error) {
	return m.lowLevelPrintWithVisited(out, r, r, onlyExprs, compact, nil)
}

func (m *ASTManager) lowLevelPrintWithVisited(out *utext.NumericTextStream, r, root astNodeRef, onlyExprs, compact bool, visited *ASTMark) (result error) {
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
	symbol, e := m.Symbols.Intern("arith")
	if e != nil {
		f.Active = false
		return e
	}
	fid, e := m.families.MkID(symbol)
	if e != nil {
		f.Active = false
		return e
	}
	f.visited = visited
	if visited == nil {
		f.marks = make(map[astNodeRef]struct{})
	}
	p := astLowLevelPrinter{manager: m, out: out, root: root, onlyExprs: onlyExprs, compact: compact, arithFamily: fid, frame: f}
	defer func() {
		if result != nil && f.bodyCause == nil {
			f.bodyCause = result
		}
		result = errors.Join(result, m.llFrameDestroy(false))
	}()
	if e := m.validNode(r); e != nil {
		return e
	}
	if m.nodeAt(r.slot).kind == ASTSort {
		return p.sort(r)
	}
	f.stack = append(f.stack, r)
	for len(f.stack) != 0 {
		curr := f.stack[len(f.stack)-1]
		if e = m.validNode(curr); e != nil {
			return e
		}
		marked, err := p.marked(curr)
		if err != nil {
			return err
		}
		if marked {
			f.stack[len(f.stack)-1] = astNodeRef{}
			f.stack = f.stack[:len(f.stack)-1]
			continue
		}
		complete := true
		switch m.nodeAt(curr.slot).kind {
		case ASTSort, ASTFuncDecl:
			info, e := p.info(curr)
			if e != nil {
				return e
			}
			if info != nil {
				complete, e = p.parameters(info)
				if e != nil {
					return e
				}
			}
			if !complete {
				continue
			}
			if m.nodeAt(curr.slot).kind == ASTFuncDecl {
				d := m.declAt(curr.slot)
				complete, e = p.arguments(d.domain, d.arity)
				if e != nil {
					return e
				}
				if !complete {
					continue
				}
				yes, err := p.marked(d.rangeSort)
				if err != nil {
					return err
				}
				if !yes {
					f.stack = append(f.stack, d.rangeSort)
					continue
				}
			}
		case ASTApp:
			a := m.appAt(curr.slot)
			yes, err := p.marked(a.decl)
			if err != nil {
				return err
			}
			if !yes {
				f.stack = append(f.stack, a.decl)
				continue
			}
			complete, e = p.arguments(a.args, a.count)
			if e != nil {
				return e
			}
			if !complete {
				continue
			}
		case ASTQuantifier:
			q := m.quantifierAt(curr.slot)
			complete, e = p.arguments(q.patterns, q.patternCount)
			if e != nil {
				return e
			}
			if !complete {
				continue
			}
			complete, e = p.arguments(q.noPatterns, q.noPatternCount)
			if e != nil {
				return e
			}
			if !complete {
				continue
			}
			yes, err := p.marked(q.body)
			if err != nil {
				return err
			}
			if !yes {
				f.stack = append(f.stack, q.body)
				continue
			}
		case ASTVar:
		default:
			return ErrASTDomain
		}
		if e = p.process(curr); e != nil {
			return e
		}
		if visited != nil {
			if err := visited.state.mark(m, m.external(curr), true); err != nil {
				return err
			}
		} else {
			f.marks[curr] = struct{}{}
		}
		f.stack[len(f.stack)-1] = astNodeRef{}
		f.stack = f.stack[:len(f.stack)-1]
	}
	return nil
}

func (p *astLowLevelPrinter) marked(r astNodeRef) (bool, error) {
	if p.frame.visited != nil {
		return p.frame.visited.state.marked(p.manager, p.manager.external(r))
	}
	_, marked := p.frame.marks[r]
	return marked, nil
}

func (p *astLowLevelPrinter) arguments(raw arena.SmallBytesRef, n uint32) (bool, error) {
	all := true
	for i := uint32(0); i < n; i++ {
		r, e := p.manager.readInternal(raw, i)
		if e != nil {
			return false, e
		}
		if e = p.manager.validNode(r); e != nil {
			return false, e
		}
		yes, err := p.marked(r)
		if err != nil {
			return false, err
		}
		if !yes {
			p.frame.stack = append(p.frame.stack, r)
			all = false
		}
	}
	return all, nil
}
func (p *astLowLevelPrinter) info(r astNodeRef) (*astInfoRecord, error) {
	m := p.manager
	switch m.nodeAt(r.slot).kind {
	case ASTSort:
		t := m.sortAt(r.slot).info
		if t == (ASTSortInfo{}) {
			return nil, nil
		}
		return m.sortInfo(t)
	case ASTFuncDecl:
		t := m.declAt(r.slot).info
		if t == (ASTFuncDeclInfo{}) {
			return nil, nil
		}
		return m.funcInfo(t)
	}
	return nil, ErrASTDomain
}
func (p *astLowLevelPrinter) parameters(info *astInfoRecord) (bool, error) {
	all := true
	for i := uint32(0); i < info.count; i++ {
		token, e := p.manager.infoParameter(info, i)
		if e != nil {
			return false, e
		}
		v, e := p.manager.parameter(token)
		if e != nil {
			return false, e
		}
		if v.Kind == ASTParameterAST {
			r := v.ast
			if e = p.manager.validNode(r); e != nil {
				return false, e
			}
			yes, err := p.marked(r)
			if err != nil {
				return false, err
			}
			if !yes {
				p.frame.stack = append(p.frame.stack, r)
				all = false
			}
		}
	}
	return all, nil
}
func (p *astLowLevelPrinter) symbol(s usymbol.Symbol) error {
	return appendASTSymbol(p.out, p.manager, s)
}
func (p *astLowLevelPrinter) ref(r astNodeRef) error {
	if e := p.out.AppendCString("#"); e != nil {
		return e
	}
	if e := p.manager.validNode(r); e != nil {
		return e
	}
	return p.out.AppendUint32(p.manager.nodeAt(r.slot).id)
}
func (p *astLowLevelPrinter) header(r astNodeRef) error {
	if r != p.root {
		if e := p.ref(r); e != nil {
			return e
		}
		return p.out.AppendCString(" := ")
	}
	return nil
}
func (p *astLowLevelPrinter) name(r astNodeRef) error {
	d := p.manager.declAt(r.slot)
	flags, e := p.manager.funcFlags(r)
	if e != nil {
		return e
	}
	if flags.Skolem && d.name.IsNumerical() {
		if e = p.out.AppendCString("z3.sk."); e != nil {
			return e
		}
		return p.out.AppendUint32(d.name.SymbolWords()[2])
	}
	return p.symbol(d.name)
}
func (p *astLowLevelPrinter) sort(r astNodeRef) error {
	if e := p.symbol(p.manager.sortAt(r.slot).name); e != nil {
		return e
	}
	return p.displayParams(r)
}
func (p *astLowLevelPrinter) displayParams(r astNodeRef) error {
	info, e := p.info(r)
	if e != nil {
		return e
	}
	if info == nil || info.count == 0 || info.private {
		return nil
	}
	if e = p.out.AppendCString("["); e != nil {
		return e
	}
	for i := uint32(0); i < info.count; i++ {
		t, e := p.manager.infoParameter(info, i)
		if e != nil {
			return e
		}
		v, e := p.manager.parameter(t)
		if e != nil {
			return e
		}
		if v.Kind == ASTParameterAST {
			e = p.child(v.ast)
		} else {
			e = p.parameter(v)
		}
		if e != nil {
			return e
		}
		if i < info.count-1 {
			if e = p.out.AppendCString(":"); e != nil {
				return e
			}
		} else {
			if e = p.out.AppendCString(""); e != nil {
				return e
			}
		}
	}
	return p.out.AppendCString("]")
}
func (p *astLowLevelPrinter) parameter(v *astParameterRecord) error {
	switch v.Kind {
	case ASTParameterInt:
		return p.out.AppendInt32(v.integer)
	case ASTParameterAST:
		return p.ref(v.ast)
	case ASTParameterSymbol:
		return p.symbol(v.symbol)
	case ASTParameterRational:
		return p.manager.numerals.InsertInto(p.out, &v.Rational)
	case ASTParameterDouble:
		return p.out.AppendFloat64(v.double)
	case ASTParameterExternal:
		if e := p.out.AppendCString("@"); e != nil {
			return e
		}
		return p.out.AppendUint32(v.external)
	}
	return ErrASTDomain
}
func (p *astLowLevelPrinter) isNumeral(r astNodeRef) (bool, error) {
	if p.manager.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	d := p.manager.declAt(p.manager.appAt(r.slot).decl.slot)
	if d.info == (ASTFuncDeclInfo{}) {
		return false, nil
	}
	info, e := p.manager.funcInfo(d.info)
	return e == nil && info.family == p.arithFamily && info.kind == 0, e
}
func (p *astLowLevelPrinter) numeral(r astNodeRef) (yes bool, result error) {
	f := p.frame
	m := p.manager
	if f.NumeralAcquired {
		return false, ErrASTPending
	}
	v, e := m.numerals.NewValue()
	if e != nil {
		return false, e
	}
	f.Numeral = v
	f.NumeralAcquired = true
	defer func() {
		var younger *unumeral.NumeralSourceFailure
		if errors.As(result, &younger) {
			if f.destructorCause == nil {
				f.destructorCause = younger
			}
			return // younger source DT protects this still-owned older NUM
		}
		cleanup := m.numerals.Del(&f.Numeral)
		if cleanup == nil {
			f.NumeralAcquired = false
		} else if f.destructorCause == nil {
			f.destructorCause = cleanup
		}
		result = errors.Join(result, cleanup)
	}()
	yes, e = p.isNumeral(r)
	if e != nil || !yes {
		return yes, e
	}
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if e != nil {
		return true, e
	}
	token, e := m.infoParameter(info, 0)
	if e != nil {
		return true, e
	}
	rat, e := m.parameter(token)
	if e != nil {
		return true, e
	}
	if rat.Kind != ASTParameterRational {
		return true, ErrASTDomain
	}
	if e = m.numerals.CopyFrom(&f.Numeral, m.numerals, &rat.Rational); e != nil {
		return true, e
	}
	token, e = m.infoParameter(info, 1)
	if e != nil {
		return true, e
	}
	isInt, e := m.parameter(token)
	if e != nil {
		return true, e
	}
	if isInt.Kind != ASTParameterInt {
		return true, ErrASTDomain
	}
	if e = m.numerals.InsertIntoTracked(p.out, &f.Numeral, &f.numeralInsertion); e != nil {
		return true, e
	}
	if e = p.out.AppendCString("::"); e != nil {
		return true, e
	}
	if isInt.integer != 0 {
		return true, p.out.AppendCString("Int")
	}
	return true, p.out.AppendCString("Real")
}
func (p *astLowLevelPrinter) child(r astNodeRef) error {
	if e := p.manager.validNode(r); e != nil {
		return e
	}
	switch p.manager.nodeAt(r.slot).kind {
	case ASTSort:
		return p.sort(r)
	case ASTApp:
		yes, e := p.numeral(r)
		if e != nil {
			return e
		}
		if yes {
			return nil
		}
		a := p.manager.appAt(r.slot)
		if a.count == 0 {
			if e = p.name(a.decl); e != nil {
				return e
			}
			return p.displayParams(a.decl)
		}
	}
	return p.ref(r)
}
func (p *astLowLevelPrinter) children(raw arena.SmallBytesRef, n uint32) error {
	for i := uint32(0); i < n; i++ {
		if i > 0 {
			if e := p.out.AppendCString(" "); e != nil {
				return e
			}
		}
		r, e := p.manager.readInternal(raw, i)
		if e != nil {
			return e
		}
		if e = p.child(r); e != nil {
			return e
		}
	}
	return nil
}
func (p *astLowLevelPrinter) process(r astNodeRef) error {
	m := p.manager
	o := p.out
	switch m.nodeAt(r.slot).kind {
	case ASTSort:
		return nil
	case ASTFuncDecl:
		if p.onlyExprs {
			return nil
		}
		d := m.declAt(r.slot)
		info, e := p.info(r)
		if e != nil {
			return e
		}
		if info != nil && info.family != NullFamilyID {
			return nil
		}
		if e = o.AppendCString("decl "); e != nil {
			return e
		}
		if e = p.name(r); e != nil {
			return e
		}
		if e = o.AppendCString(" :: "); e != nil {
			return e
		}
		if d.arity == 0 {
			if e = p.child(d.rangeSort); e != nil {
				return e
			}
		} else {
			if e = o.AppendCString("(-> "); e != nil {
				return e
			}
			if e = p.children(d.domain, d.arity); e != nil {
				return e
			}
			if e = o.AppendCString(" "); e != nil {
				return e
			}
			if e = p.child(d.rangeSort); e != nil {
				return e
			}
			if e = o.AppendCString(")"); e != nil {
				return e
			}
			if e = p.displayParams(r); e != nil {
				return e
			}
			flags, e := m.funcFlags(r)
			if e != nil {
				return e
			}
			if flags.LeftAssociative && flags.RightAssociative {
				if e = o.AppendCString(" :assoc"); e != nil {
					return e
				}
			}
			if flags.Commutative {
				if e = o.AppendCString(" :comm"); e != nil {
					return e
				}
			}
			if flags.Injective {
				if e = o.AppendCString(" :inj"); e != nil {
					return e
				}
			}
		}
		return o.AppendCString("\n")
	case ASTVar:
		if e := p.header(r); e != nil {
			return e
		}
		if e := o.AppendCString("(:var "); e != nil {
			return e
		}
		v := m.varAt(r.slot)
		if e := o.AppendUint32(v.index); e != nil {
			return e
		}
		if e := o.AppendCString(" "); e != nil {
			return e
		}
		if e := p.sort(v.sort); e != nil {
			return e
		}
		return o.AppendCString(")\n")
	case ASTApp:
		numeric, e := p.isNumeral(r)
		if e != nil {
			return e
		}
		if numeric {
			if !p.compact {
				if e = p.header(r); e != nil {
					return e
				}
			}
			if r == p.root || !p.compact {
				if _, e = p.numeral(r); e != nil {
					return e
				}
				return o.AppendCString("\n")
			}
			return nil
		}
		proof, e := m.proofIs(m.external(r))
		if e != nil {
			return e
		}
		a := m.appAt(r.slot)
		if proof {
			if e = p.header(r); e != nil {
				return e
			}
			if e = o.AppendCString("["); e != nil {
				return e
			}
			if e = p.symbol(m.declAt(a.decl.slot).name); e != nil {
				return e
			}
			info, e := p.info(a.decl)
			if e != nil {
				return e
			}
			if info != nil {
				for i := uint32(0); i < info.count; i++ {
					if e = o.AppendCString(" "); e != nil {
						return e
					}
					token, e := m.infoParameter(info, i)
					if e != nil {
						return e
					}
					v, e := m.parameter(token)
					if e != nil {
						return e
					}
					if e = p.parameter(v); e != nil {
						return e
					}
				}
			}
			has, e := m.proofHasFact(m.external(r))
			if e != nil {
				return e
			}
			count := a.count
			if has {
				count--
			}
			for i := uint32(0); i < count; i++ {
				if e = o.AppendCString(" "); e != nil {
					return e
				}
				child, e := m.readInternal(a.args, i)
				if e != nil {
					return e
				}
				if e = p.child(child); e != nil {
					return e
				}
			}
			if e = o.AppendCString("]: "); e != nil {
				return e
			}
			if has {
				fact, e := m.proofFact(m.external(r))
				if e != nil {
					return e
				}
				if e = p.child(astNodeRef{fact.slot, fact.generation}); e != nil {
					return e
				}
			} else {
				if e = o.AppendCString("*"); e != nil {
					return e
				}
			}
			return o.AppendCString("\n")
		}
		if p.compact && a.count == 0 {
			if r == p.root {
				if e = p.child(r); e != nil {
					return e
				}
				return o.AppendCString("\n")
			}
			return nil
		}
		if e = p.header(r); e != nil {
			return e
		}
		if a.count > 0 {
			if e = o.AppendCString("("); e != nil {
				return e
			}
		}
		if e = p.name(a.decl); e != nil {
			return e
		}
		if e = p.displayParams(a.decl); e != nil {
			return e
		}
		if a.count > 0 {
			if e = o.AppendCString(" "); e != nil {
				return e
			}
			if e = p.children(a.args, a.count); e != nil {
				return e
			}
			if e = o.AppendCString(")"); e != nil {
				return e
			}
		}
		return o.AppendCString("\n")
	case ASTQuantifier:
		if e := p.header(r); e != nil {
			return e
		}
		if e := o.AppendCString("("); e != nil {
			return e
		}
		q := m.quantifierAt(r.slot)
		word := "exists"
		if q.forall {
			word = "forall"
		}
		if e := o.AppendCString(word); e != nil {
			return e
		}
		if e := o.AppendCString(" "); e != nil {
			return e
		}
		if e := o.AppendCString("(vars "); e != nil {
			return e
		}
		for i := uint32(0); i < q.count; i++ {
			if i > 0 {
				if e := o.AppendCString(" "); e != nil {
					return e
				}
			}
			if e := o.AppendCString("("); e != nil {
				return e
			}
			name, e := m.readSymbol(q.names, i)
			if e != nil {
				return e
			}
			if e = p.symbol(name); e != nil {
				return e
			}
			if e = o.AppendCString(" "); e != nil {
				return e
			}
			sort, e := m.readInternal(q.sorts, i)
			if e != nil {
				return e
			}
			if e = p.sort(sort); e != nil {
				return e
			}
			if e = o.AppendCString(")"); e != nil {
				return e
			}
		}
		if e := o.AppendCString(") "); e != nil {
			return e
		}
		if q.patternCount > 0 {
			if e := o.AppendCString("(:pat "); e != nil {
				return e
			}
			if e := p.children(q.patterns, q.patternCount); e != nil {
				return e
			}
			if e := o.AppendCString(") "); e != nil {
				return e
			}
		}
		if q.noPatternCount > 0 {
			if e := o.AppendCString("(:nopat "); e != nil {
				return e
			}
			if e := p.children(q.noPatterns, q.noPatternCount); e != nil {
				return e
			}
			if e := o.AppendCString(") "); e != nil {
				return e
			}
		}
		if e := p.child(q.body); e != nil {
			return e
		}
		return o.AppendCString(")\n")
	}
	return ErrASTDomain
}

func appendASTSymbol(out *utext.NumericTextStream, m *ASTManager, s usymbol.Symbol) error {
	if ASTSourceDebug && s.IsMarked() {
		return ErrASTDomain
	}
	if !s.IsNumerical() && !s.IsMarked() {
		if s.IsNull() {
			return out.AppendCString("null")
		}
		text, _, e := m.Symbols.BareString(s)
		if e != nil {
			return e
		}
		return out.AppendCString(text)
	}
	if e := out.AppendCString("k!"); e != nil {
		return e
	}
	words := s.SymbolWords()
	return out.AppendUint32(words[2])
}

// DisplayLowLevel is the concrete source ast_ll_pp call, borrowing the existing
// formatter; it neither changes its flags nor owns the stream.
func (m *ASTManager) DisplayLowLevel(out *utext.NumericTextStream, h ASTHandle, onlyExprs, compact bool) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	return m.lowLevelPrint(out, r, onlyExprs, compact)
}

// DisplayLowLevelExpressionDefinitions is the retained asserted-formula
// ast_def_ll_pp specialization: only expressions, noncompact definitions and
// one visited set shared across all roots. The source printer has no root to
// inline; callers retain the expressions while the native marks borrow them.
func (m *ASTManager) DisplayLowLevelExpressionDefinitions(out *utext.NumericTextStream, roots []ASTHandle) (result error) {
	visited, err := NewASTMark(m)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, visited.Close()) }()
	for _, h := range roots {
		if err := m.DisplayLowLevelDefinitionWithVisited(out, h, visited); err != nil {
			return err
		}
	}
	return nil
}

// DisplayLowLevelWithVisited is the retained proof trace overload: expression
// traversal with compact output and the caller's existing concrete AST mark.
func (m *ASTManager) DisplayLowLevelWithVisited(out *utext.NumericTextStream, h ASTHandle, visited *ASTMark) error {
	return m.displayLowLevelWithVisited(out, h, visited, false)
}

// DisplayLowLevelDefinitionWithVisited is the context/assertion definition
// overload, using expression-only, noncompact output and no inlined root.
func (m *ASTManager) DisplayLowLevelDefinitionWithVisited(out *utext.NumericTextStream, h ASTHandle, visited *ASTMark) error {
	return m.displayLowLevelWithVisited(out, h, visited, true)
}

func (m *ASTManager) displayLowLevelWithVisited(out *utext.NumericTextStream, h ASTHandle, visited *ASTMark, definition bool) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	if visited == nil || visited.state.owner.manager != m {
		return ErrASTHandle
	}
	if err := visited.state.owner.check(); err != nil {
		return err
	}
	r, err := m.internal(h)
	if err != nil {
		return err
	}
	root := r
	if definition {
		root = astNodeRef{}
	}
	return m.lowLevelPrintWithVisited(out, r, root, true, !definition, visited)
}
