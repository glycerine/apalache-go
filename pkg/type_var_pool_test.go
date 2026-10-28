package apalache

import (
	"reflect"
	"testing"
)

func TestTypeVarPoolFreshScalarBatchAndZero(t *testing.T) {
	pool := NewTypeVarPool(3)
	if pool.Size() != 3 || pool.Fresh().Number() != 3 || pool.Size() != 4 {
		t.Fatalf("first allocation size = %d", pool.Size())
	}
	batch := pool.FreshN(4)
	numbers := make([]int, len(batch))
	for index, variable := range batch {
		numbers[index] = variable.Number()
	}
	if !reflect.DeepEqual(numbers, []int{4, 5, 6, 7}) || pool.Size() != 8 {
		t.Fatalf("batch = %v, size = %d", numbers, pool.Size())
	}
	if zero := pool.FreshN(0); zero == nil || len(zero) != 0 || pool.Size() != 8 {
		t.Fatalf("zero batch = %#v, size = %d", zero, pool.Size())
	}
}

func TestTypeVarPoolMatchesScalaNegativeBatchSemantics(t *testing.T) {
	pool := NewTypeVarPool(10)
	if batch := pool.FreshN(-3); batch == nil || len(batch) != 0 || pool.Size() != 7 {
		t.Fatalf("negative batch = %#v, size = %d", batch, pool.Size())
	}
	if variable := pool.Fresh(); variable.Number() != 7 || pool.Size() != 8 {
		t.Fatalf("fresh after negative batch = %v, size = %d", variable, pool.Size())
	}
}

func TestTypeVarPoolInvalidVariableFailureIsAtomic(t *testing.T) {
	assertPanic := func(body func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatal("expected invalid VarT1 construction to panic")
			}
		}()
		body()
	}
	pool := NewTypeVarPool(-1)
	if pool.Size() != -1 {
		t.Fatalf("negative start size = %d", pool.Size())
	}
	assertPanic(func() { pool.Fresh() })
	if pool.Size() != -1 {
		t.Fatalf("failed scalar allocation changed size to %d", pool.Size())
	}
	assertPanic(func() { pool.FreshN(2) })
	if pool.Size() != -1 {
		t.Fatalf("failed batch allocation changed size to %d", pool.Size())
	}
}
