package base

import "github.com/glycerine/g3/pkg/util/symbol"

// ruleName preserves the existing string convenience convention: an empty
// string means no label. Source consumers with a present empty or numeric
// symbol use the concrete WithSymbol entry points instead.
func (m *RuleManager) ruleName(text string) (symbol.Symbol, error) {
	if err := m.open(); err != nil {
		return symbol.Symbol{}, err
	}
	if text == "" {
		return symbol.Symbol{}, nil
	}
	return m.manager.Symbols.Intern(text)
}

func (m *RuleManager) checkRuleName(name symbol.Symbol) error {
	if err := m.open(); err != nil {
		return err
	}
	_, err := m.manager.Symbols.String(name)
	return err
}

// NameSymbol returns the source label identity. Null, an interned empty
// string, and numerical symbols are distinct even if their text coincides.
func (r *Rule) NameSymbol() (symbol.Symbol, error) {
	state, err := r.open()
	if err != nil {
		return symbol.Symbol{}, err
	}
	return state.name, nil
}

// NameText implements source symbol::str, including "<null>" for an absent
// label and "k!N" for a numerical label. It is a rendering, not an identity.
func (r *Rule) NameText() (string, error) {
	state, err := r.open()
	if err != nil {
		return "", err
	}
	return state.manager.Symbols.String(state.name)
}
