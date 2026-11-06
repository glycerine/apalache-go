package smtcore

// Source: src/smt/theory_datatype.cpp and src/smt/theory_datatype.h retained
// constructor, accessor and recognizer axioms.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/backtrack"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtDatatypeScope struct {
	unsupported bool
	mergeError  error
}

type smtDatatypeStatistics struct {
	constructorAxioms, accessorAxioms, updateAxioms, splits, occursChecks uint32
}

// SMTDatatypeTheory encodes constructor cases with recognizer clauses and
// publishes reconstruction and accessor equalities through the shared egraph.
type SMTDatatypeTheory struct {
	*SMTTheoryBase
	trace          *diagnostic.TraceTags
	parameterOwner *SMTDatatypeParams
	utility        *ast.DatatypeUtil
	find           *backtrack.UnionFind
	findTrail      *backtrack.TrailStack
	data           []*smtDatatypeVarData
	mergeError     error
	scopes         []smtDatatypeScope
	unsupported    bool
	statistics     smtDatatypeStatistics
}

func (t *SMTDatatypeTheory) lazySplits() uint32 {
	if t.parameterOwner != nil {
		return t.parameterOwner.LazySplits
	}
	return 1
}

func NewSMTDatatypeTheory(family ast.FamilyID) (*SMTDatatypeTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	t := &SMTDatatypeTheory{SMTTheoryBase: base, findTrail: backtrack.NewTrailStack()}
	find, err := backtrack.NewUnionFind(&smtDatatypeUnionContext{theory: t})
	if err != nil {
		return nil, err
	}
	t.find = find
	return t, nil
}

func (t *SMTDatatypeTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	if kind, kindErr := state.manager.GetKind(expression); kindErr != nil || kind != ast.ASTApp {
		return 0, false, kindErr
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

func (t *SMTDatatypeTheory) recognizerApplication(constructor, value ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parameter, err := state.manager.NewParameterAST(constructor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := state.manager.MkFamilyApp(t.FamilyID(), ast.ASTDatatypeIs, []ast.ASTParameter{parameter}, []ast.ASTHandle{value})
	deleteErr := state.manager.DelParameter(&parameter)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if deleteErr != nil {
		return ast.ASTHandle{}, deleteErr
	}
	return application, nil
}

func (t *SMTDatatypeTheory) recognizerLiteral(constructor, value ast.ASTHandle) (SMTLiteral, ast.ASTHandle, error) {
	application, err := t.recognizerApplication(constructor, value)
	if err != nil {
		return SMTNullLiteral, ast.ASTHandle{}, err
	}
	state, _ := t.State()
	if err = state.Internalize(application, false); err != nil {
		return SMTNullLiteral, ast.ASTHandle{}, err
	}
	literal, err := state.Literal(application)
	return literal, application, err
}

func (t *SMTDatatypeTheory) publishConstructor(value, constructor ast.ASTHandle) error {
	t.statistics.accessorAxioms++
	state, _ := t.State()
	accessors, err := state.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return err
	}
	count, err := state.manager.GetNumArgs(value)
	if err != nil || int(count) != len(accessors) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	for i, accessor := range accessors {
		selected, selectErr := state.manager.MkApp(accessor, []ast.ASTHandle{value})
		if selectErr != nil {
			return selectErr
		}
		argument, argumentErr := state.manager.GetArg(value, uint32(i))
		if argumentErr != nil {
			return argumentErr
		}
		if argumentErr = t.equalityAxiom(argument, selected, SMTNullLiteral); argumentErr != nil {
			return argumentErr
		}
	}
	return nil
}

func (t *SMTDatatypeTheory) publishUpdate(node *SMTEnode) error {
	t.statistics.updateAxioms++
	if len(node.arguments) != 2 {
		return ast.ErrASTDomain
	}
	state, _ := t.State()
	declaration, err := state.manager.GetDecl(node.owner)
	if err != nil {
		return fmt.Errorf("datatype update declaration: %w", err)
	}
	updatedAccessor, matched, err := state.manager.DatatypeUpdateAccessor(declaration)
	if err != nil || !matched {
		if err != nil {
			return fmt.Errorf("datatype update accessor: %w", err)
		}
		return ast.ErrASTDomain
	}
	sort, err := state.manager.GetSort(node.arguments[0].owner)
	if err != nil {
		return fmt.Errorf("datatype update base sort: %w", err)
	}
	constructors, err := state.manager.DatatypeConstructors(sort)
	if err != nil {
		return fmt.Errorf("datatype update constructors: %w", err)
	}
	var constructor ast.ASTHandle
	for _, candidate := range constructors {
		_, same, indexErr := state.manager.DatatypeAccessorIndex(updatedAccessor, candidate)
		if indexErr != nil {
			return fmt.Errorf("datatype update accessor index: %w", indexErr)
		}
		if same {
			constructor = candidate
			break
		}
	}
	if constructor.IsNull() {
		return ast.ErrASTDomain
	}
	recognizer, _, err := t.recognizerLiteral(constructor, node.arguments[0].owner)
	if err != nil {
		return fmt.Errorf("datatype update recognizer: %w", err)
	}
	accessors, err := state.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return fmt.Errorf("datatype update constructor accessors: %w", err)
	}
	for _, accessor := range accessors {
		selected, selectErr := state.manager.MkApp(accessor, []ast.ASTHandle{node.owner})
		if selectErr != nil {
			return fmt.Errorf("datatype update result accessor: %w", selectErr)
		}
		right := node.arguments[1].owner
		if accessor != updatedAccessor {
			right, selectErr = state.manager.MkApp(accessor, []ast.ASTHandle{node.arguments[0].owner})
			if selectErr != nil {
				return fmt.Errorf("datatype update base accessor: %w", selectErr)
			}
			if selectErr = state.Internalize(right, false); selectErr != nil {
				return selectErr
			}
		}
		if err = t.equalityAxiom(right, selected, recognizer); err != nil {
			return fmt.Errorf("datatype update field equality: %w", err)
		}
	}
	if err = t.equalityAxiom(node.owner, node.arguments[0].owner, recognizer.Negated()); err != nil {
		return fmt.Errorf("datatype update identity: %w", err)
	}
	return nil
}

func (t *SMTDatatypeTheory) ensureVariable(node *SMTEnode) error {
	if t.IsAttachedToVariable(node) {
		return nil
	}
	_, err := t.CreateVariable(node)
	return err
}

func (t *SMTDatatypeTheory) CreateVariable(node *SMTEnode) (SMTTheoryVar, error) {
	variable, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return variable, err
	}
	if t.find.MkVar() != uint32(variable) {
		return variable, ast.ErrASTDomain
	}
	data := &smtDatatypeVarData{}
	t.data = append(t.data, data)
	return variable, t.initializeVariable(node, data)
}

func (t *SMTDatatypeTheory) initializeVariable(node *SMTEnode, data *smtDatatypeVarData) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(node.owner)
	if err != nil {
		return err
	}
	constructor, matched, err := state.manager.DatatypeConstructorApp(node.owner)
	if err != nil {
		return err
	}
	if matched {
		data.constructor = node
		return t.publishConstructor(node.owner, constructor)
	}
	if operation, own, err := t.operation(node.owner); err != nil {
		return err
	} else if own && operation == ast.ASTDatatypeUpdateField {
		return t.publishUpdate(node)
	}
	constructors, err := state.manager.DatatypeConstructors(sort)
	if err != nil {
		return err
	}
	if len(constructors) == 1 {
		// Source mk_var asserts reconstruction unconditionally for a
		// one-constructor datatype. Waiting for a recognizer assignment can
		// omit it when that auxiliary recognizer is not relevant.
		return t.assertConstructor(node.owner, constructors[0], SMTNullLiteral)
	}
	infinite, err := state.manager.SortIsInfinite(sort)
	if err != nil {
		return err
	}
	if t.lazySplits() == 0 || t.lazySplits() == 1 && !infinite {
		return t.split(node.owner, sort)
	}
	return nil
}

func (t *SMTDatatypeTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	if err := t.traceDatatypeExpression("internalize_atom", 258, "internalizing atom:\n", expression); err != nil {
		return false, err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own || op < ast.ASTDatatypeConstructor || op > ast.ASTDatatypeUpdateField {
		return false, err
	}
	return t.InternalizeTerm(expression)
}

func (t *SMTDatatypeTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	if err := t.traceDatatypeExpression("internalize_term", 263, "internalizing term:\n", expression); err != nil {
		return false, err
	}
	operation, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	if operation < ast.ASTDatatypeConstructor || operation > ast.ASTDatatypeUpdateField {
		return false, nil
	}
	state, _ := t.State()
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		argument, argumentErr := state.manager.GetArg(expression, i)
		if argumentErr != nil {
			return false, argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return false, argumentErr
		}
		if state.context.enodes[argument] == nil && (state.manager.IsTrue(argument) || state.manager.IsFalse(argument)) {
			if _, argumentErr = state.MkEnode(argument, false, true, false); argumentErr != nil {
				return false, fmt.Errorf("datatype Boolean value enode: %w", argumentErr)
			}
		}
	}
	node := state.context.enodes[expression]
	// Source internalize_term can encounter an enode created while its
	// arguments were internalized; that term must not be initialized twice.
	if node != nil {
		return true, nil
	}
	if node == nil {
		boolean, booleanErr := state.isBoolean(expression)
		if booleanErr != nil {
			return false, booleanErr
		}
		node, err = state.MkEnode(expression, false, boolean, true)
		if err != nil {
			return false, fmt.Errorf("datatype enode: %w", err)
		}
	}
	if node.IsBool() {
		// Source creates argument enodes first, then this enode, then its
		// Boolean variable and enode flag, including in gate context.
		variable, found := state.BoolVar(expression)
		if !found {
			variable, err = state.MkBoolVar(expression)
			if err != nil {
				return false, err
			}
		}
		if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
			return false, err
		}
		if err = state.ensureBooleanEnode(expression, variable, false, true, true); err != nil {
			return false, err
		}
	}
	if operation == ast.ASTDatatypeConstructor || operation == ast.ASTDatatypeUpdateField {
		for _, argument := range node.arguments {
			sort, sortErr := state.manager.GetSort(argument.owner)
			if sortErr != nil {
				return false, sortErr
			}
			if family, familyErr := state.manager.GetFamilyID(sort); familyErr == nil && family == t.FamilyID() {
				if sortErr = t.ensureVariable(argument); sortErr != nil {
					return false, fmt.Errorf("datatype argument variable: %w", sortErr)
				}
			}
		}
		if err = t.ensureVariable(node); err != nil {
			return false, fmt.Errorf("datatype result variable: %w", err)
		}
	} else if len(node.arguments) == 1 {
		if err = t.ensureVariable(node.arguments[0]); err != nil {
			return false, err
		}
	}
	if (operation == ast.ASTDatatypeRecognizer || operation == ast.ASTDatatypeIs) && (state.relevancy == nil || !state.relevancy.enabled) {
		if err = t.addRecognizer(node.arguments[0].TheoryVar(t.FamilyID()), node); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (*SMTDatatypeTheory) InternalizeEquality(ast.ASTHandle, int32) error { return nil }

func (t *SMTDatatypeTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if node == nil {
		return ast.ErrASTDomain
	}
	if err = t.emitDatatype("datatype", "apply_sort_cnstr", 343, func(out io.Writer) error {
		_, _ = io.WriteString(out, "apply_sort_cnstr: #")
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%d\n", id)
		return err
	}); err != nil {
		return err
	}
	if err = t.traceDatatypeTaggedExpression("datatype_bug", "apply_sort_cnstr", 344, "apply_sort_cnstr:\n", node.owner); err != nil {
		return err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() || t.IsAttachedToVariable(node) {
		return err
	}
	infinite, err := state.manager.SortIsInfinite(sort)
	if err != nil {
		return err
	}
	stateHasQuantifiers := state.assertedHasQuantifiers != nil && *state.assertedHasQuantifiers
	if infinite && !stateHasQuantifiers {
		return nil
	}
	return t.ensureVariable(node)
}

func (*SMTDatatypeTheory) UseDisequalities() bool { return false }
func (t *SMTDatatypeTheory) NewEquality(left, right SMTTheoryVar) error {
	if left < 0 || right < 0 || uint32(left) >= t.find.GetNumVars() || uint32(right) >= t.find.GetNumVars() {
		return ast.ErrASTDomain
	}
	t.find.Merge(uint32(left), uint32(right))
	return t.mergeError
}
func (*SMTDatatypeTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error { return ast.ErrASTDomain }

func (t *SMTDatatypeTheory) Assign(variable int32, isTrue bool) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	expression, err := state.BoolExpression(variable)
	if err != nil {
		return err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own || op != ast.ASTDatatypeRecognizer && op != ast.ASTDatatypeIs {
		return err
	}
	recognizerNode := state.context.enodes[expression]
	if recognizerNode == nil {
		return ast.ErrASTDomain
	}
	if err = t.emitDatatype("datatype", "assign_eh", 367, func(out io.Writer) error {
		_, _ = io.WriteString(out, "assigning recognizer: #")
		id, err := recognizerNode.OwnerID()
		if err != nil {
			return err
		}
		truth := 0
		if isTrue {
			truth = 1
		}
		_, _ = fmt.Fprintf(out, "%d is_true: %d\n", id, truth)
		if err = t.displayDatatypeBounded(out, expression); err != nil {
			return err
		}
		_, err = io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	declaration, err := state.manager.GetDecl(expression)
	if err != nil {
		return err
	}
	constructor, matched, err := state.manager.DatatypeRecognizerConstructor(declaration)
	if err != nil || !matched {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	value, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	node := state.context.enodes[value]
	if node == nil {
		return ast.ErrASTDomain
	}
	root, data, err := t.rootData(node.TheoryVar(t.FamilyID()))
	if err != nil {
		return err
	}
	known := data.constructor
	if !isTrue {
		if known != nil {
			if known.declaration == constructor {
				return t.signRecognizerConflict(known, state.context.enodes[expression])
			}
			return nil
		}
		return t.propagateRecognizer(SMTTheoryVar(root), recognizerNode)
	}
	if known != nil && known.declaration == constructor {
		return nil
	}
	if err = t.assertConstructor(value, constructor, NewSMTLiteral(variable, false)); err != nil {
		return err
	}
	return nil
}

// Source assert_is_constructor_axiom; antecedent is null for the eager
// single-constructor case and the recognizer literal for assigned cases.
func (t *SMTDatatypeTheory) assertConstructor(value, constructor ast.ASTHandle, antecedent SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.traceDatatypeTaggedExpression("datatype_bug", "assert_is_constructor_axiom", 129, "creating axiom (= n (c (acc_1 n) ... (acc_m n))) for\n", value); err != nil {
		return err
	}
	t.statistics.constructorAxioms++
	accessors, err := state.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return err
	}
	arguments := make([]ast.ASTHandle, len(accessors))
	for i, accessor := range accessors {
		arguments[i], err = state.manager.MkApp(accessor, []ast.ASTHandle{value})
		if err != nil {
			return err
		}
	}
	constructed, err := state.manager.MkApp(constructor, arguments)
	if err != nil {
		return err
	}
	return t.equalityAxiom(value, constructed, antecedent)
}

func (t *SMTDatatypeTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.findTrail.PushScope()
	t.scopes = append(t.scopes, smtDatatypeScope{
		unsupported: t.unsupported, mergeError: t.mergeError,
	})
	return nil
}

func (t *SMTDatatypeTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	oldVars, err := t.OldNumVariables(count)
	if err != nil {
		return err
	}
	if err := t.findTrail.PopScope(count); err != nil {
		return err
	}
	clear(t.data[oldVars:])
	t.data = t.data[:oldVars]
	t.unsupported = scope.unsupported
	t.mergeError = scope.mergeError
	t.scopes = t.scopes[:level]
	return t.SMTTheoryBase.PopScopes(count)
}

// Source final_check_eh: visit datatype union-find roots in variable order,
// checking each root for cycles before requesting its lazy split.
func (t *SMTDatatypeTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.unsupported {
		return SMTFinalCheckGiveUp, nil
	}
	state, err := t.State()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	check := smtDatatypeOccursCheck{
		theory: t, onStack: make(map[*SMTEnode]bool),
		cycleFree: make(map[*SMTEnode]bool), parent: make(map[*SMTEnode]*SMTEnode),
	}
	result := SMTFinalCheckDone
	count := len(t.variables)
	for v := 0; v < count; v++ {
		if !t.find.IsRoot(uint32(v)) {
			continue
		}
		node := t.variables[v]
		if !check.cycleFree[node.root] {
			cycle, err := check.run(node)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if cycle {
				return SMTFinalCheckContinue, nil
			}
		}
		if t.lazySplits() > 0 {
			if t.data[v].constructor == nil {
				sort, err := state.manager.GetSort(node.owner)
				if err != nil {
					return SMTFinalCheckGiveUp, err
				}
				if err := t.split(node.owner, sort); err != nil {
					return SMTFinalCheckGiveUp, err
				}
				result = SMTFinalCheckContinue
			}
		}
	}
	return result, nil
}

func (*SMTDatatypeTheory) Name() string { return "datatype" }

func (t *SMTDatatypeTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint("datatype occurs check", t.statistics.occursChecks)
	target.UpdateUint("datatype splits", t.statistics.splits)
	target.UpdateUint("datatype constructor ax", t.statistics.constructorAxioms)
	target.UpdateUint("datatype accessor ax", t.statistics.accessorAxioms)
	target.UpdateUint("datatype update ax", t.statistics.updateAxioms)
	return nil
}

func (t *SMTDatatypeTheory) ResetStatistics() {
	if t != nil {
		t.statistics = smtDatatypeStatistics{}
	}
}

func (t *SMTDatatypeTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewDatatypeFactory(generator.model)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

type smtDatatypeValueProcedure struct {
	constructor  ast.ASTHandle
	dependencies []SMTModelValueDependency
}

func (p *smtDatatypeValueProcedure) Dependencies() []SMTModelValueDependency {
	if p == nil {
		return nil
	}
	return append([]SMTModelValueDependency(nil), p.dependencies...)
}

func (p *smtDatatypeValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || generator == nil || p.constructor.IsNull() || len(values) != len(p.dependencies) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return generator.manager.MkApp(p.constructor, values)
}

func (*smtDatatypeValueProcedure) IsFresh() bool { return false }

func (t *SMTDatatypeTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	if node == nil || node.manager != state.manager || generator == nil || generator.manager != state.manager || generator.model == nil {
		return nil, ast.ErrASTDomain
	}
	_, data, err := t.rootData(node.TheoryVar(t.FamilyID()))
	if err != nil {
		return nil, err
	}
	constructor := data.constructor
	if constructor == nil {
		return nil, ast.ErrASTDomain
	}
	dependencies := make([]SMTModelValueDependency, 0, len(constructor.arguments))
	for _, argument := range constructor.arguments {
		dependency, err := NewSMTEnodeValueDependency(argument)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	return &smtDatatypeValueProcedure{constructor: constructor.declaration, dependencies: dependencies}, nil
}
