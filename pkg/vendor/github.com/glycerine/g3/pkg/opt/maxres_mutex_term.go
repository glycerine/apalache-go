package opt

// Source: src/opt/maxsmt.cpp weighted soft map consumed by MaxRes.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type maxResWeightedTerm struct {
	expression ast.ASTHandle
	weight     big.Rat
}
