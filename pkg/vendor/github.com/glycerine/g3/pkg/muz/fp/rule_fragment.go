package fp

// Source: src/muz/base/dl_context.h; src/muz/base/dl_context.cpp check_rules,
// src/muz/fp/dl_register_engine.h; src/muz/fp/dl_register_engine.cpp retained
// Datalog/PDR/Spacer engine selection.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/muz/base"
)

// Source checks these properties before transformations. The infinite-sort
// check remains separate to preserve the integer-relation extension.
func (e *PropositionalEngine) checkDatalogEarlyRuleProperties() (result error) {
	proof, err := e.sourceRuleProofTrace()
	if err != nil {
		return err
	}
	properties, err := e.rulePropertyOwner()
	if err != nil {
		return err
	}
	if err = properties.SetGenerateProof(proof); err != nil {
		return err
	}
	if err = properties.Collect(e.set); err != nil {
		return err
	}
	if err = properties.CheckQuantifierFree(); err != nil {
		return fmt.Errorf("%w: validate datalog rule fragment: %w", ErrPropositionalHorn, err)
	}
	if err = properties.CheckUninterpretedFree(); err != nil {
		return fmt.Errorf("%w: validate datalog rule fragment: %w", ErrPropositionalHorn, err)
	}
	if err = properties.CheckNestedFree(); err != nil {
		return fmt.Errorf("%w: validate datalog rule fragment: %w", ErrPropositionalHorn, err)
	}
	return nil
}

func (e *PropositionalEngine) rulePropertyOwner() (*base.RuleProperties, error) {
	if e == nil || e.rules == nil {
		return nil, ErrPropositionalHorn
	}
	if e.ruleProperties == nil {
		properties, err := base.NewRuleProperties(e.rules)
		if err != nil {
			return nil, err
		}
		e.ruleProperties = properties
	}
	return e.ruleProperties, nil
}

// validateRuleSetProperties applies the source check_rules ordering. Ordering
// is observable: the first failing property is the diagnostic returned to the
// fixedpoint caller.
func validateRuleSetProperties(properties *base.RuleProperties, set *base.RuleSet, backend string, generateProof bool) error {
	kind, err := parseEngineKind(backend)
	if err != nil || !kind.Retained() {
		if err == nil {
			err = fmt.Errorf("unsupported fixedpoint engine %q", backend)
		}
		return fmt.Errorf("%w: %w", ErrPropositionalHorn, err)
	}
	if properties == nil {
		return ErrPropositionalHorn
	}
	if err = properties.SetGenerateProof(generateProof); err != nil {
		return fmt.Errorf("%w: %w", ErrPropositionalHorn, err)
	}
	if err = properties.Collect(set); err != nil {
		return fmt.Errorf("%w: collect rule properties: %w", ErrPropositionalHorn, err)
	}
	switch kind {
	case EngineDatalog:
		err = properties.CheckQuantifierFree()
		if err == nil {
			err = properties.CheckUninterpretedFree()
		}
		if err == nil {
			err = properties.CheckNestedFree()
		}
		if err == nil {
			err = properties.CheckInfiniteSorts()
		}
	case EnginePDR, EngineSpacer, EngineCLP, EngineTab, EngineDuality:
		err = properties.CheckExistentialTail()
		if err == nil {
			err = properties.CheckNegativePredicates()
		}
		if err == nil && kind != EngineCLP && kind != EngineTab && kind != EngineDuality {
			err = properties.CheckUninterpretedFree()
		}
	case EngineQPDR:
		err = properties.CheckNegativePredicates()
		if err == nil {
			err = properties.CheckUninterpretedFree()
		}
	case EngineBMC:
		err = properties.CheckNegativePredicates()
	case EngineQBMC:
		err = properties.CheckExistentialTail()
		if err == nil {
			err = properties.CheckNegativePredicates()
		}
	}
	if err != nil {
		var property *base.RulePropertyError
		if (kind == EnginePDR || kind == EngineQPDR || kind == EngineSpacer) && errors.As(err, &property) && property.Kind == base.RulePropertyUninterpreted {
			return fmt.Errorf("%w: uninterpreted symbols in PDR rules are unsupported: %w", ErrPropositionalHorn, err)
		}
		return fmt.Errorf("%w: validate %s rule fragment: %w", ErrPropositionalHorn, backend, err)
	}
	return nil
}
