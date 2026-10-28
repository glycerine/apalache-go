package apalache

import (
	"reflect"
	"testing"
)

func TestTypeContextIsImmutableAndSorted(t *testing.T) {
	intScheme, _ := NewTypeScheme(IntT1)
	boolScheme, _ := NewTypeScheme(BoolT1)
	original := map[string]TypeScheme{"z": intScheme}
	context := NewTypeContext(original)
	original["a"] = boolScheme
	context2 := context.Bind("b", boolScheme)
	if _, err := context.Lookup("a"); err == nil {
		t.Fatal("context retained caller map mutation")
	}
	if _, err := context.Lookup("b"); err == nil {
		t.Fatal("Bind mutated original context")
	}
	if got := context2.Names(); !reflect.DeepEqual(got, []string{"b", "z"}) {
		t.Fatalf("Names() = %v", got)
	}
	if scheme, err := context2.Lookup("b"); err != nil || !EqualTypes(scheme.Principal(), BoolT1) {
		t.Fatalf("Lookup() = %v, %v", scheme, err)
	}
}

type recordingTypeListener struct{ events []string }

func (listener *recordingTypeListener) OnTypeFound(ref ExactRef, typ Type) {
	listener.events = append(listener.events, "found:"+ref.String()+":"+typ.String())
}
func (listener *recordingTypeListener) OnTypeError(ref EtcRef, message string) {
	listener.events = append(listener.events, "error:"+ref.String()+":"+message)
}
func (listener *recordingTypeListener) OnTypeWarning(ref EtcRef, message string) {
	listener.events = append(listener.events, "warning:"+ref.String()+":"+message)
}

func TestMultiTypeCheckerListenerBroadcastsInOrder(t *testing.T) {
	first, second := &recordingTypeListener{}, &recordingTypeListener{}
	listener := NewMultiTypeCheckerListener(first, nil, second)
	exact := ExactRef{SourceID: NewUID()}
	blame := BlameRef{SourceID: exact.SourceID}
	listener.OnTypeFound(exact, IntT1)
	listener.OnTypeError(blame, "bad")
	listener.OnTypeWarning(blame, "maybe")
	want := []string{"found:" + exact.String() + ":Int", "error:" + blame.String() + ":bad", "warning:" + blame.String() + ":maybe"}
	if !reflect.DeepEqual(first.events, want) || !reflect.DeepEqual(second.events, want) {
		t.Fatalf("events = %v, %v; want %v", first.events, second.events, want)
	}
}

func TestIdleTypeCheckerListenerAcceptsEveryEvent(t *testing.T) {
	listener := IdleTypeCheckerListener{}
	exact := ExactRef{SourceID: NewUID()}
	listener.OnTypeFound(exact, IntT1)
	listener.OnTypeError(BlameRef{SourceID: exact.SourceID}, "bad")
	listener.OnTypeWarning(exact, "maybe")
}

func TestMultiTypeCheckerListenerOwnsSubscriberSequence(t *testing.T) {
	first, replacement := &recordingTypeListener{}, &recordingTypeListener{}
	subscribers := []TypeCheckerListener{first}
	listener := NewMultiTypeCheckerListener(subscribers...)
	subscribers[0] = replacement
	listener.OnTypeFound(ExactRef{SourceID: NewUID()}, BoolT1)
	if len(first.events) != 1 || len(replacement.events) != 0 {
		t.Fatalf("subscriber events = %v, %v", first.events, replacement.events)
	}
}
