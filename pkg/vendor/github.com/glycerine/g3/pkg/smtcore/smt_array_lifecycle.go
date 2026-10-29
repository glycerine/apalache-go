package smtcore

// Source: theory_array::mk_fresh, theory_array_full::mk_fresh and copy_plugins.

func (k *SMTKernel) copyArrayPlugin(source *SMTKernel) error {
	if !source.preprocessingSetupDone {
		return nil
	}
	sourceFamily, err := source.manager.ArrayFamilyID()
	if err != nil {
		return err
	}
	previous := smtArrayCore(source.internalizer.theories[sourceFamily])
	if previous == nil {
		return nil
	}
	family, err := k.manager.RegisterArrayPlugin()
	if err != nil {
		return err
	}
	var theory SMTTheoryInternalizer
	var owner *SMTArrayParams
	config := k.smtParams.array.state
	if previous.simple {
		// The simple source constructor uses new_ctx->get_fparams().
		theory, err = NewSMTSimpleArrayTheory(family)
	} else {
		// The full constructor retains the original m_params reference, also
		// through chained translations. No source AST or enode is retained.
		theory, err = NewSMTArrayTheory(family)
		owner = source.copiedArrayOwner
		if owner == nil {
			owner = &source.smtParams.array
		}
		config = owner.state
	}
	if err != nil {
		return err
	}
	if err = smtArrayCore(theory).ConfigureArray(config); err != nil {
		return err
	}
	smtArrayCore(theory).parameterOwner = owner
	if owner == nil {
		smtArrayCore(theory).parameterOwner = &k.smtParams.array
	}
	if err = k.replaceUnusedArrayTheory(theory); err != nil {
		return err
	}
	if err = smtArrayCore(theory).Setup(); err != nil {
		return err
	}
	k.copiedArrayPlugin, k.copiedArrayOwner = true, owner
	return nil
}
