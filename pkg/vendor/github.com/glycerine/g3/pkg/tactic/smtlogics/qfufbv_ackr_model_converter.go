package smtlogics

// Source: src/tactic/smtlogics/qfufbv_ackr_model_converter.cpp/.h.

import (
	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func NewQFUFBVAckrModelConverter(m *ast.ASTManager, info *ackermannization.Info, abstract *model.ModelCore, services model.ModelEvaluatorConfig) (*ackermannization.ModelConverter, error) {
	return ackermannization.NewFixedModelConverter(m, info, abstract, services)
}
