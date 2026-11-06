package opt

// Source: src/opt/maxres.cpp get_current_correction_set.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (e *maxResEngine) correctionSet(md *model.ModelCore) ([]ast.ASTHandle, error) {
	correction := make([]ast.ASTHandle, 0)
	for _, assumption := range e.active {
		truth, err := e.modelBoolean(md, assumption)
		if err != nil {
			return nil, err
		}
		if !truth {
			correction = append(correction, assumption)
		}
	}
	return correction, nil
}
