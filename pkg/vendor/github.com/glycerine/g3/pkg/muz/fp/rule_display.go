package fp

// Source: src/muz/base/dl_context.cpp output_profile and
// dl_profile_milliseconds_threshold, used by rule_set::display/rule::display.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) ruleDisplayParameters() (profile bool, threshold uint32, resultErr error) {
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	profile, err = view.Bool("datalog.output_profile", false)
	if err != nil {
		return false, 0, err
	}
	threshold, err = view.UInt("datalog.profile_timeout_milliseconds", 0)
	return profile, threshold, err
}

func (e *PropositionalEngine) writeRuleSetDisplay(out io.Writer, rules *base.RuleSet) error {
	profile, threshold, err := e.ruleDisplayParameters()
	if err != nil {
		return err
	}
	return rules.WriteDisplayWithProfile(out, profile, threshold)
}

func (e *PropositionalEngine) writeRuleDisplay(out io.Writer, rule *base.Rule) error {
	profile, _, err := e.ruleDisplayParameters()
	if err != nil {
		return err
	}
	return e.rules.WriteRuleDisplayWithProfile(out, rule, profile)
}
