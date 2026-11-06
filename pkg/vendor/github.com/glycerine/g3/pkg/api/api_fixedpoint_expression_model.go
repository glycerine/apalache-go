package api

import (
	"fmt"

	"github.com/glycerine/g3/pkg/model"
)

// LastModel returns a caller-owned model for the latest supported relation or
// expression query, subject to the backend's ordinary model-status contract.
func (f *APIFixedpoint) LastModel() (*model.Model, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	if f.hasLastQuery {
		return f.command.FixedpointModel(f.lastQuery)
	}
	if f.hasLastExpression {
		return f.command.FixedpointExpressionModel()
	}
	return nil, fmt.Errorf("fixedpoint: model requires a completed relation or expression query")
}
