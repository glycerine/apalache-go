package apalache

import (
	"strings"
	"testing"
)

type collectedTypeEvent struct {
	kind    string
	id      UID
	typ     Type
	message string
}
type collectingTypeListener struct{ events []collectedTypeEvent }

func (listener *collectingTypeListener) OnTypeFound(ref ExactRef, typ Type) {
	listener.events = append(listener.events, collectedTypeEvent{kind: "found", id: ref.ID(), typ: typ})
}
func (listener *collectingTypeListener) OnTypeError(ref EtcRef, message string) {
	listener.events = append(listener.events, collectedTypeEvent{kind: "error", id: ref.ID(), message: message})
}
func (listener *collectingTypeListener) OnTypeWarning(ref EtcRef, message string) {
	listener.events = append(listener.events, collectedTypeEvent{kind: "warning", id: ref.ID(), message: message})
}
func (listener *collectingTypeListener) found(id UID, typ Type) bool {
	for _, event := range listener.events {
		if event.kind == "found" && event.id == id && EqualTypes(event.typ, typ) {
			return true
		}
	}
	return false
}
func (listener *collectingTypeListener) errorContains(id UID, text string) bool {
	for _, event := range listener.events {
		if event.kind == "error" && event.id == id && strings.Contains(event.message, text) {
			return true
		}
	}
	return false
}

func newEtcChecker() *EtcTypeChecker { return NewEtcTypeChecker(NewTypeVarPool(1000), true) }

func TestEtcTypeCheckerMonotypeNameAndApplication(t *testing.T) {
	listener := &collectingTypeListener{}
	argument := NewUniqueEtcConst(IntT1)
	application := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(Int) => Bool")}, argument)
	typ, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), application)
	if err != nil || !ok || !EqualTypes(typ, BoolT1) {
		t.Fatalf("Compute() = %v, %v, %v", typ, ok, err)
	}
	if !listener.found(argument.SourceRef().ID(), IntT1) || !listener.found(application.SourceRef().ID(), BoolT1) {
		t.Fatalf("missing type events: %+v", listener.events)
	}

	name := NewUniqueEtcName("value")
	scheme, _ := NewTypeScheme(parsedType(t, "Set(Int)"))
	typ, ok, err = newEtcChecker().Compute(listener, NewTypeContext(map[string]TypeScheme{"value": scheme}), name)
	if err != nil || !ok || !EqualTypes(typ, parsedType(t, "Set(Int)")) {
		t.Fatalf("name Compute() = %v, %v, %v", typ, ok, err)
	}
}

func TestEtcTypeCheckerOverloadsAndCustomErrors(t *testing.T) {
	argument := NewUniqueEtcConst(IntT1)
	application := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{
		parsedType(t, "(Int) => Int"), parsedType(t, "(Bool) => Bool"),
	}, argument)
	listener := &collectingTypeListener{}
	typ, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), application)
	if err != nil || !ok || !EqualTypes(typ, IntT1) {
		t.Fatalf("overload Compute() = %v, %v, %v; events %+v", typ, ok, err, listener.events)
	}

	bad := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{parsedType(t, "(Int) => Int")}, NewUniqueEtcConst(BoolT1))
	bad.SetTypeErrorExplanation(func(_, _ []Type) (string, bool) { return "custom mismatch", true })
	listener = &collectingTypeListener{}
	if _, ok, err = newEtcChecker().Compute(listener, EmptyTypeContext(), bad); err != nil || ok {
		t.Fatalf("bad Compute() = %v, %v", ok, err)
	}
	if !listener.errorContains(bad.SourceRef().ID(), "custom mismatch") {
		t.Fatalf("missing custom error: %+v", listener.events)
	}
}

func TestEtcTypeCheckerInstantiatesPolymorphicName(t *testing.T) {
	a := variableType(t, "a")
	identity, _ := NewOperatorType([]Type{a}, a)
	scheme, _ := NewTypeScheme(identity, a.Number())
	context := NewTypeContext(map[string]TypeScheme{"F": scheme})
	intCall := NewEtcAppByName(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("F"), NewUniqueEtcConst(IntT1))
	strCall := NewEtcAppByName(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("F"), NewUniqueEtcConst(StrT1))
	pairType := parsedType(t, "(Int, Str) => Bool")
	combined := NewEtcApp(ExactRef{SourceID: NewUID()}, []Type{pairType}, intCall, strCall)
	typ, ok, err := newEtcChecker().Compute(&collectingTypeListener{}, context, combined)
	if err != nil || !ok || !EqualTypes(typ, BoolT1) {
		t.Fatalf("Compute() = %v, %v, %v", typ, ok, err)
	}
}

func TestEtcTypeCheckerAbstractionAndDomainError(t *testing.T) {
	x := NewUniqueEtcName("x")
	domain := NewUniqueEtcConst(parsedType(t, "Set(Int)"))
	abstraction := NewEtcAbs(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("x"), EtcBinding{Name: x, Domain: domain})
	listener := &collectingTypeListener{}
	typ, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), abstraction)
	if err != nil || !ok || !EqualTypes(typ, parsedType(t, "(Int) => Int")) {
		t.Fatalf("Compute() = %v, %v, %v; events %+v", typ, ok, err, listener.events)
	}
	badDomain := NewUniqueEtcConst(IntT1)
	bad := NewEtcAbs(ExactRef{SourceID: NewUID()}, NewUniqueEtcConst(BoolT1), EtcBinding{Name: NewUniqueEtcName("x"), Domain: badDomain})
	listener = &collectingTypeListener{}
	if _, ok, err = newEtcChecker().Compute(listener, EmptyTypeContext(), bad); err != nil || ok {
		t.Fatalf("bad domain Compute() = %v, %v", ok, err)
	}
	if !listener.errorContains(badDomain.SourceRef().ID(), "Expected a set. Found: Int") {
		t.Fatalf("missing domain error: %+v", listener.events)
	}
}

func TestEtcTypeCheckerInfersPolymorphicLet(t *testing.T) {
	a := variableType(t, "a")
	xName := NewUniqueEtcName("x")
	domainType, _ := NewSetType(a)
	domain := NewUniqueEtcConst(domainType)
	identity := NewEtcAbs(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("x"), EtcBinding{Name: xName, Domain: domain})
	call := NewEtcAppByName(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("F"), NewUniqueEtcConst(IntT1))
	let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", identity, call)
	listener := &collectingTypeListener{}
	typ, ok, err := newEtcChecker().Compute(listener, EmptyTypeContext(), let)
	if err != nil || !ok || !EqualTypes(typ, IntT1) {
		t.Fatalf("Compute() = %v, %v, %v; events %+v", typ, ok, err, listener.events)
	}
	if !listener.found(identity.SourceRef().ID(), parsedType(t, "(a) => a")) {
		// Variable numbers are canonicalized by the unifier; check shape if the
		// exact printed variable differs from Scala's low-number fixture.
		foundShape := false
		for _, event := range listener.events {
			if event.kind != "found" || event.id != identity.SourceRef().ID() {
				continue
			}
			if operator, isOperator := event.typ.(OperatorType); isOperator && len(operator.Arguments()) == 1 && EqualTypes(operator.Arguments()[0], operator.Result()) {
				foundShape = true
			}
		}
		if !foundShape {
			t.Fatalf("missing polymorphic identity type: %+v", listener.events)
		}
	}
}

func TestEtcTypeCheckerCanDisableLetPolymorphism(t *testing.T) {
	a := variableType(t, "a")
	domainType, _ := NewSetType(a)
	abstraction := NewEtcAbs(ExactRef{SourceID: NewUID()}, NewUniqueEtcName("x"),
		EtcBinding{Name: NewUniqueEtcName("x"), Domain: NewUniqueEtcConst(domainType)})
	let := NewEtcLet(ExactRef{SourceID: NewUID()}, "F", abstraction, NewUniqueEtcConst(BoolT1))
	listener := &collectingTypeListener{}
	checker := NewEtcTypeChecker(NewTypeVarPool(1000), false)
	if _, ok, err := checker.Compute(listener, EmptyTypeContext(), let); err != nil || ok {
		t.Fatalf("Compute() = %v, %v", ok, err)
	}
	if !listener.errorContains(let.SourceRef().ID(), "polymorphism is disabled") {
		t.Fatalf("missing polymorphism error: %+v", listener.events)
	}
}
