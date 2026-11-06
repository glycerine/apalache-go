package base

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"io"
)

type EngineGroundAnswer interface{ GroundSATAnswer() (ast.ASTRef, error) }
type EngineLevelQuery interface {
	QueryFromLevel(ast.ASTHandle, uint32) (lbool.LBool, error)
}
type EngineLevelCounter interface {
	NumLevels(ast.ASTHandle) (uint32, error)
}
type EngineReachability interface {
	Reachable(ast.ASTHandle) (ast.ASTRef, error)
}
type EngineCover interface {
	CoverDelta(int32, ast.ASTHandle) (ast.ASTRef, error)
	AddCover(int32, ast.ASTHandle, ast.ASTHandle) error
}
type EngineInvariant interface {
	AddInvariant(ast.ASTHandle, ast.ASTHandle) error
}
type EngineModelProvider interface{ Model() (*model.Model, error) }
type EngineProofProvider interface{ Proof() (ast.ASTRef, error) }
type EngineTraceProvider interface{ RulesAlongTrace() (*EngineTrace, error) }
type EngineCertificate interface{ DisplayCertificate(io.Writer) error }
type EngineStatistics interface {
	ResetStatistics() error
	CollectStatistics(*statistics.Statistics) error
	DisplayProfile(io.Writer) error
}
type EngineParameterUpdater interface{ UpdateParameters() error }
type EngineCanceler interface{ Cancel() error }
type EngineCleaner interface{ Cleanup() error }
