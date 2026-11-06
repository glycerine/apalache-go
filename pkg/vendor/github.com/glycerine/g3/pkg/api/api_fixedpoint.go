package api

// Public fixedpoint/Horn integration over the retained command context.

import (
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// FixedpointParameterDescriptor is copied metadata for one fixedpoint option.
type FixedpointParameterDescriptor struct {
	Name        string
	Kind        string
	Description string
	Default     string
}

// FixedpointParameterDescriptors returns copied metadata for every registered
// fixedpoint option. Names are directly accepted by the option setters.
func (c *SMTContext) FixedpointParameterDescriptors() ([]FixedpointParameterDescriptor, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	service, err := c.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	descriptors, err := service.SourceModuleDescriptorsName("fixedpoint")
	if err != nil {
		return nil, err
	}
	count, err := descriptors.Size()
	if err != nil {
		return nil, err
	}
	result := make([]FixedpointParameterDescriptor, 0, count)
	for i := uint32(0); i < count; i++ {
		symbol, x := descriptors.ParamName(i)
		if x != nil {
			return nil, x
		}
		name, x := service.Symbols.String(symbol)
		if x != nil {
			return nil, x
		}
		kind, x := descriptors.Kind(symbol)
		if x != nil {
			return nil, x
		}
		description, _, x := descriptors.Description(symbol)
		if x != nil {
			return nil, x
		}
		defaultValue, _, x := descriptors.Default(symbol)
		if x != nil {
			return nil, x
		}
		result = append(result, FixedpointParameterDescriptor{
			Name: "fixedpoint." + name, Kind: kind.String(),
			Description: description, Default: defaultValue,
		})
	}
	return result, nil
}

// WriteFixedpointSMT2 writes the source fixedpoint context dump using effective
// print options and the supplied queries in order. For complete declaration
// replay independent of print options, use WriteFixedpointReplaySMT2.
func (c *SMTContext) WriteFixedpointSMT2(output io.Writer, queries ...ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if output == nil {
		return ErrSMTContextOwner
	}
	return c.command.WriteFixedpointSMT2(output, queries...)
}

// FixedpointSMT2 returns the source context dump produced by WriteFixedpointSMT2.
func (c *SMTContext) FixedpointSMT2(queries ...ast.ASTHandle) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	if err := c.command.WriteFixedpointSMT2(&output, queries...); err != nil {
		return "", err
	}
	return output.String(), nil
}

// LoadFixedpointSMT2 executes fixedpoint SMT-LIB commands from input while
// discarding command output. Callers use ordinary Go readers for strings,
// files, or streams.
func (c *SMTContext) LoadFixedpointSMT2(input io.Reader) error {
	if err := c.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	return c.ExecuteSMT2(input, io.Discard)
}

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

// SetFixedpointPredicateRepresentation selects one or more native finite
// table storage kinds before the first fixedpoint query. The retained kinds
// are tr_sparse, tr_hashtable, tr_bitvector, and tr_lazy_sparse.
func (c *SMTContext) SetFixedpointPredicateRepresentation(declaration ast.ASTHandle, kinds ...string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.SetFixedpointPredicateRepresentation(declaration, kinds)
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

// AddFixedpointRuleWithBound adds a Horn rule with a per-rule recursion bound
// for Duality. Datalog and PDR ignore the bound.
func (c *SMTContext) AddFixedpointRuleWithBound(formula ast.ASTHandle, name string, bound uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointRuleWithBound(formula, name, bound)
}

// UpdateFixedpointRule adds a named Horn rule or strengthens the unique
// existing rule carrying that name.
func (c *SMTContext) UpdateFixedpointRule(formula ast.ASTHandle, name string) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.UpdateFixedpointRule(formula, name)
}

// AddFixedpointFact adds a ground relation tuple using concrete AST values.
func (c *SMTContext) AddFixedpointFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointFact(declaration, values)
}

// AddFixedpointTableFact adds a ground relation tuple using native finite
// table element numbers.
func (c *SMTContext) AddFixedpointTableFact(declaration ast.ASTHandle, values []uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AddFixedpointTableFact(declaration, values)
}

// AssertFixedpoint adds a background assertion in the current fixedpoint
// scope.
func (c *SMTContext) AssertFixedpoint(formula ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.command.AssertFixedpoint(formula)
}

// FixedpointAssertions returns owning fixedpoint background assertions in
// insertion order. Release every result through Manager().DelRef.
func (c *SMTContext) FixedpointAssertions() ([]ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.FixedpointAssertions()
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

// FixedpointRules returns normalized rules, pending closed input formulas, and
// negated output queries. Release every result through Manager().DelRef.
func (c *SMTContext) FixedpointRules() ([]ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.command.FixedpointRules()
}

// QueryFixedpoint asks whether the registered relation is reachable.
func (c *SMTContext) QueryFixedpoint(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	scope, err := c.beginFixedpointResourceLimit()
	if err != nil {
		return lbool.Undef, err
	}
	defer scope.Close()
	return c.command.QueryFixedpoint(declaration)
}

// QueryFixedpointWithOptions applies typed settings to one query without
// changing the context's fixedpoint parameters or subsequent queries.
func (c *SMTContext) QueryFixedpointWithOptions(declaration ast.ASTHandle, options *FixedpointQueryOptions) (status lbool.LBool, resultErr error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	if options == nil {
		return c.QueryFixedpoint(declaration)
	}
	parameters, context, err := c.fixedpointQueryParameters(options)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&parameters)) }()
	scope, err := c.beginFixedpointResourceLimit()
	if err != nil {
		return lbool.Undef, err
	}
	defer scope.Close()
	return c.command.QueryFixedpointWithParams(declaration, &parameters)
}

// QueryFixedpointRelations reports whether at least one requested relation is
// nonempty. PDR and Spacer accept exactly one relation.
func (c *SMTContext) QueryFixedpointRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	return c.command.QueryFixedpointRelations(declarations)
}

// QueryFixedpointFromLevel runs a Spacer query beginning at the requested
// proof level.
func (c *SMTContext) QueryFixedpointFromLevel(declaration ast.ASTHandle, level uint32) (lbool.LBool, error) {
	if err := c.open(); err != nil {
		return lbool.Undef, err
	}
	scope, err := c.beginFixedpointResourceLimit()
	if err != nil {
		return lbool.Undef, err
	}
	defer scope.Close()
	return c.command.QueryFixedpointFromLevel(declaration, level)
}

// FixedpointReasonUnknown returns the native status spelling for the latest
// fixedpoint query: "ok" for a decided query and "approximated" for an
// undecided retained query.
func (c *SMTContext) FixedpointReasonUnknown() (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	return c.command.FixedpointReasonUnknown()
}

// FixedpointAnswer returns one owning AST reference for the latest successful
// query. The caller releases it through Manager().DelRef.
func (c *SMTContext) FixedpointAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.FixedpointAnswer(declaration)
}

// FixedpointProof returns one owning PDR proof reference for the latest
// reachable query when fixedpoint.generate_proof_trace is enabled. The caller
// releases it through Manager().DelRef.
func (c *SMTContext) FixedpointProof(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return c.command.FixedpointProof(declaration)
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

// FixedpointRulesAlongTraceWithSymbols returns caller-owned trace formulas and
// their exact rule-name symbols. Release every formula through Manager().DelRef.
func (c *SMTContext) FixedpointRulesAlongTraceWithSymbols() ([]ast.ASTRef, []symbol.Symbol, error) {
	if err := c.open(); err != nil {
		return nil, nil, err
	}
	return c.command.FixedpointRulesAlongTraceWithSymbols()
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
