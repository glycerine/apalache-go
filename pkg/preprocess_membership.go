package apalache

import (
	"fmt"
	"sync/atomic"
)

// UniqueNameGenerator produces the deterministic base-36 names used by the
// Scala preprocessing passes. It is safe for concurrent use.
type UniqueNameGenerator struct{ next atomic.Uint64 }

func NewUniqueNameGenerator() *UniqueNameGenerator { return &UniqueNameGenerator{} }
func (generator *UniqueNameGenerator) NewName() string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	value := generator.next.Add(1)
	buffer := [32]byte{}
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = digits[value%36]
		value /= 36
	}
	return "t_" + string(buffer[index:])
}

// SimplifySetMembership rewrites well-typed TypeOK-style memberships. In the
// early mode it performs only nested function-range rewrites safe before
// keramelization.
func SimplifySetMembership(root Expr, generator *UniqueNameGenerator, tracker TransformationTracker, beforeKeramelizer bool) (Expr, error) {
	if root == nil {
		return nil, fmt.Errorf("cannot simplify membership in a nil expression")
	}
	if err := WatchExpression(FlatLanguagePredicate{}, root); err != nil {
		return nil, err
	}
	if generator == nil {
		generator = NewUniqueNameGenerator()
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	var transform func(Expr) (Expr, error)
	transform = func(expression Expr) (Expr, error) {
		var shallow Expr = expression
		switch typed := expression.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			changed := false
			for index, argument := range arguments {
				var err error
				arguments[index], err = transform(argument)
				if err != nil {
					return nil, err
				}
				changed = changed || arguments[index].ID() != argument.ID()
			}
			if changed {
				shallow = tracker.Hold(typed, MustOperExpr(typed.Operator(), typed.Tag(), arguments...))
			}
		case *LetInExpr:
			body, err := transform(typed.Body())
			if err != nil {
				return nil, err
			}
			declarations := typed.Declarations()
			for index, declaration := range declarations {
				body, err := transform(declaration.Body())
				if err != nil {
					return nil, err
				}
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(body)
				declarations[index] = copy
			}
			result := NewLetInExpr(body, typed.Tag(), declarations...)
			return tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
		}
		result := shallow
		if membership, ok := shallow.(*OperExpr); ok && membership.Operator().Name() == "SET_IN" {
			arguments := membership.Arguments()
			var replacement Expr
			var found bool
			var err error
			if beforeKeramelizer {
				if earlyMembershipReducible(arguments[1]) {
					replacement, found, err = simplifyMembership(arguments[0], arguments[1], generator)
				}
			} else {
				replacement, found, err = simplifyMembership(arguments[0], arguments[1], generator)
			}
			if err != nil {
				return nil, err
			}
			if found {
				result = replacement
			}
		}
		result = tracker.TrackExpression(func(Expr) Expr { return result })(shallow)
		return tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
	}
	return transform(root)
}

func isTypeDefiningSet(expression Expr) bool {
	if value, ok := expression.(*ValueExpr); ok {
		set, ok := value.Value().(PredefinedSet)
		return ok && (set == BoolSet || set == IntSet || set == RealSet || set == StringSet)
	}
	operation, ok := expression.(*OperExpr)
	if !ok {
		return false
	}
	arguments := operation.Arguments()
	switch operation.Operator().Name() {
	case "Sequences!Seq", "SET_POWERSET":
		return isTypeDefiningSet(arguments[0])
	case "FUN_SET":
		return isTypeDefiningSet(arguments[0]) && isTypeDefiningSet(arguments[1])
	case "SET_TIMES":
		for _, argument := range arguments {
			if !isTypeDefiningSet(argument) {
				return false
			}
		}
		return true
	case "RECORD_SET":
		for index := 1; index < len(arguments); index += 2 {
			if !isTypeDefiningSet(arguments[index]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func simplifyMembership(element, set Expr, generator *UniqueNameGenerator) (Expr, bool, error) {
	trueValue := func() Expr { return NewValueExpr(BoolValue(true), Typed(BoolT1)) }
	if isTypeDefiningSet(set) {
		return trueValue(), true, nil
	}
	if value, ok := set.(*ValueExpr); ok {
		if predefined, ok := value.Value().(PredefinedSet); ok && predefined == NatSet {
			return MustOperExpr(Op("GE"), Typed(BoolT1), element, NewValueExpr(Int64Value(0), Typed(IntT1))), true, nil
		}
	}
	structuredSet, ok := set.(*OperExpr)
	if !ok {
		return nil, false, nil
	}
	functionSet := structuredSet
	if functionSet.Operator().Name() != "FUN_SET" {
		return nil, false, nil
	}
	arguments := functionSet.Arguments()
	domain, codomain := arguments[0], arguments[1]
	domainEquality := func(function Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("DOMAIN"), domain.Tag(), function), domain)
	}
	if isTypeDefiningSet(codomain) {
		return domainEquality(element), true, nil
	}
	domainType, err := membershipSetElementType(domain)
	if err != nil {
		return nil, false, err
	}
	codomainType, err := membershipSetElementType(codomain)
	if err != nil {
		return nil, false, err
	}
	bound := NewNameExpr(generator.NewName(), Typed(domainType))
	application := MustOperExpr(Op("FUN_APP"), Typed(codomainType), element, bound)
	rangeConstraint, found, err := simplifyMembership(application, codomain, generator)
	if err != nil || !found {
		return nil, false, err
	}
	forall := MustOperExpr(Op("FORALL3"), Typed(BoolT1), bound, domain, rangeConstraint)
	return MustOperExpr(Op("AND"), Typed(BoolT1), domainEquality(element), forall), true, nil
}

func membershipSetElementType(expression Expr) (Type, error) {
	typ, ok := expression.Tag().Type()
	if !ok {
		return nil, NewMalformedTlaError("Expected a set, found: "+expression.Tag().String(), expression)
	}
	set, ok := typ.(SetType)
	if !ok {
		return nil, NewMalformedTlaError("Expected a set, found: "+expression.Tag().String(), expression)
	}
	return set.Element(), nil
}

func setElementType(expression Expr) (Type, error) {
	typ, ok := expression.Tag().Type()
	if !ok {
		return nil, fmt.Errorf("expected a typed set")
	}
	set, ok := typ.(SetType)
	if !ok {
		return nil, fmt.Errorf("expected a set, found %s", typ)
	}
	return set.Element(), nil
}

func earlyMembershipReducible(set Expr) bool {
	outer, ok := set.(*OperExpr)
	if !ok || outer.Operator().Name() != "FUN_SET" {
		return false
	}
	arguments := outer.Arguments()
	inner, ok := arguments[1].(*OperExpr)
	if !ok || inner.Operator().Name() != "FUN_SET" {
		return false
	}
	codomain := inner.Arguments()[1]
	for {
		if value, ok := codomain.(*ValueExpr); ok {
			predefined, ok := value.Value().(PredefinedSet)
			return ok && predefined == NatSet
		}
		nested, ok := codomain.(*OperExpr)
		if !ok || nested.Operator().Name() != "FUN_SET" {
			return false
		}
		codomain = nested.Arguments()[1]
	}
}
