package smtcore

// Source: context::copy_plugins, theory_seq::mk_fresh and theory_str::mk_fresh.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (k *SMTKernel) copySequencePlugin(source *SMTKernel) error {
	family, err := source.manager.SequenceFamilyID()
	if err != nil {
		return err
	}
	old := source.internalizer.theories[family]
	// An unconfigured eager Go placeholder does not represent an installed
	// native plugin. Explicit and previously copied plugins do, even before
	// their destination's first setup boundary.
	if old == nil || !source.preprocessingSetupDone && old == source.sequenceSetupPlaceholder {
		return nil
	}
	destinationFamily, err := k.manager.SequenceFamilyID()
	if err != nil {
		return err
	}
	if installed := k.internalizer.theories[destinationFamily]; installed != nil && installed != k.sequenceSetupPlaceholder {
		return nil // Native register_plugin keeps an already installed plugin.
	}
	switch previous := old.(type) {
	case *SMTSequenceTheory:
		return k.installFreshSequencePlugin(false, nil, false)
	case *SMTStringTheory:
		return k.installFreshSequencePlugin(true, previous.parameterOwner, true)
	default:
		return nil
	}
}

// Replace only unused destination storage, preserving registration order.
// Copied plugins clear the placeholder marker so deferred setup preserves
// the concrete source backend independently of destination options.
func (k *SMTKernel) installFreshSequencePlugin(legacy bool, owner *SMTStrParams, borrowed bool) error {
	family, err := k.manager.RegisterSequencePlugin()
	if err != nil {
		return err
	}
	s := k.internalizer
	old := s.theories[family]
	if s.theoriesSetup || len(s.scopes) != 0 {
		return ast.ErrASTDomain
	}
	position := -1
	if old != nil {
		switch previous := old.(type) {
		case *SMTSequenceTheory:
			if previous.NumVariables() != 0 {
				return ast.ErrASTDomain
			}
		case *SMTStringTheory:
			if previous.NumVariables() != 0 || len(previous.refs) != 0 {
				return ast.ErrASTDomain
			}
		default:
			return ast.ErrASTDomain
		}
		for index, theory := range s.theoryOrder {
			if theory == old {
				position = index
				break
			}
		}
		if position < 0 {
			return ast.ErrASTDomain
		}
	}
	var theory SMTTheoryInternalizer
	if legacy {
		theory, err = newSMTStringTheory(family, owner, borrowed)
	} else {
		theory, err = NewSMTSequenceTheory(family)
	}
	if err != nil {
		return err
	}
	if old == nil {
		if err = k.RegisterTheory(theory); err != nil {
			return errors.Join(err, theory.(interface{ Close() error }).Close())
		}
		k.sequenceSetupPlaceholder = nil
		return nil
	}
	if err = theory.Initialize(s); err != nil {
		return errors.Join(err, theory.(interface{ Close() error }).Close())
	}
	if err = old.(interface{ Close() error }).Close(); err != nil {
		return errors.Join(err, theory.(interface{ Close() error }).Close())
	}
	s.theories[family], s.theoryOrder[position] = theory, theory
	k.sequenceSetupPlaceholder = nil
	return nil
}
