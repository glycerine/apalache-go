package apalache

import (
	"reflect"
	"testing"
)

func TestLoggingTypeCheckerListenerSourceAwareEvents(t *testing.T) {
	store := NewSourceStore()
	id := NewUID()
	location := SourceLocation{Filename: "Types", Region: NewSourceRegion(3, 4, 3, 10)}
	if err := store.Add(id, location); err != nil {
		t.Fatal(err)
	}
	events := []string{}
	logger := TypeCheckerLoggerFunc(func(level, message string) { events = append(events, level+":"+message) })
	listener := NewLoggingTypeCheckerListener(store, logger, true)
	listener.OnTypeFound(ExactRef{SourceID: id}, parsedType(t, "a"))
	listener.OnTypeError(BlameRef{SourceID: id}, "bad")
	listener.OnTypeWarning(ExactRef{SourceID: NewUID()}, "maybe")
	want := []string{
		"error:[Types.tla:3:4-3:10]: bad",
		"debug:[unknown location]: maybe",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestLoggingTypeCheckerListenerRejectsPolymorphismWhenDisabled(t *testing.T) {
	events := []string{}
	logger := TypeCheckerLoggerFunc(func(level, message string) { events = append(events, level+":"+message) })
	listener := NewLoggingTypeCheckerListener(nil, logger, false)
	ref := ExactRef{SourceID: NewUID()}
	defer func() {
		recovered := recover()
		err, ok := recovered.(*TypingInputException)
		if !ok || err.CauseExprID() != ref.ID() || err.Error() != "Found a polymorphic type: a" {
			t.Fatalf("panic = %T %v", recovered, recovered)
		}
		want := []string{
			"error:Found a polymorphic type: a",
			"error:Probable causes: an empty set { } needs a type annotation or an incorrect record field is used",
		}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}()
	listener.OnTypeFound(ref, parsedType(t, "a"))
}

func TestLoggingTypeCheckerListenerAllowsMonotypesWhenDisabled(t *testing.T) {
	NewLoggingTypeCheckerListener(nil, nil, false).OnTypeFound(ExactRef{SourceID: NewUID()}, IntT1)
}
