package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type pbSum struct {
	terms   []ast.ASTHandle
	weights []big.Rat
	offset  big.Rat
}
