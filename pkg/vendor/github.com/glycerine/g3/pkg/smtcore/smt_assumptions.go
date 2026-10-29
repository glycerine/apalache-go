package smtcore

// Source: src/smt/smt_context.cpp validate_assumptions(),
// init_assumptions(), and reset_assumptions().

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (k *SMTKernel) validAssumption(assumption ast.ASTHandle) (bool, error) {
	sort, err := k.manager.GetSort(assumption)
	if err != nil {
		return false, err
	}
	boolean, err := k.manager.MkBoolSort()
	if err != nil || sort != boolean {
		return false, err
	}
	kind, err := k.manager.GetKind(assumption)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	count, err := k.manager.GetNumArgs(assumption)
	if err != nil || count == 0 {
		return count == 0, err
	}
	op, basic, err := k.internalizer.basicOperation(assumption)
	if err != nil || !basic || op != ast.ASTOpNot || count != 1 {
		return false, err
	}
	argument, err := k.manager.GetArg(assumption, 0)
	if err != nil {
		return false, err
	}
	argumentKind, err := k.manager.GetKind(argument)
	if err != nil || argumentKind != ast.ASTApp {
		return false, err
	}
	argumentCount, err := k.manager.GetNumArgs(argument)
	return argumentCount == 0, err
}

// CheckAssumptions checks under temporary propositional literals. The
// assumption scope and every variable introduced in it are removed before the
// method returns.
func (k *SMTKernel) CheckAssumptions(assumptions []ast.ASTHandle) (result lbool.LBool, resultErr error) {
	ready, err := k.beginCheck()
	if err != nil || !ready {
		return lbool.Undef, err
	}
	if k.internalizer.Inconsistent() {
		return lbool.False, nil
	}
	if err := k.configureCheck(false); err != nil {
		return lbool.Undef, err
	}
	all, refs, err := k.addTheoryAssumptions(assumptions)
	defer func() {
		resultErr = errors.Join(resultErr, k.releaseTheoryAssumptions(refs))
		if resultErr != nil {
			result = lbool.Undef
		}
	}()
	if err != nil {
		return lbool.Undef, err
	}
	return k.checkConfiguredAssumptions(all)
}

func (k *SMTKernel) checkConfiguredAssumptions(assumptions []ast.ASTHandle) (result lbool.LBool, resultErr error) {
	for index, assumption := range assumptions {
		valid, err := k.validAssumption(assumption)
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("assumption %d: %w", index, ast.ErrASTDomain)
		}
	}
	if len(assumptions) == 0 {
		if result, handled, err := k.tryNLSAT(); handled || err != nil {
			return k.finalizeCheck(result, err)
		}
	}
	if err := k.internalizeCheckAssertions(); err != nil {
		return lbool.Undef, err
	}
	if k.assertedFormulasInconsistent() {
		return lbool.False, nil
	}
	// The native facade also propagates an empty assumption batch here. Its
	// search-persistence boundary must capture those base theory consequences.
	if !k.internalizer.Inconsistent() {
		if _, err := k.internalizer.PropagateCore(); err != nil {
			return lbool.Undef, err
		}
	}
	if k.internalizer.Inconsistent() {
		return k.resolveCheckConflict()
	}
	if len(assumptions) == 0 {
		return k.finalizeCheck(k.search())
	}
	if canceled, err := k.manager.Canceled(); err != nil {
		return lbool.Undef, err
	} else if canceled {
		return k.finalizeCheck(k.search())
	}
	if err := k.internalizer.PushScope(); err != nil {
		return lbool.Undef, err
	}
	assumptionVariables := make([]int32, 0, len(assumptions))
	k.assumptions = make(map[SMTLiteral]ast.ASTHandle, len(assumptions))
	defer func() {
		for _, variable := range assumptionVariables {
			if variable >= SMTFirstBoolVar && int(variable) < len(k.internalizer.booleanData) {
				k.internalizer.booleanData[variable].Assumption = false
			}
		}
		resultErr = errors.Join(resultErr, k.internalizer.PopScopes(1))
		k.assumptions = nil
		// Native tracking_assumptions tests the assumption search scope,
		// including a nonempty input containing only true constants.
		if resultErr == nil && result == lbool.False {
			resultErr = k.validateUnsatCore()
			if resultErr == nil {
				for _, theory := range k.internalizer.theoryOrder {
					status, err := theory.ValidateUnsatCore(k.unsatCore)
					if err != nil {
						resultErr = err
						break
					}
					if status == lbool.Undef {
						result = lbool.Undef
						break
					}
				}
			}
		}
		if resultErr != nil {
			result = lbool.Undef
		}
	}()
	for index, assumption := range assumptions {
		if k.manager.IsTrue(assumption) {
			continue
		}
		proof := ast.ASTHandle{}
		proofs, err := k.manager.ProofsEnabled()
		if err != nil {
			return lbool.Undef, err
		}
		if proofs {
			proof, err = k.manager.MkProofAsserted(assumption)
			if err != nil {
				return lbool.Undef, err
			}
		}
		if err = k.internalizer.Assert(assumption, proof, 0); err != nil {
			return lbool.Undef, fmt.Errorf("internalize assumption %d: %w", index, err)
		}
		literal, err := k.internalizer.Literal(assumption)
		if err != nil {
			return lbool.Undef, err
		}
		if literal.Var() >= SMTFirstBoolVar {
			k.internalizer.booleanData[literal.Var()].Assumption = true
			assumptionVariables = append(assumptionVariables, literal.Var())
			k.assumptions[literal] = assumption
		}
	}
	if k.internalizer.Inconsistent() {
		return k.resolveCheckConflict()
	}
	// Finalize before the native facade removes its assumption scope; source
	// check_finalize runs while that successful search scope is still alive.
	return k.finalizeCheck(k.search())
}

func (k *SMTKernel) clearUnsatCore() error {
	if k == nil || k.manager == nil {
		return ast.ErrASTHandle
	}
	var result error
	for _, expression := range k.unsatCore {
		result = errors.Join(result, k.manager.DecRef(expression))
	}
	k.unsatCore = nil
	return result
}

func (k *SMTKernel) captureUnsatCore(resolver *SMTConflictResolver, conflict SMTBooleanJustification, notLiteral SMTLiteral) error {
	if len(k.assumptions) == 0 {
		return nil
	}
	literals, err := resolver.UnsatCore(conflict, notLiteral)
	if err != nil {
		return err
	}
	type coreEntry struct {
		expression ast.ASTHandle
		id         uint32
	}
	seen := make(map[ast.ASTHandle]struct{}, len(literals))
	entries := make([]coreEntry, 0, len(literals))
	for _, literal := range literals {
		expression, found := k.assumptions[literal]
		if !found {
			expression, found = k.assumptions[literal.Negated()]
		}
		if !found {
			return ast.ErrASTDomain
		}
		if _, duplicate := seen[expression]; duplicate {
			continue
		}
		id, idErr := k.manager.GetASTID(expression)
		if idErr != nil {
			return idErr
		}
		seen[expression] = struct{}{}
		entries = append(entries, coreEntry{expression: expression, id: id})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	for _, entry := range entries {
		if err = k.manager.IncRef(entry.expression); err != nil {
			return errors.Join(err, k.clearUnsatCore())
		}
		k.unsatCore = append(k.unsatCore, entry.expression)
	}
	return nil
}

func (k *SMTKernel) UnsatCore() []ast.ASTHandle {
	if k == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), k.unsatCore...)
}
