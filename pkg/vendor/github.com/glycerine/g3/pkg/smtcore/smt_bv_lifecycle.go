package smtcore

// Source: theory_bv::mk_fresh and context::copy_plugins/register_plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
)

func (k *SMTKernel) copyBitVectorPlugin(source *SMTKernel) error {
	if !source.preprocessingSetupDone {
		return nil
	}
	sourceFamily, err := source.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return err
	}
	if _, ok := source.internalizer.theories[sourceFamily].(*SMTBitVectorTheory); !ok {
		return nil
	}
	family, err := k.manager.RegisterBitVectorPlugin()
	if err != nil {
		return err
	}
	theory, err := NewSMTBitVectorTheory(family)
	if err != nil {
		return err
	}
	owner, bitOwner := source.copiedBVOwner, source.copiedBitBlasterOwner
	if owner == nil {
		owner = &source.smtParams.bv
		bitOwner = &source.smtParams.preprocess.bit
	}
	if err := theory.ConfigureBitVector(owner.state); err != nil {
		return err
	}
	theory.bitBlasterParams = *bitOwner
	s := k.internalizer
	if len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	old := s.theories[family]
	if old == nil {
		if err := s.RegisterTheory(theory); err != nil {
			return err
		}
	} else {
		previous, ok := old.(*SMTBitVectorTheory)
		if !ok || previous.NumVariables() != 0 || len(previous.bitTrail) != 0 || len(previous.scopes) != 0 {
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
		if err := previous.Close(); err != nil {
			return errors.Join(err, theory.Close())
		}
		s.theories[family], s.theoryOrder[position] = theory, theory
	}
	k.copiedBVOwner, k.copiedBitBlasterOwner = owner, bitOwner
	return nil
}

func (k *SMTKernel) configureBitVectorTheories(config SMTBVState, bitConfig bit_blaster.SMTBitBlasterParams) error {
	if k.copiedBVOwner != nil {
		config, bitConfig = k.copiedBVOwner.state, *k.copiedBitBlasterOwner
	}
	for _, theory := range k.internalizer.theoryOrder {
		if bv, ok := theory.(*SMTBitVectorTheory); ok {
			if err := bv.ConfigureBitVector(config); err != nil {
				return err
			}
			bv.bitBlasterParams = bitConfig
		}
	}
	return nil
}
