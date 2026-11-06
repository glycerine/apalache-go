package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// proveQuantifiedHornLiteral derives the abstraction wrapper
// (= (forall (...) P(...)) true) from one transformed universally quantified
// Horn definition. It never invents an assertion: the hyper-resolution parent
// is the exact transformed source clause and all residual theory antecedents
// remain independently checked side conditions.
func (g *RPFP) proveQuantifiedHornLiteral(factory solver.Factory, source *RPFP, current int, target ast.ASTHandle, refs *[]ast.ASTRef) (ast.ASTRef, error) {
	m := g.context.manager
	family, err := m.GetFamilyID(target)
	if err != nil {
		return ast.ASTRef{}, err
	}
	op, err := m.GetDeclKind(target)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if family != m.BasicFamilyID() || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
		return ast.ASTRef{}, nil
	}
	left, err := m.GetArg(target, 0)
	if err != nil {
		return ast.ASTRef{}, err
	}
	right, err := m.GetArg(target, 1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	// QuantifierAbstraction emits the exact (forall ...) = true orientation.
	// Keep this helper deliberately narrower than Boolean commutativity: the
	// proof produced below has precisely that fact.
	if m.IsTrue(left) || !m.IsTrue(right) {
		return ast.ASTRef{}, nil
	}
	kind, err := m.GetKind(left)
	if err != nil || kind != ast.ASTQuantifier {
		return ast.ASTRef{}, err
	}
	forall, err := m.IsForall(left)
	if err != nil || !forall {
		return ast.ASTRef{}, err
	}
	targetBody, err := m.GetQuantifierBody(left)
	if err != nil {
		return ast.ASTRef{}, err
	}
	targetDecl, err := m.GetDecl(targetBody)
	if err != nil {
		return ast.ASTRef{}, err
	}
	for i, ref := range source.sourceClauses {
		if i == current {
			continue
		}
		formula, err := m.RefGet(ref)
		if err != nil {
			return ast.ASTRef{}, err
		}
		bodyFormula := formula
		formulaKind, err := m.GetKind(formula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if formulaKind == ast.ASTQuantifier {
			isForall, err := m.IsForall(formula)
			if err != nil || !isForall {
				continue
			}
			bodyFormula, err = m.GetQuantifierBody(formula)
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
		formulaFamily, err := m.GetFamilyID(bodyFormula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		formulaOp, err := m.GetDeclKind(bodyFormula)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if formulaFamily == m.BasicFamilyID() && formulaOp == ast.ASTOpImplies {
			head, err := m.GetArg(bodyFormula, 1)
			if err != nil {
				return ast.ASTRef{}, err
			}
			headDecl, err := m.GetDecl(head)
			if err == nil && headDecl == targetDecl && formulaKind == ast.ASTQuantifier {
				supported, err := source.structuralQuantifiedDefinition(bodyFormula)
				if err != nil {
					return ast.ASTRef{}, err
				}
				if supported {
					candidate, err := g.proveQuantifiedHornDefinition(factory, formula, left, target, targetBody, refs)
					if err != nil {
						return ast.ASTRef{}, err
					}
					if candidate != (ast.ASTRef{}) {
						return candidate, nil
					}
				}
			}
			continue
		}
	}
	return ast.ASTRef{}, nil
}

func (g *RPFP) proveQuantifiedHornDefinition(factory solver.Factory, definition, quantified, target, targetBody ast.ASTHandle, refs *[]ast.ASTRef) (out ast.ASTRef, result error) {
	m := g.context.manager
	call := &proofWitnessCall{manager: m}
	defer func() {
		if err := call.close(); err != nil {
			g.pendingProofWitnesses = append(g.pendingProofWitnesses, call)
			result = errors.Join(result, err)
		}
	}()
	definitionBody, err := m.GetQuantifierBody(definition)
	if err != nil {
		return ast.ASTRef{}, err
	}
	antecedent, err := m.GetArg(definitionBody, 0)
	if err != nil {
		return ast.ASTRef{}, err
	}
	sourceHead, err := m.GetArg(definitionBody, 1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	n, err := m.GetNumDeclarations(definition)
	if err != nil {
		return ast.ASTRef{}, err
	}
	substitution := make([]ast.ASTHandle, n)
	headArgs, err := m.GetNumArgs(sourceHead)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if targetArgs, getErr := m.GetNumArgs(targetBody); getErr != nil || targetArgs != headArgs {
		return ast.ASTRef{}, getErr
	}
	for i := uint32(0); i < headArgs; i++ {
		sourceArg, err := m.GetArg(sourceHead, i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		kind, err := m.GetKind(sourceArg)
		if err != nil || kind != ast.ASTVar {
			return ast.ASTRef{}, err
		}
		index, err := m.GetVarIndex(sourceArg)
		if err != nil || index >= n {
			return ast.ASTRef{}, err
		}
		targetArg, err := m.GetArg(targetBody, i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		sourceSort, err := m.GetSort(sourceArg)
		if err != nil {
			return ast.ASTRef{}, err
		}
		targetSort, err := m.GetSort(targetArg)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if sourceSort != targetSort {
			return ast.ASTRef{}, nil
		}
		if !substitution[index].IsNull() && substitution[index] != targetArg {
			return ast.ASTRef{}, nil
		}
		substitution[index] = targetArg
	}
	atoms := []ast.ASTHandle{antecedent}
	antecedentFamily, err := m.GetFamilyID(antecedent)
	if err != nil {
		return ast.ASTRef{}, err
	}
	antecedentOp, err := m.GetDeclKind(antecedent)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if antecedentFamily == m.BasicFamilyID() && antecedentOp == ast.ASTOpAnd {
		count, err := m.GetNumArgs(antecedent)
		if err != nil {
			return ast.ASTRef{}, err
		}
		atoms = make([]ast.ASTHandle, count)
		for i := range atoms {
			atoms[i], err = m.GetArg(antecedent, uint32(i))
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
	}
	// Once one equality or Boolean-equivalence side is determined by the head
	// substitution, match the other source term against that exact target term
	// to select still-unbound source witnesses. This is constructive
	// instantiation, not inverse
	// reasoning (for example, select is never assumed injective); the complete
	// instantiated antecedent remains an independently checked proof condition.
	var known func(ast.ASTHandle) (ast.ASTHandle, bool, error)
	known = func(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
		kind, err := m.GetKind(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if kind == ast.ASTVar {
			index, err := m.GetVarIndex(term)
			if err != nil || index >= n || substitution[index].IsNull() {
				return ast.ASTHandle{}, false, err
			}
			return substitution[index], true, nil
		}
		if kind != ast.ASTApp {
			return term, true, nil
		}
		count, err := m.GetNumArgs(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		args := make([]ast.ASTHandle, count)
		changed := false
		for i := range args {
			original, err := m.GetArg(term, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			value, complete, err := known(original)
			if err != nil || !complete {
				return ast.ASTHandle{}, false, err
			}
			args[i] = value
			changed = changed || args[i] != original
		}
		if !changed {
			return term, true, nil
		}
		decl, err := m.GetDecl(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		instantiated, err := call.pin(m.MkApp(decl, args))
		return instantiated, err == nil, err
	}
	var match func(ast.ASTHandle, ast.ASTHandle) (bool, bool, error)
	match = func(pattern, value ast.ASTHandle) (bool, bool, error) {
		kind, err := m.GetKind(pattern)
		if err != nil {
			return false, false, err
		}
		if kind == ast.ASTVar {
			index, err := m.GetVarIndex(pattern)
			if err != nil || index >= n {
				return false, false, err
			}
			if !substitution[index].IsNull() {
				return substitution[index] == value, false, nil
			}
			patternSort, err := m.GetSort(pattern)
			if err != nil {
				return false, false, err
			}
			valueSort, err := m.GetSort(value)
			if err != nil {
				return false, false, err
			}
			if patternSort != valueSort {
				return false, false, nil
			}
			substitution[index] = value
			return true, true, nil
		}
		if kind != ast.ASTApp {
			return pattern == value, false, nil
		}
		valueKind, err := m.GetKind(value)
		if err != nil || valueKind != ast.ASTApp {
			return false, false, err
		}
		patternDecl, err := m.GetDecl(pattern)
		if err != nil {
			return false, false, err
		}
		valueDecl, err := m.GetDecl(value)
		if err != nil || patternDecl != valueDecl {
			return false, false, err
		}
		count, err := m.GetNumArgs(pattern)
		if err != nil {
			return false, false, err
		}
		valueCount, err := m.GetNumArgs(value)
		if err != nil || count != valueCount {
			return false, false, err
		}
		changed := false
		for i := uint32(0); i < count; i++ {
			patternArg, err := m.GetArg(pattern, i)
			if err != nil {
				return false, false, err
			}
			valueArg, err := m.GetArg(value, i)
			if err != nil {
				return false, false, err
			}
			matched, added, err := match(patternArg, valueArg)
			if err != nil || !matched {
				return false, false, err
			}
			changed = changed || added
		}
		return true, changed, nil
	}
	tryMatch := func(pattern, value ast.ASTHandle) (bool, error) {
		before := append([]ast.ASTHandle(nil), substitution...)
		matched, changed, err := match(pattern, value)
		if err != nil || !matched {
			copy(substitution, before)
			return false, err
		}
		return changed, nil
	}
	progress := true
	for progress {
		progress = false
		for _, atom := range atoms {
			family, err := m.GetFamilyID(atom)
			if err != nil {
				return ast.ASTRef{}, err
			}
			op, err := m.GetDeclKind(atom)
			if err != nil || family != m.BasicFamilyID() || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
				continue
			}
			left, err := m.GetArg(atom, 0)
			if err != nil {
				return ast.ASTRef{}, err
			}
			right, err := m.GetArg(atom, 1)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if value, complete, err := known(left); err != nil {
				return ast.ASTRef{}, err
			} else if complete {
				added, err := tryMatch(right, value)
				if err != nil {
					return ast.ASTRef{}, err
				}
				progress = progress || added
			}
			if value, complete, err := known(right); err != nil {
				return ast.ASTRef{}, err
			} else if complete {
				added, err := tryMatch(left, value)
				if err != nil {
					return ast.ASTRef{}, err
				}
				progress = progress || added
			}
		}
	}
	for _, value := range substitution {
		if value.IsNull() {
			return ast.ASTRef{}, nil
		}
	}
	definitionProof, err := m.MkProofAsserted(definition)
	if err != nil {
		return ast.ASTRef{}, err
	}
	definitionRef, err := m.NewRef(definitionProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	call.refs = append(call.refs, definitionRef)
	quantifiedProof, err := m.MkProofHyperResolve(
		[]ast.ASTHandle{definitionProof}, quantified, nil,
		[][]ast.ASTHandle{substitution},
	)
	if err != nil {
		return ast.ASTRef{}, err
	}
	quantifiedRef, err := m.NewRef(quantifiedProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	call.refs = append(call.refs, quantifiedRef)
	wrapperProof, err := m.MkProofIffTrue(quantifiedProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	wrapperFact, err := m.GetProofFact(wrapperProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if wrapperFact != target {
		return ast.ASTRef{}, nil
	}
	wrapperRef, err := m.NewRef(wrapperProof)
	if err != nil {
		return ast.ASTRef{}, err
	}
	wrapperIndex := len(call.refs)
	call.refs = append(call.refs, wrapperRef)
	valid, err := g.validQuantifiedDefinitionProof(factory, wrapperProof, call)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !valid {
		return ast.ASTRef{}, nil
	}
	if err := call.native.Close(); err != nil {
		return ast.ASTRef{}, err
	}
	call.native = nil
	if err := call.closeHandles(); err != nil {
		return ast.ASTRef{}, err
	}
	*refs = append(*refs, call.refs[:wrapperIndex]...)
	out = call.refs[wrapperIndex]
	call.refs = nil
	return out, nil
}

func (g *RPFP) validQuantifiedDefinitionProof(factory solver.Factory, proof ast.ASTHandle, call *proofWitnessCall) (valid bool, result error) {
	m := g.context.manager
	checker, err := proofs.NewChecker(m)
	if err != nil {
		return false, err
	}
	valid, side, err := checker.Check(proof)
	call.handles = append(call.handles, side...)
	if err != nil || !valid {
		return false, err
	}
	closed, err := proofs.IsClosed(m, proof)
	if err != nil || !closed {
		return false, err
	}
	call.native, err = factory.New(m, nil, false, false, false, "")
	if err != nil {
		return false, err
	}
	for _, condition := range side {
		if err := call.native.Push(); err != nil {
			return false, err
		}
		negated, err := m.MkNot(condition)
		if err != nil {
			return false, err
		}
		if err := call.native.Assert(negated); err != nil {
			return false, err
		}
		status, err := call.native.CheckSat(nil)
		if err != nil {
			return false, err
		}
		if err := call.native.Pop(1); err != nil {
			return false, err
		}
		if status != lbool.False {
			return false, nil
		}
	}
	return true, nil
}

// structuralQuantifiedDefinition bounds the specialized constructor to an
// all-variable predicate head with a relation-free antecedent. Equalities are
// used above to reconstruct otherwise missing witnesses, while any remaining
// background-theory conjuncts become independently checked side conditions.
// Program relations require the recursive parent proofs used by the source
// proof converter and therefore stay on the native fallback path.
func (g *RPFP) structuralQuantifiedDefinition(body ast.ASTHandle) (bool, error) {
	m := g.context.manager
	head, err := m.GetArg(body, 1)
	if err != nil {
		return false, err
	}
	n, err := m.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(head, i)
		if err != nil {
			return false, err
		}
		kind, err := m.GetKind(arg)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTVar {
			return false, nil
		}
	}
	antecedent, err := m.GetArg(body, 0)
	if err != nil {
		return false, err
	}
	relations, err := g.proofPredicateDeclarations(antecedent)
	if err != nil {
		return false, err
	}
	if len(relations) != 0 {
		return false, nil
	}
	return true, nil
}
