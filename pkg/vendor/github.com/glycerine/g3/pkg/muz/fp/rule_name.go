package fp

import "github.com/glycerine/g3/pkg/util/symbol"

func (e *PropositionalEngine) ruleNameSymbol(text string) (symbol.Symbol, error) {
	if err := e.open(); err != nil {
		return symbol.Symbol{}, err
	}
	if text == "" {
		return symbol.Symbol{}, nil
	}
	return e.manager.Symbols.Intern(text)
}

// String enumeration is a convenience view only. Source lookup, storage and
// query copies always carry symbols, including absent and present-empty names.
func (e *PropositionalEngine) ruleNameStrings(labels []symbol.Symbol) ([]string, error) {
	names := make([]string, len(labels))
	for i, label := range labels {
		if label == (symbol.Symbol{}) {
			continue
		}
		var err error
		names[i], err = e.manager.Symbols.String(label)
		if err != nil {
			return nil, err
		}
	}
	return names, nil
}
