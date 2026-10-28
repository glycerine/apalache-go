package apalache

import (
	"fmt"
	"strings"
)

// SMTBoolFormula is the small Boolean formula algebra from lir.smt.SmtTools.
type SMTBoolFormula interface{ smtBoolFormula() }

type SMTFalse struct{}
type SMTAnd struct{ arguments []SMTBoolFormula }
type SMTOr struct{ arguments []SMTBoolFormula }
type SMTNeg struct{ Argument SMTBoolFormula }
type SMTImplies struct{ Left, Right SMTBoolFormula }
type SMTVariable struct{ ID int64 }
type SMTLtFunctions struct{ Left, Right int64 }
type SMTNeFunctions struct{ Left, Right int64 }

func (SMTFalse) smtBoolFormula()       {}
func (SMTAnd) smtBoolFormula()         {}
func (SMTOr) smtBoolFormula()          {}
func (SMTNeg) smtBoolFormula()         {}
func (SMTImplies) smtBoolFormula()     {}
func (SMTVariable) smtBoolFormula()    {}
func (SMTLtFunctions) smtBoolFormula() {}
func (SMTNeFunctions) smtBoolFormula() {}

func NewSMTAnd(arguments ...SMTBoolFormula) SMTAnd {
	return SMTAnd{arguments: append([]SMTBoolFormula(nil), arguments...)}
}
func (formula SMTAnd) Arguments() []SMTBoolFormula {
	return append([]SMTBoolFormula(nil), formula.arguments...)
}
func NewSMTOr(arguments ...SMTBoolFormula) SMTOr {
	return SMTOr{arguments: append([]SMTBoolFormula(nil), arguments...)}
}
func (formula SMTOr) Arguments() []SMTBoolFormula {
	return append([]SMTBoolFormula(nil), formula.arguments...)
}

func FormatSMTBoolFormula(formula SMTBoolFormula) string {
	return FormatSMTBoolFormulaSymbols(formula, "b", "R")
}

func FormatSMTBoolFormulaSymbols(formula SMTBoolFormula, variableSymbol, functionSymbol string) string {
	self := func(child SMTBoolFormula) string {
		return FormatSMTBoolFormulaSymbols(child, variableSymbol, functionSymbol)
	}
	switch typed := formula.(type) {
	case SMTFalse:
		return "false"
	case SMTAnd:
		arguments := typed.Arguments()
		text := make([]string, len(arguments))
		for index, argument := range arguments {
			text[index] = self(argument)
		}
		return "( and " + strings.Join(text, " ") + " )"
	case SMTOr:
		arguments := typed.Arguments()
		text := make([]string, len(arguments))
		for index, argument := range arguments {
			text[index] = self(argument)
		}
		return "( or " + strings.Join(text, " ") + " )"
	case SMTNeg:
		// Scala's implementation intentionally recurses through the default
		// symbols here rather than through self.
		return "( not " + FormatSMTBoolFormula(typed.Argument) + " )"
	case SMTImplies:
		return "( => " + FormatSMTBoolFormula(typed.Left) + " " + FormatSMTBoolFormula(typed.Right) + " )"
	case SMTVariable:
		return fmt.Sprintf("%s_%d", variableSymbol, typed.ID)
	case SMTLtFunctions:
		return fmt.Sprintf("( < ( %s %d ) ( %s %d ) )", functionSymbol, typed.Left, functionSymbol, typed.Right)
	case SMTNeFunctions:
		return fmt.Sprintf("( not ( = ( %s %d ) ( %s %d ) ) )", functionSymbol, typed.Left, functionSymbol, typed.Right)
	default:
		panic(fmt.Sprintf("unknown SMT Boolean formula %T", formula))
	}
}

func SimplifySMTBoolFormula(formula SMTBoolFormula) SMTBoolFormula {
	switch typed := formula.(type) {
	case SMTAnd:
		arguments := typed.Arguments()
		simplified := make([]SMTBoolFormula, len(arguments))
		for index, argument := range arguments {
			simplified[index] = SimplifySMTBoolFormula(argument)
			if _, isFalse := simplified[index].(SMTFalse); isFalse {
				return SMTFalse{}
			}
		}
		return NewSMTAnd(simplified...)
	case SMTOr:
		arguments := typed.Arguments()
		simplified := make([]SMTBoolFormula, 0, len(arguments))
		for _, argument := range arguments {
			child := SimplifySMTBoolFormula(argument)
			if _, isFalse := child.(SMTFalse); !isFalse {
				simplified = append(simplified, child)
			}
		}
		switch len(simplified) {
		case 0:
			return SMTFalse{}
		case 1:
			return simplified[0]
		default:
			return NewSMTOr(simplified...)
		}
	default:
		return formula
	}
}
