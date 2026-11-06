package transforms

// Source: fixedpoint_params.pyg and dl_mk_interp_tail_simplifier.cpp:400-402,
// dl_mk_rule_inliner.cpp:129-131. Query owners resolve these two values while
// their local parameter snapshot is live; nested normalizers copy only scalars.

import "errors"

type RuleNormalizationParams struct {
	PropagateVariableEquivalences bool
	FixUnboundVariables           bool
}

func DefaultRuleNormalizationParams() RuleNormalizationParams {
	return RuleNormalizationParams{PropagateVariableEquivalences: true}
}

func (p *RuleInlinerPlugin) SetNormalizationParams(params RuleNormalizationParams) {
	p.normalization = &params
	if p.simplifier != nil {
		p.simplifier.SetNormalizationParams(params)
	}
}

func (p *RuleUnfoldPlugin) SetNormalizationParams(params RuleNormalizationParams) {
	p.resolver.SetNormalizationParams(params)
}

func (p *BitBlastPlugin) SetNormalizationParams(params RuleNormalizationParams) {
	p.simplifier.SetNormalizationParams(params)
}

func (p *ArrayBlastPlugin) SetNormalizationParams(params RuleNormalizationParams) {
	p.simplifier.SetNormalizationParams(params)
}

func (p *InterpTailSimplifierPlugin) SetNormalizationParams(params RuleNormalizationParams) {
	p.normalization = &params
}

func (p *InterpTailSimplifierPlugin) variableEquivalencePropagation() (bool, error) {
	if p.normalization != nil {
		return p.normalization.PropagateVariableEquivalences, nil
	}
	module, err := p.globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return false, err
	}
	name := "xform.tail_simplifier_pve"
	enabled, err := p.params.GetBoolName(&module, &name, true)
	return enabled, errors.Join(err, p.params.Del(&module))
}
