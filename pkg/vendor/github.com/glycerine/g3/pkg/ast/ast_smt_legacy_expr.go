package ast

// Source: src/ast/ast_smt_pp.cpp:176-206,292-417,564-607.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (p *LegacySMTPrinter) bitVectorNumeralText(h ASTHandle) (text string, matched bool, result error) {
	value, err := p.manager.numerals.NewValue()
	if err != nil {
		return "", false, err
	}
	defer func() { result = errors.Join(result, p.manager.numerals.Del(&value)) }()
	var width uint32
	matched, result = p.manager.BitVectorNumeralValue(h, &value, &width)
	if result != nil || !matched {
		return "", matched, result
	}
	valueText, err := p.manager.numerals.String(&value)
	if err != nil {
		return "", false, err
	}
	return "(_ bv" + valueText + " " + uintText(width) + ")", true, nil
}

func legacySMTRational(text string, integerSort bool) string {
	negative := strings.HasPrefix(text, "-")
	if negative {
		text = text[1:]
	}
	formatted := text
	if slash := strings.IndexByte(text, '/'); slash >= 0 {
		numerator, denominator := text[:slash], text[slash+1:]
		if !integerSort {
			numerator += ".0"
			denominator += ".0"
		}
		formatted = "(/ " + numerator + " " + denominator + ")"
	} else if !integerSort {
		formatted += ".0"
	}
	if negative {
		return "(- " + formatted + ")"
	}
	return formatted
}

type legacySMTParameterView struct {
	kind     ASTParameterKind
	integer  int32
	ast      ASTHandle
	astKind  ASTKind
	astID    uint32
	symbol   usymbol.Symbol
	rational string
	double   float64
	external uint32
}

func (m *ASTManager) legacySMTDeclParameters(decl ASTHandle) (private bool, views []legacySMTParameterView, err error) {
	if err = m.lock(); err != nil {
		return false, nil, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return false, nil, err
	}
	infoToken := m.declAt(r.slot).info
	if infoToken == (ASTFuncDeclInfo{}) {
		return false, nil, nil
	}
	info, err := m.funcInfo(infoToken)
	if err != nil {
		return false, nil, err
	}
	views = make([]legacySMTParameterView, 0, info.count)
	for i := uint32(0); i < info.count; i++ {
		parameter, e := m.infoParameter(info, i)
		if e != nil {
			return false, nil, e
		}
		raw, e := m.parameter(parameter)
		if e != nil {
			return false, nil, e
		}
		view := legacySMTParameterView{kind: raw.Kind, integer: raw.integer, symbol: raw.symbol, double: raw.double, external: raw.external}
		switch raw.Kind {
		case ASTParameterAST:
			if e = m.validNode(raw.ast); e != nil {
				return false, nil, e
			}
			view.ast = m.external(raw.ast)
			view.astKind = m.nodeAt(raw.ast.slot).kind
			view.astID = m.nodeAt(raw.ast.slot).id
		case ASTParameterRational:
			view.rational, e = m.numerals.String(&raw.Rational)
			if e != nil {
				return false, nil, e
			}
		case ASTParameterInt, ASTParameterSymbol, ASTParameterDouble, ASTParameterExternal:
		default:
			return false, nil, ErrASTDomain
		}
		views = append(views, view)
	}
	return info.private, views, nil
}

func (m *ASTManager) legacySMTBit2BoolIndex(decl ASTHandle) (index uint32, matched bool, err error) {
	if err = m.lock(); err != nil {
		return 0, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return 0, false, err
	}
	if m.bitVectorPlugin == nil {
		return 0, false, nil
	}
	family, kind, err := m.declIdentity(r)
	if err != nil || family != m.bitVectorPlugin.family || kind != 55 {
		return 0, false, err
	}
	info, err := m.funcInfo(m.declAt(r.slot).info)
	if err != nil || info.count != 1 {
		return 0, false, err
	}
	parameter, err := m.infoParameter(info, 0)
	if err != nil {
		return 0, false, err
	}
	raw, err := m.parameter(parameter)
	if err != nil {
		return 0, false, err
	}
	if raw.Kind != ASTParameterInt || raw.integer < 0 {
		return 0, false, ErrASTDomain
	}
	return uint32(raw.integer), true, nil
}

func (m *ASTManager) legacySMTIsFPANumeral(decl ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return false, err
	}
	if m.floatingPointPlugin == nil {
		return false, nil
	}
	family, kind, err := m.declIdentity(r)
	if err != nil {
		return false, err
	}
	return family == m.floatingPointPlugin.family && kind >= astFPANum && kind <= astFPAMinusZero, nil
}

func (p *LegacySMTPrinter) parameterText(parameter legacySMTParameterView, ctx *legacySMTExprContext) (string, error) {
	switch parameter.kind {
	case ASTParameterInt:
		return strconv.FormatInt(int64(parameter.integer), 10), nil
	case ASTParameterSymbol:
		return p.manager.Symbols.Display(parameter.symbol)
	case ASTParameterRational:
		return parameter.rational, nil
	case ASTParameterDouble:
		return strconv.FormatFloat(parameter.double, 'g', 6, 64), nil
	case ASTParameterExternal:
		return "@" + uintText(parameter.external), nil
	case ASTParameterAST:
		switch parameter.astKind {
		case ASTSort:
			return p.sortText(parameter.ast)
		case ASTApp, ASTVar, ASTQuantifier:
			return p.exprTextContext(parameter.ast, ctx)
		case ASTFuncDecl:
			text, err := p.declarationText(parameter.ast, ctx, true)
			return text + " ", err
		default:
			return "#" + uintText(parameter.astID), nil
		}
	}
	return "", ErrASTDomain
}

func (p *LegacySMTPrinter) declarationText(decl ASTHandle, ctx *legacySMTExprContext, includePrivate bool) (string, error) {
	family, err := p.manager.GetFamilyID(decl)
	if err != nil {
		return "", err
	}
	op, err := p.manager.GetDeclKind(decl)
	if err != nil {
		return "", err
	}
	if family == p.manager.BasicFamilyID() {
		switch op {
		case ASTOpIte:
			return "ite", nil
		case ASTOpIff:
			return "=", nil
		case ASTOpImplies:
			return "=>", nil
		}
	}
	if family == p.manager.arithmeticPluginFamily() && op == 8 {
		return "-", nil
	}
	name, err := p.manager.GetASTName(decl)
	if err != nil {
		return "", err
	}
	nameText, err := p.renamedText(name)
	if err != nil {
		return "", err
	}
	private, parameters, err := p.manager.legacySMTDeclParameters(decl)
	if err != nil {
		return "", err
	}
	if p.manager.datatypePlugin != nil && family == p.manager.datatypePlugin.family {
		if op != ASTDatatypeRecognizer && op != ASTDatatypeIs {
			return nameText, nil
		}
		private = false
	}
	if len(parameters) == 0 || private && !includePrivate {
		return nameText, nil
	}
	var out strings.Builder
	if len(parameters) == 1 && parameters[0].kind == ASTParameterAST && parameters[0].astKind == ASTSort {
		out.WriteString("(as ")
	} else {
		out.WriteString("(_ ")
	}
	out.WriteString(nameText)
	out.WriteByte(' ')
	for i, parameter := range parameters {
		if i != 0 {
			out.WriteByte(' ')
		}
		text, e := p.parameterText(parameter, ctx)
		if e != nil {
			return "", e
		}
		out.WriteString(text)
	}
	out.WriteByte(')')
	return out.String(), nil
}

func (m *ASTManager) arithmeticPluginFamily() FamilyID {
	if m.arithmeticPlugin == nil {
		return NullFamilyID
	}
	return m.arithmeticPlugin.family
}

type legacySMTExprContext struct {
	freeNames   []string
	quantifiers [][]usymbol.Symbol
	indent      uint32
	aliases     map[ASTHandle]bool
	defining    ASTHandle
}

func (p *LegacySMTPrinter) exprText(h ASTHandle, variableNames []string) (string, error) {
	return p.exprTextIndented(h, variableNames, 0)
}

func (p *LegacySMTPrinter) exprTextIndented(h ASTHandle, variableNames []string, indent uint32) (string, error) {
	ctx := legacySMTExprContext{freeNames: variableNames, indent: indent}
	return p.exprTextWithLetsContext(h, &ctx)
}

func (p *LegacySMTPrinter) variableText(index uint32, ctx *legacySMTExprContext) (string, error) {
	remaining := index
	for i := len(ctx.quantifiers); i > 0; i-- {
		names := ctx.quantifiers[i-1]
		if uint64(remaining) < uint64(len(names)) {
			return p.renamedText(names[len(names)-int(remaining)-1])
		}
		remaining -= uint32(len(names))
	}
	if int(remaining) < len(ctx.freeNames) {
		return ctx.freeNames[len(ctx.freeNames)-int(remaining)-1], nil
	}
	return "?" + uintText(remaining), nil
}

func (p *LegacySMTPrinter) isLegacySkolemHackPattern(pattern ASTHandle) (bool, error) {
	n, err := p.manager.GetNumArgs(pattern)
	if err != nil || n != 1 {
		return false, err
	}
	arg, err := p.manager.GetArg(pattern, 0)
	if err != nil {
		return false, err
	}
	kind, err := p.manager.GetKind(arg)
	if err != nil || kind != ASTApp {
		return false, err
	}
	n, err = p.manager.GetNumArgs(arg)
	if err != nil || n != 1 {
		return false, err
	}
	decl, err := p.manager.GetDecl(arg)
	if err != nil {
		return false, err
	}
	name, err := p.manager.GetASTName(decl)
	if err != nil || name.IsNumerical() || name.IsNull() {
		return false, err
	}
	text, present, err := p.manager.Symbols.BareString(name)
	return present && text == "sk_hack", err
}

func (p *LegacySMTPrinter) distinctText(h ASTHandle, ctx *legacySMTExprContext) (string, error) {
	n, err := p.manager.GetNumArgs(h)
	if err != nil {
		return "", err
	}
	args := make([]ASTHandle, n)
	sorts := make([]ASTHandle, n)
	remaining := make([]bool, n)
	for i := uint32(0); i < n; i++ {
		args[i], err = p.manager.GetArg(h, i)
		if err != nil {
			return "", err
		}
		sorts[i], err = p.manager.GetSort(args[i])
		if err != nil {
			return "", err
		}
		remaining[i] = true
	}
	var out strings.Builder
	out.WriteString("(and")
	for i := 0; i < len(args); i++ {
		if !remaining[i] {
			continue
		}
		next := i + 1
		for next < len(args) && (!remaining[next] || sorts[next] != sorts[i]) {
			next++
		}
		if next == len(args) {
			remaining[i] = false
			continue
		}
		out.WriteString(" (distinct")
		for j := i; j < len(args); j++ {
			if remaining[j] && sorts[j] == sorts[i] {
				text, e := p.exprTextContext(args[j], ctx)
				if e != nil {
					return "", e
				}
				out.WriteByte(' ')
				out.WriteString(text)
				remaining[j] = false
			}
		}
		out.WriteByte(')')
	}
	out.WriteString(" true)")
	return out.String(), nil
}

func (p *LegacySMTPrinter) isImplies(h ASTHandle) (bool, error) {
	kind, err := p.manager.GetKind(h)
	if err != nil || kind != ASTApp {
		return false, err
	}
	decl, err := p.manager.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := p.manager.GetFamilyID(decl)
	if err != nil || family != p.manager.BasicFamilyID() {
		return false, err
	}
	op, err := p.manager.GetDeclKind(decl)
	return op == ASTOpImplies, err
}

func (p *LegacySMTPrinter) simplifiedImpliesText(h ASTHandle, ctx *legacySMTExprContext) (string, error) {
	var antecedents []string
	current := h
	for {
		implies, err := p.isImplies(current)
		if err != nil {
			return "", err
		}
		if !implies {
			break
		}
		antecedent, err := p.manager.GetArg(current, 0)
		if err != nil {
			return "", err
		}
		text, err := p.exprTextContext(antecedent, ctx)
		if err != nil {
			return "", err
		}
		antecedents = append(antecedents, text)
		current, err = p.manager.GetArg(current, 1)
		if err != nil {
			return "", err
		}
	}
	consequent, err := p.exprTextContext(current, ctx)
	if err != nil {
		return "", err
	}
	return "(=> (and " + strings.Join(antecedents, " ") + ") " + consequent + ")", nil
}

func (p *LegacySMTPrinter) quantifierText(q ASTHandle, ctx *legacySMTExprContext) (string, error) {
	forall, err := p.manager.IsForall(q)
	if err != nil {
		return "", err
	}
	n, err := p.manager.GetNumDeclarations(q)
	if err != nil {
		return "", err
	}
	names := make([]usymbol.Symbol, n)
	var out strings.Builder
	out.WriteByte('(')
	if forall {
		out.WriteString("forall ")
	} else {
		out.WriteString("exists ")
	}
	out.WriteByte('(')
	for i := uint32(0); i < n; i++ {
		name, e := p.manager.GetDeclarationName(q, i)
		if e != nil {
			return "", e
		}
		names[i] = name
		nameText, e := p.renamedText(name)
		if e != nil {
			return "", e
		}
		sort, e := p.manager.GetDeclarationSort(q, i)
		if e != nil {
			return "", e
		}
		sortText, e := p.sortText(sort)
		if e != nil {
			return "", e
		}
		out.WriteByte('(')
		out.WriteString(nameText)
		out.WriteByte(' ')
		out.WriteString(sortText)
		out.WriteString(") ")
	}
	out.WriteByte(')')

	patternCount, err := p.manager.GetNumPatterns(q)
	if err != nil {
		return "", err
	}
	qid, _, err := p.manager.GetQuantifierIDs(q)
	if err != nil {
		return "", err
	}
	annotated := patternCount > 0 || !qid.IsNull()
	if annotated {
		out.WriteString("(! ")
	}
	ctx.quantifiers = append(ctx.quantifiers, names)
	defer func() { ctx.quantifiers = ctx.quantifiers[:len(ctx.quantifiers)-1] }()
	body, err := p.manager.GetQuantifierBody(q)
	if err != nil {
		return "", err
	}
	bodyContext := *ctx
	bodyContext.aliases = nil
	bodyContext.defining = ASTHandle{}
	bodyText, err := p.exprTextWithLetsContext(body, &bodyContext)
	if err != nil {
		return "", err
	}
	out.WriteString(bodyText)
	for i := uint32(0); i < patternCount; i++ {
		pattern, e := p.manager.GetPattern(q, i)
		if e != nil {
			return "", e
		}
		skip, e := p.isLegacySkolemHackPattern(pattern)
		if e != nil {
			return "", e
		}
		if skip {
			continue
		}
		args, e := p.manager.GetNumArgs(pattern)
		if e != nil {
			return "", e
		}
		out.WriteString(" :pattern ( ")
		for j := uint32(0); j < args; j++ {
			arg, e := p.manager.GetArg(pattern, j)
			if e != nil {
				return "", e
			}
			patternContext := *ctx
			patternContext.aliases = nil
			patternContext.defining = ASTHandle{}
			argText, e := p.exprTextContext(arg, &patternContext)
			if e != nil {
				return "", e
			}
			out.WriteString(argText)
			out.WriteByte(' ')
		}
		out.WriteByte(')')
	}
	if !qid.IsNull() {
		qidText, e := p.manager.Symbols.Display(qid)
		if e != nil {
			return "", e
		}
		out.WriteString(" :qid ")
		out.WriteString(qidText)
	}
	if annotated {
		out.WriteByte(')')
	}
	out.WriteString(")\n")
	if ctx.indent != 0 {
		out.WriteString(strings.Repeat(" ", int(ctx.indent)))
	}
	return out.String(), nil
}

func (p *LegacySMTPrinter) exprTextContext(h ASTHandle, ctx *legacySMTExprContext) (string, error) {
	if ctx.aliases != nil && ctx.defining != h && ctx.aliases[h] {
		return p.letIDText(h)
	}
	kind, err := p.manager.GetKind(h)
	if err != nil {
		return "", err
	}
	switch kind {
	case ASTVar:
		index, e := p.manager.GetVarIndex(h)
		if e != nil {
			return "", e
		}
		return p.variableText(index, ctx)
	case ASTQuantifier:
		return p.quantifierText(h, ctx)
	case ASTApp:
	default:
		return "", ErrASTDomain
	}
	if value, integer, matched, e := p.manager.ArithmeticNumeralText(h); e != nil {
		return "", e
	} else if matched {
		return legacySMTRational(value, integer), nil
	}
	if value, matched, e := p.manager.SequenceStringValue(h); e != nil {
		return "", e
	} else if matched {
		return "\"" + strings.ReplaceAll(encodeSequenceString(value), "\"", "\"\"") + "\"", nil
	}
	if value, matched, e := p.bitVectorNumeralText(h); e != nil {
		return "", e
	} else if matched {
		return value, nil
	}
	decl, err := p.manager.GetDecl(h)
	if err != nil {
		return "", err
	}
	if matched, e := p.manager.legacySMTIsFPANumeral(decl); e != nil {
		return "", e
	} else if matched {
		value, found, e := p.manager.FPANumeralString(h)
		if e != nil {
			return "", e
		}
		if !found {
			return "", ErrASTDomain
		}
		sort, e := p.manager.GetSort(h)
		if e != nil {
			return "", e
		}
		view, e := p.manager.smt2SortView(sort)
		if e != nil {
			return "", e
		}
		if !view.floatSort || len(view.params) != 2 || view.params[0].kind != ASTParameterInt || view.params[1].kind != ASTParameterInt {
			return "", ErrASTDomain
		}
		return "((_ to_fp " + uintText(view.params[0].ival) + " " + uintText(view.params[1].ival) + ") RTZ " + value + ")", nil
	}
	if bit, matched, e := p.manager.legacySMTBit2BoolIndex(decl); e != nil {
		return "", e
	} else if matched {
		arg, e := p.manager.GetArg(h, 0)
		if e != nil {
			return "", e
		}
		argText, e := p.exprTextContext(arg, ctx)
		if e != nil {
			return "", e
		}
		bitText := uintText(bit)
		return "(= ((_ extract " + bitText + " " + bitText + ") " + argText + ") (_ bv1 1))", nil
	}
	var labels []usymbol.Symbol
	positive := false
	if matched, e := p.manager.IsLabel(h, &positive, &labels); e != nil {
		return "", e
	} else if matched && len(labels) > 0 {
		body, e := p.manager.GetArg(h, 0)
		if e != nil {
			return "", e
		}
		bodyText, e := p.exprTextContext(body, ctx)
		if e != nil {
			return "", e
		}
		label, e := p.renamedText(labels[0])
		if e != nil {
			return "", e
		}
		attribute := ":lblneg"
		if positive {
			attribute = ":lblpos"
		}
		return "(! " + bodyText + attribute + " " + label + ")", nil
	}
	labels = labels[:0]
	if matched, e := p.manager.IsLabelLit(h, &labels); e != nil {
		return "", e
	} else if matched && len(labels) > 0 {
		label, e := p.renamedText(labels[0])
		if e != nil {
			return "", e
		}
		return "(! true :lblpos " + label + ")", nil
	}
	family, err := p.manager.GetFamilyID(decl)
	if err != nil {
		return "", err
	}
	op, err := p.manager.GetDeclKind(decl)
	if err != nil {
		return "", err
	}
	if family == p.manager.BasicFamilyID() && op == ASTOpDistinct {
		return p.distinctText(h, ctx)
	}
	if p.simplifyImplies && family == p.manager.BasicFamilyID() && op == ASTOpImplies {
		right, e := p.manager.GetArg(h, 1)
		if e != nil {
			return "", e
		}
		nested, e := p.isImplies(right)
		if e != nil {
			return "", e
		}
		if nested {
			return p.simplifiedImpliesText(h, ctx)
		}
	}
	n, err := p.manager.GetNumArgs(h)
	if err != nil {
		return "", err
	}
	name, err := p.declarationText(decl, ctx, n != 0)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return name, nil
	}
	var out strings.Builder
	out.WriteByte('(')
	out.WriteString(name)
	for i := uint32(0); i < n; i++ {
		arg, e := p.manager.GetArg(h, i)
		if e != nil {
			return "", e
		}
		text, e := p.exprTextContext(arg, ctx)
		if e != nil {
			return "", e
		}
		out.WriteByte(' ')
		out.WriteString(text)
	}
	out.WriteByte(')')
	return out.String(), nil
}

func uintText(value uint32) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [10]byte
	i := len(buffer)
	for value != 0 {
		i--
		buffer[i] = digits[value%10]
		value /= 10
	}
	return string(buffer[i:])
}

func (p *LegacySMTPrinter) DisplayExpr(out io.Writer, expr ASTHandle, variableNames []string) error {
	if p == nil || p.manager == nil || p.renaming == nil || out == nil {
		return ErrASTHandle
	}
	text, err := p.exprText(expr, variableNames)
	if err != nil {
		return err
	}
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}
