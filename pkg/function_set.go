package apalache

import "fmt"

type emptinessKind uint8

const (
	emptyStatic emptinessKind = iota
	nonEmptyStatic
	emptySymbolic
)

type setEmptiness struct {
	kind      emptinessKind
	predicate Expr
}

func staticEmpty() setEmptiness { return setEmptiness{kind: emptyStatic, predicate: boolExpr(true)} }
func staticNonEmpty() setEmptiness {
	return setEmptiness{kind: nonEmptyStatic, predicate: boolExpr(false)}
}
func symbolicEmpty(predicate Expr) setEmptiness {
	return setEmptiness{kind: emptySymbolic, predicate: predicate}
}

func boolConstant(expression Expr) (bool, bool) {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return false, false
	}
	boolean, ok := value.Value().(BoolValue)
	return bool(boolean), ok
}

func (rewriter *StratifiedRewriter) classifySetEmptiness(scope RewriterScope, set ArenaCell) (setEmptiness, error) {
	switch set.Type.(type) {
	case BasicCellType:
		if !isFiniteSetCellType(set.Type) {
			return setEmptiness{}, fmt.Errorf("expected a finite set, found %s", set.Type)
		}
		potential := make([]Expr, 0)
		for _, pointer := range scope.Arena.GetHas(set) {
			guard := pointer.ToSMT()
			if value, constant := boolConstant(guard); constant {
				if value {
					return staticNonEmpty(), nil
				}
				continue
			}
			potential = append(potential, MustOperExpr(Op("NOT"), Typed(BoolT1), guard))
		}
		if len(potential) == 0 {
			return staticEmpty(), nil
		}
		return symbolicEmpty(booleanAnd(potential...)), nil
	case PowerSetCellType, InfiniteSetCellType:
		return staticNonEmpty(), nil
	case FiniteFunctionSetCellType:
		domain, err := scope.Arena.Domain(set)
		if err != nil {
			return setEmptiness{}, err
		}
		codomain, err := scope.Arena.Codomain(set)
		if err != nil {
			return setEmptiness{}, err
		}
		domainEmpty, err := rewriter.classifySetEmptiness(scope, domain)
		if err != nil {
			return setEmptiness{}, err
		}
		codomainEmpty, err := rewriter.classifySetEmptiness(scope, codomain)
		if err != nil {
			return setEmptiness{}, err
		}
		return functionSetEmptiness(domainEmpty, codomainEmpty), nil
	default:
		return setEmptiness{}, fmt.Errorf("expected a set cell, found %s", set.Type)
	}
}

func functionSetEmptiness(domain, codomain setEmptiness) setEmptiness {
	if domain.kind == emptyStatic || codomain.kind == nonEmptyStatic {
		return staticNonEmpty()
	}
	if domain.kind == nonEmptyStatic && codomain.kind == emptyStatic {
		return staticEmpty()
	}
	if domain.kind == nonEmptyStatic && codomain.kind == emptySymbolic {
		return symbolicEmpty(codomain.predicate)
	}
	if domain.kind == emptySymbolic && codomain.kind == emptyStatic {
		return symbolicEmpty(MustOperExpr(Op("NOT"), Typed(BoolT1), domain.predicate))
	}
	return symbolicEmpty(booleanAnd(
		MustOperExpr(Op("NOT"), Typed(BoolT1), domain.predicate), codomain.predicate))
}

func (rewriter *StratifiedRewriter) rewriteFunctionSet(expression *OperExpr, scope RewriterScope) (RewriterScope, ArenaCell, error) {
	arguments := expression.Arguments()
	if len(arguments) != 2 {
		return scope, ArenaCell{}, fmt.Errorf("function set requires a domain and codomain")
	}
	scope, domain, err := rewriter.Rewrite(arguments[0], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, codomain, err := rewriter.Rewrite(arguments[1], scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	typ, typed := expression.Tag().Type()
	if !typed {
		return scope, ArenaCell{}, fmt.Errorf("function set must be typed")
	}
	setType, ok := typ.(SetType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("function set has non-set type %s", typ)
	}
	functionType, ok := setType.Element().(FunType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("function set element has non-function type %s", setType.Element())
	}
	scope, domain, err = rewriter.coerceCell(scope, domain, mustSet(functionType.Argument()))
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("function set domain cannot be widened: %w", err)
	}
	scope, codomain, err = rewriter.coerceCell(scope, codomain, mustSet(functionType.Result()))
	if err != nil {
		return scope, ArenaCell{}, fmt.Errorf("function set codomain cannot be widened: %w", err)
	}
	domainSetType, domainIsSet := domain.Type.TLAType().(SetType)
	codomainSetType, codomainIsSet := codomain.Type.TLAType().(SetType)
	if !domainIsSet || !codomainIsSet || !EqualTypes(domainSetType.Element(), functionType.Argument()) ||
		!EqualTypes(codomainSetType.Element(), functionType.Result()) {
		return scope, ArenaCell{}, fmt.Errorf("function set operands do not match %s", functionType)
	}
	domainEmpty, err := rewriter.classifySetEmptiness(scope, domain)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	codomainEmpty, err := rewriter.classifySetEmptiness(scope, codomain)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	if domainEmpty.kind == emptyStatic {
		return rewriter.makeEmptyFunctionSingleton(scope, functionType, boolExpr(true))
	}
	if codomainEmpty.kind == emptyStatic {
		return rewriter.makeEmptyFunctionSingleton(scope, functionType, domainEmpty.predicate)
	}
	cellType, err := NewFiniteFunctionSetCellType(domain.Type, codomain.Type)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	newArena, result := scope.Arena.AppendCellType(cellType)
	scope.Arena = newArena.SetDomain(result, domain).SetCodomain(result, codomain)
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) makeEmptyFunctionSingleton(
	scope RewriterScope,
	functionType FunType,
	condition Expr,
) (RewriterScope, ArenaCell, error) {
	newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: mustSet(functionType)})
	scope.Arena = newArena
	if value, constant := boolConstant(condition); constant && !value {
		return scope, result, nil
	}
	var emptyFunction ArenaCell
	var err error
	scope, emptyFunction, err = rewriter.defaults.GetOrCreate(scope, functionType)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	pointer := ElementPointer(FixedElementPointer{Cell: emptyFunction})
	if _, constant := boolConstant(condition); !constant {
		pointer = SMTElementPointer{Cell: emptyFunction, Expression: condition}
	}
	scope.Arena = scope.Arena.AppendHas(result, pointer)
	rewriter.addMembershipConstraints(scope.Arena, result)
	return scope, result, nil
}

func (rewriter *StratifiedRewriter) pickFromCompactCollection(scope RewriterScope, set ArenaCell) (RewriterScope, ArenaCell, error) {
	switch set.Type.(type) {
	case FiniteFunctionSetCellType:
		return rewriter.pickFunctionFromSet(scope, set)
	case PowerSetCellType:
		domain, err := scope.Arena.Domain(set)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		pointers := make([]ElementPointer, len(scope.Arena.GetHas(domain)))
		for index, pointer := range scope.Arena.GetHas(domain) {
			newArena, choice := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
			scope.Arena = newArena
			pointers[index] = SMTElementPointer{Cell: pointer.Element(),
				Expression: booleanAnd(pointer.ToSMT(), choice.NameExpr())}
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: set.Type.(PowerSetCellType).Domain})
		scope.Arena = newArena.AppendHas(result, pointers...)
		rewriter.addMembershipConstraints(scope.Arena, result)
		return scope, result, nil
	case InfiniteSetCellType:
		if !set.Equal(scope.Arena.IntSetCell()) && !set.Equal(scope.Arena.NatSetCell()) {
			return scope, ArenaCell{}, fmt.Errorf("choice from infinite set %s is unsupported", set.Type)
		}
		newArena, result := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
		scope.Arena = newArena
		if set.Equal(scope.Arena.NatSetCell()) {
			rewriter.assert(MustOperExpr(Op("GE"), Typed(BoolT1), result.NameExpr(),
				NewValueExpr(Int64Value(0), Typed(IntT1))))
		}
		return scope, result, nil
	case BasicCellType:
		if !isFiniteSetCellType(set.Type) {
			return scope, ArenaCell{}, fmt.Errorf("choice requires a set, found %s", set.Type)
		}
		setType := set.Type.TLAType().(SetType)
		pointers := scope.Arena.GetHas(set)
		if len(pointers) == 0 {
			return rewriter.defaults.GetOrCreate(scope, setType.Element())
		}
		candidates := make([]ArenaCell, len(pointers))
		for index, pointer := range pointers {
			candidates[index] = pointer.Element()
		}
		scope, oracle, err := CreateIntOracle(scope, len(candidates)+1)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		assertions := make([]Expr, oracle.Size())
		for index, pointer := range pointers {
			assertions[index] = pointer.ToSMT()
		}
		assertions[len(pointers)] = boolExpr(false)
		constraint, err := oracle.Cases(scope, assertions, nil)
		if err != nil {
			return scope, ArenaCell{}, err
		}
		rewriter.assert(constraint)
		return NewCherryPicker(rewriter).Pick(scope, oracle, candidates)
	default:
		return scope, ArenaCell{}, fmt.Errorf("choice from %s is unsupported", set.Type)
	}
}

func (rewriter *StratifiedRewriter) pickFunctionFromSet(scope RewriterScope, functionSet ArenaCell) (RewriterScope, ArenaCell, error) {
	typ, ok := functionSet.Type.(FiniteFunctionSetCellType)
	if !ok {
		return scope, ArenaCell{}, fmt.Errorf("expected a compact function set, found %s", functionSet.Type)
	}
	domain, err := scope.Arena.Domain(functionSet)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	codomain, err := scope.Arena.Codomain(functionSet)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	functionType, _ := NewFunType(typ.ArgumentType(), typ.ResultType())
	pairType, _ := NewTupleType(functionType.Argument(), functionType.Result())
	domainPointers := scope.Arena.GetHas(domain)
	relationPointers := make([]ElementPointer, len(domainPointers))
	for index, pointer := range domainPointers {
		var result ArenaCell
		var pickErr error
		scope, result, pickErr = rewriter.pickFromCompactCollection(scope, codomain)
		if pickErr != nil {
			return scope, ArenaCell{}, fmt.Errorf("pick result for function argument %d: %w", index+1, pickErr)
		}
		newArena, pair := scope.Arena.AppendCellType(BasicCellType{Type: pairType})
		scope.Arena = newArena.AppendHas(pair,
			FixedElementPointer{Cell: pointer.Element()}, FixedElementPointer{Cell: result})
		conditions := []Expr{pointer.ToSMT()}
		for previous := 0; previous < index; previous++ {
			equal, equalErr := rewriter.equality.Equal(scope.Arena,
				pointer.Element(), domainPointers[previous].Element())
			if equalErr != nil {
				return scope, ArenaCell{}, equalErr
			}
			conditions = append(conditions, booleanOr(
				MustOperExpr(Op("NOT"), Typed(BoolT1), domainPointers[previous].ToSMT()),
				MustOperExpr(Op("NOT"), Typed(BoolT1), equal)))
		}
		relationPointers[index] = SMTElementPointer{Cell: pair, Expression: booleanAnd(conditions...)}
	}
	newArena, relation := scope.Arena.AppendCellType(BasicCellType{Type: functionRelationType(functionType)})
	scope.Arena = newArena.AppendHas(relation, relationPointers...)
	rewriter.addMembershipConstraints(scope.Arena, relation)
	newArena, function := scope.Arena.AppendCellType(BasicCellType{Type: functionType})
	scope.Arena = newArena.SetDomain(function, domain).SetCodomain(function, relation)
	return scope, function, nil
}
