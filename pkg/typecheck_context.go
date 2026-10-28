package apalache

import (
	"fmt"
	"sort"
)

// TypeContext assigns type schemes to operator and bound-variable names.
type TypeContext struct{ types map[string]TypeScheme }

func NewTypeContext(types map[string]TypeScheme) TypeContext {
	copy := make(map[string]TypeScheme, len(types))
	for name, scheme := range types {
		copy[name] = scheme
	}
	return TypeContext{types: copy}
}

func EmptyTypeContext() TypeContext { return NewTypeContext(nil) }

func (context TypeContext) Lookup(name string) (TypeScheme, error) {
	scheme, found := context.types[name]
	if !found {
		return TypeScheme{}, fmt.Errorf("No type binding for %s in the type context", name)
	}
	return scheme, nil
}

func (context TypeContext) Bind(name string, scheme TypeScheme) TypeContext {
	types := context.Types()
	types[name] = scheme
	return NewTypeContext(types)
}

func (context TypeContext) Types() map[string]TypeScheme {
	result := make(map[string]TypeScheme, len(context.types))
	for name, scheme := range context.types {
		result[name] = scheme
	}
	return result
}

func (context TypeContext) Names() []string {
	names := make([]string, 0, len(context.types))
	for name := range context.types {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type EtcRef interface {
	ID() UID
	isEtcRef()
	fmt.Stringer
}

type ExactRef struct{ SourceID UID }

func (ExactRef) isEtcRef()          {}
func (ref ExactRef) ID() UID        { return ref.SourceID }
func (ref ExactRef) String() string { return "E" + ref.SourceID.String() }

type BlameRef struct{ SourceID UID }

func (BlameRef) isEtcRef()          {}
func (ref BlameRef) ID() UID        { return ref.SourceID }
func (ref BlameRef) String() string { return "B" + ref.SourceID.String() }

type TypeCheckerListener interface {
	OnTypeFound(ExactRef, Type)
	OnTypeError(EtcRef, string)
	OnTypeWarning(EtcRef, string)
}

type IdleTypeCheckerListener struct{}

func (IdleTypeCheckerListener) OnTypeFound(ExactRef, Type)   {}
func (IdleTypeCheckerListener) OnTypeError(EtcRef, string)   {}
func (IdleTypeCheckerListener) OnTypeWarning(EtcRef, string) {}

type MultiTypeCheckerListener struct{ subscribers []TypeCheckerListener }

func NewMultiTypeCheckerListener(subscribers ...TypeCheckerListener) *MultiTypeCheckerListener {
	copy := make([]TypeCheckerListener, 0, len(subscribers))
	for _, subscriber := range subscribers {
		if subscriber != nil {
			copy = append(copy, subscriber)
		}
	}
	return &MultiTypeCheckerListener{subscribers: copy}
}

func (listener *MultiTypeCheckerListener) OnTypeFound(ref ExactRef, typ Type) {
	for _, subscriber := range listener.subscribers {
		subscriber.OnTypeFound(ref, typ)
	}
}
func (listener *MultiTypeCheckerListener) OnTypeError(ref EtcRef, message string) {
	for _, subscriber := range listener.subscribers {
		subscriber.OnTypeError(ref, message)
	}
}
func (listener *MultiTypeCheckerListener) OnTypeWarning(ref EtcRef, message string) {
	for _, subscriber := range listener.subscribers {
		subscriber.OnTypeWarning(ref, message)
	}
}
