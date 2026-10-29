package api

// Public fixedpoint/Horn integration over the retained command context.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// SetFixedpointEngine selects the fixedpoint backend used by later queries.
// Supported retained choices are auto-config, datalog, pdr, and spacer.
func (c *SMTContext) SetFixedpointEngine(name string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetSymbolOption("fixedpoint.engine", name)
}

// DeclareRelation declares and registers a Boolean fixedpoint relation.
func (c *SMTContext) DeclareRelation(name string, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.command.DeclareRelation(name, domain)
}

// DeclareFixedpointVariable declares one implicit variable used by later
// fixedpoint rules and returns its borrowed constant application.
func (c *SMTContext) DeclareFixedpointVariable(name string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := c.command.DeclareFixedpointVariable(name, sort); err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := c.command.FindFunctionDeclaration(name, nil, nil, ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkApp(declaration, nil)
}

// AddFixedpointRule adds a Horn rule to the current fixedpoint scope.
func (c *SMTContext) AddFixedpointRule(formula ast.ASTHandle, name string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointRule(formula, name)
}

// AddFixedpointRuleWithBound adds a Horn rule with the retained duality-engine
// recursion-bound field. Current datalog/PDR backends consume but ignore it.
func (c *SMTContext) AddFixedpointRuleWithBound(formula ast.ASTHandle, name string, bound uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointRuleWithBound(formula, name, bound)
}

// PushFixedpoint saves the current fixedpoint rule set.
func (c *SMTContext) PushFixedpoint() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.PushFixedpoint()
}

// PopFixedpoint restores the most recent fixedpoint rule-set snapshot.
func (c *SMTContext) PopFixedpoint() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.PopFixedpoint()
}

// QueryFixedpoint asks whether the registered relation is reachable.
func (c *SMTContext) QueryFixedpoint(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.command.QueryFixedpoint(declaration)
}

// QueryFixedpointFromLevel runs a Spacer query beginning at the requested
// proof level.
func (c *SMTContext) QueryFixedpointFromLevel(declaration ast.ASTHandle, level uint32) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.command.QueryFixedpointFromLevel(declaration, level)
}

// FixedpointAnswer returns one owning AST reference for the latest successful
// query. The caller releases it through Manager().DelRef.
func (c *SMTContext) FixedpointAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.FixedpointAnswer(declaration)
}

// FixedpointModel returns the inductive model from the latest unreachable PDR
// or Spacer query. The caller owns and closes the returned model.
func (c *SMTContext) FixedpointModel(declaration ast.ASTHandle) (*model.Model, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.FixedpointModel(declaration)
}

// EvaluateFixedpointModel evaluates an expression in the inductive model from
// the latest unreachable PDR or Spacer query and returns one owning AST
// reference.
func (c *SMTContext) EvaluateFixedpointModel(declaration, expression ast.ASTHandle, completion bool) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.EvaluateFixedpointModel(declaration, expression, completion)
}

// FixedpointNumLevels returns the current number of finite PDR frames for a
// relation. It is available after a PDR/Spacer query.
func (c *SMTContext) FixedpointNumLevels(declaration ast.ASTHandle) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.command.FixedpointNumLevels(declaration)
}

// FixedpointCoverDelta returns an owning reference to exactly one PDR or
// Spacer cover frame. Level -1 denotes the invariant frame; release the result
// via Manager().DelRef.
func (c *SMTContext) FixedpointCoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.FixedpointCoverDelta(level, declaration)
}

// AddFixedpointCover adds a property over bound predicate arguments to one PDR
// or Spacer frame. Level -1 denotes the invariant frame.
func (c *SMTContext) AddFixedpointCover(level int, declaration, property ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointCover(level, declaration, property)
}

// AddFixedpointInvariant adds a property over bound predicate arguments to the
// invariant frame of a PDR or Spacer predicate.
func (c *SMTContext) AddFixedpointInvariant(declaration, property ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointInvariant(declaration, property)
}

// FixedpointReachable returns one owning reference to the accumulated Spacer
// reach summary. Release it through Manager().DelRef.
func (c *SMTContext) FixedpointReachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.FixedpointReachable(declaration)
}

// FixedpointGroundSatAnswer returns one owning reference to the latest
// successful Spacer ground reach-fact answer. Release it through
// Manager().DelRef.
func (c *SMTContext) FixedpointGroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.FixedpointGroundSatAnswer(declaration)
}

// FixedpointRulesAlongTrace returns owning rule formulas and corresponding
// names in preorder for the latest successful Spacer query. Release every
// formula through Manager().DelRef.
func (c *SMTContext) FixedpointRulesAlongTrace() ([]ast.ASTRef, []string, error) {
	if err := c.open(); err != nil {
		return nil, nil, err
	}
	return c.command.FixedpointRulesAlongTrace()
}

// FixedpointStatistics returns an owning Go copy of current engine statistics.
func (c *SMTContext) FixedpointStatistics() (statistics.Statistics, error) {
	if err := c.open(); err != nil {
		return statistics.Statistics{}, err
	}
	return c.command.FixedpointStatistics()
}

// ResetFixedpointStatistics clears counters retained by the current engine.
func (c *SMTContext) ResetFixedpointStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.ResetFixedpointStatistics()
}
