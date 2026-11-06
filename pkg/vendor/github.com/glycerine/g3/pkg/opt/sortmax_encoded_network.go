package opt

// Source: src/util/sorting_network.h psort_nw::sorting/merge/dsorting, used
// by src/opt/sortmax.cpp. This is a concrete Boolean-wire implementation.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) network(inputs []ast.ASTHandle) ([]ast.ASTHandle, error) {
	switch len(inputs) {
	case 0:
		return nil, nil
	case 1:
		return append([]ast.ASTHandle(nil), inputs...), nil
	case 2:
		return e.merge(inputs[:1], inputs[1:])
	}
	if e.useDirectSort(len(inputs)) {
		return e.directSort(inputs)
	}
	middle := len(inputs) / 2
	left, err := e.network(inputs[:middle])
	if err != nil {
		return nil, err
	}
	right, err := e.network(inputs[middle:])
	if err != nil {
		return nil, err
	}
	return e.merge(left, right)
}

// directSort emits the source's LE-direction subset clauses. In the 4.7.1
// sortmax caller, psort_nw's comparator mode has the zero-valued LE tag.
func (e *sortMaxEncoder) directSort(inputs []ast.ASTHandle) ([]ast.ASTHandle, error) {
	outputs := make([]ast.ASTHandle, len(inputs))
	for i := range outputs {
		wire, err := e.fresh()
		if err != nil {
			return nil, err
		}
		outputs[i] = wire
	}
	for k, output := range outputs {
		if err := e.addSubsetImplications(output, inputs, k+1); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

// addSubsetImplications emits every clause requiring k true inputs to set the
// sorted output true. The source LE mode does not emit the reverse direction.
func (e *sortMaxEncoder) addSubsetImplications(output ast.ASTHandle, inputs []ast.ASTHandle, k int) error {
	prefix := []ast.ASTHandle{output}
	var visit func(start, left int) error
	visit = func(start, left int) error {
		if left == 0 {
			return e.clause(prefix...)
		}
		last := len(inputs) - left
		for i := start; i <= last; i++ {
			literal := inputs[i]
			var err error
			literal, err = e.negate(literal)
			if err != nil {
				return err
			}
			prefix = append(prefix, literal)
			if err := visit(i+1, left-1); err != nil {
				return err
			}
			prefix = prefix[:len(prefix)-1]
		}
		return nil
	}
	return visit(0, k)
}

func (e *sortMaxEncoder) merge(left, right []ast.ASTHandle) ([]ast.ASTHandle, error) {
	a, b := len(left), len(right)
	if a == 0 {
		return append([]ast.ASTHandle(nil), right...), nil
	}
	if b == 0 {
		return append([]ast.ASTHandle(nil), left...), nil
	}
	if a == 1 && b == 1 {
		high, low, err := e.comparator(left[0], right[0])
		if err != nil {
			return nil, err
		}
		return []ast.ASTHandle{high, low}, nil
	}
	if e.useDirectMerge(a, b, a+b) {
		return e.directMerge(left, right)
	}
	if a%2 == 0 && b%2 == 1 {
		return e.merge(right, left)
	}
	evenLeft, oddLeft := splitSortMax(left)
	evenRight, oddRight := splitSortMax(right)
	first, err := e.merge(evenLeft, evenRight)
	if err != nil {
		return nil, err
	}
	second, err := e.merge(oddLeft, oddRight)
	if err != nil {
		return nil, err
	}
	return e.interleave(first, second)
}

func (e *sortMaxEncoder) directMerge(left, right []ast.ASTHandle) ([]ast.ASTHandle, error) {
	a, b := len(left), len(right)
	outputs := make([]ast.ASTHandle, a+b)
	for i := range outputs {
		wire, err := e.fresh()
		if err != nil {
			return nil, err
		}
		outputs[i] = wire
	}
	for i, input := range left {
		notInput, err := e.negate(input)
		if err != nil {
			return nil, err
		}
		if err = e.clause(notInput, outputs[i]); err != nil {
			return nil, err
		}
	}
	for i, input := range right {
		notInput, err := e.negate(input)
		if err != nil {
			return nil, err
		}
		if err = e.clause(notInput, outputs[i]); err != nil {
			return nil, err
		}
	}
	for i := 1; i <= a; i++ {
		for j := 1; j <= b && i+j <= a+b; j++ {
			notLeft, err := e.negate(left[i-1])
			if err != nil {
				return nil, err
			}
			notRight, err := e.negate(right[j-1])
			if err != nil {
				return nil, err
			}
			if err = e.clause(notLeft, notRight, outputs[i+j-1]); err != nil {
				return nil, err
			}
		}
	}
	return outputs, nil
}

func splitSortMax(values []ast.ASTHandle) (even, odd []ast.ASTHandle) {
	even = make([]ast.ASTHandle, 0, (len(values)+1)/2)
	odd = make([]ast.ASTHandle, 0, len(values)/2)
	for i, value := range values {
		if i%2 == 0 {
			even = append(even, value)
		} else {
			odd = append(odd, value)
		}
	}
	return even, odd
}

func (e *sortMaxEncoder) interleave(even, odd []ast.ASTHandle) ([]ast.ASTHandle, error) {
	outputs := make([]ast.ASTHandle, 0, len(even)+len(odd))
	outputs = append(outputs, even[0])
	count := len(even) - 1
	if len(odd) < count {
		count = len(odd)
	}
	for i := 0; i < count; i++ {
		high, low, err := e.comparator(even[i+1], odd[i])
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, high, low)
	}
	if len(even) == len(odd) {
		outputs = append(outputs, odd[count])
	} else if len(even) == len(odd)+2 {
		outputs = append(outputs, even[count+1])
	}
	return outputs, nil
}
