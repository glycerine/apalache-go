package ast

// Source: src/ast/ast.cpp:115-130,219-226,283-294. The extension-model
// debug trace prints attached func_decl_info after each model declaration.

import (
	"strconv"
	"strings"
)

// FuncDeclInfoText returns the source ostream representation of the attached
// declaration info. A plain declaration has no info and returns present=false.
func (m *ASTManager) FuncDeclInfoText(d ASTHandle) (body string, present bool, err error) {
	if err = m.lock(); err != nil {
		return "", false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(d, ASTFuncDecl)
	if err != nil {
		return "", false, err
	}
	token := m.declAt(r.slot).info
	if token == (ASTFuncDeclInfo{}) {
		return "", false, nil
	}
	info, err := m.funcInfo(token)
	if err != nil {
		return "", false, err
	}
	var out strings.Builder
	out.WriteString(":fid ")
	out.WriteString(strconv.FormatInt(int64(info.family), 10))
	out.WriteString(" :decl-kind ")
	out.WriteString(strconv.FormatInt(int64(info.kind), 10))
	out.WriteString(" :parameters (")
	for i := uint32(0); i < info.count; i++ {
		if i != 0 {
			out.WriteByte(' ')
		}
		p, e := m.infoParameter(info, i)
		if e != nil {
			return "", false, e
		}
		v, e := m.parameter(p)
		if e != nil {
			return "", false, e
		}
		switch v.Kind {
		case ASTParameterInt:
			out.WriteString(strconv.FormatInt(int64(v.integer), 10))
		case ASTParameterSymbol:
			spelling, e := m.Symbols.String(v.symbol)
			if e != nil {
				return "", false, e
			}
			out.WriteString(spelling)
		case ASTParameterRational:
			spelling, e := m.numerals.String(&v.Rational)
			if e != nil {
				return "", false, e
			}
			out.WriteString(spelling)
		case ASTParameterAST:
			if e := m.validNode(v.ast); e != nil {
				return "", false, e
			}
			out.WriteByte('#')
			out.WriteString(strconv.FormatUint(uint64(m.nodeAt(v.ast.slot).id), 10))
		case ASTParameterDouble:
			out.WriteString(strconv.FormatFloat(v.double, 'g', 6, 64))
		case ASTParameterExternal:
			out.WriteByte('@')
			out.WriteString(strconv.FormatUint(uint64(v.external), 10))
		default:
			return "", false, ErrASTDomain
		}
	}
	out.WriteByte(')')
	flags := info.flags
	for _, flag := range [...]struct {
		name    string
		enabled bool
	}{
		{"left-assoc", flags.LeftAssociative},
		{"right-assoc", flags.RightAssociative},
		{"flat-associative", flags.FlatAssociative},
		{"commutative", flags.Commutative},
		{"chainable", flags.Chainable},
		{"pairwise", flags.Pairwise},
		{"injective", flags.Injective},
		{"idempotent", flags.Idempotent},
		{"skolem", flags.Skolem},
	} {
		out.WriteString(" :")
		out.WriteString(flag.name)
		if flag.enabled {
			out.WriteString(" 1")
		} else {
			out.WriteString(" 0")
		}
	}
	return out.String(), true, nil
}
