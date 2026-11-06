package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Source mk_rule_inliner::count_pred_occurrences protects every nonempty
// extensional relation in addition to the rule set's output predicates.
func (e *PropositionalEngine) datalogInliner(context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*transforms.RuleInlinerPlugin, error) {
	normalization, err := e.ruleNormalizationParams()
	if err != nil {
		return nil, err
	}
	eager, linear, branching := true, true, false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return nil, err
		}
		eager, err = view.Bool("xform.inline_eager", true)
		if err == nil {
			linear, err = view.Bool("xform.inline_linear", true)
		}
		if err == nil {
			branching, err = view.Bool("xform.inline_linear_branch", false)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return nil, err
		}
	}
	var protected []ast.ASTHandle
	for _, declaration := range e.predicateOrder {
		hasFacts, err := e.relations.HasFacts(declaration)
		if err != nil {
			return nil, err
		}
		if hasFacts {
			protected = append(protected, declaration)
		}
	}
	plugin := transforms.NewRuleInlinerPluginWithServices(e.manager, e.rules, nil, eager, linear, context, globals, 0)
	plugin.SetNormalizationParams(normalization)
	plugin.SetLinearBranching(branching)
	if err := plugin.PreserveFacts(protected); err != nil {
		return nil, errors.Join(err, plugin.Close())
	}
	return plugin, nil
}

// A service-free engine still applies the source inline model converter. Its
// evaluator and rewriter use temporary services without changing engine setup.
func (e *PropositionalEngine) expandDatalogModel(source *model.Model) (_ *model.Model, resultErr error) {
	needsServices := false
	for _, conversion := range e.datalogConversions {
		needsServices = needsServices || conversion.kind == hornConvertInline && len(conversion.inline) != 0
	}
	if !needsServices || e.params != nil || e.globals != nil {
		return e.expandHornModel(source, e.datalogConversions)
	}
	if err := e.clearDatalogTransformCleanup(); err != nil {
		return source, err
	}
	owners := &datalogTransformOwners{}
	defer func() {
		e.datalogTransformCleanup = owners
		resultErr = errors.Join(resultErr, e.clearDatalogTransformCleanup())
	}()
	context, globals, err := owners.services(e)
	if err != nil {
		return source, err
	}
	owners.guard, err = astrewriter.NewTheoryRewriter(e.manager, context, globals, nil)
	if err != nil {
		return source, err
	}
	converter := *e
	converter.params, converter.globals, converter.guard = context, globals, owners.guard
	return converter.expandHornModel(source, e.datalogConversions)
}
