package smtcore

// Source: theory_array::internalize_term and setup::setup_arrays (AR_SIMPLE).

import "github.com/glycerine/g3/pkg/ast"

// SMTSimpleArrayTheory shares store/select and model infrastructure with the
// full theory. Its immutable operation boundary excludes full-array axioms.
// Full constructor scheduling remains a separate partial port.
type SMTSimpleArrayTheory struct{ *SMTArrayTheory }

func NewSMTSimpleArrayTheory(family ast.FamilyID) (*SMTSimpleArrayTheory, error) {
	core, err := NewSMTArrayTheory(family)
	if err != nil {
		return nil, err
	}
	core.simple = true
	return &SMTSimpleArrayTheory{core}, nil
}

func smtArrayCore(theory SMTTheoryInternalizer) *SMTArrayTheory {
	switch value := theory.(type) {
	case *SMTArrayTheory:
		return value
	case *SMTSimpleArrayTheory:
		return value.SMTArrayTheory
	default:
		return nil
	}
}

func (k *SMTKernel) setupArrayBackend() error {
	if k.copiedArrayPlugin {
		return k.configureArrayTheories(k.smtParams.array.state)
	}
	family, err := k.manager.ArrayFamilyID()
	if err != nil {
		return err
	}
	s := k.internalizer
	old := s.theories[family]
	core := smtArrayCore(old)
	if core == nil {
		return nil
	}
	simple := k.smtParams.array.state.Mode == 1
	if k.smtParams.array.state.Mode != 1 && k.smtParams.array.state.Mode != 3 {
		return ast.ErrASTDomain
	}
	if core.simple == simple {
		return k.configureArrayTheories(k.smtParams.array.state)
	}
	if core.NumVariables() != 0 || len(core.storeTrail) != 0 || len(core.selectTrail) != 0 || len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	var theory SMTTheoryInternalizer
	if simple {
		theory, err = NewSMTSimpleArrayTheory(family)
	} else {
		theory, err = NewSMTArrayTheory(family)
	}
	if err != nil {
		return err
	}
	if err = smtArrayCore(theory).ConfigureArray(k.smtParams.array.state); err != nil {
		return err
	}
	return k.replaceUnusedArrayTheory(theory)
}

func (k *SMTKernel) replaceUnusedArrayTheory(theory SMTTheoryInternalizer) error {
	core := smtArrayCore(theory)
	if core.parameterOwner == nil {
		core.parameterOwner = &k.smtParams.array
	}
	s := k.internalizer
	if len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	old := s.theories[theory.FamilyID()]
	if old == nil {
		return s.RegisterTheory(theory)
	}
	previous := smtArrayCore(old)
	if previous == nil || previous.NumVariables() != 0 || len(previous.storeTrail) != 0 || len(previous.selectTrail) != 0 || len(previous.scopes) != 0 {
		return ast.ErrASTDomain
	}
	position := -1
	for i, registered := range s.theoryOrder {
		if registered == old {
			position = i
			break
		}
	}
	if position < 0 {
		return ast.ErrASTDomain
	}
	if err := theory.Initialize(s); err != nil {
		return err
	}
	s.theories[theory.FamilyID()], s.theoryOrder[position] = theory, theory
	return nil
}
