package apalache

import (
	"fmt"
	"sort"
)

// StratifiedRewriter lowers a supported ground/state expression into a
// PureArena graph and records the ground constraints separately.
type StratifiedRewriter struct {
	integers    *IntValueCache
	constraints []Expr
}

func NewStratifiedRewriter() *StratifiedRewriter {
	return &StratifiedRewriter{integers: NewIntValueCache()}
}

func (rewriter *StratifiedRewriter) IntegerCache() *IntValueCache { return rewriter.integers }

func (rewriter *StratifiedRewriter) Constraints() []Expr {
	return append([]Expr(nil), rewriter.constraints...)
}

func (rewriter *StratifiedRewriter) ResetConstraints() { rewriter.constraints = nil }

func (rewriter *StratifiedRewriter) assert(expression Expr) {
	rewriter.constraints = append(rewriter.constraints, expression)
}

func (rewriter *StratifiedRewriter) Rewrite(expression Expr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	if expression == nil || scope.Arena == nil {
		return scope, ArenaCell{}, fmt.Errorf("stratified rewriting requires an expression and arena")
	}
	switch typed := expression.(type) {
	case *ValueExpr:
		return rewriter.rewriteValue(typed, scope)
	case *NameExpr:
		return rewriter.rewriteName(typed, scope)
	case *OperExpr:
		switch typed.Operator().Name() {
		case "Apalache!:=":
			return rewriter.rewriteAssignment(typed, scope)
		case "AND", "OR":
			return rewriter.rewriteBoolean(typed, scope)
		case "SET_ENUM":
			return rewriter.rewriteSetConstructor(typed, scope)
		case "SET_UNION2":
			return rewriter.rewriteSetUnion(typed, scope)
		case "SET_FILTER":
			return rewriter.rewriteSetFilter(typed, scope)
		}
	}
	return scope, ArenaCell{}, fmt.Errorf("no stratified rule is applicable to %T", expression)
}

func (rewriter *StratifiedRewriter) rewriteValue(expression *ValueExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	switch value := expression.Value().(type) {
	case BoolValue:
		if bool(value) {
			return scope, scope.Arena.TrueCell(), nil
		}
		return scope, scope.Arena.FalseCell(), nil
	case PredefinedSet:
		switch value {
		case BoolSet:
			return scope, scope.Arena.BooleanSetCell(), nil
		case NatSet:
			return scope, scope.Arena.NatSetCell(), nil
		case IntSet:
			return scope, scope.Arena.IntSetCell(), nil
		}
	case IntValue:
		arena, cell, err := rewriter.integers.GetOrCreate(scope.Arena, value.BigInt())
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope.Arena = arena
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), cell.NameExpr(), NewValueExpr(value, Typed(IntT1))))
		return scope, cell, nil
	}
	return scope, ArenaCell{}, fmt.Errorf("unsupported stratified literal %T", expression.Value())
}

func (rewriter *StratifiedRewriter) rewriteName(expression *NameExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	if IsArenaCellName(expression.Name()) {
		cell, err := scope.Arena.FindCellByName(expression.Name())
		return scope, cell, err
	}
	cell, found := scope.Binding.Get(expression.Name())
	if !found {
		return scope, ArenaCell{}, fmt.Errorf("variable %s is not assigned a value", expression.Name())
	}
	return scope, cell, nil
}

func (rewriter *StratifiedRewriter) rewriteAssignment(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("assignment requires two arguments")
	}
	prime, ok := arguments[0].(*OperExpr)
	if !ok || prime.Operator().Name() != "PRIME" || len(prime.Arguments()) != 1 {
		return scope, ArenaCell{}, fmt.Errorf("assignment left-hand side must be a primed name")
	}
	name, ok := prime.Arguments()[0].(*NameExpr)
	if !ok || IsArenaCellName(name.Name()) {
		return scope, ArenaCell{}, fmt.Errorf("assignment left-hand side must be a variable name")
	}
	primedName := name.Name() + "'"
	if scope.Binding.Contains(primedName) {
		return scope, ArenaCell{}, fmt.Errorf("variable %s is already assigned", primedName)
	}
	newScope, cell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newScope.Binding = newScope.Binding.With(primedName, cell)
	return newScope, newScope.Arena.TrueCell(), nil
}

func (rewriter *StratifiedRewriter) rewriteBoolean(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) == 0 {
		if expression.Operator().Name() == "AND" {
			return scope, scope.Arena.TrueCell(), nil
		}
		return scope, scope.Arena.FalseCell(), nil
	}
	if len(arguments) == 1 {
		return rewriter.Rewrite(arguments[0], scope)
	}
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	scope.Arena = newArena
	terms := make([]Expr, len(arguments))
	for index, argument := range arguments {
		var cell ArenaCell
		var err error
		scope, cell, err = rewriter.Rewrite(argument, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		terms[index] = cell.NameExpr()
	}
	rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), result.NameExpr(), MustOperExpr(expression.Operator(), Typed(BoolT1), terms...)))
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) rewriteSetConstructor(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	cellType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if _, ok := cellType.TLAType().(SetType); !ok {
		return scope, ArenaCell{}, fmt.Errorf("set constructor has non-set type %s", cellType.TLAType())
	}
	unique := make(map[int]ArenaCell)
	for _, element := range expression.Arguments() {
		var cell ArenaCell
		scope, cell, err = rewriter.Rewrite(element, scope)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		unique[cell.ID] = cell
	}
	ids := make([]int, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	newArena, setCell := scope.Arena.AppendCellType(cellType)
	pointers := make([]ElementPointer, len(ids))
	for index, id := range ids {
		pointers[index] = FixedElementPointer{Cell: unique[id]}
	}
	scope.Arena = newArena.AppendHas(setCell, pointers...)
	rewriter.addMembershipConstraints(scope.Arena, setCell)
	return scope, setCell, nil
}

func (rewriter *StratifiedRewriter) rewriteSetUnion(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("set union requires two operands")
	}
	leftScope, left, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	rightScope, right, err := rewriter.Rewrite(arguments[1], leftScope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	pointers, err := MergeElementPointersByCell(append(rightScope.Arena.GetHas(left), rightScope.Arena.GetHas(right)...))
	if err != nil {
		return scope, ArenaCell{}, err
	}
	cellType, err := CellTypeFromTag(expression.Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, setCell := rightScope.Arena.AppendCellType(cellType)
	rightScope.Arena = newArena.AppendHas(setCell, pointers...)
	rewriter.addMembershipConstraints(rightScope.Arena, setCell)
	return rightScope, setCell, nil
}

func (rewriter *StratifiedRewriter) rewriteSetFilter(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 3 {
		return scope, ArenaCell{}, fmt.Errorf("set filter requires a bound name, set, and predicate")
	}
	variable, ok := arguments[0].(*NameExpr)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("set filter binder must be a name")
	}
	if value, ok := arguments[1].(*ValueExpr); ok {
		if predefined, ok := value.Value().(PredefinedSet); ok && (predefined == IntSet || predefined == NatSet) {
			return scope, ArenaCell{}, fmt.Errorf("set filter over infinite set %s is not supported", predefined.Name())
		}
	}
	setScope, setCell, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	baseBinding := setScope.Binding
	pointers := setScope.Arena.GetHas(setCell)
	restricted := make([]ElementPointer, len(pointers))
	for index, pointer := range pointers {
		predicateScope := RewriterScope{Arena: setScope.Arena, Binding: baseBinding.With(variable.Name(), pointer.Element())}
		predicateScope, predicateCell, rewriteErr := rewriter.Rewrite(arguments[2], predicateScope)
		if rewriteErr != nil {
			return scope, ArenaCell{}, rewriteErr
		}
		setScope.Arena = predicateScope.Arena
		restricted[index] = pointer.Restrict(predicateCell.NameExpr())
	}
	setScope.Binding = baseBinding
	cellType, err := CellTypeFromTag(arguments[1].Tag())
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, result := setScope.Arena.AppendCellType(cellType)
	setScope.Arena = newArena.AppendHas(result, restricted...)
	rewriter.addMembershipConstraints(setScope.Arena, result)
	return setScope, result, nil
}

func (rewriter *StratifiedRewriter) addMembershipConstraints(arena *PureArena, setCell ArenaCell) {
	for _, pointer := range arena.GetHas(setCell) {
		membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), pointer.Element().NameExpr(), setCell.NameExpr())
		rewriter.assert(MustOperExpr(Op("EQ"), Typed(BoolT1), membership, pointer.ToSMT()))
	}
}
