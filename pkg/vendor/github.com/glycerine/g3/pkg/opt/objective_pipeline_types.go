package opt

// Source: src/opt/opt_context.cpp from_fmls/internalize and optsmt result flow.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type ObjectivePipelineValue struct {
	SourceKind FormulaObjectiveKind
	Soft       bool
	Exact      big.Rat
	Arithmetic smtcore.SMTArithmeticObjectiveValue
	ModelValue smtcore.SMTArithmeticObjectiveValue
	ModelValid bool
	Interval   bool
	OpenBounds bool
}

type ObjectivePipelineStatistics struct {
	Arithmetic bool
	MaxSMT     []MaxSMTStatistics
	Objectives uint32
	Hard       uint32
}

type ObjectivePipelineResult struct {
	Status     lbool.LBool
	Optimal    bool
	Values     []ObjectivePipelineValue
	Model      *model.ModelCore
	Statistics ObjectivePipelineStatistics
}

func (r *ObjectivePipelineResult) Close() error {
	if r == nil || r.Model == nil {
		return nil
	}
	err := r.Model.DecRef()
	if err == nil {
		r.Model = nil
	}
	return err
}
