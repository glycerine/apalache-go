package apalache

import "fmt"

// TypeUnifier computes most-general unifiers for Type System 1. Instances are
// intentionally stateful and must not be shared by concurrent callers.
type TypeUnifier struct {
	pool       *TypeVarPool
	varToClass map[int]*EqClass
	solution   map[*EqClass]Type
}

func NewTypeUnifier(pool *TypeVarPool) *TypeUnifier {
	if pool == nil {
		panic("NewTypeUnifier requires a type variable pool")
	}
	return &TypeUnifier{pool: pool}
}

// Unify returns the most-general substitution and unified type. ok is false
// when the types do not unify; err reports malformed input state.
func (unifier *TypeUnifier) Unify(initial *Substitution, left, right Type) (result *Substitution, unified Type, ok bool, err error) {
	if initial == nil {
		initial = EmptySubstitution()
	}
	if left == nil || right == nil {
		return nil, nil, false, fmt.Errorf("cannot unify nil types")
	}
	unifier.varToClass = make(map[int]*EqClass)
	unifier.solution = make(map[*EqClass]Type)
	defer func() {
		unifier.varToClass = nil
		unifier.solution = nil
	}()

	used := make(map[int]struct{})
	addUsedNames(used, left)
	addUsedNames(used, right)
	for _, entry := range initial.entries {
		classCopy := entry.Class.Copy()
		unifier.solution[classCopy] = entry.Type
		for _, variable := range classCopy.Variables() {
			if _, exists := unifier.varToClass[variable]; exists {
				return nil, nil, false, fmt.Errorf("initial substitution has overlapping class for variable %d", variable)
			}
			unifier.varToClass[variable] = classCopy
		}
		addUsedNames(used, entry.Type)
	}
	for variable := range used {
		if _, exists := unifier.varToClass[variable]; !exists {
			class := NewEqClass(variable)
			unifier.varToClass[variable] = class
			typeVariable, _ := NewVarType(variable)
			unifier.solution[class] = typeVariable
		}
	}

	unified, ok = unifier.compute(left, right)
	if !ok {
		return nil, nil, false, nil
	}
	canonicalEntries := make([]SubstitutionEntry, 0, len(unifier.solution))
	for class := range unifier.solution {
		variable, _ := NewVarType(class.Representative())
		canonicalEntries = append(canonicalEntries, SubstitutionEntry{Class: class, Type: variable})
	}
	canonical, err := NewSubstitution(canonicalEntries...)
	if err != nil {
		return nil, nil, false, err
	}
	entries := make([]SubstitutionEntry, 0, len(unifier.solution))
	for class, typ := range unifier.solution {
		canonicalType, _, applyErr := canonical.ApplyOnce(typ)
		if applyErr != nil {
			return nil, nil, false, applyErr
		}
		entries = append(entries, SubstitutionEntry{Class: class, Type: canonicalType})
	}
	result, err = NewSubstitution(entries...)
	if err != nil {
		return nil, nil, false, err
	}
	unified, _, err = result.ApplyOnce(unified)
	if err != nil {
		return nil, nil, false, err
	}
	return result, unified, true, nil
}

func addUsedNames(set map[int]struct{}, typ Type) {
	for _, variable := range typ.UsedNames() {
		set[variable] = struct{}{}
	}
}

func (unifier *TypeUnifier) compute(left, right Type) (Type, bool) {
	if leftPrimitive, ok := left.(primitiveType); ok {
		if rightPrimitive, rightOK := right.(primitiveType); rightOK {
			return left, leftPrimitive == rightPrimitive
		}
	}
	if leftConst, ok := left.(ConstType); ok {
		if rightConst, rightOK := right.(ConstType); rightOK {
			return left, leftConst.name == rightConst.name
		}
	}
	if leftVar, ok := left.(VarType); ok {
		if rightVar, rightOK := right.(VarType); rightOK {
			if leftVar.no == rightVar.no {
				return leftVar, true
			}
			return unifier.mergeClasses(leftVar.no, rightVar.no)
		}
		return unifier.unifyVariableWithTerm(leftVar.no, right)
	}
	if rightVar, ok := right.(VarType); ok {
		return unifier.unifyVariableWithTerm(rightVar.no, left)
	}

	switch leftTyped := left.(type) {
	case FunType:
		rightTyped, ok := right.(FunType)
		if !ok {
			return nil, false
		}
		argument, ok := unifier.compute(leftTyped.arg, rightTyped.arg)
		if !ok {
			return nil, false
		}
		resultType, ok := unifier.compute(leftTyped.result, rightTyped.result)
		if !ok {
			return nil, false
		}
		result, _ := NewFunType(argument, resultType)
		return result, true
	case OperatorType:
		rightTyped, ok := right.(OperatorType)
		if !ok {
			return nil, false
		}
		leftTypes := append([]Type{leftTyped.result}, leftTyped.arguments...)
		rightTypes := append([]Type{rightTyped.result}, rightTyped.arguments...)
		unified, ok := unifier.computeSlices(leftTypes, rightTypes)
		if !ok {
			return nil, false
		}
		result, _ := NewOperatorType(unified[1:], unified[0])
		return result, true
	case SetType:
		rightTyped, ok := right.(SetType)
		if !ok {
			return nil, false
		}
		element, ok := unifier.compute(leftTyped.element, rightTyped.element)
		if !ok {
			return nil, false
		}
		result, _ := NewSetType(element)
		return result, true
	case SeqType:
		rightTyped, ok := right.(SeqType)
		if !ok {
			return nil, false
		}
		element, ok := unifier.compute(leftTyped.element, rightTyped.element)
		if !ok {
			return nil, false
		}
		result, _ := NewSeqType(element)
		return result, true
	case TupleType:
		if rightSparse, ok := right.(SparseTupleType); ok {
			return unifier.computeSparseAndTuple(rightSparse, leftTyped)
		}
		rightTyped, ok := right.(TupleType)
		if !ok {
			return nil, false
		}
		elements, ok := unifier.computeSlices(leftTyped.elements, rightTyped.elements)
		if !ok {
			return nil, false
		}
		result, _ := NewTupleType(elements...)
		return result, true
	case SparseTupleType:
		if rightTuple, ok := right.(TupleType); ok {
			return unifier.computeSparseAndTuple(leftTyped, rightTuple)
		}
		rightTyped, ok := right.(SparseTupleType)
		if !ok {
			return nil, false
		}
		fields, ok := unifier.computeIntFieldUnion(leftTyped.fields, rightTyped.fields)
		if !ok {
			return nil, false
		}
		result, _ := NewSparseTupleType(fields)
		return result, true
	case RecordType:
		if rightRow, ok := right.(RecordRowType); ok {
			return unifier.compute(rightRow, leftTyped)
		}
		rightTyped, ok := right.(RecordType)
		if !ok {
			return nil, false
		}
		fields, ok := unifier.computeStringFieldUnion(leftTyped.fields, rightTyped.fields)
		if !ok {
			return nil, false
		}
		result, _ := NewRecordType(fields)
		return result, true
	case RowType:
		rightTyped, ok := right.(RowType)
		if !ok {
			return nil, false
		}
		row, ok := unifier.computeRows(leftTyped.fields, rightTyped.fields, leftTyped.other, rightTyped.other)
		return row, ok
	case RecordRowType:
		switch rightTyped := right.(type) {
		case RecordRowType:
			row, ok := unifier.computeRows(leftTyped.row.fields, rightTyped.row.fields, leftTyped.row.other, rightTyped.row.other)
			if !ok {
				return nil, false
			}
			return NewRecordRowType(row), true
		case RecordType:
			closedRow, _ := NewRowType(rightTyped.fields, nil)
			return unifier.compute(leftTyped, NewRecordRowType(closedRow))
		default:
			return nil, false
		}
	case VariantType:
		rightTyped, ok := right.(VariantType)
		if !ok {
			return nil, false
		}
		row, ok := unifier.computeRows(leftTyped.row.fields, rightTyped.row.fields, leftTyped.row.other, rightTyped.row.other)
		if !ok {
			return nil, false
		}
		return NewVariantType(row), true
	default:
		return nil, false
	}
}

func (unifier *TypeUnifier) computeSlices(left, right []Type) ([]Type, bool) {
	if len(left) != len(right) {
		return nil, false
	}
	result := make([]Type, len(left))
	for index := range left {
		var ok bool
		result[index], ok = unifier.compute(left[index], right[index])
		if !ok {
			return nil, false
		}
	}
	return result, true
}

func (unifier *TypeUnifier) computeIntFieldUnion(left, right map[int]Type) (map[int]Type, bool) {
	result := make(map[int]Type, len(left)+len(right))
	for key, typ := range left {
		result[key] = typ
	}
	for key, rightType := range right {
		if leftType, exists := result[key]; exists {
			unified, ok := unifier.compute(leftType, rightType)
			if !ok {
				return nil, false
			}
			result[key] = unified
		} else {
			result[key] = rightType
		}
	}
	return result, true
}

func (unifier *TypeUnifier) computeStringFieldUnion(left, right map[string]Type) (map[string]Type, bool) {
	result := make(map[string]Type, len(left)+len(right))
	for key, typ := range left {
		result[key] = typ
	}
	for key, rightType := range right {
		if leftType, exists := result[key]; exists {
			unified, ok := unifier.compute(leftType, rightType)
			if !ok {
				return nil, false
			}
			result[key] = unified
		} else {
			result[key] = rightType
		}
	}
	return result, true
}

func (unifier *TypeUnifier) computeSparseAndTuple(sparse SparseTupleType, tuple TupleType) (Type, bool) {
	for index := range sparse.fields {
		if index < 1 || index > len(tuple.elements) {
			return nil, false
		}
	}
	full := make(map[int]Type, len(tuple.elements))
	for index, typ := range tuple.elements {
		full[index+1] = typ
	}
	fullSparse, _ := NewSparseTupleType(full)
	unified, ok := unifier.compute(sparse, fullSparse)
	if !ok {
		return nil, false
	}
	unifiedSparse, ok := unified.(SparseTupleType)
	if !ok {
		return nil, false
	}
	elements := make([]Type, len(tuple.elements))
	for index := range elements {
		elements[index] = unifiedSparse.fields[index+1]
	}
	result, _ := NewTupleType(elements...)
	return result, true
}

func (unifier *TypeUnifier) unifyVariableWithTerm(variable int, term Type) (Type, bool) {
	class := unifier.varToClass[variable]
	if class == nil || unifier.doesUseClass(term, class) {
		return nil, false
	}
	assigned := unifier.solution[class]
	if _, free := assigned.(VarType); free {
		unifier.solution[class] = term
		return term, true
	}
	unified, ok := unifier.compute(assigned, term)
	if ok {
		unifier.solution[class] = unified
	}
	return unified, ok
}

func (unifier *TypeUnifier) mergeClasses(leftVariable, rightVariable int) (Type, bool) {
	leftClass := unifier.varToClass[leftVariable]
	rightClass := unifier.varToClass[rightVariable]
	if leftClass == nil || rightClass == nil {
		return nil, false
	}
	if leftClass == rightClass {
		variable, _ := NewVarType(leftClass.Representative())
		return variable, true
	}
	leftTerm := unifier.solution[leftClass]
	rightTerm := unifier.solution[rightClass]
	mergedVariables := append(leftClass.Variables(), rightClass.Variables()...)
	leftClass.SetVariables(mergedVariables...)
	for _, variable := range rightClass.Variables() {
		unifier.varToClass[variable] = leftClass
	}
	delete(unifier.solution, rightClass)
	unified, ok := unifier.compute(leftTerm, rightTerm)
	if ok {
		unifier.solution[leftClass] = unified
	}
	return unified, ok
}

func (unifier *TypeUnifier) doesUseClass(typ Type, target *EqClass) bool {
	visited := make(map[*EqClass]struct{})
	toVisit := make([]*EqClass, 0)
	queued := make(map[*EqClass]struct{})
	for _, variable := range typ.UsedNames() {
		class := unifier.varToClass[variable]
		if class != nil {
			if _, exists := queued[class]; !exists {
				toVisit = append(toVisit, class)
				queued[class] = struct{}{}
			}
		}
	}
	for len(toVisit) > 0 {
		class := toVisit[len(toVisit)-1]
		toVisit = toVisit[:len(toVisit)-1]
		if class == target {
			return true
		}
		if _, done := visited[class]; done {
			continue
		}
		visited[class] = struct{}{}
		for _, variable := range unifier.solution[class].UsedNames() {
			next := unifier.varToClass[variable]
			if next != nil {
				if _, done := visited[next]; !done {
					toVisit = append(toVisit, next)
				}
			}
		}
	}
	return false
}

func (unifier *TypeUnifier) freshVariable() VarType {
	variable := unifier.pool.Fresh()
	class := NewEqClass(variable.no)
	unifier.varToClass[variable.no] = class
	unifier.solution[class] = variable
	return variable
}

func (unifier *TypeUnifier) computeRows(leftFields, rightFields map[string]Type, leftTail, rightTail *VarType) (RowType, bool) {
	if len(leftFields) == 0 {
		switch {
		case leftTail == nil && rightTail == nil:
			if len(rightFields) != 0 {
				return RowType{}, false
			}
			result, _ := NewRowType(nil, nil)
			return result, true
		case leftTail != nil && rightTail != nil:
			if len(rightFields) == 0 {
				return unifier.computeAsRow(*leftTail, *rightTail)
			}
			rightRow, _ := NewRowType(rightFields, rightTail)
			return unifier.unifyVariableWithTermAsRow(leftTail.no, rightRow)
		case leftTail != nil:
			rightRow, _ := NewRowType(rightFields, nil)
			return unifier.unifyVariableWithTermAsRow(leftTail.no, rightRow)
		default:
			if len(rightFields) != 0 {
				return RowType{}, false
			}
			empty, _ := NewRowType(nil, nil)
			return unifier.unifyVariableWithTermAsRow(rightTail.no, empty)
		}
	}
	if len(rightFields) == 0 {
		return unifier.computeRows(rightFields, leftFields, rightTail, leftTail)
	}

	shared := make(map[string]struct{})
	for name := range leftFields {
		if _, exists := rightFields[name]; exists {
			shared[name] = struct{}{}
		}
	}
	if len(shared) == 0 {
		tail := unifier.freshVariable()
		leftRemainder, _ := NewRowType(rightFields, &tail)
		rightRemainder, _ := NewRowType(leftFields, &tail)
		var leftBase Type
		if leftTail == nil {
			leftBase, _ = NewRowType(nil, nil)
		} else {
			leftBase = *leftTail
		}
		var rightBase Type
		if rightTail == nil {
			rightBase, _ = NewRowType(nil, nil)
		} else {
			rightBase = *rightTail
		}
		if _, ok := unifier.compute(leftBase, leftRemainder); !ok {
			return RowType{}, false
		}
		if _, ok := unifier.compute(rightBase, rightRemainder); !ok {
			return RowType{}, false
		}
		originalLeft, _ := NewRowType(leftFields, leftTail)
		substitution, err := unifier.currentSubstitution()
		if err != nil {
			return RowType{}, false
		}
		substituted, _, err := substitution.ApplyOnce(originalLeft)
		if err != nil {
			return RowType{}, false
		}
		result, ok := substituted.(RowType)
		return result, ok
	}

	leftUnique := make(map[string]Type)
	for name, typ := range leftFields {
		if _, exists := shared[name]; !exists {
			leftUnique[name] = typ
		}
	}
	rightUnique := make(map[string]Type)
	for name, typ := range rightFields {
		if _, exists := shared[name]; !exists {
			rightUnique[name] = typ
		}
	}
	disjoint, ok := unifier.computeRows(leftUnique, rightUnique, leftTail, rightTail)
	if !ok {
		return RowType{}, false
	}
	fields := disjoint.Fields()
	for name := range shared {
		unified, fieldOK := unifier.compute(leftFields[name], rightFields[name])
		if !fieldOK {
			return RowType{}, false
		}
		fields[name] = unified
	}
	tail, hasTail := disjoint.Other()
	if hasTail {
		result, _ := NewRowType(fields, &tail)
		return result, true
	}
	result, _ := NewRowType(fields, nil)
	return result, true
}

func (unifier *TypeUnifier) computeAsRow(left, right Type) (RowType, bool) {
	result, ok := unifier.compute(left, right)
	if !ok {
		return RowType{}, false
	}
	return asRow(result)
}

func (unifier *TypeUnifier) unifyVariableWithTermAsRow(variable int, term Type) (RowType, bool) {
	result, ok := unifier.unifyVariableWithTerm(variable, term)
	if !ok {
		return RowType{}, false
	}
	return asRow(result)
}

func asRow(typ Type) (RowType, bool) {
	switch typed := typ.(type) {
	case RowType:
		return typed, true
	case VarType:
		row, _ := NewRowType(nil, &typed)
		return row, true
	default:
		return RowType{}, false
	}
}

func (unifier *TypeUnifier) currentSubstitution() (*Substitution, error) {
	entries := make([]SubstitutionEntry, 0, len(unifier.solution))
	for class, typ := range unifier.solution {
		entries = append(entries, SubstitutionEntry{Class: class, Type: typ})
	}
	return NewSubstitution(entries...)
}
