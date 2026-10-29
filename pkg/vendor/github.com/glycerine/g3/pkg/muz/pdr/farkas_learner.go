package pdr

// Source: the Farkas-lemma recognition and constraint-combination paths in
// src/muz/pdr/pdr_farkas_learner.h and pdr_farkas_learner.cpp.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// FarkasLearner owns the arithmetic utility used to combine proof premises.
// Returned ASTRef values are owned by the caller and must be released with
// ASTManager.DelRef.
type FarkasLearner struct {
	manager             *ast.ASTManager
	utility             *smtcore.SMTFarkasUtil
	combineCoefficients bool
	closed              bool
}

func NewFarkasLearner(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*FarkasLearner, error) {
	if manager == nil || context == nil || globals == nil {
		return nil, ast.ErrASTDomain
	}
	utility, err := smtcore.NewSMTFarkasUtil(manager, context, globals)
	if err != nil {
		return nil, err
	}
	return &FarkasLearner{manager: manager, utility: utility, combineCoefficients: true}, nil
}

func (f *FarkasLearner) open() error {
	if f == nil || f.closed || f.manager == nil || f.utility == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (f *FarkasLearner) Close() error {
	if f == nil {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	f.closed = true
	err := f.utility.Close()
	f.utility = nil
	return err
}

// SetCombineCoefficients selects the source learner's coefficient-combination
// mode. When disabled, CombineConstraints returns the conjunction of the
// negated literals, represented by not(or(literals)).
func (f *FarkasLearner) SetCombineCoefficients(enabled bool) error {
	if err := f.open(); err != nil {
		return err
	}
	f.combineCoefficients = enabled
	return nil
}

// SetSplitLiterals selects whether disconnected arithmetic constraints are
// returned as separate disjuncts, matching smt::farkas_util.
func (f *FarkasLearner) SetSplitLiterals(enabled bool) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.utility.SetSplitLiterals(enabled)
}

func (f *FarkasLearner) parameterIsSymbol(parameter ast.ASTParameter, want string) (bool, error) {
	symbol, err := f.manager.ParameterSymbol(parameter)
	if err != nil {
		return false, err
	}
	text, present, err := f.manager.Symbols.BareString(symbol)
	return present && text == want, err
}

// IsFarkasLemma recognizes arithmetic theory lemmas carrying the source
// "arith", "farkas", and per-parent coefficient parameter layout.
func (f *FarkasLearner) IsFarkasLemma(proof ast.ASTHandle) (_ bool, resultErr error) {
	if err := f.open(); err != nil {
		return false, err
	}
	isTheoryLemma, err := f.manager.IsProofKind(proof, ast.ASTPrTheoryLemma)
	if err != nil || !isTheoryLemma {
		return false, err
	}
	declaration, err := f.manager.GetDecl(proof)
	if err != nil {
		return false, err
	}
	parameters, err := f.manager.GetDeclParametersCopy(declaration)
	if err != nil {
		return false, err
	}
	defer func() {
		for i := range parameters {
			resultErr = errors.Join(resultErr, f.manager.DelParameter(&parameters[i]))
		}
	}()
	parents, err := f.manager.GetProofNumParents(proof)
	if err != nil {
		return false, err
	}
	if len(parameters) < 2 || uint32(len(parameters)) < parents+2 {
		return false, nil
	}
	arithmetic, err := f.parameterIsSymbol(parameters[0], "arith")
	if err != nil || !arithmetic {
		return false, err
	}
	farkas, err := f.parameterIsSymbol(parameters[1], "farkas")
	return farkas, err
}

func (f *FarkasLearner) rationalParameter(parameter ast.ASTParameter) (_ *big.Rat, resultErr error) {
	service := f.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	matched, err := f.manager.ParameterRationalCopy(parameter, &value)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ast.ErrASTDomain
	}
	text, err := service.String(&value)
	if err != nil {
		return nil, err
	}
	coefficient, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return coefficient, nil
}

// FarkasPremises returns the borrowed fact of each proof parent and its
// coefficient. Coefficients are made nonnegative, matching the source
// learner's workaround before it combines constraints.
func (f *FarkasLearner) FarkasPremises(proof ast.ASTHandle) (_ []ast.ASTHandle, _ []*big.Rat, _ bool, resultErr error) {
	matched, err := f.IsFarkasLemma(proof)
	if err != nil || !matched {
		return nil, nil, matched, err
	}
	declaration, err := f.manager.GetDecl(proof)
	if err != nil {
		return nil, nil, false, err
	}
	parameters, err := f.manager.GetDeclParametersCopy(declaration)
	if err != nil {
		return nil, nil, false, err
	}
	defer func() {
		for i := range parameters {
			resultErr = errors.Join(resultErr, f.manager.DelParameter(&parameters[i]))
		}
	}()
	count, err := f.manager.GetProofNumParents(proof)
	if err != nil {
		return nil, nil, false, err
	}
	facts := make([]ast.ASTHandle, 0, count)
	coefficients := make([]*big.Rat, 0, count)
	for i := uint32(0); i < count; i++ {
		parent, err := f.manager.GetProofParent(proof, i)
		if err != nil {
			return nil, nil, false, err
		}
		fact, err := f.manager.GetProofFact(parent)
		if err != nil {
			return nil, nil, false, err
		}
		coefficient, err := f.rationalParameter(parameters[i+2])
		if err != nil {
			return nil, nil, false, err
		}
		coefficient.Abs(coefficient)
		facts = append(facts, fact)
		coefficients = append(coefficients, coefficient)
	}
	return facts, coefficients, true, nil
}

// CombineFarkasLemma performs the theory-lemma branch of get_lemmas after the
// proof traversal has classified each parent. A true bPure entry removes that
// B-derived parent from the arithmetic combination. Coefficients attached to
// the lemma conclusion are applied to the negated conclusion literals.
func (f *FarkasLearner) CombineFarkasLemma(proof ast.ASTHandle, bPure []bool) (_ ast.ASTRef, _ bool, resultErr error) {
	facts, coefficients, matched, err := f.FarkasPremises(proof)
	if err != nil || !matched {
		return ast.ASTRef{}, false, err
	}
	if len(bPure) != len(facts) {
		return ast.ASTRef{}, false, ast.ErrASTDomain
	}
	literals := make([]ast.ASTHandle, 0, len(facts))
	selectedCoefficients := make([]*big.Rat, 0, len(coefficients))
	bPureCount := 0
	for i := range facts {
		if bPure[i] {
			bPureCount++
			continue
		}
		literals = append(literals, facts[i])
		selectedCoefficients = append(selectedCoefficients, coefficients[i])
	}
	if bPureCount == 0 {
		return ast.ASTRef{}, false, nil
	}

	declaration, err := f.manager.GetDecl(proof)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	parameters, err := f.manager.GetDeclParametersCopy(declaration)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	defer func() {
		for i := range parameters {
			resultErr = errors.Join(resultErr, f.manager.DelParameter(&parameters[i]))
		}
	}()
	parentParameterEnd := len(facts) + 2
	if len(parameters) > parentParameterEnd {
		fact, err := f.manager.GetProofFact(proof)
		if err != nil {
			return ast.ASTRef{}, false, err
		}
		conclusionLiterals := []ast.ASTHandle{fact}
		family, err := f.manager.GetFamilyID(fact)
		if err != nil {
			return ast.ASTRef{}, false, err
		}
		kind, err := f.manager.GetDeclKind(fact)
		if err != nil {
			return ast.ASTRef{}, false, err
		}
		if family == f.manager.BasicFamilyID() && kind == ast.ASTOpOr {
			count, err := f.manager.GetNumArgs(fact)
			if err != nil {
				return ast.ASTRef{}, false, err
			}
			conclusionLiterals = make([]ast.ASTHandle, count)
			for i := uint32(0); i < count; i++ {
				conclusionLiterals[i], err = f.manager.GetArg(fact, i)
				if err != nil {
					return ast.ASTRef{}, false, err
				}
			}
		}
		if len(parameters) != parentParameterEnd+len(conclusionLiterals) {
			return ast.ASTRef{}, false, ast.ErrASTDomain
		}
		negationReferences := make([]ast.ASTRef, 0, len(conclusionLiterals))
		defer func() {
			for i := range negationReferences {
				resultErr = errors.Join(resultErr, f.manager.DelRef(&negationReferences[i]))
			}
		}()
		for i, literal := range conclusionLiterals {
			negation, err := f.manager.MkNot(literal)
			if err != nil {
				return ast.ASTRef{}, false, err
			}
			negationReference, err := f.manager.NewRef(negation)
			if err != nil {
				return ast.ASTRef{}, false, err
			}
			negationReferences = append(negationReferences, negationReference)
			coefficient, err := f.rationalParameter(parameters[parentParameterEnd+i])
			if err != nil {
				return ast.ASTRef{}, false, err
			}
			coefficient.Abs(coefficient)
			literals = append(literals, negation)
			selectedCoefficients = append(selectedCoefficients, coefficient)
		}
	}
	if len(literals) == 0 {
		return ast.ASTRef{}, false, ast.ErrASTDomain
	}
	combined, err := f.CombineConstraints(literals, selectedCoefficients)
	return combined, err == nil, err
}

func (f *FarkasLearner) proofPostOrder(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	type frame struct {
		proof ast.ASTHandle
		exit  bool
	}
	stack := []frame{{proof: root}}
	visited := make(map[ast.ASTHandle]bool)
	order := make([]ast.ASTHandle, 0)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.exit {
			order = append(order, current.proof)
			continue
		}
		if visited[current.proof] {
			continue
		}
		visited[current.proof] = true
		stack = append(stack, frame{proof: current.proof, exit: true})
		count, err := f.manager.GetProofNumParents(current.proof)
		if err != nil {
			return nil, err
		}
		for i := count; i != 0; i-- {
			parent, err := f.manager.GetProofParent(current.proof, i-1)
			if err != nil {
				return nil, err
			}
			stack = append(stack, frame{proof: parent})
		}
	}
	return order, nil
}

func copyFarkasHypotheses(source map[ast.ASTHandle]struct{}) map[ast.ASTHandle]struct{} {
	result := make(map[ast.ASTHandle]struct{}, len(source))
	for hypothesis := range source {
		result[hypothesis] = struct{}{}
	}
	return result
}

func (f *FarkasLearner) collectUninterpretedDeclarations(root ast.ASTHandle, declarations map[ast.ASTHandle]struct{}) error {
	visited := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		kind, err := f.manager.GetKind(current)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := f.manager.GetDecl(current)
			if err != nil {
				return err
			}
			family, err := f.manager.GetFamilyID(declaration)
			if err != nil {
				return err
			}
			if family == ast.NullFamilyID {
				declarations[declaration] = struct{}{}
			}
			count, err := f.manager.GetNumArgs(current)
			if err != nil {
				return err
			}
			for i := count; i != 0; i-- {
				child, err := f.manager.GetArg(current, i-1)
				if err != nil {
					return err
				}
				stack = append(stack, child)
			}
		case ast.ASTQuantifier:
			body, err := f.manager.GetQuantifierBody(current)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			patterns, err := f.manager.GetNumPatterns(current)
			if err != nil {
				return err
			}
			for i := patterns; i != 0; i-- {
				pattern, err := f.manager.GetPattern(current, i-1)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
			noPatterns, err := f.manager.GetNumNoPatterns(current)
			if err != nil {
				return err
			}
			for i := noPatterns; i != 0; i-- {
				pattern, err := f.manager.GetNoPattern(current, i-1)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (f *FarkasLearner) expressionIsPure(root ast.ASTHandle, declarations map[ast.ASTHandle]struct{}) (bool, error) {
	reached := make(map[ast.ASTHandle]struct{})
	if err := f.collectUninterpretedDeclarations(root, reached); err != nil {
		return false, err
	}
	for declaration := range reached {
		if _, found := declarations[declaration]; !found {
			return false, nil
		}
	}
	return true, nil
}

// GetLemmas performs the source proof preprocessing and dependency traversal,
// recovering general B-pure units and arithmetic lemmas produced at Farkas
// theory nodes. Final tactic subsumption remains a separate porting step.
// Every returned ASTRef is caller-owned.
func (f *FarkasLearner) GetLemmas(root ast.ASTHandle, bAssertions []ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	rootReference, err := f.manager.NewRef(root)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, f.manager.DelRef(&rootReference)) }()
	if err = proofs.ReduceHypotheses(f.manager, rootReference); err != nil {
		return nil, err
	}
	if err = proofs.PermuteUnitResolution(f.manager, rootReference); err != nil {
		return nil, err
	}
	root, err = f.manager.RefGet(rootReference)
	if err != nil {
		return nil, err
	}
	order, err := f.proofPostOrder(root)
	if err != nil {
		return nil, err
	}
	bSet := make(map[ast.ASTHandle]struct{}, len(bAssertions))
	bDeclarations := make(map[ast.ASTHandle]struct{})
	for _, assertion := range bAssertions {
		bSet[assertion] = struct{}{}
		if err := f.collectUninterpretedDeclarations(assertion, bDeclarations); err != nil {
			return nil, err
		}
	}
	hypotheses := make(map[ast.ASTHandle]map[ast.ASTHandle]struct{}, len(order))
	aDependent := make(map[ast.ASTHandle]bool, len(order))
	bDependent := make(map[ast.ASTHandle]bool, len(order))
	temporaryReferences := make([]ast.ASTRef, 0)
	defer func() {
		for i := range temporaryReferences {
			resultErr = errors.Join(resultErr, f.manager.DelRef(&temporaryReferences[i]))
		}
	}()
	results := make([]ast.ASTRef, 0)
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := range results {
			resultErr = errors.Join(resultErr, f.manager.DelRef(&results[i]))
		}
	}()
	seenResults := make(map[ast.ASTHandle]bool)
	appendResult := func(handle ast.ASTHandle) error {
		if seenResults[handle] {
			return nil
		}
		reference, err := f.manager.NewRef(handle)
		if err != nil {
			return err
		}
		seenResults[handle] = true
		results = append(results, reference)
		return nil
	}
	bClosed := make(map[ast.ASTHandle]bool)
	var recoverAsserted func(ast.ASTHandle) error
	recoverAsserted = func(root ast.ASTHandle) error {
		stack := []ast.ASTHandle{root}
		visited := make(map[ast.ASTHandle]bool)
		for len(stack) != 0 {
			last := len(stack) - 1
			current := stack[last]
			stack = stack[:last]
			if visited[current] || bClosed[current] {
				continue
			}
			visited[current] = true
			count, err := f.manager.GetProofNumParents(current)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				parent, err := f.manager.GetProofParent(current, i)
				if err != nil {
					return err
				}
				stack = append(stack, parent)
			}
			asserted, err := f.manager.IsProofKind(current, ast.ASTPrAsserted)
			if err != nil {
				return err
			}
			if asserted {
				fact, err := f.manager.GetProofFact(current)
				if err != nil {
					return err
				}
				if _, found := bSet[fact]; found {
					if err = appendResult(fact); err != nil {
						return err
					}
					bClosed[current] = true
				}
			}
		}
		return nil
	}
	for _, proof := range order {
		count, err := f.manager.GetProofNumParents(proof)
		if err != nil {
			return nil, err
		}
		hyps := make(map[ast.ASTHandle]struct{})
		parents := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			parent, err := f.manager.GetProofParent(proof, i)
			if err != nil {
				return nil, err
			}
			parents[i] = parent
			aDependent[proof] = aDependent[proof] || aDependent[parent]
			bDependent[proof] = bDependent[proof] || bDependent[parent]
			for hypothesis := range hypotheses[parent] {
				hyps[hypothesis] = struct{}{}
			}
		}
		hypotheses[proof] = hyps
		kind, err := f.manager.GetDeclKind(proof)
		if err != nil {
			return nil, err
		}
		farkasLemma, err := f.IsFarkasLemma(proof)
		if err != nil {
			return nil, err
		}
		if (len(hyps) != 0 || aDependent[proof]) && bDependent[proof] && !farkasLemma {
			for _, parent := range parents {
				bPure := bDependent[parent] && !aDependent[parent] && len(hypotheses[parent]) == 0
				if !bPure {
					continue
				}
				fact, err := f.manager.GetProofFact(parent)
				if err != nil {
					return nil, err
				}
				pure, err := f.expressionIsPure(fact, bDeclarations)
				if err != nil {
					return nil, err
				}
				if pure {
					if err = appendResult(fact); err != nil {
						return nil, err
					}
				} else {
					if err = recoverAsserted(proof); err != nil {
						return nil, err
					}
					bClosed[proof] = true
				}
			}
		}
		switch kind {
		case ast.ASTPrAsserted:
			fact, err := f.manager.GetProofFact(proof)
			if err != nil {
				return nil, err
			}
			if _, found := bSet[fact]; found {
				bDependent[proof] = true
			} else {
				aDependent[proof] = true
			}
		case ast.ASTPrHypothesis:
			fact, err := f.manager.GetProofFact(proof)
			if err != nil {
				return nil, err
			}
			hyps = copyFarkasHypotheses(hyps)
			hyps[fact] = struct{}{}
			hypotheses[proof] = hyps
		case ast.ASTPrDefAxiom:
			fact, err := f.manager.GetProofFact(proof)
			if err != nil {
				return nil, err
			}
			pure, err := f.expressionIsPure(fact, bDeclarations)
			if err != nil {
				return nil, err
			}
			if !pure {
				aDependent[proof] = true
			}
		case ast.ASTPrLemma:
			fact, err := f.manager.GetProofFact(proof)
			if err != nil {
				return nil, err
			}
			hyps = copyFarkasHypotheses(hyps)
			delete(hyps, fact)
			factKind, err := f.manager.GetKind(fact)
			if err != nil {
				return nil, err
			}
			isOr := false
			if factKind == ast.ASTApp {
				family, err := f.manager.GetFamilyID(fact)
				if err != nil {
					return nil, err
				}
				declKind, err := f.manager.GetDeclKind(fact)
				if err != nil {
					return nil, err
				}
				isOr = family == f.manager.BasicFamilyID() && declKind == ast.ASTOpOr
			}
			if isOr {
				arguments, err := f.manager.GetNumArgs(fact)
				if err != nil {
					return nil, err
				}
				for i := uint32(0); i < arguments; i++ {
					literal, err := f.manager.GetArg(fact, i)
					if err != nil {
						return nil, err
					}
					negation, err := f.manager.MkNot(literal)
					if err != nil {
						return nil, err
					}
					negationReference, err := f.manager.NewRef(negation)
					if err != nil {
						return nil, err
					}
					temporaryReferences = append(temporaryReferences, negationReference)
					delete(hyps, negation)
				}
			}
			hypotheses[proof] = hyps
		case ast.ASTPrTheoryLemma:
			bPure := make([]bool, len(parents))
			for i, parent := range parents {
				bPure[i] = bDependent[parent] && !aDependent[parent] && len(hypotheses[parent]) == 0
			}
			lemma, produced, err := f.CombineFarkasLemma(proof, bPure)
			if err != nil {
				return nil, err
			}
			if produced {
				bClosed[proof] = true
				handle, err := f.manager.RefGet(lemma)
				if err != nil {
					return nil, err
				}
				if seenResults[handle] {
					if err = f.manager.DelRef(&lemma); err != nil {
						return nil, err
					}
				} else {
					seenResults[handle] = true
					results = append(results, lemma)
				}
			}
		}
	}
	committed = true
	return results, nil
}

// GetConsequences runs get_lemmas in the source literal-combination mode and
// restores coefficient combination before returning.
func (f *FarkasLearner) GetConsequences(root ast.ASTHandle, bAssertions []ast.ASTHandle) ([]ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	previous := f.combineCoefficients
	f.combineCoefficients = false
	defer func() { f.combineCoefficients = previous }()
	return f.GetLemmas(root, bAssertions)
}

// CombineConstraints applies the source learner's two combination modes.
func (f *FarkasLearner) CombineConstraints(literals []ast.ASTHandle, coefficients []*big.Rat) (_ ast.ASTRef, resultErr error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if len(literals) != len(coefficients) {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	if f.combineCoefficients {
		if err := f.utility.Reset(); err != nil {
			return ast.ASTRef{}, err
		}
		for i := range literals {
			if coefficients[i] == nil {
				return ast.ASTRef{}, ast.ErrASTDomain
			}
			if err := f.utility.Add(coefficients[i], literals[i]); err != nil {
				return ast.ASTRef{}, err
			}
		}
		out, err := f.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = f.utility.Get(out); err != nil {
			resultErr = errors.Join(err, f.manager.DelRef(&out))
			return ast.ASTRef{}, resultErr
		}
		return out, nil
	}

	disjunction, err := f.manager.MkOrArray(literals)
	if err != nil {
		return ast.ASTRef{}, err
	}
	disjunctionRef, err := f.manager.NewRef(disjunction)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, f.manager.DelRef(&disjunctionRef)) }()
	negation, err := f.manager.MkNot(disjunction)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return f.manager.NewRef(negation)
}
