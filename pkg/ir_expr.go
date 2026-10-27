package apalache

import "fmt"

// Expr is the closed core expression algebra used by the Apalache IR.
// Implementations are immutable and own any slices passed to their constructors.
type Expr interface {
	ID() UID
	Tag() TypeTag
	WithTag(TypeTag) Expr
	exprNode()
}

type exprIdentity struct {
	id  UID
	tag TypeTag
}

func newExprIdentity(tag TypeTag) exprIdentity { return exprIdentity{id: NewUID(), tag: tag} }
func (identity exprIdentity) ID() UID          { return identity.id }
func (identity exprIdentity) Tag() TypeTag     { return identity.tag }

type nullExpr struct{ exprIdentity }

func (*nullExpr) exprNode() {}
func (expr *nullExpr) WithTag(TypeTag) Expr {
	return expr
}

var nullExpression Expr = &nullExpr{exprIdentity: newExprIdentity(Untyped())}

// NullExpr returns the singleton expression used for definitions without a
// meaningful body. It is always untyped.
func NullExpr() Expr { return nullExpression }

type ValueExpr struct {
	exprIdentity
	value Value
}

func (*ValueExpr) exprNode() {}
func NewValueExpr(value Value, tag TypeTag) *ValueExpr {
	if value == nil {
		panic("NewValueExpr requires a non-nil value")
	}
	return &ValueExpr{exprIdentity: newExprIdentity(tag), value: value}
}
func (expr *ValueExpr) Value() Value { return expr.value }
func (expr *ValueExpr) WithTag(tag TypeTag) Expr {
	return NewValueExpr(expr.value, tag)
}

type NameExpr struct {
	exprIdentity
	name string
}

func (*NameExpr) exprNode() {}
func NewNameExpr(name string, tag TypeTag) *NameExpr {
	return &NameExpr{exprIdentity: newExprIdentity(tag), name: name}
}
func (expr *NameExpr) Name() string { return expr.name }
func (expr *NameExpr) WithTag(tag TypeTag) Expr {
	return NewNameExpr(expr.name, tag)
}

type LetInExpr struct {
	exprIdentity
	body  Expr
	decls []*OperDecl
}

func (*LetInExpr) exprNode() {}
func NewLetInExpr(body Expr, tag TypeTag, decls ...*OperDecl) *LetInExpr {
	if body == nil {
		panic("NewLetInExpr requires a non-nil body")
	}
	for _, decl := range decls {
		if decl == nil {
			panic("NewLetInExpr requires non-nil declarations")
		}
	}
	return &LetInExpr{exprIdentity: newExprIdentity(tag), body: body, decls: append([]*OperDecl(nil), decls...)}
}
func (expr *LetInExpr) Body() Expr { return expr.body }
func (expr *LetInExpr) Declarations() []*OperDecl {
	return append([]*OperDecl(nil), expr.decls...)
}
func (expr *LetInExpr) WithTag(tag TypeTag) Expr {
	return NewLetInExpr(expr.body, tag, expr.decls...)
}

type OperExpr struct {
	exprIdentity
	operator Operator
	args     []Expr
}

func (*OperExpr) exprNode() {}

// NewOperExpr checks the same arity and operator-specific invariants as the
// Scala OperEx constructor.
func NewOperExpr(operator Operator, tag TypeTag, args ...Expr) (*OperExpr, error) {
	if !operator.Constructible() {
		return nil, fmt.Errorf("operator %s cannot be constructed directly", operator.Name())
	}
	if !operator.IsCorrectArity(len(args)) {
		return nil, fmt.Errorf("unexpected arity %d in %s", len(args), operator.Name())
	}
	for index, arg := range args {
		if arg == nil {
			return nil, fmt.Errorf("nil argument %d in %s", index, operator.Name())
		}
	}
	if operator.Name() == "LABEL" {
		for _, arg := range args[1:] {
			valueExpr, ok := arg.(*ValueExpr)
			if !ok {
				return nil, fmt.Errorf("the invariant of LABEL is violated by its arguments")
			}
			if _, ok := valueExpr.Value().(StringValue); !ok {
				return nil, fmt.Errorf("the invariant of LABEL is violated by its arguments")
			}
		}
	}
	return &OperExpr{exprIdentity: newExprIdentity(tag), operator: operator, args: append([]Expr(nil), args...)}, nil
}

// MustOperExpr is convenient for statically known, valid IR construction.
func MustOperExpr(operator Operator, tag TypeTag, args ...Expr) *OperExpr {
	expr, err := NewOperExpr(operator, tag, args...)
	if err != nil {
		panic(err)
	}
	return expr
}

func (expr *OperExpr) Operator() Operator { return expr.operator }
func (expr *OperExpr) Arguments() []Expr  { return append([]Expr(nil), expr.args...) }
func (expr *OperExpr) WithTag(tag TypeTag) Expr {
	return MustOperExpr(expr.operator, tag, expr.args...)
}

// EqualExpr compares expression structure, like Scala case-class equality.
// Node IDs and type tags are intentionally excluded; use EqualTypedExpr when
// the root type tag is significant.
func EqualExpr(left, right Expr) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	switch l := left.(type) {
	case *nullExpr:
		_, ok := right.(*nullExpr)
		return ok
	case *ValueExpr:
		r, ok := right.(*ValueExpr)
		return ok && EqualValues(l.value, r.value)
	case *NameExpr:
		r, ok := right.(*NameExpr)
		return ok && l.name == r.name
	case *LetInExpr:
		r, ok := right.(*LetInExpr)
		if !ok || !EqualExpr(l.body, r.body) || len(l.decls) != len(r.decls) {
			return false
		}
		for index := range l.decls {
			if !EqualDecl(l.decls[index], r.decls[index]) {
				return false
			}
		}
		return true
	case *OperExpr:
		r, ok := right.(*OperExpr)
		if !ok || !l.operator.Equal(r.operator) || len(l.args) != len(r.args) {
			return false
		}
		for index := range l.args {
			if !EqualExpr(l.args[index], r.args[index]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func EqualTypedExpr(left, right Expr) bool {
	return EqualExpr(left, right) && left != nil && right != nil && left.Tag().Equal(right.Tag())
}
