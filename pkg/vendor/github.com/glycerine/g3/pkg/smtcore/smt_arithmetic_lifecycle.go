package smtcore

// Source: concrete arithmetic mk_fresh methods and context::copy_plugins.

func (k *SMTKernel) copyArithmeticPlugin(source *SMTKernel) error {
	// Go eagerly installs constructor placeholders. Native arithmetic plugins
	// are installed at the first push/check boundary, so an unconfigured
	// placeholder must not become a fixed plugin in the destination.
	if !source.preprocessingSetupDone {
		return nil
	}
	sourceFamily, err := source.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	old := source.internalizer.theories[sourceFamily]
	if old == nil {
		return nil
	}
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	var theory SMTTheoryInternalizer
	owner := source.copiedArithmeticOwner
	if owner == nil && source.smtParams != nil {
		owner = &source.smtParams.arith
	}
	switch previous := old.(type) {
	case *SMTLRATheory:
		theory, err = NewSMTLRATheory(family)
	case *SMTLinearArithmeticTheory:
		if previous.infinite {
			theory, err = NewSMTInfiniteArithmeticTheory(family)
		} else if previous.mixed {
			theory, err = NewSMTMixedLinearArithmeticTheory(family)
		} else if previous.integer {
			theory, err = NewSMTLinearIntegerArithmeticTheory(family)
		} else {
			theory, err = NewSMTLinearArithmeticTheory(family)
		}
	case *SMTRealDiffTheory:
		if previous.mixed {
			theory, err = NewSMTMixedDiffTheory(family)
		} else {
			theory, err = NewSMTRealDiffTheory(family)
		}
	case *SMTDenseDiffTheory:
		theory, err = NewSMTDenseDiffTheory(family)
	case *SMTUTVPITheory:
		theory, err = NewSMTUTVPITheory(family)
		owner = nil // Source UTVPI owns constructor defaults.
	case *SMTDummyTheory:
		theory, err = NewSMTDummyTheory(family, previous.name)
		owner = nil
	default:
		return nil // Other concrete arithmetic backends remain unported.
	}
	if err != nil {
		return err
	}
	if owner != nil {
		if configurable, ok := theory.(interface{ ConfigureArithmetic(SMTArithState) error }); ok {
			if err := configurable.ConfigureArithmetic(owner.state); err != nil {
				return err
			}
		}
	}
	if err = k.replaceUnusedSetupArithmetic(theory); err != nil {
		return err
	}
	k.copiedArithmeticPlugin = true
	k.copiedArithmeticOwner = owner
	if owner != nil {
		k.copiedArithmeticConfig = owner.state
	}
	return nil
}

// Fresh arithmetic plugins borrow the source parameter owner. Read its live
// scalar state without reopening its services: a Go copy safely retains the
// last configuration even after the original solver closes. Only reconfigure
// on changes, since doing so resets some source arithmetic search heuristics.
func (k *SMTKernel) configureArithmeticTheories(config SMTArithState) error {
	if k.copiedArithmeticOwner != nil {
		config = k.copiedArithmeticOwner.state
		if config == k.copiedArithmeticConfig {
			return nil
		}
	}
	for _, theory := range k.internalizer.theoryOrder {
		if configurable, ok := theory.(interface{ ConfigureArithmetic(SMTArithState) error }); ok {
			if err := configurable.ConfigureArithmetic(config); err != nil {
				return err
			}
		}
	}
	if k.copiedArithmeticOwner != nil {
		k.copiedArithmeticConfig = config
	}
	return nil
}
