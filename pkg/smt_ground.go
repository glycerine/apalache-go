package apalache

import (
	"fmt"
	"sort"
)

// G3GroundEncoder translates the ground Boolean/integer arena IR emitted by
// caches and rewriting rules into terms owned by one g3 solver context. It uses
// the OOPSLA-style encoding in which each potential set-membership edge is an
// independent Boolean constant.
type G3GroundEncoder struct {
	solver      *G3SolverContext
	cells       map[string]SMTTerm
	memberships map[[2]int]SMTTerm
}

func NewG3GroundEncoder(solver *G3SolverContext) (*G3GroundEncoder, error) {
	if solver == nil {
		return nil, fmt.Errorf("ground encoder requires a solver")
	}
	if err := solver.validate(); err != nil {
		return nil, err
	}
	return &G3GroundEncoder{solver: solver, cells: make(map[string]SMTTerm), memberships: make(map[[2]int]SMTTerm)}, nil
}

func (encoder *G3GroundEncoder) DeclareCell(cell ArenaCell) error {
	name := cell.String()
	if _, found := encoder.cells[name]; found {
		return fmt.Errorf("arena cell %s is already declared", cell)
	}
	var term SMTTerm
	var err error
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

func smtCellSortName(cellType CellType) string {
	return fmt.Sprintf("CellSort_%x", []byte(cellType.Signature()))
}

func MembershipPredicateName(set, element ArenaCell) string {
	return fmt.Sprintf("in_%s%d_%s%d", element.Type.Signature(), element.ID, set.Type.Signature(), set.ID)
}

func (encoder *G3GroundEncoder) DeclareMembership(set, element ArenaCell) error {
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

func isMembershipOperator(name string) bool {
	return name == "SET_IN" || name == "Apalache!SelectInSet" || name == "Apalache!StoreInSet"
}

func isNegatedMembershipOperator(name string) bool {
	return name == "SET_NOT_IN" || name == "Apalache!storeNotInSet"
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

// EncodeRewriterScope declares a completed solver-free rewrite scope and
// discharges its collected constraints into a fresh solver encoder.
func EncodeRewriterScope(solver *G3SolverContext, scope RewriterScope, constraints []Expr) (*G3GroundEncoder, error) {
	if scope.Arena == nil {
		return nil, fmt.Errorf("cannot encode a rewrite scope without an arena")
	}
	encoder, err := NewG3GroundEncoder(solver)
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
