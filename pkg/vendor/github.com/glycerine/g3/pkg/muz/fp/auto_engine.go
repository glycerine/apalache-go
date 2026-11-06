package fp

// Source: src/muz/base/dl_context.cpp, engine_type_proc and configure_engine.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) autoRuleKind(rule *base.Rule, arithmetic, datatype ast.FamilyID, boolean ast.ASTHandle) (EngineKind, error) {
	head, err := rule.Head()
	if err != nil {
		return EngineDatalog, err
	}
	count, err := rule.TailSize()
	if err != nil {
		return EngineDatalog, err
	}
	roots := []ast.ASTHandle{head}
	for i := 0; i < count; i++ {
		tail, _, cause := rule.Tail(i)
		if cause != nil {
			return EngineDatalog, cause
		}
		roots = append(roots, tail)
	}
	return e.autoRootsKind(roots, arithmetic, datatype, boolean)
}

// autoRootsKind examines the complete rule before returning: a quantifier in
// any part takes precedence over arithmetic seen earlier in that rule.
func (e *PropositionalEngine) autoRootsKind(roots []ast.ASTHandle, arithmetic, datatype ast.FamilyID, boolean ast.ASTHandle) (EngineKind, error) {
	kind := EngineDatalog
	seen := make(map[ast.ASTHandle]bool)
	for len(roots) != 0 {
		last := len(roots) - 1
		expr := roots[last]
		roots = roots[:last]
		if seen[expr] {
			continue
		}
		seen[expr] = true
		node, cause := e.manager.GetKind(expr)
		if cause != nil {
			return EngineDatalog, cause
		}
		if node == ast.ASTQuantifier {
			return EngineQPDR, nil
		}
		sort, cause := e.manager.GetSort(expr)
		if cause != nil {
			return EngineDatalog, cause
		}
		family, cause := e.manager.GetFamilyID(sort)
		if cause != nil {
			return EngineDatalog, cause
		}
		if family == arithmetic || (datatype != ast.NullFamilyID && family == datatype) || (node == ast.ASTVar && sort == boolean) {
			kind = EnginePDR
		}
		if node == ast.ASTApp {
			n, cause := e.manager.GetNumArgs(expr)
			if cause != nil {
				return EngineDatalog, cause
			}
			for i := uint32(0); i < n; i++ {
				arg, x := e.manager.GetArg(expr, i)
				if x != nil {
					return EngineDatalog, x
				}
				roots = append(roots, arg)
			}
		}
	}
	return kind, nil
}

func (e *PropositionalEngine) autoFamilies() (arithmetic, datatype ast.FamilyID, boolean ast.ASTHandle, err error) {
	arithmetic, err = e.manager.ArithmeticFamilyID()
	if err != nil {
		return
	}
	datatype, err = e.manager.DatatypeFamilyID()
	if errors.Is(err, ast.ErrASTDomain) {
		datatype, err = ast.NullFamilyID, nil
	}
	if err != nil {
		return
	}
	boolean, err = e.manager.MkBoolSort()
	return
}

func (e *PropositionalEngine) autoFormulaKind(formula ast.ASTHandle) (EngineKind, error) {
	// Rule-level quantifiers bind variables and are stripped by the source.
	for {
		kind, err := e.manager.GetKind(formula)
		if err != nil {
			return EngineDatalog, err
		}
		if kind != ast.ASTQuantifier {
			break
		}
		formula, err = e.manager.GetQuantifierBody(formula)
		if err != nil {
			return EngineDatalog, err
		}
	}
	arithmetic, datatype, boolean, err := e.autoFamilies()
	if err != nil {
		return EngineDatalog, err
	}
	return e.autoRootsKind([]ast.ASTHandle{formula}, arithmetic, datatype, boolean)
}

func (e *PropositionalEngine) autoEngineKind() (EngineKind, error) {
	arithmetic, datatype, boolean, err := e.autoFamilies()
	if err != nil {
		return EngineDatalog, err
	}
	count, err := e.set.Len()
	if err != nil {
		return EngineDatalog, err
	}
	// Eager normalization also stores pending inputs in e.set. Source scans
	// only its normalized prefix first, then the still-pending raw archive.
	pending := make([]bool, count)
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		if raw.first < 0 || raw.end > count || raw.first > raw.end {
			return EngineDatalog, ErrPropositionalEngine
		}
		for i := raw.first; i < raw.end; i++ {
			pending[i] = true
		}
	}
	for i := 0; i < count; i++ {
		if pending[i] {
			continue
		}
		rule, cause := e.set.Rule(i)
		if cause != nil {
			return EngineDatalog, cause
		}
		kind, cause := e.autoRuleKind(rule, arithmetic, datatype, boolean)
		cause = errors.Join(cause, rule.Close())
		if cause != nil || kind != EngineDatalog {
			return kind, cause
		}
	}
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		formula, cause := e.manager.RefGet(raw.formula)
		if cause != nil {
			return EngineDatalog, cause
		}
		kind, cause := e.autoFormulaKind(formula)
		if cause != nil || kind != EngineDatalog {
			return kind, cause
		}
	}
	return EngineDatalog, nil
}
