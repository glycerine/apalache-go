package opt

// Source: src/opt/opt_context.cpp to_fmls/from_fmls objective markers.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type FormulaObjectiveKind uint8

const (
	FormulaMaximize FormulaObjectiveKind = iota + 1
	FormulaMinimize
	FormulaMaxSAT
)

type FormulaObjective struct {
	Kind      FormulaObjectiveKind
	Term      ast.ASTHandle
	SoftTerms []ast.ASTHandle
	Weights   []*big.Rat
	ID        string
}

type DecodedFormulaObjective struct {
	Kind         FormulaObjectiveKind
	Term         ast.ASTHandle
	OriginalTerm ast.ASTHandle
	SoftTerms    []ast.ASTHandle
	Weights      []big.Rat
	Offset       big.Rat
	Negate       bool
	ID           string
}

type DecodedFormulaSet struct {
	Hard       []ast.ASTHandle
	Objectives []DecodedFormulaObjective
}
