package apalache

import "fmt"

const (
	TemporalSavedPrefix = "__saved_"
	TemporalInLoopName  = "__InLoop"
	TemporalLoopOKName  = "__LoopOK"
)

// TemporalLoopEncoding is the module produced by the first stage of the
// temporal pass together with its rewritten behavior predicates.
type TemporalLoopEncoding struct {
	Module *Module
	Init   *OperDecl
	Next   *OperDecl
	LoopOK *OperDecl
}

// TemporalLoopEncoder adds the lasso-selection state used by the Scala
// LoopEncoder. A false-to-true transition of __InLoop saves the source state;
// __LoopOK holds when the final state closes that selected loop.
type TemporalLoopEncoder struct {
	tracker TransformationTracker
}

func NewTemporalLoopEncoder(tracker TransformationTracker) *TemporalLoopEncoder {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &TemporalLoopEncoder{tracker: tracker}
}

func (encoder *TemporalLoopEncoder) SavedVariable(variable *VarDecl) *VarDecl {
	return NewVarDecl(TemporalSavedPrefix+variable.Name(), variable.Tag())
}

func temporalName(declaration interface {
	Name() string
	Tag() TypeTag
}) Expr {
	return NewNameExpr(declaration.Name(), declaration.Tag())
}

func temporalPrime(expression Expr) Expr {
	return MustOperExpr(Op("PRIME"), expression.Tag(), expression)
}

func temporalBool(value bool) Expr {
	return NewValueExpr(BoolValue(value), Typed(BoolT1))
}

func temporalAnd(expressions ...Expr) Expr {
	return flattenConjunction(MustOperExpr(Op("AND"), Typed(BoolT1), expressions...))
}

func (encoder *TemporalLoopEncoder) extendDeclaration(declaration *OperDecl, expression Expr) *OperDecl {
	return encoder.tracker.TrackOperatorDeclaration(func(original *OperDecl) *OperDecl {
		copy := original.copyWithTag(original.Tag())
		copy.SetBody(temporalAnd(original.Body(), expression))
		return copy
	})(declaration)
}

func (encoder *TemporalLoopEncoder) AddToInit(
	variables, saved []*VarDecl,
	init *OperDecl,
) *OperDecl {
	inLoop := NewNameExpr(TemporalInLoopName, Typed(BoolT1))
	conjuncts := []Expr{MustOperExpr(Op("EQ"), Typed(BoolT1), inLoop, temporalBool(false))}
	for index, variable := range variables {
		conjuncts = append(conjuncts, MustOperExpr(Op("EQ"), Typed(BoolT1),
			temporalName(saved[index]), temporalName(variable)))
	}
	return encoder.extendDeclaration(init, temporalAnd(conjuncts...))
}

// SavedVariableUpdate returns
// saved' = IF __InLoop' = __InLoop THEN saved ELSE variable.
func (encoder *TemporalLoopEncoder) SavedVariableUpdate(variable, saved *VarDecl) Expr {
	inLoop := NewNameExpr(TemporalInLoopName, Typed(BoolT1))
	inLoopPrime := temporalPrime(inLoop)
	condition := MustOperExpr(Op("EQ"), Typed(BoolT1), inLoop, inLoopPrime)
	value := MustOperExpr(Op("IF_THEN_ELSE"), variable.Tag(), condition, temporalName(saved), temporalName(variable))
	return MustOperExpr(Op("EQ"), Typed(BoolT1), temporalPrime(temporalName(saved)), value)
}

func (encoder *TemporalLoopEncoder) AddToNext(
	variables, saved []*VarDecl,
	next *OperDecl,
) *OperDecl {
	inLoop := NewNameExpr(TemporalInLoopName, Typed(BoolT1))
	inLoopPrime := temporalPrime(inLoop)
	boolSetType, _ := NewSetType(BoolT1)
	boolSet := MustOperExpr(Op("SET_ENUM"), Typed(boolSetType), temporalBool(true), temporalBool(false))
	inLoopUpdate := temporalAnd(
		MustOperExpr(Op("SET_IN"), Typed(BoolT1), inLoopPrime, boolSet),
		MustOperExpr(Op("IMPLIES"), Typed(BoolT1), inLoop, inLoopPrime),
	)

	equalities := make([]Expr, len(variables))
	savedNames := make([]Expr, len(saved))
	savedTypes := make([]Type, len(saved))
	for index, variable := range variables {
		equalities[index] = MustOperExpr(Op("EQ"), Typed(BoolT1),
			temporalPrime(temporalName(saved[index])), temporalName(variable))
		savedNames[index] = temporalName(saved[index])
		typ, ok := saved[index].Tag().Type()
		if !ok {
			panic("TemporalLoopEncoder requires typed variables")
		}
		savedTypes[index] = typ
	}
	tupleType, err := NewTupleType(savedTypes...)
	if err != nil {
		panic(err)
	}
	unchanged := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1),
		MustOperExpr(Op("TUPLE"), Typed(tupleType), savedNames...))
	condition := MustOperExpr(Op("EQ"), Typed(BoolT1), inLoop, inLoopPrime)
	savedUpdate := MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), condition, unchanged, temporalAnd(equalities...))
	return encoder.extendDeclaration(next, temporalAnd(inLoopUpdate, savedUpdate))
}

func (encoder *TemporalLoopEncoder) LoopPredicate(variables, saved []*VarDecl) *OperDecl {
	conjuncts := []Expr{NewNameExpr(TemporalInLoopName, Typed(BoolT1))}
	for index, variable := range variables {
		conjuncts = append(conjuncts, MustOperExpr(Op("EQ"), Typed(BoolT1),
			temporalName(variable), temporalName(saved[index])))
	}
	return NewOperDecl(TemporalLoopOKName, nil, temporalAnd(conjuncts...), Typed(mustOperator(nil, BoolT1)))
}

func (encoder *TemporalLoopEncoder) AddLoopLogic(module *Module, initName, nextName string) (TemporalLoopEncoding, error) {
	if module == nil {
		return TemporalLoopEncoding{}, fmt.Errorf("cannot add temporal loop logic to a nil module")
	}
	initDeclaration, initFound := declarationNamed(module, initName)
	nextDeclaration, nextFound := declarationNamed(module, nextName)
	init, initOK := initDeclaration.(*OperDecl)
	next, nextOK := nextDeclaration.(*OperDecl)
	if !initFound || !initOK || len(init.FormalParams()) != 0 {
		return TemporalLoopEncoding{}, fmt.Errorf("temporal init operator %s was not found or has parameters", initName)
	}
	if !nextFound || !nextOK || len(next.FormalParams()) != 0 {
		return TemporalLoopEncoding{}, fmt.Errorf("temporal next operator %s was not found or has parameters", nextName)
	}
	variables := module.VarDeclarations()
	saved := make([]*VarDecl, len(variables))
	for index, variable := range variables {
		if _, typed := variable.Tag().Type(); !typed {
			return TemporalLoopEncoding{}, fmt.Errorf("temporal variable %s is untyped", variable.Name())
		}
		saved[index] = encoder.SavedVariable(variable)
	}
	newInit := encoder.AddToInit(variables, saved, init)
	newNext := encoder.AddToNext(variables, saved, next)
	loopOK := encoder.LoopPredicate(variables, saved)
	declarations := make([]Decl, 0, len(module.Declarations())+len(saved)+2)
	for _, declaration := range saved {
		declarations = append(declarations, declaration)
	}
	declarations = append(declarations, NewVarDecl(TemporalInLoopName, Typed(BoolT1)))
	for _, declaration := range module.Declarations() {
		switch declaration.Name() {
		case initName:
			declarations = append(declarations, newInit)
		case nextName:
			declarations = append(declarations, newNext)
		default:
			declarations = append(declarations, declaration)
		}
	}
	declarations = append(declarations, loopOK)
	encoded := NewModule(module.Name(), declarations)
	encoded.properties = cloneMap(module.properties)
	return TemporalLoopEncoding{Module: encoded, Init: newInit, Next: newNext, LoopOK: loopOK}, nil
}
