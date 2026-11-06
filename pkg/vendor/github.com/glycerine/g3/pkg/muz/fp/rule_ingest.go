package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// addFormula applies mk_horn_rule's option gate before unbound-variable
// reduction. Source dl_rule.cpp:218 skips the entire rewrite when disabled.
func (e *PropositionalEngine) addFormula(set *base.RuleSet, formula ast.ASTHandle, name symbol.Symbol) error {
	return e.addFormulaWithProof(set, formula, ast.ASTHandle{}, name)
}

// addFormulaWithProof carries the source asserted proof through Horn
// conversion while retaining the same unbound-variable policy.
func (e *PropositionalEngine) addFormulaWithProof(set *base.RuleSet, formula, proof ast.ASTHandle, name symbol.Symbol) error {
	if e.params == nil || e.globals == nil {
		return e.rules.AddFormulaWithProofAndSymbol(set, formula, proof, name)
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	fixUnbound, getErr := parameters.Bool("xform.fix_unbound_vars", false)
	err = errors.Join(getErr, parameters.Close())
	if err != nil {
		return err
	}
	if !fixUnbound {
		return e.rules.AddFormulaWithProofAndSymbol(set, formula, proof, name)
	}
	// A failed QE call may leave mutable phase state unsuitable for reuse even
	// after its pending native owners have been cleaned up. Close it on the
	// next enabled conversion, keeping the owner available if cleanup fails.
	if e.unboundLiteDirty {
		if e.unboundLite != nil {
			if err = e.unboundLite.Close(); err != nil {
				return err
			}
			e.unboundLite = nil
		}
		if e.unboundCooperation != nil {
			if err = e.unboundCooperation.Close(); err != nil {
				return err
			}
			e.unboundCooperation = nil
		}
		e.unboundLiteDirty = false
	}
	if e.unboundLite == nil {
		cooperation, createErr := cooperate.NewCooperation(e.manager.Budget())
		if createErr != nil {
			return createErr
		}
		lite, createErr := qe.NewLite(e.manager, nil, false, qe.Config{Context: e.params, Globals: e.globals, Cooperation: cooperation})
		if createErr != nil {
			if lite != nil {
				createErr = errors.Join(createErr, lite.Close())
			}
			return errors.Join(createErr, cooperation.Close())
		}
		e.unboundCooperation = cooperation
		e.unboundLite = lite
	}
	err = e.rules.AddFormulaWithProofAndSymbolAndUnboundFix(set, formula, proof, name, e.unboundLite, true)
	if err != nil {
		e.unboundLiteDirty = true
	}
	return err
}
