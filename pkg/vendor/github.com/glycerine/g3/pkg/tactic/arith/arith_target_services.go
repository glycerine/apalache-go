package arith

// Source: process-service borrowing in FM and NLA2BV manager translation.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The source borrows process services across managers. Parameter and numeral
// owners are explicit in Go; only the process cooperation service is shared.
func arithmeticTargetServices(m *ast.ASTManager, source model.ModelEvaluatorConfig) (model.ModelEvaluatorConfig, error) {
	if m == nil {
		return model.ModelEvaluatorConfig{}, ast.ErrASTHandle
	}
	c, g, err := m.RewriterParameterServices()
	if err != nil {
		return model.ModelEvaluatorConfig{}, err
	}
	if c == nil {
		c, g = source.Context, source.Globals
	}
	if c == nil || g == nil || g.Context != c {
		return model.ModelEvaluatorConfig{}, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return model.ModelEvaluatorConfig{}, err
	}
	return model.ModelEvaluatorConfig{Context: c, Globals: g, Memory: m.NumeralService().RuntimeOwner(), Cooperation: source.Cooperation, Trace: m.BorrowSourceTraceTags()}, nil
}
