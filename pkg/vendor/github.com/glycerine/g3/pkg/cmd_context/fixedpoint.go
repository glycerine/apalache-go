package cmd_context

// Source: src/muz/fp/dl_cmds.cpp declare-rel/rule/query command context and
// src/muz/base/dl_context.cpp retained orchestration. The current backend is
// the positive finite-domain fragment exposed by muz/fp.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (c *Context) fixedpointContext() (*muzfp.PropositionalEngine, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if c.fixedpoint != nil {
		return c.fixedpoint, nil
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	engine, err := muzfp.NewPropositionalEngineWithServices(manager, c.config.Params, c.config.Globals)
	if err != nil {
		return nil, err
	}
	c.fixedpoint = engine
	return engine, nil
}

func (c *Context) DeclareRelation(name string, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := c.DeclareFunction(name, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = engine.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	return declaration, nil
}

// SetFixedpointPredicateRepresentation selects native finite-table storage
// names for a registered relation before its first query.
func (c *Context) SetFixedpointPredicateRepresentation(declaration ast.ASTHandle, kinds []string) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.SetPredicateRepresentation(declaration, kinds)
}

func (c *Context) AddFixedpointRule(formula ast.ASTHandle, name string) error {
	return c.AddFixedpointRuleWithBound(formula, name, ^uint32(0))
}

// AddFixedpointRuleWithBound preserves the rule command's recursion bound for
// Duality's raw-formula preparation. Other engines ignore this metadata.
func (c *Context) AddFixedpointRuleWithBound(formula ast.ASTHandle, name string, bound uint32) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddRuleWithBound(formula, name, bound)
}

// AddFixedpointRuleWithSymbol preserves null, empty and numerical label identity.
func (c *Context) AddFixedpointRuleWithSymbol(formula ast.ASTHandle, name symbol.Symbol) error {
	return c.AddFixedpointRuleWithSymbolAndBound(formula, name, ^uint32(0))
}

// AddFixedpointRuleWithSymbolAndBound also preserves the original recursion bound.
func (c *Context) AddFixedpointRuleWithSymbolAndBound(formula ast.ASTHandle, name symbol.Symbol, bound uint32) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddRuleWithSymbolAndBound(formula, name, bound)
}

// UpdateFixedpointRule adds a named Horn rule or strengthens the unique
// existing rule carrying that name.
func (c *Context) UpdateFixedpointRule(formula ast.ASTHandle, name string) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.UpdateRule(formula, name)
}

// UpdateFixedpointRuleWithSymbol matches the exact source label when replacing a rule.
func (c *Context) UpdateFixedpointRuleWithSymbol(formula ast.ASTHandle, name symbol.Symbol) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.UpdateRuleWithSymbol(formula, name)
}

// FixedpointNamedRulesWithSymbols returns owned formulas and exact source labels.
// The caller releases each formula through Manager.DelRef.
func (c *Context) FixedpointNamedRulesWithSymbols() ([]ast.ASTRef, []symbol.Symbol, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, nil, err
	}
	return engine.NamedRulesWithSymbols()
}

// AddFixedpointFact adds a ground relation tuple using concrete AST values.
func (c *Context) AddFixedpointFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddFact(declaration, values)
}

// AddFixedpointTableFact adds a ground relation tuple using native finite
// table element numbers.
func (c *Context) AddFixedpointTableFact(declaration ast.ASTHandle, values []uint32) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddTableFact(declaration, values)
}

// AssertFixedpoint adds a background assertion in the current fixedpoint
// scope.
func (c *Context) AssertFixedpoint(formula ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AssertBackground(formula)
}

// FixedpointAssertions returns owning fixedpoint background assertions in
// insertion order.
func (c *Context) FixedpointAssertions() ([]ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.BackgroundAssertions()
}

// DeclareFixedpointVariable declares and registers an implicit Datalog
// variable, matching the declare-var command's constant-to-bound-variable path.
func (c *Context) DeclareFixedpointVariable(name string, sort ast.ASTHandle) error {
	declaration, err := c.DeclareFunction(name, nil, sort)
	if err != nil {
		return err
	}
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	variable, err := manager.MkApp(declaration, nil)
	if err != nil {
		return err
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.RegisterVariable(variable)
}

func (c *Context) PushFixedpoint() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.Push()
}

func (c *Context) PopFixedpoint() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.Pop()
}

// SetFixedpointParameters immediately updates the fixedpoint-local parameter
// snapshot, matching fixedpoint_context::updt_params before engine selection.
func (c *Context) SetFixedpointParameters(params *uparams.ParamsRef) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.SetQueryParameters(params)
}

func (c *Context) prepareFixedpointQuery(params *uparams.ParamsRef, includeCurrentAssertions bool) (*muzfp.PropositionalEngine, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	if err = engine.PrepareQueryParameters(params); err != nil {
		return nil, err
	}
	var assertions []ast.ASTHandle
	if includeCurrentAssertions {
		assertions, err = c.Assertions()
		if err != nil {
			return nil, err
		}
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	fixedpointAssertions, err := engine.BackgroundAssertions()
	if err != nil {
		return nil, err
	}
	for i := range fixedpointAssertions {
		handle, x := manager.RefGet(fixedpointAssertions[i])
		if x == nil {
			assertions = append(assertions, handle)
		}
		err = errors.Join(err, x)
	}
	for i := len(fixedpointAssertions) - 1; i >= 0; i-- {
		err = errors.Join(err, manager.DelRef(&fixedpointAssertions[i]))
	}
	if err != nil {
		return nil, err
	}
	background, err := manager.MkAndArray(assertions)
	if err != nil {
		return nil, err
	}
	if err = engine.SetBackground(background); err != nil {
		return nil, err
	}
	if err = engine.SelectConfiguredEngine(); err != nil {
		return nil, err
	}
	return engine, nil
}

func (c *Context) QueryFixedpoint(declaration ast.ASTHandle) (lbool.LBool, error) {
	return c.QueryFixedpointWithParams(declaration, nil)
}

// FixedpointPredicates returns borrowed registered relation declarations in
// source declaration order for frontend saturation and inspection.
func (c *Context) FixedpointPredicates() ([]ast.ASTHandle, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.Predicates()
}

// QueryFixedpointWithParams applies query-local fixedpoint settings before
// module settings and bounds the query with the effective fixedpoint timeout.
func (c *Context) QueryFixedpointWithParams(declaration ast.ASTHandle, params *uparams.ParamsRef) (result lbool.LBool, resultErr error) {
	return c.queryFixedpoint(declaration, params, false, nil, true)
}

// QueryFixedpointCommandWithParams executes dl_query_cmd: each current SMT
// assertion is appended to the fixedpoint background before parameters update.
func (c *Context) QueryFixedpointCommandWithParams(declaration ast.ASTHandle, params *uparams.ParamsRef) (lbool.LBool, error) {
	assertions, err := c.Assertions()
	if err != nil {
		return lbool.Undef, err
	}
	for _, assertion := range assertions {
		if err := c.AssertFixedpoint(assertion); err != nil {
			return lbool.Undef, err
		}
	}
	return c.queryFixedpoint(declaration, params, false, nil, false)
}

// QueryFixedpointExpression evaluates a Boolean query with the configured timeout.
func (c *Context) QueryFixedpointExpression(query ast.ASTHandle) (lbool.LBool, error) {
	return c.queryFixedpoint(query, nil, true, nil, true)
}

// QueryFixedpointExpressionWithParams applies fixedpoint-local settings to a Boolean query.
func (c *Context) QueryFixedpointExpressionWithParams(query ast.ASTHandle, params *uparams.ParamsRef) (lbool.LBool, error) {
	return c.queryFixedpoint(query, params, true, nil, true)
}

func (c *Context) queryFixedpoint(query ast.ASTHandle, params *uparams.ParamsRef, expression bool, level *uint32, includeCurrentAssertions bool) (result lbool.LBool, resultErr error) {
	engine, err := c.prepareFixedpointQuery(params, includeCurrentAssertions)
	if err != nil {
		return lbool.Undef, err
	}
	scope, err := c.beginFixedpointTimeout(params)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	if expression {
		if level != nil {
			return engine.QueryExpressionFromLevel(query, *level)
		}
		return engine.Query(query)
	}
	if level != nil {
		return engine.QueryPredicateFromLevel(query, *level)
	}
	result, err = engine.QueryPredicate(query)
	return result, classifyFixedpointQueryException(err)
}

// QueryFixedpointRelations reports whether at least one requested relation is
// nonempty. PDR and Spacer accept exactly one relation, matching the native
// fixedpoint engine contract.
func (c *Context) QueryFixedpointRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	return c.QueryFixedpointRelationsWithParams(declarations, nil)
}

// QueryFixedpointRelationsWithParams applies fixedpoint-local relation-query settings.
func (c *Context) QueryFixedpointRelationsWithParams(declarations []ast.ASTHandle, params *uparams.ParamsRef) (result lbool.LBool, resultErr error) {
	engine, err := c.prepareFixedpointQuery(params, true)
	if err != nil {
		return lbool.Undef, err
	}
	scope, err := c.beginFixedpointTimeout(params)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	return engine.QueryPredicates(declarations)
}

// QueryFixedpointFromLevel runs a Spacer query beginning at the requested
// proof level.
func (c *Context) QueryFixedpointFromLevel(declaration ast.ASTHandle, level uint32) (lbool.LBool, error) {
	return c.QueryFixedpointFromLevelWithParams(declaration, level, nil)
}

// QueryFixedpointFromLevelWithParams runs a bounded Spacer query with local settings.
func (c *Context) QueryFixedpointFromLevelWithParams(declaration ast.ASTHandle, level uint32, params *uparams.ParamsRef) (lbool.LBool, error) {
	return c.queryFixedpoint(declaration, params, false, &level, true)
}

// QueryFixedpointExpressionFromLevelWithParams runs a general Spacer query
// beginning at the requested proof level with fixedpoint-local settings.
func (c *Context) QueryFixedpointExpressionFromLevelWithParams(query ast.ASTHandle, level uint32, params *uparams.ParamsRef) (lbool.LBool, error) {
	return c.queryFixedpoint(query, params, true, &level, true)
}

// FixedpointReasonUnknown returns the execution status of the latest
// fixedpoint query using the native fixedpoint status spellings.
func (c *Context) FixedpointReasonUnknown() (string, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return "", err
	}
	return engine.ReasonUnknown()
}

// ResetFixedpointCommandStatus applies the status reset from fixedpoint
// command cleanup after the query result has been printed.
func (c *Context) ResetFixedpointCommandStatus() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.ResetCommandStatus()
}

// FixedpointAnswer returns an owning formula for the most recently saturated
// finite relation. The caller releases it through Manager.DelRef.
func (c *Context) FixedpointAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.Answer(declaration)
}

// FixedpointProof returns the owning proof trace for the latest reachable PDR
// query when fixedpoint.generate_proof_trace is enabled.
func (c *Context) FixedpointProof(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.Proof(declaration)
}

// WriteFixedpointCertificate writes the PDR certificate selected by the
// fixedpoint query command. The Boolean result is false for backends whose
// source certificate display is intentionally empty.
func (c *Context) WriteFixedpointCertificate(out io.Writer, declaration ast.ASTHandle) (bool, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return false, err
	}
	return engine.WriteCertificate(out, declaration, c.WriteExpression)
}

// FixedpointModel returns the inductive model produced by the latest
// unreachable PDR or Spacer query. The caller owns and closes the returned
// model.
func (c *Context) FixedpointModel(declaration ast.ASTHandle) (*model.Model, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.Model(declaration)
}

// EvaluateFixedpointModel evaluates an expression in the inductive model from
// the latest unreachable PDR or Spacer query and returns one owning AST
// reference.
func (c *Context) EvaluateFixedpointModel(declaration, expression ast.ASTHandle, completion bool) (result ast.ASTRef, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	inductiveModel, err := c.FixedpointModel(declaration)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, inductiveModel.Close()) }()
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = inductiveModel.Evaluate(expression, result, completion, c.config.ModelEvaluator); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Context) FixedpointNumLevels(declaration ast.ASTHandle) (uint32, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return 0, err
	}
	if err = engine.SelectConfiguredEngine(); err != nil {
		return 0, err
	}
	return engine.NumLevels(declaration)
}

// FixedpointCoverDelta returns an owning formula for exactly one PDR or Spacer
// frame; level -1 selects the invariant frame.
func (c *Context) FixedpointCoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = engine.SelectConfiguredEngine(); err != nil {
		return ast.ASTRef{}, err
	}
	return engine.CoverDelta(level, declaration)
}

// AddFixedpointCover installs a bound-variable property into one PDR or Spacer
// frame.
func (c *Context) AddFixedpointCover(level int, declaration, property ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	if err = engine.SelectConfiguredEngine(); err != nil {
		return err
	}
	return engine.AddCover(level, declaration, property)
}

// AddFixedpointInvariant installs a bound-variable property in the invariant
// frame of a PDR or Spacer predicate.
func (c *Context) AddFixedpointInvariant(declaration, property ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddInvariant(declaration, property)
}

// FixedpointReachable returns the accumulated Spacer reach summary as one
// owning AST reference.
func (c *Context) FixedpointReachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.Reachable(declaration)
}

// FixedpointGroundSatAnswer returns the latest successful Spacer ground
// reach-fact answer as one owning AST reference.
func (c *Context) FixedpointGroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.GroundSatAnswer(declaration)
}

// FixedpointRulesAlongTrace returns owning rule formulas and their names in
// preorder for the latest successful Spacer query.
func (c *Context) FixedpointRulesAlongTrace() ([]ast.ASTRef, []string, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, nil, err
	}
	return engine.RulesAlongTrace()
}

// FixedpointRulesAlongTraceWithSymbols returns caller-owned formulas and the
// exact source rule-name symbols, including absent, empty and numerical names.
func (c *Context) FixedpointRulesAlongTraceWithSymbols() ([]ast.ASTRef, []symbol.Symbol, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, nil, err
	}
	return engine.RulesAlongTraceWithSymbols()
}

func (c *Context) FixedpointStatistics() (statistics.Statistics, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return statistics.Statistics{}, err
	}
	var result statistics.Statistics
	if err = engine.CollectStatistics(&result); err != nil {
		return statistics.Statistics{}, err
	}
	return result, nil
}

func (c *Context) ResetFixedpointStatistics() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.ResetStatistics()
}

// FixedpointRules returns the source public rule enumeration, preserving pending
// closed formulas and appending negated output queries. The caller owns each ref.
func (c *Context) FixedpointRules() ([]ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.RuleFormulas()
}
