package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/transforms"
)

// Copy the two rule-normalization switches before the query-local parameter
// view closes. Nested plugins and delayed proof converters own only scalars.
func (e *PropositionalEngine) ruleNormalizationParams() (out transforms.RuleNormalizationParams, resultErr error) {
	out = transforms.DefaultRuleNormalizationParams()
	if e.params == nil || e.globals == nil {
		return out, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return out, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	out.PropagateVariableEquivalences, err = view.Bool("xform.tail_simplifier_pve", true)
	if err != nil {
		return out, err
	}
	out.FixUnboundVariables, err = view.Bool("xform.fix_unbound_vars", false)
	return out, err
}
