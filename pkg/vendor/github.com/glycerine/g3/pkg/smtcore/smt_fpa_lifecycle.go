package smtcore

// Source: theory_fpa::mk_fresh and context::copy_plugins.

import "github.com/glycerine/g3/pkg/ast"

func (k *SMTKernel) copyFloatingPointPlugin(source *SMTKernel) error {
	if !source.preprocessingSetupDone {
		return nil
	}
	sourceFamily, err := source.manager.ExistingFloatingPointFamilyID()
	if err != nil {
		return err
	}
	if sourceFamily == ast.NullFamilyID {
		return nil
	}
	if _, ok := source.internalizer.theories[sourceFamily].(*SMTFloatingPointTheory); !ok {
		return nil
	}
	family, err := k.manager.RegisterFloatingPointPlugin()
	if err != nil {
		return err
	}
	if old := k.internalizer.theories[family]; old != nil {
		// Logic setup may already have installed the exact fresh theory that
		// mk_fresh would create. It is valid only before it has acquired state.
		previous, ok := old.(*SMTFloatingPointTheory)
		if !ok || k.internalizer.theoriesSetup || len(k.internalizer.scopes) != 0 || previous.NumVariables() != 0 || len(previous.conversion) != 0 || len(previous.scopes) != 0 {
			return ast.ErrASTDomain
		}
		return nil
	}
	theory, err := NewSMTFloatingPointTheory(family)
	if err != nil {
		return err
	}
	return k.RegisterTheory(theory)
}
