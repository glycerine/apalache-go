package apalache

import (
	"sync"
	"testing"
)

func TestTypesPackagePremadeBuilders(t *testing.T) {
	var instruction BuilderT = TLA.Apply(Op("PLUS"), TLA.Int64(2), TLA.Int64(3))
	var expression BuilderUT
	var err error
	expression, err = instruction.Build()
	if err != nil {
		t.Fatal(err)
	}
	typ, err := ExpressionType(expression)
	if err != nil || !EqualTypes(typ, IntT1) {
		t.Fatalf("TLA.plus type = %v, %v", typ, err)
	}

	unsafe, err := TLAUnsafe.Plus(TLAUnsafe.Int64(2), TLAUnsafe.Int64(3))
	if err != nil {
		t.Fatal(err)
	}
	typ, err = ExpressionType(unsafe)
	if err != nil || !EqualTypes(typ, IntT1) {
		t.Fatalf("TLAUnsafe.plus type = %v, %v", typ, err)
	}

	if _, err = TLAUnsafe.Plus(TLAUnsafe.Int64(1), TLAUnsafe.Bool(true)); err == nil {
		t.Fatal("strict premade unsafe builder accepted ill-typed addition")
	}
}

func TestTypesPackageScopedBuilderUsesFreshConcurrentContexts(t *testing.T) {
	const builds = 16
	errors := make(chan error, builds)
	var wait sync.WaitGroup
	for range builds {
		wait.Add(1)
		go func() {
			defer wait.Done()
			instruction := TLA.Eql(TLA.Name("x", IntT1), TLA.Int64(1))
			_, err := instruction.Build()
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent package builder failed: %v", err)
		}
	}
}
