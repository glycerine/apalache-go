package smtcore

// Source: src/smt/smt_model_checker.cpp auxiliary counterexample discovery
// and delayed instance publication.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	coremodel "github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtAuxiliarySolverFactory func() (*SMTSolver, error)

// SMTModelChecker checks universal quantifiers using a cached auxiliary SMT
// solver with Skolem constants and finite-universe restrictions. Counterexamples
// retain expression bindings until restart, when they enter the ordinary
// quantifier queue at the restored base scope.
type SMTModelChecker struct {
	manager          *ast.ASTManager
	state            *SMTInternalizerState
	registry         *SMTQuantifierRegistry
	modelFinder      *SMTModelFinder
	initialMaxCexs   uint32
	maxCexs          uint32
	maxIterations    uint32
	iteration        uint32
	exhausted        bool
	trace            bool
	valueTerms       map[ast.ASTHandle]ast.ASTHandle
	auxiliary        smtAuxiliarySolverFactory
	auxiliaryContext *SMTSolver
	newInstances     []smtModelCheckerInstance
	closed           bool
}

func (c *SMTModelChecker) SetModelFinder(finder *SMTModelFinder) error {
	if err := c.open(); err != nil {
		return err
	}
	if finder == nil || finder.manager != c.manager || finder.state != c.state {
		return ast.ErrASTDomain
	}
	c.modelFinder = finder
	return nil
}

func (c *SMTModelChecker) SetAuxiliarySolverFactory(factory smtAuxiliarySolverFactory) error {
	if err := c.open(); err != nil {
		return err
	}
	c.auxiliary = factory
	return nil
}

func (c *SMTModelChecker) initAuxiliaryContext() error {
	if c.auxiliaryContext != nil || c.auxiliary == nil {
		return nil
	}
	next, err := c.auxiliary()
	if err != nil {
		if next != nil {
			err = errors.Join(err, next.Close())
		}
		return err
	}
	if next == nil {
		return ast.ErrASTHandle
	}
	c.auxiliaryContext = next
	return nil
}

func (c *SMTModelChecker) SetCandidateTerms(rootValues map[*SMTEnode]ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	c.valueTerms = make(map[ast.ASTHandle]ast.ASTHandle, len(rootValues))
	for root, value := range rootValues {
		if root != nil && !value.IsNull() {
			c.valueTerms[value] = root.EqualityClassMinGeneration().owner
		}
	}
	return nil
}

func NewSMTModelChecker(state *SMTInternalizerState, maxCexs uint32) (*SMTModelChecker, error) {
	if state == nil || state.manager == nil || state.quantifiers == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTModelChecker{
		manager: state.manager, state: state, registry: state.quantifiers,
		initialMaxCexs: maxCexs, maxCexs: maxCexs, maxIterations: ^uint32(0),
	}, nil
}

func (c *SMTModelChecker) ConfigureLifecycle(maxCexs, maxIterations uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	c.initialMaxCexs = maxCexs
	c.maxIterations = maxIterations
	return nil
}

func (c *SMTModelChecker) InitSearch() error {
	if err := c.open(); err != nil {
		return err
	}
	c.maxCexs = c.initialMaxCexs
	c.iteration = 0
	c.exhausted = false
	return nil
}

func (c *SMTModelChecker) Exhausted() bool {
	return c != nil && !c.closed && c.exhausted
}

func (c *SMTModelChecker) open() error {
	if c == nil || c.manager == nil || c.state == nil || c.registry == nil || c.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *SMTModelChecker) universe(model *protomodel.ProtoModel, sort ast.ASTHandle) ([]ast.ASTHandle, error) {
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	if sort == boolean {
		f, err := c.manager.MkFalse()
		if err != nil {
			return nil, err
		}
		t, err := c.manager.MkTrue()
		if err != nil {
			return nil, err
		}
		return []ast.ASTHandle{f, t}, nil
	}
	values, err := model.KnownUniverse(sort)
	if err != nil {
		return nil, err
	}
	if len(values) != 0 {
		return values, nil
	}
	value, err := model.SomeValue(sort)
	if err != nil {
		return nil, err
	}
	return []ast.ASTHandle{value}, nil
}

func (c *SMTModelChecker) addCounterexample(quantifier ast.ASTHandle, values []ast.ASTHandle) (bool, error) {
	return c.enqueueCounterexample(quantifier, values, false, 0)
}

func (c *SMTModelChecker) addFiniteCounterexample(quantifier ast.ASTHandle, values []ast.ASTHandle) (bool, error) {
	count, err := c.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return false, err
	}
	if uint32(len(values)) < count {
		return false, ast.ErrASTDomain
	}
	bindings := append([]ast.ASTHandle(nil), values[:count]...)
	if c.modelFinder == nil {
		// Standalone finite checking has no instantiation-set phase. Keep
		// its per-quantifier cap while recovering complete context bindings.
		_, err = c.addCounterexample(quantifier, values[:count])
		return !c.exhausted && err == nil, err
	}
	var maxGeneration uint32
	for i, value := range bindings {
		term, generation, found, err := c.modelFinder.Inverse(quantifier, uint32(i), value)
		if err != nil || !found {
			return false, err
		}
		bindings[i] = term
		if generation > maxGeneration {
			maxGeneration = generation
		}
	}
	// A restricted counterexample must recover every binding from the
	// instantiation sets. Source add_instance(use_inv=true) never combines
	// inverse terms with representatives from the unrestricted context.
	_, err = c.enqueueCounterexample(quantifier, bindings, true, maxGeneration)
	return !c.exhausted && err == nil, err
}

func (c *SMTModelChecker) restrictSkolemToUniverse(auxiliary *SMTSolver, skolem ast.ASTHandle, universe []ast.ASTHandle) (result error) {
	if len(universe) == 0 {
		return ast.ErrASTDomain
	}
	equalities := make([]ast.ASTHandle, 0, len(universe))
	refs := make([]ast.ASTRef, 0, len(universe))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
	}()
	for _, value := range universe {
		equal, err := c.manager.MkEq(skolem, value)
		if err != nil {
			return err
		}
		ref, err := c.manager.NewRef(equal)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		equalities = append(equalities, equal)
	}
	clause, err := c.manager.MkOrArgs(equalities)
	if err != nil {
		return err
	}
	clauseRef, err := c.manager.NewRef(clause)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&clauseRef)) }()
	return auxiliary.Assert(clause)
}

func (c *SMTModelChecker) addAuxiliaryBlockingClause(auxiliary *SMTSolver, skolems, values []ast.ASTHandle) (result error) {
	disequalities := make([]ast.ASTHandle, 0, len(skolems))
	refs := make([]ast.ASTRef, 0, len(skolems))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
	}()
	for i, skolem := range skolems {
		if skolem.IsNull() || values[i].IsNull() {
			continue
		}
		equal, err := c.manager.MkEq(skolem, values[i])
		if err != nil {
			return err
		}
		disequal, err := c.manager.MkNot(equal)
		if err != nil {
			return err
		}
		ref, err := c.manager.NewRef(disequal)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		disequalities = append(disequalities, disequal)
	}
	if len(disequalities) == 0 {
		return ast.ErrASTPending
	}
	clause, err := c.manager.MkOrArgs(disequalities)
	if err != nil {
		return err
	}
	clauseRef, err := c.manager.NewRef(clause)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&clauseRef)) }()
	return auxiliary.Assert(clause)
}

func (c *SMTModelChecker) addInstantiationSetRestrictions(auxiliary *SMTSolver, quantifier ast.ASTHandle, skolems []ast.ASTHandle) (asserted bool, result error) {
	if c.modelFinder == nil {
		return false, nil
	}
	count, err := c.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return false, err
	}
	if uint32(len(skolems)) < count {
		return false, ast.ErrASTDomain
	}
	for i, skolem := range skolems[:count] {
		if skolem.IsNull() {
			continue
		}
		values, err := c.modelFinder.InstantiationValues(quantifier, uint32(i))
		if err != nil {
			return false, err
		}
		if len(values) == 0 {
			continue
		}
		equalities := make([]ast.ASTHandle, 0, len(values))
		refs := make([]ast.ASTRef, 0, len(values))
		for _, value := range values {
			equal, e := c.manager.MkEq(skolem, value)
			if e != nil {
				for j := range refs {
					result = errors.Join(result, c.manager.DelRef(&refs[j]))
				}
				return false, errors.Join(e, result)
			}
			ref, e := c.manager.NewRef(equal)
			if e != nil {
				for j := range refs {
					result = errors.Join(result, c.manager.DelRef(&refs[j]))
				}
				return false, errors.Join(e, result)
			}
			refs = append(refs, ref)
			equalities = append(equalities, equal)
		}
		clause, err := c.manager.MkOrArgs(equalities)
		if err == nil {
			var clauseRef ast.ASTRef
			clauseRef, err = c.manager.NewRef(clause)
			if err == nil {
				err = auxiliary.Assert(clause)
				err = errors.Join(err, c.manager.DelRef(&clauseRef))
			}
		}
		for j := range refs {
			err = errors.Join(err, c.manager.DelRef(&refs[j]))
		}
		if err != nil {
			return false, err
		}
		asserted = true
	}
	return asserted, nil
}

func (c *SMTModelChecker) auxiliaryBindings(cex *coremodel.ModelCore, quantifier ast.ASTHandle, values []ast.ASTHandle, skolemDecls []ast.ASTHandle, requireInverse bool) (raw, bindings []ast.ASTHandle, inverseOK bool, generation uint32, err error) {
	flat := quantifier
	if c.modelFinder != nil {
		flat = c.modelFinder.flatQuantifier(quantifier)
	}
	count, err := c.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return nil, nil, false, 0, err
	}
	raw = append([]ast.ASTHandle(nil), values...)
	for i, declaration := range skolemDecls {
		if declaration.IsNull() {
			continue
		}
		value, e := cex.ConstantInterpretation(declaration)
		if e != nil {
			return nil, nil, false, 0, e
		}
		if value.IsNull() {
			sort, e := c.manager.GetDeclarationSort(flat, uint32(i))
			if e != nil {
				return nil, nil, false, 0, e
			}
			value, e = cex.GetSomeValue(sort)
			if e != nil {
				return nil, nil, false, 0, e
			}
		}
		raw[i] = value
	}
	bindings = append([]ast.ASTHandle(nil), raw...)
	inverseOK = true
	if c.modelFinder == nil {
		return raw, bindings, !requireInverse, generation, nil
	}
	if !requireInverse {
		return raw, bindings, true, generation, nil
	}
	for i := range bindings {
		if uint32(i) >= count {
			continue
		}
		term, inverseGeneration, found, e := c.modelFinder.Inverse(quantifier, uint32(i), bindings[i])
		if e != nil {
			return nil, nil, false, 0, e
		}
		if found {
			bindings[i] = term
			if inverseGeneration > generation {
				generation = inverseGeneration
			}
		} else if requireInverse {
			inverseOK = false
		}
	}
	return raw, bindings, inverseOK, generation, nil
}

func (c *SMTModelChecker) auxiliaryCounterexamples(model *protomodel.ProtoModel, quantifier ast.ASTHandle, remaining *uint32) (found bool, result error) {
	if c.auxiliary == nil {
		return false, nil
	}
	flat := quantifier
	if c.modelFinder != nil {
		flat = c.modelFinder.flatQuantifier(quantifier)
	}
	body, err := c.manager.GetQuantifierBody(flat)
	if err != nil {
		return false, err
	}
	evaluated, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&evaluated)) }()
	if _, err = model.Eval(body, evaluated, true); err != nil {
		return false, err
	}
	evaluatedBody, err := c.manager.RefGet(evaluated)
	if err != nil {
		return false, err
	}
	count, err := c.manager.GetNumDeclarations(flat)
	if err != nil {
		return false, err
	}
	values := make([]ast.ASTHandle, count)
	skolems := make([]ast.ASTHandle, count)
	skolemDecls := make([]ast.ASTHandle, len(skolems))
	skolemRefs := make([]ast.ASTRef, 0, len(skolems))
	defer func() {
		for i := len(skolemRefs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&skolemRefs[i]))
		}
	}()
	// Native creates the innermost variable's fresh constant first while
	// retaining declaration order in the Skolem/substitution vectors.
	for i := len(skolems) - 1; i >= 0; i-- {
		sort, e := c.manager.GetDeclarationSort(flat, uint32(i))
		if e != nil {
			return false, e
		}
		skolems[i], e = c.manager.MkFreshConst("mbqi", sort)
		if e != nil {
			return false, e
		}
		ref, e := c.manager.NewRef(skolems[i])
		if e != nil {
			return false, e
		}
		skolemRefs = append(skolemRefs, ref)
		skolemDecls[i], e = c.manager.GetDecl(skolems[i])
		if e != nil {
			return false, e
		}
	}
	substitution, err := rewriter.NewVarSubstitution(c.manager, true)
	// Member construction can return a charged partial owner. Retain its
	// cleanup even when construction stops before the reducer is ready.
	if substitution != nil {
		defer func() { result = errors.Join(result, substitution.Close()) }()
	}
	if err != nil {
		return false, err
	}
	ground, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&ground)) }()
	if err = substitution.Substitute(evaluatedBody, skolems, ground); err != nil {
		return false, err
	}
	groundBody, err := c.manager.RefGet(ground)
	if err != nil {
		return false, err
	}
	negated, err := c.manager.MkNot(groundBody)
	if err != nil {
		return false, err
	}
	negatedRef, err := c.manager.NewRef(negated)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&negatedRef)) }()
	if err = c.initAuxiliaryContext(); err != nil {
		return false, err
	}
	auxiliary := c.auxiliaryContext
	if err = auxiliary.PushCore(); err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, auxiliary.PopCore(1)) }()
	for i := len(skolems) - 1; i >= 0; i-- {
		skolem := skolems[i]
		sort, e := c.manager.GetDeclarationSort(flat, uint32(i))
		if e != nil {
			return false, e
		}
		finite, e := model.IsFinite(sort)
		if e != nil {
			return false, e
		}
		if finite {
			universe, e := model.KnownUniverse(sort)
			if e != nil {
				return false, e
			}
			if e = c.restrictSkolemToUniverse(auxiliary, skolem, universe); e != nil {
				return false, e
			}
		}
	}
	if err = auxiliary.Assert(negated); err != nil {
		return false, err
	}
	status, err := auxiliary.CheckSat(nil)
	if err != nil {
		return false, err
	}
	if status == lbool.False {
		return false, nil
	}
	if status != lbool.True {
		c.exhausted = true
		return false, nil
	}
	complete, err := auxiliary.AcquireModel()
	if err != nil {
		return false, err
	}
	if complete == nil {
		return false, ast.ErrASTPending
	}
	defer func() { result = errors.Join(result, complete.DecRef()) }()
	restricted, err := c.addInstantiationSetRestrictions(auxiliary, quantifier, skolems)
	if err != nil {
		return false, err
	}
	// Native attempts inverse recovery even when no instantiation-set clause
	// was added, then falls back to the saved complete counterexample.
	useInverse := c.modelFinder != nil
	first := complete
	for *remaining != 0 {
		cex := first
		first = nil
		if useInverse || restricted || cex == nil {
			status, err = auxiliary.CheckSat(nil)
			if err != nil {
				return false, err
			}
			if status == lbool.False {
				break
			}
			if status != lbool.True {
				// The complete query already supplied a valid witness. Native
				// stops restricted enumeration and can still use that witness.
				break
			}
			cex, err = auxiliary.AcquireModel()
			if err != nil {
				return false, err
			}
			if cex == nil {
				return false, ast.ErrASTPending
			}
		}
		raw, bindings, inverseOK, generation, err := c.auxiliaryBindings(cex, quantifier, values, skolemDecls, useInverse)
		if err != nil {
			if cex != complete {
				err = errors.Join(err, cex.DecRef())
			}
			return false, err
		}
		if useInverse && !inverseOK {
			if cex != complete {
				if err = cex.DecRef(); err != nil {
					return false, err
				}
			}
			break
		}
		inserted, err := c.enqueueCounterexample(quantifier, bindings, useInverse, generation)
		if err != nil {
			if cex != complete {
				err = errors.Join(err, cex.DecRef())
			}
			return false, err
		}
		if inserted {
			found = true
		}
		if !c.exhausted {
			*remaining--
		}
		if *remaining != 0 {
			if err = c.addAuxiliaryBlockingClause(auxiliary, skolems, raw); err != nil {
				if cex != complete {
					err = errors.Join(err, cex.DecRef())
				}
				return false, err
			}
		}
		if cex != complete {
			if err = cex.DecRef(); err != nil {
				return false, err
			}
		}
		if c.exhausted {
			break
		}
	}
	if useInverse && !found && *remaining != 0 {
		_, bindings, _, _, err := c.auxiliaryBindings(complete, quantifier, values, skolemDecls, false)
		if err != nil {
			return false, err
		}
		inserted, err := c.addCounterexample(quantifier, bindings)
		if err != nil {
			return false, err
		}
		if inserted {
			found = true
			*remaining--
		}
	}
	// The complete auxiliary check already proved that the candidate
	// violates this quantifier, even if its binding was queued previously.
	return true, nil
}

func (c *SMTModelChecker) checkQuantifier(model *protomodel.ProtoModel, quantifier ast.ASTHandle, remaining *uint32) (bool, error) {
	flat := quantifier
	if c.modelFinder != nil {
		flat = c.modelFinder.flatQuantifier(quantifier)
	}
	count, err := c.manager.GetNumDeclarations(flat)
	if err != nil {
		return false, err
	}
	if c.auxiliary != nil {
		return c.auxiliaryCounterexamples(model, quantifier, remaining)
	}
	// Standalone typed callers can check a supplied model without a solver.
	universes := make([][]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		sort, err := c.manager.GetDeclarationSort(flat, i)
		if err != nil {
			return false, err
		}
		universes[i], err = c.universe(model, sort)
		if err != nil {
			return false, err
		}
	}
	values := make([]ast.ASTHandle, count)
	found := false
	restricted := false
	var complete []ast.ASTHandle
	var visit func(uint32) error
	visit = func(index uint32) (result error) {
		if *remaining == 0 {
			return nil
		}
		if index < count {
			for _, value := range universes[index] {
				values[index] = value
				if err := visit(index + 1); err != nil || *remaining == 0 {
					return err
				}
			}
			return nil
		}
		instance, err := c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, c.manager.DelRef(&instance)) }()
		if err = rewriter.InstantiateQuantifier(c.manager, flat, values, instance); err != nil {
			return err
		}
		term, err := c.manager.RefGet(instance)
		if err != nil {
			return err
		}
		evaluated, err := c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, c.manager.DelRef(&evaluated)) }()
		if _, err = model.Eval(term, evaluated, true); err != nil {
			return err
		}
		value, err := c.manager.RefGet(evaluated)
		if err != nil {
			return err
		}
		if c.manager.IsTrue(value) {
			return nil
		}
		if !c.manager.IsFalse(value) {
			c.exhausted = true
			return nil
		}
		found = true
		if complete == nil {
			complete = append([]ast.ASTHandle{}, values...)
		}
		accepted, err := c.addFiniteCounterexample(quantifier, values)
		if err != nil {
			return err
		}
		if accepted {
			restricted = true
			*remaining--
		}
		return nil
	}
	if err = visit(0); err != nil {
		return false, err
	}
	if found && !restricted && *remaining != 0 {
		// No restricted witness exists. Recover the entire complete witness
		// from the candidate context, as in source check's complete_cex path.
		_, err = c.addCounterexample(quantifier, complete)
		if err != nil {
			return false, err
		}
		if !c.exhausted {
			*remaining--
		}
	}
	return found, nil
}

// Check returns true when every supplied quantifier holds in the candidate
// finite model. A false result means a counterexample was found or checking could
// not safely finish; Exhausted distinguishes the latter case.
func (c *SMTModelChecker) Check(model *protomodel.ProtoModel, quantifiers []ast.ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if model == nil || model.Core() == nil {
		return false, ast.ErrASTHandle
	}
	// The supplied list is filtered by relevance, assignment and MBQI ID.
	// Source skips the whole lifecycle only when the registry itself is empty.
	// Standalone callers may supply quantifiers without registering them.
	if c.registry.Empty() && len(quantifiers) == 0 {
		return true, nil
	}
	if c.iteration >= c.maxIterations {
		c.exhausted = true
		return false, c.reportIterationLimit()
	}
	if err := model.Compress(); err != nil {
		return false, err
	}
	if c.trace {
		if err := writeMBQIDiagnostic("(smt.mbqi \"started\")\n"); err != nil {
			return false, err
		}
	}
	if err := c.initAuxiliaryContext(); err != nil {
		return false, err
	}
	c.exhausted = false
	if len(quantifiers) != 0 {
		c.iteration++
	}
	failures, err := c.checkQuantifiers(model, quantifiers, false)
	if err != nil {
		return false, err
	}
	c.maxCexs += c.initialMaxCexs
	if failures == 0 && !c.exhausted {
		valid, err := c.validateGroundAssignments(model)
		if err != nil {
			return false, err
		}
		if !valid {
			strictFailures, err := c.checkQuantifiers(model, quantifiers, true)
			if err != nil {
				return false, err
			}
			failures = 1 + strictFailures
		}
	}
	if failures == 0 && !c.exhausted {
		if err := model.Cleanup(); err != nil {
			return false, err
		}
	}
	if c.trace {
		if err := c.reportResult(failures); err != nil {
			return false, err
		}
	}
	return failures == 0 && !c.exhausted, nil
}

func (c *SMTModelChecker) checkQuantifiers(md *protomodel.ProtoModel, quantifiers []ast.ASTHandle, strict bool) (failures uint32, err error) {
	for _, q := range quantifiers {
		if err = c.reportQuantifier("checking", q, true); err != nil {
			return 0, err
		}
		recursive, err := c.manager.IsRecursiveFunctionDefinition(q)
		if err != nil {
			return 0, err
		}
		if recursive {
			satisfied, err := c.checkRecursiveFunction(md, q, strict)
			if err != nil {
				return 0, err
			}
			if !satisfied {
				failures++
			}
			continue
		}
		remaining := c.maxCexs
		if remaining == 0 {
			remaining = 1
		}
		// Preserve the aggregate give-up flag while counting the outcome of
		// this quantifier independently, as native check_quantifiers does.
		previouslyExhausted := c.exhausted
		c.exhausted = false
		added, err := c.checkQuantifier(md, q, &remaining)
		incomplete := c.exhausted
		c.exhausted = previouslyExhausted || incomplete
		if err != nil {
			return 0, err
		}
		if added || incomplete {
			failures++
			if err = c.reportQuantifier("failed", q, false); err != nil {
				return 0, err
			}
		}
	}
	return failures, nil
}

func (c *SMTModelChecker) Close() error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	result := c.ResetNewInstances()
	if c.auxiliaryContext != nil {
		result = errors.Join(result, c.auxiliaryContext.Close())
		c.auxiliaryContext = nil
	}
	c.closed = true
	c.state = nil
	c.registry = nil
	c.modelFinder = nil
	c.valueTerms = nil
	c.auxiliary = nil
	c.manager = nil
	return result
}
