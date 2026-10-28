package apalache

import (
	"fmt"
	"strings"
)

type TypeErrorExplanation func(expected, actual []Type) (string, bool)

type EtcExpr interface {
	SourceRef() EtcRef
	Explain(expected, actual []Type) (string, bool)
	SetTypeErrorExplanation(TypeErrorExplanation)
	fmt.Stringer
	etcExpr()
}

type etcExprBase struct {
	sourceRef   EtcRef
	explanation TypeErrorExplanation
}

func newEtcExprBase(source EtcRef) etcExprBase {
	if source == nil {
		panic("ETC expression requires a source reference")
	}
	return etcExprBase{sourceRef: source}
}
func (base *etcExprBase) SourceRef() EtcRef { return base.sourceRef }
func (base *etcExprBase) Explain(expected, actual []Type) (string, bool) {
	if base.explanation == nil {
		return "", false
	}
	return base.explanation(append([]Type(nil), expected...), append([]Type(nil), actual...))
}
func (base *etcExprBase) SetTypeErrorExplanation(explanation TypeErrorExplanation) {
	base.explanation = explanation
}

type EtcConst struct {
	etcExprBase
	polytype Type
}

func (*EtcConst) etcExpr() {}
func NewEtcConst(source EtcRef, polytype Type) *EtcConst {
	if polytype == nil {
		panic("ETC constant requires a type")
	}
	return &EtcConst{etcExprBase: newEtcExprBase(source), polytype: polytype}
}
func (expr *EtcConst) Polytype() Type { return expr.polytype }
func (expr *EtcConst) String() string { return fmt.Sprintf("%s@%s", expr.sourceRef, expr.polytype) }

type EtcName struct {
	etcExprBase
	name string
}

func (*EtcName) etcExpr() {}
func NewEtcName(source EtcRef, name string) *EtcName {
	return &EtcName{etcExprBase: newEtcExprBase(source), name: name}
}
func (expr *EtcName) Name() string   { return expr.name }
func (expr *EtcName) String() string { return fmt.Sprintf("%s@%s", expr.sourceRef, expr.name) }

type EtcBinding struct {
	Name   *EtcName
	Domain EtcExpr
}

type EtcAbs struct {
	etcExprBase
	body     EtcExpr
	bindings []EtcBinding
}

func (*EtcAbs) etcExpr() {}
func NewEtcAbs(source EtcRef, body EtcExpr, bindings ...EtcBinding) *EtcAbs {
	if body == nil {
		panic("ETC abstraction requires a body")
	}
	copy := make([]EtcBinding, len(bindings))
	for i, binding := range bindings {
		if binding.Name == nil || binding.Domain == nil {
			panic("ETC abstraction requires valid bindings")
		}
		copy[i] = binding
	}
	return &EtcAbs{etcExprBase: newEtcExprBase(source), body: body, bindings: copy}
}
func (expr *EtcAbs) Body() EtcExpr { return expr.body }
func (expr *EtcAbs) Bindings() []EtcBinding {
	return append([]EtcBinding(nil), expr.bindings...)
}
func (expr *EtcAbs) String() string {
	bindings := make([]string, len(expr.bindings))
	for i, binding := range expr.bindings {
		bindings[i] = fmt.Sprintf("%s ∈ %s", binding.Name, binding.Domain)
	}
	return fmt.Sprintf("%s@λ %s. %s", expr.sourceRef, strings.Join(bindings, ", "), expr.body)
}

type EtcApp struct {
	etcExprBase
	operatorTypes []Type
	arguments     []EtcExpr
}

func (*EtcApp) etcExpr() {}
func NewEtcApp(source EtcRef, operatorTypes []Type, arguments ...EtcExpr) *EtcApp {
	types := append([]Type(nil), operatorTypes...)
	for _, typ := range types {
		if typ == nil {
			panic("ETC application requires non-nil operator types")
		}
	}
	args := append([]EtcExpr(nil), arguments...)
	for _, argument := range args {
		if argument == nil {
			panic("ETC application requires non-nil arguments")
		}
	}
	return &EtcApp{etcExprBase: newEtcExprBase(source), operatorTypes: types, arguments: args}
}
func (expr *EtcApp) OperatorTypes() []Type { return append([]Type(nil), expr.operatorTypes...) }
func (expr *EtcApp) Arguments() []EtcExpr  { return append([]EtcExpr(nil), expr.arguments...) }
func (expr *EtcApp) String() string {
	types := make([]string, len(expr.operatorTypes))
	for i, typ := range expr.operatorTypes {
		types[i] = typ.String()
	}
	arguments := make([]string, len(expr.arguments))
	for i, argument := range expr.arguments {
		arguments[i] = argument.String()
	}
	return fmt.Sprintf("%s@((%s) %s)", expr.sourceRef, strings.Join(types, " | "), strings.Join(arguments, " "))
}

type EtcAppByName struct {
	etcExprBase
	name      *EtcName
	arguments []EtcExpr
}

func (*EtcAppByName) etcExpr() {}
func NewEtcAppByName(source EtcRef, name *EtcName, arguments ...EtcExpr) *EtcAppByName {
	if name == nil {
		panic("named ETC application requires a name")
	}
	args := append([]EtcExpr(nil), arguments...)
	for _, argument := range args {
		if argument == nil {
			panic("named ETC application requires non-nil arguments")
		}
	}
	return &EtcAppByName{etcExprBase: newEtcExprBase(source), name: name, arguments: args}
}
func (expr *EtcAppByName) Name() *EtcName       { return expr.name }
func (expr *EtcAppByName) Arguments() []EtcExpr { return append([]EtcExpr(nil), expr.arguments...) }
func (expr *EtcAppByName) String() string {
	arguments := make([]string, len(expr.arguments))
	for i, argument := range expr.arguments {
		arguments[i] = argument.String()
	}
	return fmt.Sprintf("%s@(%s %s)", expr.sourceRef, expr.name, strings.Join(arguments, " "))
}

type EtcLet struct {
	etcExprBase
	name  string
	bound EtcExpr
	body  EtcExpr
}

func (*EtcLet) etcExpr() {}
func NewEtcLet(source EtcRef, name string, bound, body EtcExpr) *EtcLet {
	if bound == nil || body == nil {
		panic("ETC let requires bound and body expressions")
	}
	return &EtcLet{etcExprBase: newEtcExprBase(source), name: name, bound: bound, body: body}
}
func (expr *EtcLet) Name() string   { return expr.name }
func (expr *EtcLet) Bound() EtcExpr { return expr.bound }
func (expr *EtcLet) Body() EtcExpr  { return expr.body }
func (expr *EtcLet) String() string {
	return fmt.Sprintf("%s@let %s = %s in %s", expr.sourceRef, expr.name, expr.bound, expr.body)
}

type EtcTypeDecl struct {
	etcExprBase
	name         string
	declaredType Type
	scoped       EtcExpr
}

func (*EtcTypeDecl) etcExpr() {}
func NewEtcTypeDecl(source EtcRef, name string, declaredType Type, scoped EtcExpr) *EtcTypeDecl {
	if declaredType == nil || scoped == nil {
		panic("ETC type declaration requires a type and scoped expression")
	}
	return &EtcTypeDecl{etcExprBase: newEtcExprBase(source), name: name, declaredType: declaredType, scoped: scoped}
}
func (expr *EtcTypeDecl) Name() string       { return expr.name }
func (expr *EtcTypeDecl) DeclaredType() Type { return expr.declaredType }
func (expr *EtcTypeDecl) Scoped() EtcExpr    { return expr.scoped }
func (expr *EtcTypeDecl) String() string {
	return fmt.Sprintf("%s@%s: %s in %s", expr.sourceRef, expr.name, expr.declaredType, expr.scoped)
}

func NewUniqueEtcConst(typ Type) *EtcConst  { return NewEtcConst(ExactRef{SourceID: NewUID()}, typ) }
func NewBlameEtcConst(typ Type) *EtcConst   { return NewEtcConst(BlameRef{SourceID: NewUID()}, typ) }
func NewUniqueEtcName(name string) *EtcName { return NewEtcName(ExactRef{SourceID: NewUID()}, name) }
func NewBlameEtcName(name string) *EtcName  { return NewEtcName(BlameRef{SourceID: NewUID()}, name) }
func NewUniqueEtcAbs(body EtcExpr, bindings ...EtcBinding) *EtcAbs {
	return NewEtcAbs(ExactRef{SourceID: NewUID()}, body, bindings...)
}
func NewBlameEtcAbs(body EtcExpr, bindings ...EtcBinding) *EtcAbs {
	return NewEtcAbs(BlameRef{SourceID: NewUID()}, body, bindings...)
}
func NewUniqueEtcApp(operatorTypes []Type, arguments ...EtcExpr) *EtcApp {
	return NewEtcApp(ExactRef{SourceID: NewUID()}, operatorTypes, arguments...)
}
func NewBlameEtcApp(operatorTypes []Type, arguments ...EtcExpr) *EtcApp {
	return NewEtcApp(BlameRef{SourceID: NewUID()}, operatorTypes, arguments...)
}
func NewUniqueEtcAppByName(name *EtcName, arguments ...EtcExpr) *EtcAppByName {
	return NewEtcAppByName(ExactRef{SourceID: NewUID()}, name, arguments...)
}
func NewBlameEtcAppByName(name *EtcName, arguments ...EtcExpr) *EtcAppByName {
	return NewEtcAppByName(BlameRef{SourceID: NewUID()}, name, arguments...)
}
func NewUniqueEtcLet(name string, bound, body EtcExpr) *EtcLet {
	return NewEtcLet(ExactRef{SourceID: NewUID()}, name, bound, body)
}
func NewBlameEtcLet(name string, bound, body EtcExpr) *EtcLet {
	return NewEtcLet(BlameRef{SourceID: NewUID()}, name, bound, body)
}
func NewUniqueEtcTypeDecl(name string, declaredType Type, scoped EtcExpr) *EtcTypeDecl {
	return NewEtcTypeDecl(ExactRef{SourceID: NewUID()}, name, declaredType, scoped)
}
func NewBlameEtcTypeDecl(name string, declaredType Type, scoped EtcExpr) *EtcTypeDecl {
	return NewEtcTypeDecl(BlameRef{SourceID: NewUID()}, name, declaredType, scoped)
}

// EqualEtcExpr compares ETC structure while deliberately ignoring source
// references and diagnostic callbacks, matching Scala case-class equality.
func EqualEtcExpr(left, right EtcExpr) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	switch l := left.(type) {
	case *EtcConst:
		r, ok := right.(*EtcConst)
		return ok && EqualTypes(l.polytype, r.polytype)
	case *EtcName:
		r, ok := right.(*EtcName)
		return ok && l.name == r.name
	case *EtcAbs:
		r, ok := right.(*EtcAbs)
		if !ok || !EqualEtcExpr(l.body, r.body) || len(l.bindings) != len(r.bindings) {
			return false
		}
		for i := range l.bindings {
			if !EqualEtcExpr(l.bindings[i].Name, r.bindings[i].Name) || !EqualEtcExpr(l.bindings[i].Domain, r.bindings[i].Domain) {
				return false
			}
		}
		return true
	case *EtcApp:
		r, ok := right.(*EtcApp)
		if !ok || len(l.operatorTypes) != len(r.operatorTypes) || len(l.arguments) != len(r.arguments) {
			return false
		}
		for i := range l.operatorTypes {
			if !EqualTypes(l.operatorTypes[i], r.operatorTypes[i]) {
				return false
			}
		}
		for i := range l.arguments {
			if !EqualEtcExpr(l.arguments[i], r.arguments[i]) {
				return false
			}
		}
		return true
	case *EtcAppByName:
		r, ok := right.(*EtcAppByName)
		if !ok || !EqualEtcExpr(l.name, r.name) || len(l.arguments) != len(r.arguments) {
			return false
		}
		for i := range l.arguments {
			if !EqualEtcExpr(l.arguments[i], r.arguments[i]) {
				return false
			}
		}
		return true
	case *EtcLet:
		r, ok := right.(*EtcLet)
		return ok && l.name == r.name && EqualEtcExpr(l.bound, r.bound) && EqualEtcExpr(l.body, r.body)
	case *EtcTypeDecl:
		r, ok := right.(*EtcTypeDecl)
		return ok && l.name == r.name && EqualTypes(l.declaredType, r.declaredType) && EqualEtcExpr(l.scoped, r.scoped)
	default:
		return false
	}
}
