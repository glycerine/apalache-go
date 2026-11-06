package api

import "strconv"

type Z3LogSymbolKind uint8

const (
	Z3LogNullSymbol Z3LogSymbolKind = iota
	Z3LogNumericalSymbol
	Z3LogNamedSymbol
)

// Z3LogSymbol is the retained concrete representation of Z3_symbol's three
// forms. It avoids importing the C-pointer encoding used by the native API.
type Z3LogSymbol struct {
	Kind   Z3LogSymbolKind
	Number uint32
	Name   string
}

func NullZ3LogSymbol() Z3LogSymbol { return Z3LogSymbol{Kind: Z3LogNullSymbol} }

func NumericalZ3LogSymbol(number uint32) Z3LogSymbol {
	return Z3LogSymbol{Kind: Z3LogNumericalSymbol, Number: number}
}

func NamedZ3LogSymbol(name string) Z3LogSymbol {
	return Z3LogSymbol{Kind: Z3LogNamedSymbol, Name: name}
}

func (log *Z3LogWriter) Symbol(symbol Z3LogSymbol) error {
	switch symbol.Kind {
	case Z3LogNullSymbol:
		return log.writeRecord("N\n")
	case Z3LogNumericalSymbol:
		return log.writeRecord("# " + strconv.FormatUint(uint64(symbol.Number), 10) + "\n")
	case Z3LogNamedSymbol:
		return log.writeRecord("$ |" + z3LogEscape(symbol.Name) + "|\n")
	default:
		return ErrZ3LogSymbol
	}
}
