package smtcore

// Source: src/smt/smt_setup.cpp setup_QF_IDL/setup_QF_RDL overloads and is_dense.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func smtDenseArithmetic(features *ast.StaticFeatures) bool {
	return features.NumUninterpretedConstants < 1000 &&
		features.NumArithEqualities+features.NumArithInequalities > features.NumUninterpretedConstants*9
}

func (k *SMTKernel) setupIntegerDifferencePolicy(useStaticFeatures bool) (result error) {
	if k.setupLogic != "QF_IDL" {
		return nil
	}
	features, err := k.setupStaticFeatures(useStaticFeatures)
	if err != nil {
		return err
	}
	if features != nil {
		defer func() { result = errors.Join(result, features.Close()) }()
		if features.NumArithEqualities != features.NumDiffEqualities || features.NumArithTerms != features.NumDiffTerms || features.NumArithInequalities != features.NumDiffInequalities {
			return fmt.Errorf("Benchmark is not in QF_IDL (integer difference logic).")
		}
		if features.HasReal {
			return fmt.Errorf("Benchmark has real variables but it is marked as QF_IDL (integer difference logic).")
		}
		if features.NumUninterpretedFunctions != 0 {
			return fmt.Errorf("Benchmark contains uninterpreted function symbols, but specified logic does not support them.")
		}
	}
	smt := k.smtParams.state
	arithmetic := k.smtParams.arith.state
	preprocess := k.smtParams.preprocess.state
	smt.RelevancyLvl = 0
	arithmetic.Eq2Ineq = true
	arithmetic.Reflect = false
	arithmetic.PropagateEqs = false
	arithmetic.SmallLemmaSize = 30
	preprocess.NNFCNF = false
	if features != nil {
		dense := smtDenseArithmetic(features)
		if features.NumUninterpretedConstants > 5000 {
			smt.RelevancyLvl = 2
		} else if features.CNF && !dense {
			smt.PhaseSelection = 4 // PS_CACHING_CONSERVATIVE2
		} else {
			smt.PhaseSelection = 2 // PS_CACHING
		}
		if dense && features.NumBinClauses+features.NumUnits == features.NumClauses {
			smt.RestartAdaptive = false
			smt.RestartStrategy = 0 // RS_GEOMETRIC
		}
		if features.CNF && features.NumUnits == features.NumClauses {
			smt.RandomInitialActivity = 2 // IA_RANDOM
		}
		proofs, err := k.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs && !k.copiedArithmeticPlugin {
			family, err := k.manager.ArithmeticFamilyID()
			if err != nil {
				return err
			}
			if theory, ok := k.internalizer.theories[family].(*SMTLinearArithmeticTheory); ok {
				// Both concrete specializations share this implementation.
				// Select per-column integrality before any asserted term is internalized.
				theory.integer, theory.mixed = false, true
			}
		}
		if !proofs && !arithmetic.AutoConfigSimplex && dense {
			smt.PhaseSelection = 3 // PS_CACHING_CONSERVATIVE
			if err := k.installDenseSetup(arithmetic); err != nil {
				return err
			}
		}
	}
	return k.applyArithmeticSetup(smt, arithmetic, preprocess)
}

// The source creates this plugin at the first setup boundary. Replace only
// the unused constructor-time arithmetic specialization, before any theory term
// or logical scope can refer to it, preserving the registry order.
func (k *SMTKernel) installDenseSetup(config SMTArithState) error {
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	theory, err := NewSMTDenseDiffTheory(family)
	if err != nil {
		return err
	}
	if err := theory.ConfigureArithmetic(config); err != nil {
		return err
	}
	return k.replaceUnusedSetupArithmetic(theory)
}

func (k *SMTKernel) replaceUnusedSetupArithmetic(theory SMTTheoryInternalizer) error {
	family, err := k.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	if theory == nil || theory.FamilyID() != family {
		return ast.ErrASTDomain
	}
	s := k.internalizer
	old := s.theories[family]
	if k.copiedArithmeticPlugin && old != nil {
		// Source register_plugin preserves an already copied plugin and
		// destroys the unused candidate produced by destination setup.
		if closer, ok := theory.(interface{ Close() error }); ok {
			return closer.Close()
		}
		return nil
	}
	if old == nil {
		return k.RegisterTheory(theory)
	}
	unused := false
	switch previous := old.(type) {
	case *SMTLinearArithmeticTheory:
		unused = len(previous.variables) == 0
	case *SMTDummyTheory:
		unused = len(previous.variables) == 0 && !previous.theoryExprs
	}
	if !unused || len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	position := -1
	for i, theory := range s.theoryOrder {
		if theory == old {
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
	if closer, ok := old.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	s.theories[family] = theory
	s.theoryOrder[position] = theory
	return nil
}

func (k *SMTKernel) setupRealDifferencePolicy(useStaticFeatures bool) (result error) {
	if k.setupLogic != "QF_RDL" {
		return nil
	}
	features, err := k.setupStaticFeatures(useStaticFeatures)
	if err != nil {
		return err
	}
	if features != nil {
		defer func() { result = errors.Join(result, features.Close()) }()
		if features.NumArithEqualities != features.NumDiffEqualities || features.NumArithTerms != features.NumDiffTerms || features.NumArithInequalities != features.NumDiffInequalities {
			return fmt.Errorf("Benchmark is not in QF_RDL (real difference logic).")
		}
		if features.HasInt {
			return fmt.Errorf("Benchmark has integer variables but it is marked as QF_RDL (real difference logic).")
		}
		if features.NumUninterpretedFunctions != 0 {
			return fmt.Errorf("Benchmark contains uninterpreted function symbols, but specified logic does not support them.")
		}
	}
	smt, arithmetic, preprocess := k.smtParams.state, k.smtParams.arith.state, k.smtParams.preprocess.state
	smt.RelevancyLvl = 0
	arithmetic.Eq2Ineq = true
	arithmetic.Reflect = false
	arithmetic.PropagateEqs = false
	preprocess.NNFCNF = false
	if features == nil {
		// The logic-only overload delegates to setup_mi_arith. The static
		// overload below selects its backend directly and deliberately does not.
		if err := k.installMixedArithmeticSetup(arithmetic); err != nil {
			return err
		}
	}
	if features != nil {
		dense := smtDenseArithmetic(features)
		if dense {
			smt.RestartStrategy = 0 // RS_GEOMETRIC
			smt.RestartAdaptive = false
			smt.PhaseSelection = 2 // PS_CACHING
		}
		proofs, err := k.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if !proofs && !arithmetic.AutoConfigSimplex && dense {
			if err := k.installDenseSetup(arithmetic); err != nil {
				return err
			}
		} else if !proofs && !arithmetic.AutoConfigSimplex && features.NumUninterpretedConstants <= 4*features.NumBoolConstants && features.NumITETerms == 0 {
			arithmetic.BoundProp = 0           // BP_NONE
			arithmetic.PropagationStrategy = 0 // ARITH_PROP_AGILITY
			arithmetic.AddBinaryBounds = true
			family, err := k.manager.ArithmeticFamilyID()
			if err != nil {
				return err
			}
			theory, err := NewSMTRealDiffTheory(family)
			if err != nil {
				return err
			}
			if err := theory.ConfigureArithmetic(arithmetic); err != nil {
				return err
			}
			if err := k.replaceUnusedSetupArithmetic(theory); err != nil {
				return err
			}
		}
	}
	return k.applyArithmeticSetup(smt, arithmetic, preprocess)
}

// Unlike IDL and RDL, the static UFIDL source overload only rejects real
// arithmetic; it does not validate the difference-term counters.
func (k *SMTKernel) setupUFIntegerDifferencePolicy(useStaticFeatures bool) (result error) {
	if k.setupLogic != "QF_UFIDL" {
		return nil
	}
	features, err := k.setupStaticFeatures(useStaticFeatures)
	if err != nil {
		return err
	}
	if features != nil {
		defer func() { result = errors.Join(result, features.Close()) }()
		if features.HasReal {
			return fmt.Errorf("Benchmark has real variables but it is marked as QF_UFIDL (uninterpreted functions and difference logic).")
		}
	}
	smt, arithmetic, preprocess := k.smtParams.state, k.smtParams.arith.state, k.smtParams.preprocess.state
	smt.RelevancyLvl = 0
	arithmetic.Reflect = false
	preprocess.NNFCNF = false
	dense := false
	if features == nil {
		arithmetic.Eq2Ineq = true
	} else {
		if features.NumUninterpretedFunctions == 0 {
			arithmetic.Eq2Ineq = true
			arithmetic.PropagateEqs = false
			dense = smtDenseArithmetic(features)
		}
		proofs, err := k.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs && !k.copiedArithmeticPlugin {
			family, err := k.manager.ArithmeticFamilyID()
			if err != nil {
				return err
			}
			if theory, ok := k.internalizer.theories[family].(*SMTLinearArithmeticTheory); ok {
				theory.integer, theory.mixed = false, true
			}
		}
		if dense {
			arithmetic.SmallLemmaSize = 128
			smt.LemmaGcHalf = true
			smt.RestartStrategy = 0 // RS_GEOMETRIC
			if !proofs {
				if err := k.installDenseSetup(arithmetic); err != nil {
					return err
				}
			}
		}
	}
	if !dense {
		arithmetic.EqBounds = true
		smt.PhaseSelection = 0  // PS_ALWAYS_FALSE
		smt.RestartStrategy = 0 // RS_GEOMETRIC
		smt.RestartFactor = 1.5
		smt.RestartAdaptive = false
	}
	return k.applyArithmeticSetup(smt, arithmetic, preprocess)
}
