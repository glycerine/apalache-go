package apalache

import (
	"fmt"
	"math/big"
	"sort"
)

// G3GroundEncoder translates the ground Boolean/integer arena IR emitted by
// caches and rewriting rules into terms owned by one g3 solver context. It uses
// the OOPSLA-style encoding in which each potential set-membership edge is an
// independent Boolean constant.
type G3GroundEncoder struct {
	solver        *G3SolverContext
	encoding      SMTEncoding
	cells         map[string]SMTTerm
	memberships   map[[2]int]SMTTerm
	arrayVersions map[string]int
	defaultValues map[string]SMTTerm
	cellStack     []map[string]SMTTerm
	inStack       []map[[2]int]SMTTerm
	versionStack  []map[string]int
	boundIndices  map[string]uint32
}

func NewG3GroundEncoder(solver *G3SolverContext) (*G3GroundEncoder, error) {
	return NewG3GroundEncoderWithEncoding(solver, SMTEncodingOOPSLA19)
}

func NewG3GroundEncoderWithEncoding(
	solver *G3SolverContext,
	encoding SMTEncoding,
) (*G3GroundEncoder, error) {
	if solver == nil {
		return nil, fmt.Errorf("ground encoder requires a solver")
	}
	if err := solver.validate(); err != nil {
		return nil, err
	}
	if encoding != SMTEncodingOOPSLA19 && encoding != SMTEncodingArrays && encoding != SMTEncodingFunArrays {
		return nil, fmt.Errorf("unsupported ground SMT encoding %s", encoding)
	}
	return &G3GroundEncoder{
		solver: solver, encoding: encoding,
		cells: make(map[string]SMTTerm), memberships: make(map[[2]int]SMTTerm),
		arrayVersions: make(map[string]int), defaultValues: make(map[string]SMTTerm),
		boundIndices: make(map[string]uint32),
	}, nil
}

func (encoder *G3GroundEncoder) DeclareCell(cell ArenaCell) error {
	name := cell.String()
	if _, found := encoder.cells[name]; found {
		return fmt.Errorf("arena cell %s is already declared", cell)
	}
	var term SMTTerm
	var err error
	if encoder.encoding == SMTEncodingArrays {
		if setType, set := cell.Type.TLAType().(SetType); set {
			term, err = encoder.declareSetArray(cell, setType.Element())
			if err != nil {
				return fmt.Errorf("declare arena set array %s: %w", cell, err)
			}
			encoder.cells[name] = term
			return nil
		}
	}
	if encoder.encoding == SMTEncodingArrays || encoder.encoding == SMTEncodingFunArrays {
		if functionType, function := cell.Type.TLAType().(FunType); function {
			term, err = encoder.declareFunctionArray(cell, functionType)
			if err != nil {
				return fmt.Errorf("declare arena function array %s: %w", cell, err)
			}
			encoder.cells[name] = term
			return nil
		}
	}
	switch typ := cell.Type.TLAType().(type) {
	case primitiveType:
		switch typ {
		case BoolT1:
			term, err = encoder.solver.Declare(name, SMTBool)
		case IntT1:
			term, err = encoder.solver.Declare(name, SMTInt)
		default:
			term, err = encoder.solver.DeclareUninterpreted(name, smtCellSortName(cell.Type))
		}
	default:
		term, err = encoder.solver.DeclareUninterpreted(name, smtCellSortName(cell.Type))
	}
	if err != nil {
		return fmt.Errorf("declare arena cell %s: %w", cell, err)
	}
	encoder.cells[name] = term
	return nil
}

func (encoder *G3GroundEncoder) declareFunctionArray(cell ArenaCell, functionType FunType) (SMTTerm, error) {
	argumentSpec, err := smtSortSpecForType(functionType.Argument(), encoder.encoding)
	if err != nil {
		return SMTTerm{}, err
	}
	resultSpec, err := smtSortSpecForType(functionType.Result(), encoder.encoding)
	if err != nil {
		return SMTTerm{}, err
	}
	array, err := encoder.solver.declareArrayWithSpecs(cell.String(), argumentSpec, resultSpec)
	if err != nil || cell.Unconstrained {
		return array, err
	}
	defaultValue, err := encoder.defaultValue(functionType.Result(), resultSpec)
	if err != nil {
		return SMTTerm{}, err
	}
	constant, err := encoder.solver.constantArrayWithIndexSpec(argumentSpec, defaultValue)
	if err != nil {
		return SMTTerm{}, err
	}
	equal, err := encoder.solver.Equal(array, constant)
	if err != nil {
		return SMTTerm{}, err
	}
	if err = encoder.solver.Assert(equal); err != nil {
		return SMTTerm{}, err
	}
	return array, nil
}

func (encoder *G3GroundEncoder) defaultValue(typ Type, spec *smtSortSpec) (SMTTerm, error) {
	if EqualTypes(typ, BoolT1) {
		return encoder.solver.Bool(false)
	}
	if EqualTypes(typ, IntT1) {
		return encoder.solver.Int(0)
	}
	sortID := spec.sortID()
	if value, found := encoder.defaultValues[sortID]; found {
		return value, nil
	}
	var value SMTTerm
	var err error
	if spec.kind == SMTArray {
		var valueType Type
		switch typed := typ.(type) {
		case SetType:
			valueType = BoolT1
		case FunType:
			valueType = typed.Result()
		default:
			return SMTTerm{}, fmt.Errorf("array sort %s has non-array TLA type %s", sortID, typ)
		}
		var nestedDefault SMTTerm
		nestedDefault, err = encoder.defaultValue(valueType, spec.value)
		if err == nil {
			value, err = encoder.solver.constantArrayWithIndexSpec(spec.index, nestedDefault)
		}
	} else {
		value, err = encoder.solver.DeclareUninterpreted("$Default$"+sortID, sortID)
	}
	if err != nil {
		return SMTTerm{}, err
	}
	encoder.defaultValues[sortID] = value
	return value, nil
}

func (encoder *G3GroundEncoder) declareSetArray(cell ArenaCell, elementType Type) (SMTTerm, error) {
	elementSpec, err := smtSortSpecForType(elementType, encoder.encoding)
	if err != nil {
		return SMTTerm{}, err
	}
	array, err := encoder.solver.declareArrayWithSpecs(
		cell.String(), elementSpec, scalarSMTSortSpec(SMTBool, ""))
	if err != nil || cell.Unconstrained {
		return array, err
	}
	defaultMembership := false
	if cell.String() == ArenaBooleanSet {
		defaultMembership = true
	} else if _, infinite := cell.Type.(InfiniteSetCellType); infinite {
		defaultMembership = true
	}
	defaultValue, err := encoder.solver.Bool(defaultMembership)
	if err != nil {
		return SMTTerm{}, err
	}
	constant, err := encoder.solver.constantArrayWithIndexSpec(elementSpec, defaultValue)
	if err != nil {
		return SMTTerm{}, err
	}
	equal, err := encoder.solver.Equal(array, constant)
	if err != nil {
		return SMTTerm{}, err
	}
	if err = encoder.solver.Assert(equal); err != nil {
		return SMTTerm{}, err
	}
	return array, nil
}

func smtSortSpecForType(typ Type, encoding SMTEncoding) (*smtSortSpec, error) {
	if EqualTypes(typ, BoolT1) {
		return scalarSMTSortSpec(SMTBool, ""), nil
	}
	if EqualTypes(typ, IntT1) {
		return scalarSMTSortSpec(SMTInt, ""), nil
	}
	if setType, ok := typ.(SetType); ok && encoding == SMTEncodingArrays {
		elementSpec, err := smtSortSpecForType(setType.Element(), encoding)
		if err != nil {
			return nil, err
		}
		return arraySMTSortSpec(elementSpec, scalarSMTSortSpec(SMTBool, "")), nil
	}
	if functionType, ok := typ.(FunType); ok &&
		(encoding == SMTEncodingArrays || encoding == SMTEncodingFunArrays) {
		argumentSpec, err := smtSortSpecForType(functionType.Argument(), encoding)
		if err != nil {
			return nil, err
		}
		resultSpec, err := smtSortSpecForType(functionType.Result(), encoding)
		if err != nil {
			return nil, err
		}
		return arraySMTSortSpec(argumentSpec, resultSpec), nil
	}
	cellType, err := CellTypeFromType(typ)
	if err != nil {
		return nil, err
	}
	return scalarSMTSortSpec(SMTUninterpreted, smtCellSortName(cellType)), nil
}

func smtCellSortName(cellType CellType) string {
	return fmt.Sprintf("CellSort_%x", []byte(cellType.Signature()))
}

func MembershipPredicateName(set, element ArenaCell) string {
	return fmt.Sprintf("in_%s%d_%s%d", element.Type.Signature(), element.ID, set.Type.Signature(), set.ID)
}

func (encoder *G3GroundEncoder) DeclareMembership(set, element ArenaCell) error {
	if encoder.encoding == SMTEncodingArrays {
		return nil
	}
	key := [2]int{set.ID, element.ID}
	if _, found := encoder.memberships[key]; found {
		return nil
	}
	term, err := encoder.solver.Declare(MembershipPredicateName(set, element), SMTBool)
	if err != nil {
		return fmt.Errorf("declare membership %s in %s: %w", element, set, err)
	}
	encoder.memberships[key] = term
	return nil
}

func cloneSMTTerms[K comparable](terms map[K]SMTTerm) map[K]SMTTerm {
	cloned := make(map[K]SMTTerm, len(terms))
	for key, term := range terms {
		cloned[key] = term
	}
	return cloned
}

func (encoder *G3GroundEncoder) Push() {
	encoder.cellStack = append(encoder.cellStack, cloneSMTTerms(encoder.cells))
	encoder.inStack = append(encoder.inStack, cloneSMTTerms(encoder.memberships))
	encoder.versionStack = append(encoder.versionStack, cloneMap(encoder.arrayVersions))
}

func (encoder *G3GroundEncoder) Pop(count int) error {
	if count < 0 || count > len(encoder.cellStack) {
		return fmt.Errorf("cannot pop %d levels from ground encoder at level %d", count, len(encoder.cellStack))
	}
	if count == 0 {
		return nil
	}
	encoder.cells = encoder.cellStack[len(encoder.cellStack)-count]
	encoder.memberships = encoder.inStack[len(encoder.inStack)-count]
	encoder.arrayVersions = encoder.versionStack[len(encoder.versionStack)-count]
	encoder.cellStack = encoder.cellStack[:len(encoder.cellStack)-count]
	encoder.inStack = encoder.inStack[:len(encoder.inStack)-count]
	encoder.versionStack = encoder.versionStack[:len(encoder.versionStack)-count]
	return nil
}

func (encoder *G3GroundEncoder) DeclaredCellNames() []string {
	names := make([]string, 0, len(encoder.cells))
	for name := range encoder.cells {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (encoder *G3GroundEncoder) Encode(expression Expr) (SMTTerm, error) {
	if expression == nil {
		return SMTTerm{}, fmt.Errorf("cannot encode a nil expression")
	}
	switch typed := expression.(type) {
	case *NameExpr:
		if index, found := encoder.boundIndices[typed.Name()]; found {
			return encoder.solver.Bound(index, SMTInt)
		}
		term, found := encoder.cells[typed.Name()]
		if !found {
			return SMTTerm{}, fmt.Errorf("SMT name %s is not a declared arena cell", typed.Name())
		}
		return term, nil
	case *ValueExpr:
		switch value := typed.Value().(type) {
		case BoolValue:
			return encoder.solver.Bool(bool(value))
		case IntValue:
			return encoder.solver.BigInt(value.BigInt())
		default:
			return SMTTerm{}, fmt.Errorf("unsupported ground SMT value %T", value)
		}
	case *OperExpr:
		return encoder.encodeOperator(typed)
	default:
		return SMTTerm{}, fmt.Errorf("unsupported ground SMT expression %T", expression)
	}
}

func (encoder *G3GroundEncoder) encodeOperator(expression *OperExpr) (SMTTerm, error) {
	arguments := expression.Arguments()
	if expression.Operator().Name() == "ApalacheInternal!IntExists" {
		return encoder.encodeIntegerExistential(arguments)
	}
	if (expression.Operator().Name() == "SET_IN" || expression.Operator().Name() == "Apalache!SelectInSet") &&
		encoder.encoding == SMTEncodingArrays {
		return encoder.encodeArraySelect(arguments)
	}
	if expression.Operator().Name() == "SET_NOT_IN" && encoder.encoding == SMTEncodingArrays {
		membership, err := encoder.encodeArraySelect(arguments)
		if err != nil {
			return SMTTerm{}, err
		}
		return encoder.solver.Not(membership)
	}
	if expression.Operator().Name() == "Apalache!SelectInFun" &&
		(encoder.encoding == SMTEncodingArrays || encoder.encoding == SMTEncodingFunArrays) {
		return encoder.encodeArraySelect(arguments)
	}
	if expression.Operator().Name() == "Apalache!StoreInSet" {
		if len(arguments) == 2 && encoder.encoding == SMTEncodingArrays {
			return encoder.encodeArrayStore(arguments[1], arguments[0])
		}
		if len(arguments) == 3 && (encoder.encoding == SMTEncodingArrays || encoder.encoding == SMTEncodingFunArrays) {
			return encoder.encodeArrayStore(arguments[1], arguments[2], arguments[0])
		}
	}
	if (expression.Operator().Name() == "Apalache!storeNotInSet" && encoder.encoding == SMTEncodingArrays) ||
		(expression.Operator().Name() == "Apalache!storeNotInFun" &&
			(encoder.encoding == SMTEncodingArrays || encoder.encoding == SMTEncodingFunArrays)) {
		return encoder.encodeUnchangedArray(arguments)
	}
	if expression.Operator().Name() == "Apalache!UnconstrainArray" &&
		(encoder.encoding == SMTEncodingArrays || encoder.encoding == SMTEncodingFunArrays) {
		if len(arguments) != 1 {
			return SMTTerm{}, fmt.Errorf("unconstrain array requires one argument")
		}
		if _, err := encoder.freshArrayVersion(arguments[0]); err != nil {
			return SMTTerm{}, err
		}
		return encoder.solver.Bool(true)
	}
	if (expression.Operator().Name() == "Apalache!SmtMap_AND" ||
		expression.Operator().Name() == "Apalache!SmtMap_OR") && encoder.encoding == SMTEncodingArrays {
		if len(arguments) != 2 {
			return SMTTerm{}, fmt.Errorf("array map requires two arguments")
		}
		input, err := encoder.Encode(arguments[0])
		if err != nil {
			return SMTTerm{}, err
		}
		constraints, err := encoder.Encode(arguments[1])
		if err != nil {
			return SMTTerm{}, err
		}
		operator := "AND"
		if expression.Operator().Name() == "Apalache!SmtMap_OR" {
			operator = "OR"
		}
		mapped, err := encoder.solver.ArrayMapBoolean(operator, input, constraints)
		if err != nil {
			return SMTTerm{}, err
		}
		updated, err := encoder.freshArrayVersion(arguments[1])
		if err != nil {
			return SMTTerm{}, err
		}
		return encoder.solver.Equal(updated, mapped)
	}
	if isMembershipOperator(expression.Operator().Name()) {
		return encoder.encodeMembership(arguments)
	}
	if isNegatedMembershipOperator(expression.Operator().Name()) {
		membership, err := encoder.encodeMembership(arguments)
		if err != nil {
			return SMTTerm{}, err
		}
		return encoder.solver.Not(membership)
	}
	terms := make([]SMTTerm, len(arguments))
	for index, argument := range arguments {
		term, err := encoder.Encode(argument)
		if err != nil {
			return SMTTerm{}, fmt.Errorf("encode argument %d of %s: %w", index, expression.Operator().Name(), err)
		}
		terms[index] = term
	}
	binary := func(operation func(SMTTerm, SMTTerm) (SMTTerm, error)) (SMTTerm, error) {
		if len(terms) != 2 {
			return SMTTerm{}, fmt.Errorf("operator %s requires two SMT terms", expression.Operator().Name())
		}
		return operation(terms[0], terms[1])
	}
	switch expression.Operator().Name() {
	case "EQ", "EQUIV":
		return binary(encoder.solver.Equal)
	case "NE":
		equal, err := binary(encoder.solver.Equal)
		if err != nil {
			return SMTTerm{}, err
		}
		return encoder.solver.Not(equal)
	case "Apalache!Distinct":
		return encoder.solver.Distinct(terms...)
	case "AND":
		return encoder.solver.And(terms...)
	case "OR":
		return encoder.solver.Or(terms...)
	case "NOT":
		if len(terms) != 1 {
			return SMTTerm{}, fmt.Errorf("operator NOT requires one SMT term")
		}
		return encoder.solver.Not(terms[0])
	case "IMPLIES":
		return binary(encoder.solver.Implies)
	case "IF_THEN_ELSE":
		if len(terms) != 3 {
			return SMTTerm{}, fmt.Errorf("operator IF_THEN_ELSE requires three SMT terms")
		}
		return encoder.solver.IfThenElse(terms[0], terms[1], terms[2])
	case "PLUS":
		return binary(encoder.solver.Add)
	case "MINUS":
		return binary(encoder.solver.Subtract)
	case "MULT":
		return binary(encoder.solver.Multiply)
	case "DIV":
		return binary(encoder.solver.Divide)
	case "MOD":
		return binary(encoder.solver.Modulo)
	case "POW":
		return binary(encoder.solver.Power)
	case "UNARY_MINUS":
		if len(terms) != 1 {
			return SMTTerm{}, fmt.Errorf("operator UNARY_MINUS requires one SMT term")
		}
		return encoder.solver.Negate(terms[0])
	case "LT":
		return binary(encoder.solver.Less)
	case "LE":
		return binary(encoder.solver.LessEqual)
	case "GT":
		return binary(encoder.solver.Greater)
	case "GE":
		return binary(encoder.solver.GreaterEqual)
	default:
		return SMTTerm{}, fmt.Errorf("unsupported ground SMT operator %s", expression.Operator().Name())
	}
}

func (encoder *G3GroundEncoder) encodeIntegerExistential(arguments []Expr) (SMTTerm, error) {
	if len(arguments) != 4 {
		return SMTTerm{}, fmt.Errorf("integer existential requires four arguments")
	}
	binder, ok := arguments[0].(*NameExpr)
	if !ok || binder.Name() == "" {
		return SMTTerm{}, fmt.Errorf("integer existential binder must be a name")
	}
	if _, duplicate := encoder.boundIndices[binder.Name()]; duplicate {
		return SMTTerm{}, fmt.Errorf("integer existential binder %s is already active", binder.Name())
	}
	for name, index := range encoder.boundIndices {
		encoder.boundIndices[name] = index + 1
	}
	encoder.boundIndices[binder.Name()] = 0
	defer func() {
		delete(encoder.boundIndices, binder.Name())
		for name, index := range encoder.boundIndices {
			encoder.boundIndices[name] = index - 1
		}
	}()
	lower, err := encoder.Encode(arguments[1])
	if err != nil {
		return SMTTerm{}, fmt.Errorf("encode integer existential lower bound: %w", err)
	}
	upper, err := encoder.Encode(arguments[2])
	if err != nil {
		return SMTTerm{}, fmt.Errorf("encode integer existential upper bound: %w", err)
	}
	bound, err := encoder.solver.Bound(0, SMTInt)
	if err != nil {
		return SMTTerm{}, err
	}
	body, err := encoder.Encode(arguments[3])
	if err != nil {
		return SMTTerm{}, fmt.Errorf("encode integer existential predicate: %w", err)
	}
	lowerBound, err := encoder.solver.LessEqual(lower, bound)
	if err != nil {
		return SMTTerm{}, err
	}
	upperBound, err := encoder.solver.LessEqual(bound, upper)
	if err != nil {
		return SMTTerm{}, err
	}
	quantifiedBody, err := encoder.solver.And(lowerBound, upperBound, body)
	if err != nil {
		return SMTTerm{}, err
	}
	return encoder.solver.Exists(binder.Name(), bound, quantifiedBody)
}

func (encoder *G3GroundEncoder) freshArrayVersion(expression Expr) (SMTTerm, error) {
	name, ok := expression.(*NameExpr)
	if !ok {
		return SMTTerm{}, fmt.Errorf("array update requires an arena-cell name")
	}
	current, found := encoder.cells[name.Name()]
	if !found || current.sort != SMTArray {
		return SMTTerm{}, fmt.Errorf("SMT name %s is not a declared array cell", name.Name())
	}
	encoder.arrayVersions[name.Name()]++
	versionedName := fmt.Sprintf("%s_ssa%d", name.Name(), encoder.arrayVersions[name.Name()])
	if current.arraySpec == nil {
		return SMTTerm{}, fmt.Errorf("SMT name %s has no array sort metadata", name.Name())
	}
	updated, err := encoder.solver.declareArrayWithSpecs(
		versionedName, current.arraySpec.index, current.arraySpec.value)
	if err != nil {
		return SMTTerm{}, err
	}
	encoder.cells[name.Name()] = updated
	return updated, nil
}

func (encoder *G3GroundEncoder) encodeArrayStore(arrayExpression Expr, operands ...Expr) (SMTTerm, error) {
	if len(operands) != 1 && len(operands) != 2 {
		return SMTTerm{}, fmt.Errorf("array store requires an index and optional value")
	}
	oldArray, err := encoder.Encode(arrayExpression)
	if err != nil {
		return SMTTerm{}, err
	}
	index, err := encoder.Encode(operands[0])
	if err != nil {
		return SMTTerm{}, err
	}
	var value SMTTerm
	if len(operands) == 1 {
		value, err = encoder.solver.Bool(true)
	} else {
		value, err = encoder.Encode(operands[1])
	}
	if err != nil {
		return SMTTerm{}, err
	}
	stored, err := encoder.solver.Store(oldArray, index, value)
	if err != nil {
		return SMTTerm{}, err
	}
	updated, err := encoder.freshArrayVersion(arrayExpression)
	if err != nil {
		return SMTTerm{}, err
	}
	return encoder.solver.Equal(updated, stored)
}

func (encoder *G3GroundEncoder) encodeUnchangedArray(arguments []Expr) (SMTTerm, error) {
	if len(arguments) != 2 {
		return SMTTerm{}, fmt.Errorf("unchanged array requires two arguments")
	}
	oldArray, err := encoder.Encode(arguments[1])
	if err != nil {
		return SMTTerm{}, err
	}
	updated, err := encoder.freshArrayVersion(arguments[1])
	if err != nil {
		return SMTTerm{}, err
	}
	return encoder.solver.Equal(updated, oldArray)
}

func (encoder *G3GroundEncoder) encodeArraySelect(arguments []Expr) (SMTTerm, error) {
	if len(arguments) != 2 {
		return SMTTerm{}, fmt.Errorf("array selection requires two arguments")
	}
	index, err := encoder.Encode(arguments[0])
	if err != nil {
		return SMTTerm{}, err
	}
	array, err := encoder.Encode(arguments[1])
	if err != nil {
		return SMTTerm{}, err
	}
	return encoder.solver.Select(array, index)
}

func isMembershipOperator(name string) bool {
	return name == "SET_IN" || name == "Apalache!SelectInSet" || name == "Apalache!SelectInFun" ||
		name == "Apalache!StoreInSet"
}

func isNegatedMembershipOperator(name string) bool {
	return name == "SET_NOT_IN" || name == "Apalache!storeNotInSet" || name == "Apalache!storeNotInFun"
}

func (encoder *G3GroundEncoder) encodeMembership(arguments []Expr) (SMTTerm, error) {
	if len(arguments) != 2 {
		return SMTTerm{}, fmt.Errorf("set membership requires two arguments")
	}
	elementName, elementOK := arguments[0].(*NameExpr)
	setName, setOK := arguments[1].(*NameExpr)
	if !elementOK || !setOK {
		return SMTTerm{}, fmt.Errorf("ground set membership requires arena-cell names")
	}
	elementID, err := ArenaCellIDFromName(elementName.Name())
	if err != nil {
		return SMTTerm{}, err
	}
	setID, err := ArenaCellIDFromName(setName.Name())
	if err != nil {
		return SMTTerm{}, err
	}
	term, found := encoder.memberships[[2]int{setID, elementID}]
	if !found {
		return SMTTerm{}, fmt.Errorf("membership predicate for %s in %s is not declared", elementName.Name(), setName.Name())
	}
	return term, nil
}

func (encoder *G3GroundEncoder) Assert(expression Expr) error {
	term, err := encoder.Encode(expression)
	if err != nil {
		return err
	}
	return encoder.solver.Assert(term)
}

func (encoder *G3GroundEncoder) AssertAll(expressions []Expr) error {
	for index, expression := range expressions {
		if err := encoder.Assert(expression); err != nil {
			return fmt.Errorf("assert expression %d: %w", index, err)
		}
	}
	return nil
}

func (encoder *G3GroundEncoder) EvaluateBool(expression Expr) (bool, error) {
	term, err := encoder.Encode(expression)
	if err != nil {
		return false, err
	}
	return encoder.solver.EvaluateBool(term)
}

func (encoder *G3GroundEncoder) EvaluateInt(expression Expr) (*big.Int, error) {
	term, err := encoder.Encode(expression)
	if err != nil {
		return nil, err
	}
	return encoder.solver.EvaluateBigInt(term)
}

// EncodeRewriterScope declares a completed solver-free rewrite scope and
// discharges its collected constraints into a fresh solver encoder.
func EncodeRewriterScope(solver *G3SolverContext, scope RewriterScope, constraints []Expr) (*G3GroundEncoder, error) {
	return EncodeRewriterScopeWithEncoding(solver, scope, constraints, SMTEncodingOOPSLA19)
}

// EncodeRewriterScopeWithEncoding is EncodeRewriterScope with an explicit
// encoding selection. The default wrapper retains OOPSLA19 compatibility.
func EncodeRewriterScopeWithEncoding(
	solver *G3SolverContext,
	scope RewriterScope,
	constraints []Expr,
	encoding SMTEncoding,
) (*G3GroundEncoder, error) {
	if scope.Arena == nil {
		return nil, fmt.Errorf("cannot encode a rewrite scope without an arena")
	}
	encoder, err := NewG3GroundEncoderWithEncoding(solver, encoding)
	if err != nil {
		return nil, err
	}
	cellsByName := scope.Arena.CellMap()
	cells := make([]ArenaCell, 0, len(cellsByName))
	for _, cell := range cellsByName {
		cells = append(cells, cell)
	}
	sort.Slice(cells, func(left, right int) bool { return cells[left].ID < cells[right].ID })
	for _, cell := range cells {
		if err := encoder.DeclareCell(cell); err != nil {
			return nil, err
		}
	}
	for _, parent := range cells {
		for _, pointer := range scope.Arena.GetHas(parent) {
			if err := encoder.DeclareMembership(parent, pointer.Element()); err != nil {
				return nil, err
			}
		}
	}
	falseCell, falseErr := scope.Arena.FindCellByName(ArenaFalseName)
	trueCell, trueErr := scope.Arena.FindCellByName(ArenaTrueName)
	booleanSet, booleanErr := scope.Arena.FindCellByName(ArenaBooleanSet)
	if falseErr == nil && trueErr == nil {
		base := []Expr{
			MustOperExpr(Op("NOT"), Typed(BoolT1), falseCell.NameExpr()),
			trueCell.NameExpr(),
		}
		if booleanErr == nil {
			base = append(base,
				MustOperExpr(Op("SET_IN"), Typed(BoolT1), falseCell.NameExpr(), booleanSet.NameExpr()),
				MustOperExpr(Op("SET_IN"), Typed(BoolT1), trueCell.NameExpr(), booleanSet.NameExpr()))
		}
		if err := encoder.AssertAll(base); err != nil {
			return nil, fmt.Errorf("assert rewrite-scope base constraints: %w", err)
		}
	}
	if err := encoder.AssertAll(constraints); err != nil {
		return nil, fmt.Errorf("assert rewrite-scope constraints: %w", err)
	}
	return encoder, nil
}
