package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

type arithmeticBranchSubstitution struct {
	coefficient *big.Rat
	term        ast.ASTHandle
}
