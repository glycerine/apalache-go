package opt

// Source: src/opt/maxres.cpp minimize_core using solver::mus from src/solver/mus.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *maxResEngine) minimizeCore(core []ast.ASTHandle) ([]ast.ASTHandle, lbool.LBool, error) {
	if e.config.satCoreEnabled || len(core) <= 1 {
		return core, lbool.True, nil
	}
	if e.mus == nil {
		return nil, lbool.Undef, errMaxResState
	}
	if err := e.mus.Reset(); err != nil {
		return nil, lbool.Undef, err
	}
	if err := e.mus.AddSoftAll(core); err != nil {
		return nil, lbool.Undef, err
	}
	status, minimized, err := e.mus.GetMUS()
	e.stats.CoreMinChecks += e.mus.CheckCount()
	if err != nil {
		return nil, lbool.Undef, err
	}
	return minimized, status, nil
}
