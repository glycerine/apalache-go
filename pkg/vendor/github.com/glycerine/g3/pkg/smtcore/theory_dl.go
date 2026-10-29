package smtcore

// Source: src/smt/theory_dl.cpp finite-domain reduction to bit-vectors.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type smtDatalogDeclarations struct {
	rep  ast.ASTHandle
	abs  ast.ASTHandle
	bits uint32
}

type smtDatalogScope struct{ declarations, references int }

// SMTDatalogTheory injects each retained finite datalog sort into a bit-vector
// at the source width of 64 bits and defines strict order by unsigned comparison.
type SMTDatalogTheory struct {
	*SMTTheoryBase
	declarations     map[ast.ASTHandle]smtDatalogDeclarations
	representatives  map[ast.ASTHandle]ast.ASTHandle
	abstractions     map[ast.ASTHandle]ast.ASTHandle
	declarationRefs  []ast.ASTRef
	declarationTrail []ast.ASTHandle
	scopes           []smtDatalogScope
	trace            *diagnostic.TraceTags
}

func NewSMTDatalogTheory(family ast.FamilyID) (*SMTDatalogTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTDatalogTheory{
		SMTTheoryBase:   base,
		declarations:    make(map[ast.ASTHandle]smtDatalogDeclarations),
		representatives: make(map[ast.ASTHandle]ast.ASTHandle),
		abstractions:    make(map[ast.ASTHandle]ast.ASTHandle),
	}, nil
}

func (t *SMTDatalogTheory) declarationsFor(sort ast.ASTHandle) (smtDatalogDeclarations, error) {
	if declarations, found := t.declarations[sort]; found {
		return declarations, nil
	}
	state, err := t.State()
	if err != nil {
		return smtDatalogDeclarations{}, err
	}
	if err = datalogAssert(205, "!m_reps.contains(s)", func() bool { _, found := t.representatives[sort]; return !found }); err != nil {
		return smtDatalogDeclarations{}, err
	}
	const width uint32 = 64
	bitVector, err := state.manager.BitVectorSort(width)
	if err != nil {
		return smtDatalogDeclarations{}, err
	}
	rep, err := state.manager.MkDatalogRepDecl(sort, bitVector)
	if err != nil {
		return smtDatalogDeclarations{}, err
	}
	abs, err := state.manager.MkDatalogAbsDecl(bitVector, sort)
	if err != nil {
		return smtDatalogDeclarations{}, err
	}
	// Source get_rep publishes both raw declaration maps before add_trail
	// acquires either AST reference. Preserve that observable failure prefix;
	// the completed record below is published only after both trail owners.
	t.representatives[sort] = rep
	t.abstractions[sort] = abs
	repRef, err := state.manager.NewRef(rep)
	if err != nil {
		return smtDatalogDeclarations{}, err
	}
	t.declarationRefs = append(t.declarationRefs, repRef)
	absRef, err := state.manager.NewRef(abs)
	if err != nil {
		return smtDatalogDeclarations{}, err
	}
	t.declarationRefs = append(t.declarationRefs, absRef)
	declarations := smtDatalogDeclarations{rep: rep, abs: abs, bits: width}
	t.declarations[sort] = declarations
	t.declarationTrail = append(t.declarationTrail, sort)
	return declarations, nil
}

func (t *SMTDatalogTheory) rep(expression ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declarations, err := t.declarationsFor(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkApp(declarations.rep, []ast.ASTHandle{expression})
}

func (t *SMTDatalogTheory) ensureVariable(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
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
	}
	node := state.context.enodes[expression]
	if node == nil {
		node, err = state.MkEnode(expression, false, false, true)
		if err != nil {
			return false, err
		}
	}
	if t.IsAttachedToVariable(node) {
		return false, nil
	}
	if err = t.traceDatalogExpression("mk_rep", 234, expression); err != nil {
		return false, err
	}
	_, err = t.CreateVariable(node)
	return err == nil, err
}

func (t *SMTDatalogTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if err = t.traceDatalogExpression("internalize_term", 140, expression); err != nil {
		return false, err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	finite, err := state.manager.IsDatalogFiniteSort(sort)
	if err != nil || !finite {
		return false, err
	}
	return t.ensureVariable(expression)
}

func (t *SMTDatalogTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if err = t.traceDatalogExpression("internalize_atom", 117, expression); err != nil {
		return false, err
	}
	if _, found := state.BoolVar(expression); found {
		return true, nil
	}
	less, err := state.manager.IsDatalogLT(expression)
	if err != nil || !less {
		return false, err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	if err = state.Internalize(left, false); err != nil {
		return false, err
	}
	if err = state.Internalize(right, false); err != nil {
		return false, err
	}
	variable, found := state.BoolVar(expression)
	if !found {
		variable, err = state.MkBoolVar(expression)
		if err != nil {
			return false, err
		}
	}
	data, err := state.BooleanData(variable)
	if err != nil {
		return false, err
	}
	if !data.IsTheoryAtom() {
		if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
			return false, err
		}
	}
	leftRep, err := t.rep(left)
	if err != nil {
		return false, err
	}
	rightRep, err := t.rep(right)
	if err != nil {
		return false, err
	}
	reverseLE, err := state.manager.MkBitVectorULE(rightRep, leftRep)
	if err != nil {
		return false, err
	}
	if err = state.Internalize(expression, false); err != nil {
		return false, err
	}
	if err = state.Internalize(reverseLE, false); err != nil {
		return false, err
	}
	lessLiteral := NewSMTLiteral(variable, false)
	leLiteral, err := state.Literal(reverseLE)
	if err != nil {
		return false, err
	}
	if err = state.markRelevant(expression); err != nil {
		return false, err
	}
	if leLiteral.Var() != SMTTrueBoolVar {
		atom, e := state.BoolExpression(leLiteral.Var())
		if e != nil {
			return false, e
		}
		if e = state.markRelevant(atom); e != nil {
			return false, e
		}
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{lessLiteral, leLiteral}); err != nil {
		return false, err
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{lessLiteral.Negated(), leLiteral.Negated()}); err != nil {
		return false, err
	}
	return true, nil
}

func (*SMTDatalogTheory) InternalizeEquality(ast.ASTHandle, int32) error { return nil }
func (*SMTDatalogTheory) NewEquality(SMTTheoryVar, SMTTheoryVar) error   { return nil }
func (*SMTDatalogTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error {
	return nil
}
func (*SMTDatalogTheory) Assign(int32, bool) error { return nil }

func (t *SMTDatalogTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	if node == nil {
		return ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	finite, err := state.manager.IsDatalogFiniteSort(sort)
	if err != nil || !finite {
		return err
	}
	_, err = t.ensureVariable(node.owner)
	return err
}

func (t *SMTDatalogTheory) Relevant(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	finite, err := state.manager.IsDatalogFiniteSort(sort)
	if err != nil || !finite {
		return err
	}
	declarations, err := t.declarationsFor(sort)
	if err != nil {
		return err
	}
	declaration, err := state.manager.GetDecl(expression)
	if err != nil {
		return err
	}
	if declaration == declarations.abs {
		return nil
	}
	representative, err := t.rep(expression)
	if err != nil {
		return err
	}
	value, numeral, err := state.manager.DatalogNumeralValue(expression)
	if err != nil {
		return err
	}
	var constraint ast.ASTHandle
	if numeral {
		constant, constantErr := state.manager.BitVectorNumeralUint64(value, declarations.bits)
		if constantErr != nil {
			return constantErr
		}
		constraint, err = state.manager.MkEq(representative, constant)
	} else {
		abstracted, applicationErr := state.manager.MkApp(declarations.abs, []ast.ASTHandle{representative})
		if applicationErr != nil {
			return applicationErr
		}
		inverse, equalityErr := state.manager.MkEq(abstracted, expression)
		if equalityErr != nil {
			return equalityErr
		}
		if err = t.assertConstraint(inverse); err != nil {
			return err
		}
		maximum, maximumErr := t.datalogMaximum(sort)
		if maximumErr != nil {
			return maximumErr
		}
		constraint, err = state.manager.MkBitVectorULE(representative, maximum)
	}
	if err != nil {
		return err
	}
	if err = t.assertConstraint(constraint); err != nil {
		return err
	}
	return nil
}

func (t *SMTDatalogTheory) assertConstraint(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.traceDatalogExpression("assert_cnstr", 272, expression); err != nil {
		return err
	}
	if err = state.Internalize(expression, false); err != nil {
		return err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return err
	}
	if literal.Var() != SMTTrueBoolVar {
		atom, e := state.BoolExpression(literal.Var())
		if e != nil {
			return e
		}
		if e = state.markRelevant(atom); e != nil {
			return e
		}
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
}

func (t *SMTDatalogTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, smtDatalogScope{declarations: len(t.declarationTrail), references: len(t.declarationRefs)})
	return nil
}

func (t *SMTDatalogTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	declarationLimit := t.scopes[level].declarations
	for _, sort := range t.declarationTrail[declarationLimit:] {
		delete(t.declarations, sort)
		delete(t.representatives, sort)
		delete(t.abstractions, sort)
	}
	t.declarationTrail = t.declarationTrail[:declarationLimit]
	referenceLimit := t.scopes[level].references
	for i := len(t.declarationRefs) - 1; i >= referenceLimit; i-- {
		if t.declarationRefs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := t.state.manager.DelRef(&t.declarationRefs[i]); err != nil {
			return err
		}
	}
	t.declarationRefs = t.declarationRefs[:referenceLimit]
	t.scopes = t.scopes[:level]
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTDatalogTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewDatalogFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

// Source mk_value only creates the deferred dl_value_proc.
func (t *SMTDatalogTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	if node == nil || generator == nil || generator.manager != state.manager || !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	return &smtDatalogValueProcedure{theory: t, node: node}, nil
}

func (t *SMTDatalogTheory) Close() (result error) {
	if t == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	for i := len(t.declarationRefs) - 1; i >= 0; i-- {
		if t.declarationRefs[i] == (ast.ASTRef{}) {
			continue
		}
		result = errors.Join(result, state.manager.DelRef(&t.declarationRefs[i]))
	}
	if result == nil {
		t.declarationRefs = nil
		t.declarationTrail = nil
		t.declarations = nil
		t.representatives = nil
		t.abstractions = nil
		t.scopes = nil
	}
	return result
}
