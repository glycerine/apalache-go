package pdr

// Source: the retained level-frame, safe-assumption, subset-core and model
// paths in src/muz/pdr/pdr_prop_solver.h and pdr_prop_solver.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	tacticarith "github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrPropSolverOwner = errors.New("muz/pdr: invalid or closed propositional solver")

type PropCheckResult struct {
	Status       lbool.LBool
	Core         []ast.ASTHandle
	Model        *model.ModelCore
	AssumesLevel bool
	FarkasCore   bool
	Consequences []ast.ASTHandle
}

// PropSolver is the PDR-facing frame and safe-assumption wrapper over one
// guarded SMTContext. Returned cores borrow their input formulas; synthesized
// expansion terms remain valid until the next check or Close. A nonnil model
// is an owning model reference and must be released by the caller.
type PropSolver struct {
	manager        *ast.ASTManager
	contexts       *SMTContextManager
	context        *SMTContext
	name           string
	levelPositive  []ast.ASTHandle
	levelNegative  []ast.ASTHandle
	levelAtoms     map[ast.ASTHandle]bool
	proxies        []ast.ASTHandle
	auxiliary      map[ast.ASTHandle]bool
	references     []ast.ASTRef
	coreReferences []ast.ASTRef
	useFarkas      bool
	subsetCore     bool
	closed         bool
}

func NewPropSolver(manager *ast.ASTManager, contexts *SMTContextManager, background ast.ASTHandle, name string) (_ *PropSolver, resultErr error) {
	if manager == nil || contexts == nil {
		return nil, ErrPropSolverOwner
	}
	context, err := contexts.Fresh()
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, context.Close())
		}
	}()
	if err = context.Assert(background); err != nil {
		return nil, err
	}
	p := &PropSolver{
		manager:    manager,
		contexts:   contexts,
		context:    context,
		name:       name,
		levelAtoms: make(map[ast.ASTHandle]bool),
		auxiliary:  make(map[ast.ASTHandle]bool),
	}
	committed = true
	return p, nil
}

func (p *PropSolver) open() error {
	if p == nil || p.closed || p.manager == nil || p.context == nil {
		return ErrPropSolverOwner
	}
	return p.context.open()
}

// SetUseFarkas selects proof-derived arithmetic cores when proof production is
// enabled. The source default is false.
func (p *PropSolver) SetUseFarkas(enabled bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.useFarkas = enabled
	return nil
}

func (p *PropSolver) UseFarkas() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.useFarkas, nil
}

// SetSubsetBasedCore forces the source subset-core path even when Farkas core
// extraction is enabled.
func (p *PropSolver) SetSubsetBasedCore(enabled bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.subsetCore = enabled
	return nil
}

func (p *PropSolver) pin(handle ast.ASTHandle) error {
	reference, err := p.manager.NewRef(handle)
	if err != nil {
		return err
	}
	p.references = append(p.references, reference)
	return nil
}

func (p *PropSolver) addLevel() error {
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("%s#level_%d", p.name, len(p.levelPositive))
	positive, err := p.manager.MkFreshConst(prefix, boolean)
	if err != nil {
		return err
	}
	if err = p.pin(positive); err != nil {
		return err
	}
	negative, err := p.manager.MkNot(positive)
	if err != nil {
		return err
	}
	if err = p.pin(negative); err != nil {
		return err
	}
	declaration, err := p.manager.GetDecl(positive)
	if err != nil {
		return err
	}
	p.levelPositive = append(p.levelPositive, positive)
	p.levelNegative = append(p.levelNegative, negative)
	p.levelAtoms[positive] = true
	p.levelAtoms[negative] = true
	p.auxiliary[declaration] = true
	return nil
}

func (p *PropSolver) ensureLevel(level uint32) error {
	for uint32(len(p.levelPositive)) <= level {
		if err := p.addLevel(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PropSolver) LevelCount() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return uint32(len(p.levelPositive)), nil
}

// EnsureLevel creates every frame guard through level. It exposes the source
// add_level operation to predicate transformers without asserting a dummy
// formula into a newly created frame.
func (p *PropSolver) EnsureLevel(level uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	return p.ensureLevel(level)
}

func (p *PropSolver) IsAuxSymbol(declaration ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if p.auxiliary[declaration] {
		return true, nil
	}
	return p.context.IsAuxPredicate(declaration)
}

func (p *PropSolver) AddFormula(formula ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	return p.context.Assert(formula)
}

func (p *PropSolver) AddLevelFormula(formula ast.ASTHandle, level uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.ensureLevel(level); err != nil {
		return err
	}
	guarded, err := p.manager.MkOr(formula, p.levelPositive[level])
	if err != nil {
		return err
	}
	return p.context.Assert(guarded)
}

func (p *PropSolver) isDirectAssumption(expression ast.ASTHandle) (bool, error) {
	if negated, argument, err := p.manager.IsNot(expression); err != nil {
		return false, err
	} else if negated {
		expression = argument
	}
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	declaration, err := p.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	family, err := p.manager.GetFamilyID(declaration)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	arity, err := p.manager.GetArity(declaration)
	return arity == 0, err
}

func (p *PropSolver) proxy(index uint32) (ast.ASTHandle, error) {
	if index < uint32(len(p.proxies)) {
		return p.proxies[index], nil
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proxy, err := p.manager.MkFreshConst(fmt.Sprintf("pdr_proxy_%d", len(p.proxies)), boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.pin(proxy); err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.manager.GetDecl(proxy)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.proxies = append(p.proxies, proxy)
	p.auxiliary[declaration] = true
	return proxy, nil
}

type safeAssumptions struct {
	atoms          []ast.ASTHandle
	proxyToFormula map[ast.ASTHandle]ast.ASTHandle
	formulaToProxy map[ast.ASTHandle]ast.ASTHandle
	references     []ast.ASTRef
	generated      map[ast.ASTHandle]bool
}

func (p *PropSolver) binaryEquality(expression ast.ASTHandle) (left, right ast.ASTHandle, matched bool, result error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != p.manager.BasicFamilyID() {
		return left, right, false, err
	}
	operation, err := p.manager.GetDeclKind(expression)
	if err != nil || operation != ast.ASTOpEq {
		return left, right, false, err
	}
	left, err = p.manager.GetArg(expression, 0)
	if err != nil {
		return left, right, false, err
	}
	right, err = p.manager.GetArg(expression, 1)
	if err != nil {
		return left, right, false, err
	}
	return left, right, true, nil
}

func (p *PropSolver) arithmeticEquality(expression ast.ASTHandle) (left, right ast.ASTHandle, matched bool, result error) {
	left, right, matched, err := p.binaryEquality(expression)
	if err != nil || !matched {
		return left, right, false, err
	}
	sort, err := p.manager.GetSort(left)
	if err != nil {
		return left, right, false, err
	}
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return left, right, false, err
	}
	sortFamily, err := p.manager.GetFamilyID(sort)
	if err != nil || sortFamily != arithmetic {
		return left, right, false, err
	}
	sortKind, err := p.manager.GetDeclKind(sort)
	if err != nil {
		return left, right, false, err
	}
	return left, right, sortKind == 0 || sortKind == 1, nil
}

func (p *PropSolver) datatypeEquality(expression ast.ASTHandle) (term, value, constructor ast.ASTHandle, matched bool, result error) {
	left, right, equality, err := p.binaryEquality(expression)
	if err != nil || !equality {
		return term, value, constructor, false, err
	}
	constructor, matched, err = p.manager.DatatypeConstructorApp(right)
	if err != nil {
		return term, value, constructor, false, err
	}
	if matched {
		return left, right, constructor, true, nil
	}
	constructor, matched, err = p.manager.DatatypeConstructorApp(left)
	if err != nil || !matched {
		return term, value, constructor, false, err
	}
	return right, left, constructor, true, nil
}

func (p *PropSolver) bitVectorEquality(expression ast.ASTHandle) (term ast.ASTHandle, bits []bool, matched bool, resultErr error) {
	left, right, equality, err := p.binaryEquality(expression)
	if err != nil || !equality {
		return term, nil, false, err
	}
	service := p.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return term, nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	var width uint32
	matched, err = p.manager.BitVectorNumeralValue(right, &value, &width)
	if err != nil {
		return term, nil, false, err
	}
	if matched {
		term = left
	} else {
		matched, err = p.manager.BitVectorNumeralValue(left, &value, &width)
		if err != nil || !matched {
			return term, nil, false, err
		}
		term = right
	}
	two, err := service.NewValue()
	if err != nil {
		return term, nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&two)) }()
	if err = service.SetUnsigned(&two, 2); err != nil {
		return term, nil, false, err
	}
	bits = make([]bool, width)
	for index := range bits {
		remainder, x := service.Mod(&value, &two)
		if x != nil {
			return term, nil, false, x
		}
		zero, x := service.IsZero(&remainder)
		x = errors.Join(x, service.Del(&remainder))
		if x != nil {
			return term, nil, false, x
		}
		bits[index] = !zero
		quotient, x := service.IDiv(&value, &two)
		if x != nil {
			return term, nil, false, x
		}
		x = service.Del(&value)
		value = quotient
		if x != nil {
			return term, nil, false, x
		}
	}
	return term, bits, true, nil
}

func (p *PropSolver) expandEqualities(atoms []ast.ASTHandle) (result []ast.ASTHandle, references []ast.ASTRef, generated map[ast.ASTHandle]bool, resultErr error) {
	generated = make(map[ast.ASTHandle]bool)
	pin := func(expression ast.ASTHandle, topLevel bool) error {
		reference, err := p.manager.NewRef(expression)
		if err != nil {
			return err
		}
		references = append(references, reference)
		if topLevel {
			generated[expression] = true
		}
		return nil
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&references[index]))
		}
	}()
	for index := 0; index < len(atoms); index++ {
		left, right, matched, err := p.arithmeticEquality(atoms[index])
		if err != nil {
			return nil, references, generated, err
		}
		if matched {
			lessEqual, err := p.manager.MkArithmeticLE(left, right)
			if err != nil {
				return nil, references, generated, err
			}
			if err = pin(lessEqual, true); err != nil {
				return nil, references, generated, err
			}
			greaterEqual, err := p.manager.MkArithmeticGE(left, right)
			if err != nil {
				return nil, references, generated, err
			}
			if err = pin(greaterEqual, true); err != nil {
				return nil, references, generated, err
			}
			atoms[index] = lessEqual
			if index+1 == len(atoms) {
				atoms = append(atoms, greaterEqual)
			} else {
				next := atoms[index+1]
				atoms = append(atoms, next)
				atoms[index+1] = greaterEqual
			}
			index++
			continue
		}
		term, value, constructor, matched, err := p.datatypeEquality(atoms[index])
		if err != nil {
			return nil, references, generated, err
		}
		if !matched {
			bitTerm, bits, bitVector, x := p.bitVectorEquality(atoms[index])
			if x != nil {
				return nil, references, generated, x
			}
			if !bitVector {
				continue
			}
			one, x := p.manager.BitVectorNumeralUint64(1, 1)
			if x != nil {
				return nil, references, generated, x
			}
			if x = pin(one, false); x != nil {
				return nil, references, generated, x
			}
			for bitIndex, set := range bits {
				extract, x := p.manager.MkBitVectorExtract(uint32(bitIndex), uint32(bitIndex), bitTerm)
				if x != nil {
					return nil, references, generated, x
				}
				if x = pin(extract, false); x != nil {
					return nil, references, generated, x
				}
				bitEquality, x := p.manager.MkEq(one, extract)
				if x != nil {
					return nil, references, generated, x
				}
				if x = pin(bitEquality, set); x != nil {
					return nil, references, generated, x
				}
				literal := bitEquality
				if !set {
					literal, x = p.manager.MkNot(bitEquality)
					if x != nil {
						return nil, references, generated, x
					}
					if x = pin(literal, true); x != nil {
						return nil, references, generated, x
					}
				}
				if bitIndex == 0 {
					atoms[index] = literal
				} else {
					atoms = append(atoms, literal)
				}
			}
			continue
		}
		recognizer, err := p.manager.DatatypeConstructorRecognizer(constructor)
		if err != nil {
			return nil, references, generated, err
		}
		recognized, err := p.manager.MkApp(recognizer, []ast.ASTHandle{term})
		if err != nil {
			return nil, references, generated, err
		}
		if err = pin(recognized, true); err != nil {
			return nil, references, generated, err
		}
		atoms[index] = recognized
		accessors, err := p.manager.DatatypeConstructorAccessors(constructor)
		if err != nil {
			return nil, references, generated, err
		}
		termConstructor, sameConstructor, err := p.manager.DatatypeConstructorApp(term)
		if err != nil {
			return nil, references, generated, err
		}
		for fieldIndex, accessor := range accessors {
			var selected ast.ASTHandle
			if sameConstructor && termConstructor == constructor {
				selected, err = p.manager.GetArg(term, uint32(fieldIndex))
			} else {
				selected, err = p.manager.MkApp(accessor, []ast.ASTHandle{term})
				if err == nil {
					err = pin(selected, false)
				}
			}
			if err != nil {
				return nil, references, generated, err
			}
			expected, err := p.manager.GetArg(value, uint32(fieldIndex))
			if err != nil {
				return nil, references, generated, err
			}
			fieldEquality, err := p.manager.MkEq(selected, expected)
			if err != nil {
				return nil, references, generated, err
			}
			if err = pin(fieldEquality, true); err != nil {
				return nil, references, generated, err
			}
			atoms = append(atoms, fieldEquality)
		}
	}
	committed = true
	return atoms, references, generated, nil
}

func (s *safeAssumptions) close(manager *ast.ASTManager) (result error) {
	for index := len(s.references) - 1; index >= 0; index-- {
		result = errors.Join(result, manager.DelRef(&s.references[index]))
	}
	s.references = nil
	return result
}

func (p *PropSolver) makeSafeAssumptions(input []ast.ASTHandle) (_ *safeAssumptions, resultErr error) {
	atoms, err := p.manager.FlattenAnd(append([]ast.ASTHandle(nil), input...))
	if err != nil {
		return nil, err
	}
	atoms, references, generated, err := p.expandEqualities(atoms)
	if err != nil {
		return nil, err
	}
	safe := &safeAssumptions{
		atoms:          atoms,
		proxyToFormula: make(map[ast.ASTHandle]ast.ASTHandle),
		formulaToProxy: make(map[ast.ASTHandle]ast.ASTHandle),
		references:     references,
		generated:      generated,
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, safe.close(p.manager))
		}
	}()
	nextProxy := uint32(0)
	for index, atom := range safe.atoms {
		direct, x := p.isDirectAssumption(atom)
		if x != nil {
			return nil, x
		}
		if direct {
			continue
		}
		if proxy, found := safe.formulaToProxy[atom]; found {
			safe.atoms[index] = proxy
			continue
		}
		proxy, x := p.proxy(nextProxy)
		if x != nil {
			return nil, x
		}
		nextProxy++
		implication, x := p.manager.MkImplies(proxy, atom)
		if x != nil {
			return nil, x
		}
		if x = p.context.Assert(implication); x != nil {
			return nil, x
		}
		safe.proxyToFormula[proxy] = atom
		safe.formulaToProxy[atom] = proxy
		safe.atoms[index] = proxy
	}
	committed = true
	return safe, nil
}

func (p *PropSolver) levelAssumptions(level uint32) []ast.ASTHandle {
	result := make([]ast.ASTHandle, len(p.levelPositive))
	for index := range result {
		if uint32(index) >= level {
			result[index] = p.levelNegative[index]
		} else {
			result[index] = p.levelPositive[index]
		}
	}
	return result
}

func (p *PropSolver) clearCoreReferences() (result error) {
	for index := len(p.coreReferences) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DelRef(&p.coreReferences[index]))
	}
	p.coreReferences = nil
	return result
}

func (p *PropSolver) publishTheoryResults(safe *safeAssumptions, learned []ast.ASTRef) (_ []ast.ASTHandle, resultErr error) {
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := range learned {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&learned[i]))
		}
	}()
	if len(learned) == 0 {
		committed = true
		return nil, nil
	}
	if len(safe.proxyToFormula) != 0 {
		substitution, err := p.manager.NewExprSubstitutionPolicy(false, false)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, p.manager.DelExprSubstitution(&substitution)) }()
		for proxy, formula := range safe.proxyToFormula {
			if err = p.manager.ExprSubstitutionInsert(substitution, proxy, formula, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
				return nil, err
			}
		}
		replacer, err := rewriter.NewDefaultExprReplacer(p.manager)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, replacer.Close()) }()
		if err = replacer.SetSubstitution(&substitution); err != nil {
			return nil, err
		}
		for i := range learned {
			original, err := p.manager.RefGet(learned[i])
			if err != nil {
				return nil, err
			}
			rewritten, err := p.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return nil, err
			}
			if err = replacer.Rewrite(original, rewritten); err != nil {
				resultErr = errors.Join(err, p.manager.DelRef(&rewritten))
				return nil, resultErr
			}
			if err = p.manager.DelRef(&learned[i]); err != nil {
				_ = p.manager.DelRef(&rewritten)
				return nil, err
			}
			learned[i] = rewritten
		}
	}
	result := make([]ast.ASTHandle, 0, len(learned))
	for i := range learned {
		handle, err := p.manager.RefGet(learned[i])
		if err != nil {
			return nil, err
		}
		result = append(result, handle)
	}
	p.coreReferences = append(p.coreReferences, learned...)
	committed = true
	return result, nil
}

func (p *PropSolver) simplifyTheoryResults(learned []ast.ASTRef) (_ []ast.ASTRef, resultErr error) {
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := range learned {
			if learned[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, p.manager.DelRef(&learned[i]))
			}
		}
	}()
	formulas := make([]ast.ASTHandle, len(learned))
	for i := range learned {
		var err error
		formulas[i], err = p.manager.RefGet(learned[i])
		if err != nil {
			return nil, err
		}
	}
	keep, err := tacticarith.ArithmeticBoundsKeep(p.manager, formulas)
	if err != nil {
		return nil, err
	}
	kept := learned[:0]
	for i := range learned {
		if keep[i] {
			kept = append(kept, learned[i])
		} else if err = p.manager.DelRef(&learned[i]); err != nil {
			return nil, err
		}
	}
	committed = true
	return kept, nil
}

func (p *PropSolver) extractTheoryCore(safe *safeAssumptions, wantConsequences bool) (_ []ast.ASTHandle, _ []ast.ASTHandle, resultErr error) {
	if p.contexts.config.Params == nil || p.contexts.config.Globals == nil {
		return nil, nil, nil
	}
	proof, err := p.context.Proof()
	if err != nil || proof.IsNull() {
		return nil, nil, err
	}
	learner, err := NewFarkasLearner(p.manager, p.contexts.config.Params, p.contexts.config.Globals)
	if err != nil {
		return nil, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, learner.Close()) }()
	learned, err := learner.GetLemmas(proof, safe.atoms)
	if err != nil {
		return nil, nil, fmt.Errorf("muz/pdr: Farkas lemma extraction: %w", err)
	}
	learned, err = p.simplifyTheoryResults(learned)
	if err != nil {
		return nil, nil, fmt.Errorf("muz/pdr: Farkas lemma simplification: %w", err)
	}
	core, err := p.publishTheoryResults(safe, learned)
	if err != nil || len(core) == 0 || !wantConsequences {
		if err != nil {
			err = fmt.Errorf("muz/pdr: Farkas core publication: %w", err)
		}
		return core, nil, err
	}
	learnedConsequences, err := learner.GetConsequences(proof, safe.atoms)
	if err != nil {
		return nil, nil, fmt.Errorf("muz/pdr: Farkas consequence extraction: %w", err)
	}
	learnedConsequences, err = p.simplifyTheoryResults(learnedConsequences)
	if err != nil {
		return nil, nil, fmt.Errorf("muz/pdr: Farkas consequence simplification: %w", err)
	}
	consequences, err := p.publishTheoryResults(safe, learnedConsequences)
	return core, consequences, err
}

func (p *PropSolver) check(atoms []ast.ASTHandle, formula ast.ASTHandle, level *uint32, wantCore, wantModel, wantConsequences bool) (result PropCheckResult, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	defer func() { p.subsetCore = false }()
	if err := p.clearCoreReferences(); err != nil {
		return result, err
	}
	if level != nil {
		if err := p.ensureLevel(*level); err != nil {
			return result, err
		}
	}
	scope, err := p.context.BeginDelay()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	safe, err := p.makeSafeAssumptions(atoms)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, safe.close(p.manager)) }()
	truth, err := p.manager.MkTrue()
	if err != nil {
		return result, err
	}
	if formula != truth {
		if err = p.context.Assert(formula); err != nil {
			return result, err
		}
	}
	assumptions := append([]ast.ASTHandle(nil), safe.atoms...)
	if level != nil {
		assumptions = append(assumptions, p.levelAssumptions(*level)...)
	}
	result.Status, err = p.context.Check(assumptions)
	if err != nil {
		return result, err
	}
	if result.Status == lbool.False {
		core, x := p.context.UnsatCore()
		if x != nil {
			return result, x
		}
		for _, item := range core {
			if p.levelAtoms[item] {
				result.AssumesLevel = true
				continue
			}
			if original, proxied := safe.proxyToFormula[item]; proxied {
				if wantCore {
					if safe.generated[original] {
						reference, x := p.manager.NewRef(original)
						if x != nil {
							return result, x
						}
						p.coreReferences = append(p.coreReferences, reference)
					}
					result.Core = append(result.Core, original)
				}
				continue
			}
			declaration, x := p.manager.GetDecl(item)
			if x != nil {
				return result, x
			}
			auxiliary, x := p.IsAuxSymbol(declaration)
			if x != nil {
				return result, x
			}
			if wantCore && !auxiliary {
				result.Core = append(result.Core, item)
			}
		}
		proofsEnabled, x := p.manager.ProofsEnabled()
		if x != nil {
			return result, x
		}
		if wantCore && p.useFarkas && !p.subsetCore && proofsEnabled {
			theoryCore, consequences, x := p.extractTheoryCore(safe, wantConsequences)
			if x != nil {
				return result, x
			}
			if len(theoryCore) != 0 {
				result.Core = theoryCore
				result.FarkasCore = true
				result.Consequences = consequences
			}
		}
	} else if result.Status == lbool.True && wantModel {
		result.Model, err = p.context.Model()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (p *PropSolver) CheckAssumptions(atoms []ast.ASTHandle, wantCore, wantModel bool) (PropCheckResult, error) {
	if err := p.open(); err != nil {
		return PropCheckResult{}, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return PropCheckResult{}, err
	}
	return p.check(atoms, truth, nil, wantCore, wantModel, false)
}

func (p *PropSolver) CheckAtLevel(level uint32, atoms []ast.ASTHandle, wantCore, wantModel bool) (PropCheckResult, error) {
	if err := p.open(); err != nil {
		return PropCheckResult{}, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return PropCheckResult{}, err
	}
	return p.check(atoms, truth, &level, wantCore, wantModel, false)
}

// CheckAtLevelAndFormula checks assumptions against a temporary formula while
// selecting one PDR frame. The temporary formula is rolled back after the
// check and is not included in the returned assumption core.
func (p *PropSolver) CheckAtLevelAndFormula(level uint32, atoms []ast.ASTHandle, formula ast.ASTHandle, wantCore, wantModel bool) (PropCheckResult, error) {
	if err := p.open(); err != nil {
		return PropCheckResult{}, err
	}
	return p.check(atoms, formula, &level, wantCore, wantModel, false)
}

func (p *PropSolver) CheckAssumptionsAndFormula(atoms []ast.ASTHandle, formula ast.ASTHandle, wantCore, wantModel bool) (PropCheckResult, error) {
	return p.check(atoms, formula, nil, wantCore, wantModel, false)
}

// CheckAssumptionsWithConsequences requests the source proof-derived core and
// its consequence-mode companion. Consequences are retained until the next
// check or Close, like Farkas core terms.
func (p *PropSolver) CheckAssumptionsWithConsequences(atoms []ast.ASTHandle, wantModel bool) (PropCheckResult, error) {
	if err := p.open(); err != nil {
		return PropCheckResult{}, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return PropCheckResult{}, err
	}
	return p.check(atoms, truth, nil, true, wantModel, true)
}

func (p *PropSolver) Close() (result error) {
	if p == nil || p.manager == nil {
		return ErrPropSolverOwner
	}
	if p.closed {
		return nil
	}
	if p.context != nil {
		result = errors.Join(result, p.context.Close())
	}
	result = errors.Join(result, p.clearCoreReferences())
	for index := len(p.references) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DelRef(&p.references[index]))
	}
	if result == nil {
		p.context = nil
		p.manager = nil
		p.contexts = nil
		p.references = nil
		p.closed = true
	}
	return result
}
