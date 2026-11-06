package opt

// Source: src/opt/maxsmt.h get_model(model_ref&, svector<symbol>&).

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (s *ArithmeticSolver) ResultLabels(result *ArithmeticResult) ([]usymbol.Symbol, error) {
	if s == nil || result == nil || result.Model == nil {
		return nil, ast.ErrASTHandle
	}
	return cloneLabels(result.Labels), nil
}
