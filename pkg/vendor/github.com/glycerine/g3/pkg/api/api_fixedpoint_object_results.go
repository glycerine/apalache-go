package api

// Source: src/api/api_datalog.cpp:314-349, 405-440.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// NumLevels reports the number of PDR or Spacer frames for a relation.
func (f *APIFixedpoint) NumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.command.FixedpointNumLevels(declaration)
}

// CoverDelta returns an owning reference to a PDR or Spacer frame.
func (f *APIFixedpoint) CoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointCoverDelta(level, declaration)
}

// AddCover adds a property to a PDR or Spacer frame.
func (f *APIFixedpoint) AddCover(level int, declaration, property ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointCover(level, declaration, property)
}

// AddInvariant adds a property to the invariant frame.
func (f *APIFixedpoint) AddInvariant(declaration, property ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointInvariant(declaration, property)
}

// Reachable returns an owning reference to the Spacer reach summary.
func (f *APIFixedpoint) Reachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointReachable(declaration)
}

// RulesAlongTrace returns owned rule formulas and copied names for the latest trace.
func (f *APIFixedpoint) RulesAlongTrace() ([]ast.ASTRef, []string, error) {
	if err := f.open(); err != nil {
		return nil, nil, err
	}
	return f.command.FixedpointRulesAlongTrace()
}

// RulesAlongTraceWithSymbols returns caller-owned trace formulas and the exact
// source rule-name symbols, preserving absent, empty and numerical names.
func (f *APIFixedpoint) RulesAlongTraceWithSymbols() ([]ast.ASTRef, []symbol.Symbol, error) {
	if err := f.open(); err != nil {
		return nil, nil, err
	}
	return f.command.FixedpointRulesAlongTraceWithSymbols()
}

func (f *APIFixedpoint) ReasonUnknown() (string, error) {
	if err := f.open(); err != nil {
		return "", err
	}
	return f.command.FixedpointReasonUnknown()
}

func (f *APIFixedpoint) Answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointAnswer(declaration)
}

func (f *APIFixedpoint) Model(declaration ast.ASTHandle) (*model.Model, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return f.command.FixedpointModel(declaration)
}

func (f *APIFixedpoint) EvaluateModel(declaration, expression ast.ASTHandle, completion bool) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.EvaluateFixedpointModel(declaration, expression, completion)
}

// LastAnswer returns an owning formula for the latest supported query.
func (f *APIFixedpoint) LastAnswer() (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if f.hasLastQuery {
		return f.command.FixedpointAnswer(f.lastQuery)
	}
	if f.hasLastExpression {
		return f.command.FixedpointExpressionAnswer()
	}
	if f.hasLastRelations {
		return f.command.FixedpointRelationsAnswer()
	}
	return ast.ASTRef{}, fmt.Errorf("fixedpoint: answer requires a completed query")
}

// LastGroundSatAnswer returns the latest Spacer ground answer as an owning
// reference, whether the query was an expression or a relation.
func (f *APIFixedpoint) LastGroundSatAnswer() (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if f.hasLastQuery {
		return f.command.FixedpointGroundSatAnswer(f.lastQuery)
	}
	if f.hasLastExpression {
		return f.command.FixedpointExpressionGroundSatAnswer()
	}
	return ast.ASTRef{}, fmt.Errorf("fixedpoint: ground answer requires a completed query")
}
