package smtcore

// Source: src/smt/smt_setup.h and src/smt/smt_setup.cpp retained explicit
// logic routing for theories whose concrete Go solvers are not yet available.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// smtLogicUsesUnknownSetup mirrors setup::setup_default's fallthrough to
// setup_unknown. QF_FD is a retained Go specialization whose concrete finite
// domain theories are selected below.
func smtLogicUsesUnknownSetup(logic string) bool {
	switch logic {
	case "QF_UF", "QF_RDL", "QF_IDL", "QF_UFIDL", "QF_LRA", "QF_LIA", "QF_UFLIA", "QF_UFLRA",
		"QF_AX", "QF_AUFLIA", "QF_BV", "QF_AUFBV", "QF_ABV", "QF_UFBV", "QF_BVRE",
		"AUFLIA", "AUFLIRA", "AUFNIRA", "AUFLIA+", "AUFLIA-", "AUFLIRA+", "AUFLIRA-",
		"AUFNIRA+", "AUFNIRA-", "UFNIA", "UFLRA", "LRA", "QF_FP", "QF_FPBV", "QF_BVFP",
		"QF_S", "QF_DT", "QF_FD":
		return false
	default:
		return true
	}
}

func smtLogicUsesArithmetic(logic string) bool {
	if smtLogicUsesUnknownSetup(logic) {
		return true
	}
	switch logic {
	case "QF_RDL", "QF_IDL", "QF_UFIDL", "QF_LRA", "QF_LIA", "QF_UFLIA", "QF_UFLRA",
		"QF_AUFLIA", "AUFLIA", "AUFLIRA", "AUFNIRA", "AUFLIA+", "AUFLIA-", "AUFLIRA+",
		"AUFLIRA-", "AUFNIRA+", "AUFNIRA-", "UFNIA", "UFLRA", "LRA", "QF_BVRE", "QF_S", "ALL":
		return true
	default:
		return false
	}
}

func smtLogicUsesArrays(logic string) bool {
	if smtLogicUsesUnknownSetup(logic) {
		return true
	}
	switch logic {
	case "QF_AX", "QF_AUFBV", "QF_ABV", "QF_UFBV", "QF_AUFLIA", "AUFLIA", "AUFLIRA", "AUFNIRA", "UFLRA", "UFNIA",
		"AUFLIA+", "AUFLIA-", "AUFLIRA+", "AUFLIRA-", "AUFNIRA+", "AUFNIRA-":
		return true
	default:
		return false
	}
}

func smtLogicUsesBitVectors(logic string) bool {
	if smtLogicUsesUnknownSetup(logic) {
		return true
	}
	switch logic {
	case "QF_BV", "QF_UFBV", "QF_AUFBV", "QF_ABV", "QF_BVRE", "QF_FP", "QF_FPBV", "QF_BVFP", "QF_FD", "ALL":
		return true
	default:
		return false
	}
}

func smtLogicUsesDatatypes(logic string) bool {
	if smtLogicUsesUnknownSetup(logic) {
		return true
	}
	switch logic {
	case "QF_DT", "QF_FD", "ALL":
		return true
	default:
		return false
	}
}

func smtLogicUsesPseudoBooleans(logic string) bool {
	return logic == "QF_FD" || smtLogicUsesUnknownSetup(logic)
}

func smtLogicUsesDatalog(logic string) bool {
	return logic == "QF_FD" || smtLogicUsesUnknownSetup(logic)
}

func smtLogicUsesFloatingPoint(logic string) bool {
	return logic == "QF_FP" || logic == "QF_FPBV" || logic == "QF_BVFP" || smtLogicUsesUnknownSetup(logic)
}

func smtLogicUsesSequences(logic string) bool {
	return logic == "QF_BVRE" || logic == "QF_S" || smtLogicUsesUnknownSetup(logic)
}

// setup_unknown(st) may dispatch to a specialized arithmetic/array/BV
// profile before reaching setup_unknown()/setup_seq_str. Keep that decision
// separate from the generic string_solver parameter.
func smtUnknownSetupUsesSequences(f *ast.StaticFeatures) bool {
	if f.NumQuantifiers != 0 {
		return true
	}
	if f.NumNonUFTheories() == 0 {
		return false
	}
	n := f.NumTheories()
	difference := (f.NumDiffTerms != 0 || f.NumDiffEqualities != 0 || f.NumDiffInequalities != 0) && f.NumArithEqualities == f.NumDiffEqualities && f.NumArithTerms == f.NumDiffTerms && f.NumArithInequalities == f.NumDiffInequalities
	arithmetic := f.NumArithEqualities != 0 || f.NumArithTerms != 0 || f.NumArithInequalities != 0
	if n == 1 && difference {
		return f.HasInt == f.HasReal
	}
	if n == 2 && f.HasUF() && difference {
		return f.HasReal || !f.HasInt
	}
	if n == 1 && arithmetic {
		return false
	}
	if n == 2 && f.HasUF() && arithmetic {
		return f.HasReal && (f.HasInt || f.NumNonLinear != 0)
	}
	if n == 1 && (f.HasBitVector || f.HasFloatingPoint || f.HasArrays) || n == 2 && f.HasFloatingPoint && f.HasBitVector {
		return false
	}
	if n == 2 && f.HasUF() && f.HasArrays && (f.HasBitVector || f.HasInt) {
		return false
	}
	return true
}

// Native plugin selection happens at the first push/check, after parameter
// updates and asserted-formula collection. Constructor plugins are only Go
// placeholders until this boundary.
func (k *SMTKernel) setupSequenceBackend(useStaticFeatures bool) (result error) {
	legacy := k.smtState.AutoConfig && k.setupLogic == "QF_S"
	if !legacy && k.smtState.AutoConfig && k.setupLogic != "QF_BVRE" && !smtLogicUsesUnknownSetup(k.setupLogic) {
		return nil
	}
	if !legacy && !(k.smtState.AutoConfig && k.setupLogic == "QF_BVRE") {
		features, err := k.setupStaticFeatures(true)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, features.Close()) }()
		if k.smtState.AutoConfig && useStaticFeatures && !smtUnknownSetupUsesSequences(features) {
			return nil
		}
		choice, _, err := k.manager.Symbols.BareString(k.smtState.StringSolver)
		if err != nil {
			return err
		}
		switch choice {
		case "seq":
		case "z3str3":
			legacy = true
		case "auto":
			legacy = !features.HasNonStringSequence
		default:
			return fmt.Errorf("invalid parameter for smt.string_solver, valid options are 'z3str3', 'seq', 'auto'")
		}
	}
	family, err := k.manager.RegisterSequencePlugin()
	if err != nil {
		return err
	}
	s := k.internalizer
	old := s.theories[family]
	// context::register_plugin preserves an explicitly installed/copied
	// plugin. Only the eager Go constructor placeholder may be replaced.
	if old != nil && old != k.sequenceSetupPlaceholder {
		return nil
	}
	if legacy {
		if _, ok := old.(*SMTStringTheory); ok {
			return nil
		}
	} else if _, ok := old.(*SMTSequenceTheory); ok {
		return nil
	}
	return k.installFreshSequencePlugin(legacy, &k.smtParams.str, false)
}

func smtInstallRetainedSetup(kernel *SMTKernel, manager *ast.ASTManager, logic string) error {
	if kernel == nil || manager == nil {
		return ast.ErrASTHandle
	}
	kernel.setupLogic = logic
	if logic == "QF_UFLIA" && kernel.smtParams != nil && kernel.smtState.AutoConfig {
		// Source setup_QF_UFLIA mutates the owning parameter object before
		// constructing integer arithmetic. Its threshold is an internal
		// setup field, so later updt_params calls must retain it.
		kernel.smtParams.state.RelevancyLvl = 0
		kernel.smtParams.arith.state.Reflect = false
		kernel.smtParams.arith.state.PropagationThreshold = 1000
		kernel.smtParams.preprocess.state.NNFCNF = false
		kernel.smtState.RelevancyLvl = 0
		kernel.arithState.Reflect = false
		kernel.arithState.PropagationThreshold = 1000
		kernel.preprocessState.NNFCNF = false
		kernel.relevancy.enabled = false
		kernel.internalizer.relevancyLevel = 0
	}
	install := func(family ast.FamilyID, name string) error {
		dummy, err := NewSMTDummyTheory(family, name)
		if err != nil {
			return err
		}
		return kernel.RegisterTheory(dummy)
	}
	if smtLogicUsesArithmetic(logic) {
		family, err := manager.ArithmeticFamilyID()
		if err != nil {
			return err
		}
		if logic == "QF_S" || logic == "QF_RDL" && kernel.smtState.AutoConfig {
			theory, theoryErr := NewSMTMixedLinearArithmeticTheory(family)
			if theoryErr != nil {
				return theoryErr
			}
			err = kernel.RegisterTheory(theory)
		} else if (logic == "QF_IDL" || logic == "QF_UFIDL") && kernel.smtState.AutoConfig {
			theory, theoryErr := NewSMTLinearIntegerArithmeticTheory(family)
			if theoryErr != nil {
				return theoryErr
			}
			err = kernel.RegisterTheory(theory)
		} else if logic == "QF_LRA" || logic == "QF_UFLRA" {
			theory, theoryErr := NewSMTLinearArithmeticTheory(family)
			if theoryErr != nil {
				return theoryErr
			}
			err = kernel.RegisterTheory(theory)
		} else if logic == "QF_LIA" || logic == "QF_UFLIA" || logic == "QF_AUFLIA" || logic == "AUFLIA" ||
			logic == "AUFLIA+" || logic == "AUFLIA-" || logic == "UFNIA" || logic == "QF_BVRE" || logic == "QF_S" {
			theory, theoryErr := NewSMTLinearIntegerArithmeticTheory(family)
			if theoryErr != nil {
				return theoryErr
			}
			err = kernel.RegisterTheory(theory)
		} else {
			if kernel.arithState.Mode == 0 {
				err = install(family, "no arithmetic")
			} else {
				theory, theoryErr := NewSMTMixedLinearArithmeticTheory(family)
				if theoryErr != nil {
					return theoryErr
				}
				err = kernel.RegisterTheory(theory)
			}
		}
		if err != nil {
			return err
		}
	}
	if smtLogicUsesArrays(logic) {
		family, err := manager.ArrayFamilyID()
		if err != nil {
			return err
		}
		theory, theoryErr := NewSMTArrayTheory(family)
		if theoryErr != nil {
			return theoryErr
		}
		if err = kernel.RegisterTheory(theory); err != nil {
			return err
		}
	}
	if smtLogicUsesBitVectors(logic) {
		name, err := manager.Symbols.Intern("bv")
		if err != nil {
			return err
		}
		family, err := manager.FamilyIDForSymbol(name)
		if err != nil {
			return err
		}
		theory, theoryErr := NewSMTBitVectorTheory(family)
		if theoryErr != nil {
			return theoryErr
		}
		if err = kernel.RegisterTheory(theory); err != nil {
			return err
		}
	}
	if smtLogicUsesFloatingPoint(logic) {
		if _, err := manager.RegisterArithmeticPlugin(); err != nil {
			return err
		}
		if _, err := manager.RegisterBitVectorPlugin(); err != nil {
			return err
		}
		family, err := manager.RegisterFloatingPointPlugin()
		if err != nil {
			return err
		}
		theory, theoryErr := NewSMTFloatingPointTheory(family)
		if theoryErr != nil {
			return theoryErr
		}
		if err = kernel.RegisterTheory(theory); err != nil {
			return err
		}
	}
	if smtLogicUsesSequences(logic) {
		if _, err := manager.RegisterArithmeticPlugin(); err != nil {
			return err
		}
		if _, err := manager.RegisterBitVectorPlugin(); err != nil {
			return err
		}
		if _, err := manager.RegisterArrayPlugin(); err != nil {
			return err
		}
		family, err := manager.RegisterSequencePlugin()
		if err != nil {
			return err
		}
		if logic == "QF_S" && kernel.smtState.AutoConfig {
			// Native setup_QF_S installs theory_str unconditionally, even
			// when the generic string_solver setting requests seq/auto.
			// Disabled auto_config uses CFG_BASIC's generic sequence path.
			if kernel.smtParams == nil {
				return ast.ErrASTDomain
			}
			theory, theoryErr := NewSMTStringTheory(family, &kernel.smtParams.str)
			if theoryErr != nil {
				return theoryErr
			}
			if err = kernel.RegisterTheory(theory); err != nil {
				return err
			}
		} else {
			theory, theoryErr := NewSMTSequenceTheory(family)
			if theoryErr != nil {
				return theoryErr
			}
			if err = kernel.RegisterTheory(theory); err != nil {
				return err
			}
		}
		kernel.sequenceSetupPlaceholder = kernel.internalizer.theories[family]
	}
	if smtLogicUsesDatatypes(logic) {
		family, err := manager.RegisterDatatypePlugin()
		if err != nil {
			return err
		}
		theory, theoryErr := NewSMTDatatypeTheory(family)
		if theoryErr != nil {
			return theoryErr
		}
		if err = kernel.RegisterTheory(theory); err != nil {
			return err
		}
	}
	if smtLogicUsesPseudoBooleans(logic) {
		family, err := manager.RegisterPseudoBooleanPlugin()
		if err != nil {
			return err
		}
		theory, theoryErr := NewSMTPseudoBooleanTheory(family)
		if theoryErr != nil {
			return theoryErr
		}
		if err = kernel.RegisterTheory(theory); err != nil {
			return err
		}
	}
	if smtLogicUsesDatalog(logic) {
		family, err := manager.RegisterDatalogPlugin()
		if err != nil {
			return err
		}
		theory, theoryErr := NewSMTDatalogTheory(family)
		if theoryErr != nil {
			return theoryErr
		}
		if err = kernel.RegisterTheory(theory); err != nil {
			return err
		}
	}
	return nil
}
