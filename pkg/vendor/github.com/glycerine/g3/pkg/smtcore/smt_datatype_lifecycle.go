package smtcore

// Source: theory_datatype::mk_fresh and context::copy_plugins.

import "github.com/glycerine/g3/pkg/ast"

// Source theory_datatype::restart_eh resets its utility even when no
// constructor preference has yet been requested.
func (t *SMTDatatypeTheory) Restart() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if t.utility == nil {
		t.utility, err = ast.NewDatatypeUtil(state.manager)
		if err != nil {
			return err
		}
	}
	return t.utility.Reset()
}

func (k *SMTKernel) copyDatatypePlugin(source *SMTKernel) error {
	if !source.preprocessingSetupDone {
		return nil
	}
	family, err := source.manager.DatatypeFamilyID()
	if err != nil {
		return err
	}
	previous, ok := source.internalizer.theories[family].(*SMTDatatypeTheory)
	if !ok {
		return nil
	}
	family, err = k.manager.RegisterDatatypePlugin()
	if err != nil {
		return err
	}
	owner := previous.parameterOwner
	if owner == nil && source.smtParams != nil {
		owner = &source.smtParams.datatype
	}
	if old := k.internalizer.theories[family]; old != nil {
		// The destination's eagerly constructed placeholder is already a fresh
		// datatype instance. It must still be unused at this copy boundary.
		next, ok := old.(*SMTDatatypeTheory)
		if !ok || k.internalizer.theoriesSetup || len(k.internalizer.scopes) != 0 || next.NumVariables() != 0 || len(next.data) != 0 {
			return ast.ErrASTDomain
		}
		next.parameterOwner = owner
		return nil
	}
	next, err := NewSMTDatatypeTheory(family)
	if err != nil {
		return err
	}
	next.parameterOwner = owner
	return k.RegisterTheory(next)
}
