package ast

// Source: src/ast/ast_smt_pp.cpp:24-124; src/ast/ast_smt_pp.h:24-37.

import (
	"strconv"
	"strings"

	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SMTRenaming is the legacy SMT benchmark printer's collision-free symbol
// translation. It borrows one symbol manager and returns interned symbols from
// that same manager.
type SMTRenaming struct {
	symbols *usymbol.Manager
	forward map[usymbol.Symbol]usymbol.Symbol
	reverse map[usymbol.Symbol]usymbol.Symbol
}

func NewSMTRenaming(symbols *usymbol.Manager) (*SMTRenaming, error) {
	if symbols == nil {
		return nil, ErrASTHandle
	}
	r := &SMTRenaming{symbols: symbols, forward: make(map[usymbol.Symbol]usymbol.Symbol), reverse: make(map[usymbol.Symbol]usymbol.Symbol)}
	for _, name := range []string{"=", ">=", "<=", "+", "-", "*", ">", "<", "!=", "or", "and", "implies", "not", "iff", "xor", "true", "false", "forall", "exists", "let", "flet"} {
		s, err := symbols.Intern(name)
		if err != nil {
			return nil, err
		}
		r.forward[s], r.reverse[s] = s, s
	}
	return r, nil
}

func legacySMTLegal(c byte) bool {
	return c == '.' || c == '_' || c == '\'' || c == '?' || c == '!' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func legacySMTSpecial(s string) bool {
	return len(s) >= 2 && s[0] == '|' && s[len(s)-1] == '|' && !strings.Contains(s[1:len(s)-1], "|")
}

func legacySMTNumerical(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func legacySMTAllLegal(s string) bool {
	if legacySMTNumerical(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !legacySMTLegal(s[i]) {
			return false
		}
	}
	return true
}

func (r *SMTRenaming) fixSymbol(s usymbol.Symbol, suffix int) (usymbol.Symbol, error) {
	if err := r.symbols.Validate(s); err != nil {
		return usymbol.Symbol{}, err
	}
	if s.IsNumerical() {
		text, err := r.symbols.Display(s)
		if err != nil {
			return usymbol.Symbol{}, err
		}
		return r.symbols.Intern(text + strconv.Itoa(suffix))
	}
	text, _, err := r.symbols.BareString(s)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	legalText := text
	if len(text) == 1 {
		switch text[0] {
		case '/':
			legalText = "op_div"
		case '%':
			legalText = "op_mod"
		}
	}
	if suffix == 0 && text != "" && (legacySMTSpecial(legalText) || legacySMTAllLegal(legalText)) {
		return s, nil
	}
	base := text
	quoted, err := usmt2.IsSMT2QuotedSymbol(r.symbols, s)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	if quoted {
		base, err = usmt2.MkSMT2QuotedSymbol(r.symbols, s, nil)
		if err != nil {
			return usymbol.Symbol{}, err
		}
	}
	if suffix > 0 {
		base += strconv.Itoa(suffix)
	}
	return r.symbols.Intern(base)
}

func (r *SMTRenaming) GetSymbol(source usymbol.Symbol) (usymbol.Symbol, error) {
	if r == nil || r.symbols == nil {
		return usymbol.Symbol{}, ErrASTHandle
	}
	if err := r.symbols.Validate(source); err != nil {
		return usymbol.Symbol{}, err
	}
	if translated, found := r.forward[source]; found {
		return translated, nil
	}
	for suffix := 0; ; suffix++ {
		translated, err := r.fixSymbol(source, suffix)
		if err != nil {
			return usymbol.Symbol{}, err
		}
		if _, exists := r.reverse[translated]; exists {
			continue
		}
		r.forward[source] = translated
		r.reverse[translated] = source
		return translated, nil
	}
}
