package rel

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// CompileGuard translates the retained UDoc guard language into a DOC union.
func (r *UDocRelation) CompileGuard(manager *ast.ASTManager, expression ast.ASTHandle) (*UDocRelation, error) {
	return r.compileGuard(manager, expression, make([]bool, r.NumBits()))
}

func (r *UDocRelation) compileGuard(manager *ast.ASTManager, expression ast.ASTHandle, discarded []bool) (*UDocRelation, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if manager == nil || expression == (ast.ASTHandle{}) || len(discarded) != int(r.NumBits()) {
		return nil, ErrDOC
	}
	compiled, err := NewUDocRelation(r.manager, r.widths)
	if err != nil {
		return nil, err
	}
	initial, err := r.manager.AllocateX()
	if err == nil {
		err = compiled.elems.PushOwned(initial)
	}
	classes := backtrack.NewDefaultUnionFind()
	for i := uint32(0); i < r.NumBits(); i++ {
		classes.MkVar()
	}
	if err == nil {
		err = compiled.applyGuard(manager, expression, classes, discarded)
	}
	if err != nil {
		_ = compiled.Close()
		return nil, err
	}
	return compiled, nil
}

// FilterGuardAndProject applies the DOC guard before projecting the requested
// columns, preserving the source's discarded-column equality merge behavior.
func (r *UDocRelation) FilterGuardAndProject(manager *ast.ASTManager, expression ast.ASTHandle, removed []uint32, destination *DocManager) (result *UDocRelation, resultErr error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if destination == nil || destination.closed {
		return nil, ErrDOC
	}
	discarded := make([]bool, r.NumBits())
	seen := make([]bool, len(r.widths))
	for _, column := range removed {
		if column >= uint32(len(r.widths)) || seen[column] {
			return nil, ErrDOC
		}
		seen[column] = true
		for bit := r.offsets[column]; bit < r.offsets[column+1]; bit++ {
			discarded[bit] = true
		}
	}
	remainder, equalities, err := r.extractRangeEqualities(manager, expression)
	if err != nil {
		return nil, err
	}
	guard, reduced, err := r.ExtractGuard(manager, remainder)
	if err != nil {
		return nil, err
	}
	if !manager.IsTrue(reduced) {
		compilable, e := udocGuardCompilable(manager, reduced)
		if e != nil {
			return nil, e
		}
		if !compilable {
			return nil, ErrDOC
		}
		guard, err = udocMakeAnd(manager, []ast.ASTHandle{guard, reduced})
		if err != nil {
			return nil, err
		}
	}
	compiled, err := r.compileGuard(manager, guard, discarded)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, compiled.Close()) }()
	filtered, err := r.Clone()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, filtered.Close()) }()
	if err = filtered.elems.Intersect(compiled.elems); err != nil {
		return nil, err
	}
	for i := 0; i < len(filtered.elems.values); {
		valid := true
		for _, root := range equalities.roots {
			var e error
			valid, e = r.manager.MergeRange(filtered.elems.values[i], root, 1, equalities.classes, discarded)
			if e != nil {
				return nil, e
			}
			if !valid {
				break
			}
		}
		if !valid {
			if err = filtered.elems.erase(i); err != nil {
				return nil, err
			}
			continue
		}
		i++
	}
	return filtered.Project(removed, destination)
}

func udocGuardCompilable(m *ast.ASTManager, expression ast.ASTHandle) (bool, error) {
	if recognized, err := udocIsGuard(m, expression); err != nil || recognized {
		return recognized, err
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := m.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	declKind, err := m.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	if family != m.BasicFamilyID() {
		return false, nil
	}
	if declKind == ast.ASTOpAnd || declKind == ast.ASTOpOr || declKind == ast.ASTOpNot {
		n, e := m.GetNumArgs(expression)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			arg, e := m.GetArg(expression, i)
			if e != nil {
				return false, e
			}
			yes, e := udocGuardCompilable(m, arg)
			if e != nil || !yes {
				return yes, e
			}
		}
		return true, nil
	}
	if declKind == ast.ASTOpIff {
		return true, nil
	}
	if declKind != ast.ASTOpEq {
		return false, nil
	}
	left, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := m.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	leftSort, err := m.GetSort(left)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	if leftSort == boolean {
		return true, nil
	}
	_, _, _, leftRange, err := udocGuardRange(m, left)
	if err != nil {
		return false, err
	}
	_, _, _, rightRange, err := udocGuardRange(m, right)
	if err != nil {
		return false, err
	}
	if leftRange && rightRange {
		return true, nil
	}
	if leftRange {
		_, _, matched, e := m.BitVectorNumeralBigInt(right)
		return matched, e
	}
	if rightRange {
		_, _, matched, e := m.BitVectorNumeralBigInt(left)
		return matched, e
	}
	_, leftConcat, err := m.BitVectorConcatArguments(left)
	if err != nil {
		return false, err
	}
	_, rightConcat, err := m.BitVectorConcatArguments(right)
	return leftConcat || rightConcat, err
}

type udocRangeEqualities struct {
	classes *backtrack.UnionFind
	roots   []uint32
}

func (r *UDocRelation) extractRangeEqualities(m *ast.ASTManager, condition ast.ASTHandle) (ast.ASTHandle, udocRangeEqualities, error) {
	classes := backtrack.NewDefaultUnionFind()
	for i := uint32(0); i < r.NumBits(); i++ {
		classes.MkVar()
	}
	equalities := udocRangeEqualities{classes: classes}
	var terms []ast.ASTHandle
	if err := udocFlattenAnd(m, condition, &terms); err != nil {
		return ast.ASTHandle{}, equalities, err
	}
	remaining := make([]ast.ASTHandle, 0, len(terms))
	for _, term := range terms {
		kind, err := m.GetKind(term)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		if kind != ast.ASTApp {
			remaining = append(remaining, term)
			continue
		}
		family, err := m.GetFamilyID(term)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		declKind, err := m.GetDeclKind(term)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		if family != m.BasicFamilyID() || declKind != ast.ASTOpEq {
			remaining = append(remaining, term)
			continue
		}
		left, err := m.GetArg(term, 0)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		right, err := m.GetArg(term, 1)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		leftVar, leftLow, leftHigh, leftOK, err := udocProjectEqualityRange(m, left)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		rightVar, rightLow, rightHigh, rightOK, err := udocProjectEqualityRange(m, right)
		if err != nil {
			return ast.ASTHandle{}, equalities, err
		}
		if !leftOK || !rightOK {
			handled, pieces, e := r.extractConcatEquality(m, left, right, &equalities)
			if e != nil {
				return ast.ASTHandle{}, equalities, e
			}
			if handled {
				remaining = append(remaining, pieces...)
				continue
			}
			remaining = append(remaining, term)
			continue
		}
		if leftHigh < leftLow || rightHigh < rightLow || leftHigh-leftLow != rightHigh-rightLow || leftVar >= uint32(len(r.widths)) || rightVar >= uint32(len(r.widths)) {
			return ast.ASTHandle{}, equalities, ErrDOC
		}
		leftIndex, rightIndex := r.offsets[leftVar]+leftLow, r.offsets[rightVar]+rightLow
		length := leftHigh - leftLow + 1
		if leftIndex+length > r.NumBits() || rightIndex+length > r.NumBits() {
			return ast.ASTHandle{}, equalities, ErrDOC
		}
		for bit := uint32(0); bit < length; bit++ {
			classes.Merge(leftIndex+bit, rightIndex+bit)
			equalities.roots = append(equalities.roots, leftIndex+bit)
		}
	}
	rest, err := udocMakeAnd(m, remaining)
	return rest, equalities, err
}

func (r *UDocRelation) extractConcatEquality(m *ast.ASTManager, left, right ast.ASTHandle, equalities *udocRangeEqualities) (handled bool, residual []ast.ASTHandle, err error) {
	_, leftConcat, err := m.BitVectorConcatArguments(left)
	if err != nil {
		return false, nil, err
	}
	_, rightConcat, err := m.BitVectorConcatArguments(right)
	if err != nil {
		return false, nil, err
	}
	if !leftConcat && !rightConcat {
		return false, nil, nil
	}
	leftParts, _, err := udocFlattenConcat(m, left)
	if err != nil {
		return false, nil, err
	}
	rightParts, _, err := udocFlattenConcat(m, right)
	if err != nil {
		return false, nil, err
	}
	leftWidths, leftTotal, err := udocPartWidths(m, leftParts)
	if err != nil {
		return false, nil, err
	}
	rightWidths, rightTotal, err := udocPartWidths(m, rightParts)
	if err != nil {
		return false, nil, err
	}
	if leftTotal != rightTotal {
		return false, nil, ErrDOC
	}
	i, j := 0, 0
	leftRemain, rightRemain := leftWidths[0], rightWidths[0]
	for i < len(leftParts) && j < len(rightParts) {
		length := leftRemain
		if rightRemain < length {
			length = rightRemain
		}
		leftSlice, e := udocSliceBitVector(m, leftParts[i], leftRemain-1, leftRemain-length)
		if e != nil {
			return false, nil, e
		}
		rightSlice, e := udocSliceBitVector(m, rightParts[j], rightRemain-1, rightRemain-length)
		if e != nil {
			return false, nil, e
		}
		leftVar, leftLow, leftHigh, leftRange, e := udocGuardRange(m, leftSlice)
		if e != nil {
			return false, nil, e
		}
		rightVar, rightLow, rightHigh, rightRange, e := udocGuardRange(m, rightSlice)
		if e != nil {
			return false, nil, e
		}
		if leftRange && rightRange {
			if leftVar >= uint32(len(r.widths)) || rightVar >= uint32(len(r.widths)) || leftHigh-leftLow != rightHigh-rightLow {
				return false, nil, ErrDOC
			}
			leftIndex, rightIndex := r.offsets[leftVar]+leftLow, r.offsets[rightVar]+rightLow
			for bit := uint32(0); bit < length; bit++ {
				equalities.classes.Merge(leftIndex+bit, rightIndex+bit)
				equalities.roots = append(equalities.roots, leftIndex+bit)
			}
		} else {
			equality, e := m.MkEq(leftSlice, rightSlice)
			if e != nil {
				return false, nil, e
			}
			residual = append(residual, equality)
		}
		leftRemain -= length
		rightRemain -= length
		if leftRemain == 0 {
			i++
			if i < len(leftParts) {
				leftRemain = leftWidths[i]
			}
		}
		if rightRemain == 0 {
			j++
			if j < len(rightParts) {
				rightRemain = rightWidths[j]
			}
		}
	}
	return true, residual, nil
}

func udocPartWidths(m *ast.ASTManager, parts []ast.ASTHandle) ([]uint32, uint32, error) {
	widths := make([]uint32, len(parts))
	var total uint32
	for i, part := range parts {
		sort, err := m.GetSort(part)
		if err != nil {
			return nil, 0, err
		}
		widths[i], err = m.BitVectorSize(sort)
		if err != nil || widths[i] == 0 || widths[i] > ^uint32(0)-total {
			return nil, 0, ErrDOC
		}
		total += widths[i]
	}
	if len(widths) == 0 {
		return nil, 0, ErrDOC
	}
	return widths, total, nil
}

// FilterGuard intersects the relation with the DOC union compiled from a guard.
func (r *UDocRelation) FilterGuard(manager *ast.ASTManager, expression ast.ASTHandle) (resultErr error) {
	if err := r.check(); err != nil {
		return err
	}
	compiled, err := r.CompileGuard(manager, expression)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, compiled.Close()) }()
	if err = r.elems.Intersect(compiled.elems); err != nil {
		return err
	}
	return r.elems.Simplify()
}

func (r *UDocRelation) applyGuard(m *ast.ASTManager, expression ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
	if r.elems.Empty() {
		return nil
	}
	if m.IsTrue(expression) {
		return nil
	}
	if m.IsFalse(expression) {
		return r.elems.Reset()
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, e := m.GetVarIndex(expression)
		if e != nil {
			return e
		}
		if index >= uint32(len(r.widths)) || r.widths[index] != 1 {
			return ErrDOC
		}
		sort, e := m.GetSort(expression)
		if e != nil {
			return e
		}
		boolean, e := m.MkBoolSort()
		if e != nil {
			return e
		}
		if sort != boolean {
			return ErrDOC
		}
		cube, e := r.manager.AllocateX()
		if e != nil {
			return e
		}
		if e = r.manager.Set(cube, r.offsets[index], TBit1); e != nil {
			return errors.Join(e, r.manager.Deallocate(cube))
		}
		e = r.elems.IntersectDoc(cube)
		return errors.Join(e, r.manager.Deallocate(cube))
	}
	if kind != ast.ASTApp {
		return ErrDOC
	}
	family, err := m.GetFamilyID(expression)
	if err != nil {
		return err
	}
	declKind, err := m.GetDeclKind(expression)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() {
		return ErrDOC
	}
	switch declKind {
	case ast.ASTOpAnd:
		n, e := m.GetNumArgs(expression)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n && !r.elems.Empty(); i++ {
			arg, e := m.GetArg(expression, i)
			if e != nil {
				return e
			}
			if e = r.applyGuard(m, arg, classes, discarded); e != nil {
				return e
			}
		}
		return nil
	case ast.ASTOpNot:
		arg, e := m.GetArg(expression, 0)
		if e != nil {
			return e
		}
		temporary, e := r.guardUniverse()
		if e != nil {
			return e
		}
		if e = temporary.applyGuard(m, arg, classes, make([]bool, len(discarded))); e == nil {
			e = r.elems.Subtract(temporary.elems)
		}
		return errors.Join(e, temporary.Close())
	case ast.ASTOpOr:
		temporary, e := r.guardUniverse()
		if e != nil {
			return e
		}
		n, e := m.GetNumArgs(expression)
		for i := uint32(0); e == nil && i < n && !temporary.elems.Empty(); i++ {
			arg, readErr := m.GetArg(expression, i)
			if readErr != nil {
				e = readErr
				break
			}
			negated, makeErr := m.MkNot(arg)
			if makeErr != nil {
				e = makeErr
				break
			}
			e = temporary.applyGuard(m, negated, classes, discarded)
		}
		if e == nil {
			e = r.elems.Subtract(temporary.elems)
		}
		return errors.Join(e, temporary.Close())
	case ast.ASTOpEq:
		left, e := m.GetArg(expression, 0)
		if e != nil {
			return e
		}
		right, e := m.GetArg(expression, 1)
		if e != nil {
			return e
		}
		leftSort, e := m.GetSort(left)
		if e != nil {
			return e
		}
		boolean, e := m.MkBoolSort()
		if e != nil {
			return e
		}
		if leftSort == boolean {
			return r.applyBooleanIff(m, left, right, classes, discarded)
		}
		return r.applyBitVectorEquality(m, left, right, classes, discarded)
	case ast.ASTOpIff:
		left, e := m.GetArg(expression, 0)
		if e != nil {
			return e
		}
		right, e := m.GetArg(expression, 1)
		if e != nil {
			return e
		}
		return r.applyBooleanIff(m, left, right, classes, discarded)
	default:
		return ErrDOC
	}
}

func (r *UDocRelation) applyBooleanIff(m *ast.ASTManager, left, right ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
	leftSort, err := m.GetSort(left)
	if err != nil {
		return err
	}
	rightSort, err := m.GetSort(right)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if leftSort != boolean || rightSort != boolean {
		return ErrDOC
	}
	leftFalse, err := m.MkNot(left)
	if err != nil {
		return err
	}
	rightFalse, err := m.MkNot(right)
	if err != nil {
		return err
	}
	for _, mismatch := range [][2]ast.ASTHandle{{left, rightFalse}, {leftFalse, right}} {
		difference, e := r.guardUniverse()
		if e != nil {
			return e
		}
		if e = difference.applyGuard(m, mismatch[0], classes, discarded); e == nil {
			e = difference.applyGuard(m, mismatch[1], classes, discarded)
		}
		if e == nil {
			e = r.elems.Subtract(difference.elems)
		}
		if e = errors.Join(e, difference.Close()); e != nil {
			return e
		}
	}
	return nil
}

func (r *UDocRelation) guardUniverse() (*UDocRelation, error) {
	temporary, err := NewUDocRelation(r.manager, r.widths)
	if err != nil {
		return nil, err
	}
	document, err := r.manager.AllocateX()
	if err == nil {
		err = temporary.elems.PushOwned(document)
	}
	if err != nil {
		_ = temporary.Close()
		return nil, err
	}
	return temporary, nil
}

func (r *UDocRelation) applyBitVectorEquality(m *ast.ASTManager, left, right ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
	if parts, found, err := m.BitVectorConcatArguments(left); err != nil {
		return err
	} else if found {
		return r.applyConcatEquality(m, parts, right, classes, discarded)
	}
	if parts, found, err := m.BitVectorConcatArguments(right); err != nil {
		return err
	} else if found {
		return r.applyConcatEquality(m, parts, left, classes, discarded)
	}
	leftVar, leftLow, leftHigh, leftOK, err := udocGuardRange(m, left)
	if err != nil {
		return err
	}
	rightVar, rightLow, rightHigh, rightOK, err := udocGuardRange(m, right)
	if err != nil {
		return err
	}
	if leftOK && rightOK {
		length := leftHigh - leftLow + 1
		if rightHigh < rightLow || rightHigh-rightLow+1 != length || leftVar >= uint32(len(r.widths)) || rightVar >= uint32(len(r.widths)) {
			return ErrDOC
		}
		leftIndex := r.offsets[leftVar] + leftLow
		rightIndex := r.offsets[rightVar] + rightLow
		if leftIndex+length > r.NumBits() || rightIndex+length > r.NumBits() {
			return ErrDOC
		}
		for i := uint32(0); i < length; i++ {
			classes.Merge(leftIndex+i, rightIndex+i)
		}
		for i := 0; i < len(r.elems.values); {
			valid, e := r.manager.MergeRange(r.elems.values[i], leftIndex, length, classes, discarded)
			if e != nil {
				return e
			}
			if !valid {
				if e = r.elems.erase(i); e != nil {
					return e
				}
				continue
			}
			i++
		}
		return nil
	}
	ground := right
	if !leftOK {
		if rightOK {
			leftVar, leftLow, leftHigh, leftOK = rightVar, rightLow, rightHigh, true
			ground = left
		}
	}
	if !leftOK {
		return ErrDOC
	}
	value, width, matched, err := m.BitVectorNumeralBigInt(ground)
	if err != nil {
		return err
	}
	if !matched {
		return ErrDOC
	}
	if leftVar >= uint32(len(r.widths)) || leftHigh < leftLow || leftHigh >= r.widths[leftVar] || width != leftHigh-leftLow+1 || value.Sign() < 0 || value.BitLen() > int(width) {
		return ErrDOC
	}
	cube, err := r.manager.AllocateX()
	if err != nil {
		return err
	}
	if err = r.manager.vectors.SetBigInt(cube.positive, new(big.Int).Set(value), r.offsets[leftVar]+leftHigh, r.offsets[leftVar]+leftLow); err != nil {
		return errors.Join(err, r.manager.Deallocate(cube))
	}
	err = r.elems.IntersectDoc(cube)
	return errors.Join(err, r.manager.Deallocate(cube))
}

func (r *UDocRelation) applyConcatEquality(m *ast.ASTManager, parts []ast.ASTHandle, other ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
	otherParts, otherIsConcat, err := udocFlattenConcat(m, other)
	if err != nil {
		return err
	}
	if otherIsConcat {
		return r.applyConcatPartsEquality(m, parts, otherParts, classes, discarded)
	}
	otherValue, otherWidth, isNumeral, err := m.BitVectorNumeralBigInt(other)
	if err != nil {
		return err
	}
	if !isNumeral {
		sort, e := m.GetSort(other)
		if e != nil {
			return e
		}
		otherWidth, e = m.BitVectorSize(sort)
		if e != nil {
			return e
		}
	}
	var total uint32
	widths := make([]uint32, len(parts))
	for i, part := range parts {
		sort, e := m.GetSort(part)
		if e != nil {
			return e
		}
		widths[i], e = m.BitVectorSize(sort)
		if e != nil || widths[i] > ^uint32(0)-total {
			return ErrDOC
		}
		total += widths[i]
	}
	if total != otherWidth {
		return ErrDOC
	}
	remaining := total
	for i, part := range parts {
		width := widths[i]
		remaining -= width
		var slice ast.ASTHandle
		if isNumeral {
			sliceValue := new(big.Int).Rsh(new(big.Int).Set(otherValue), uint(remaining))
			mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))
			sliceValue.And(sliceValue, mask)
			slice, err = m.BitVectorNumeralRat(new(big.Rat).SetInt(sliceValue), width)
		} else {
			slice, err = m.MkBitVectorExtract(remaining+width-1, remaining, other)
		}
		if err != nil {
			return err
		}
		if err = r.applyBitVectorEquality(m, part, slice, classes, discarded); err != nil {
			return err
		}
	}
	return nil
}

func udocFlattenConcat(m *ast.ASTManager, expression ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	parts, matched, err := m.BitVectorConcatArguments(expression)
	if err != nil || !matched {
		return []ast.ASTHandle{expression}, false, err
	}
	result := make([]ast.ASTHandle, 0, len(parts))
	for _, part := range parts {
		children, nested, e := udocFlattenConcat(m, part)
		if e != nil {
			return nil, true, e
		}
		if nested {
			result = append(result, children...)
		} else {
			result = append(result, part)
		}
	}
	return result, true, nil
}

func (r *UDocRelation) applyConcatPartsEquality(m *ast.ASTManager, left, right []ast.ASTHandle, classes *backtrack.UnionFind, discarded []bool) error {
	leftWidths := make([]uint32, len(left))
	rightWidths := make([]uint32, len(right))
	var leftTotal, rightTotal uint32
	for i, part := range left {
		sort, err := m.GetSort(part)
		if err != nil {
			return err
		}
		leftWidths[i], err = m.BitVectorSize(sort)
		if err != nil || leftWidths[i] > ^uint32(0)-leftTotal {
			return ErrDOC
		}
		leftTotal += leftWidths[i]
	}
	for i, part := range right {
		sort, err := m.GetSort(part)
		if err != nil {
			return err
		}
		rightWidths[i], err = m.BitVectorSize(sort)
		if err != nil || rightWidths[i] > ^uint32(0)-rightTotal {
			return ErrDOC
		}
		rightTotal += rightWidths[i]
	}
	if leftTotal != rightTotal {
		return ErrDOC
	}
	i, j := 0, 0
	leftRemain, rightRemain := leftWidths[0], rightWidths[0]
	for i < len(left) && j < len(right) {
		length := leftRemain
		if rightRemain < length {
			length = rightRemain
		}
		leftSlice, err := udocSliceBitVector(m, left[i], leftRemain-1, leftRemain-length)
		if err != nil {
			return err
		}
		rightSlice, err := udocSliceBitVector(m, right[j], rightRemain-1, rightRemain-length)
		if err != nil {
			return err
		}
		if err = r.applyBitVectorEquality(m, leftSlice, rightSlice, classes, discarded); err != nil {
			return err
		}
		leftRemain -= length
		rightRemain -= length
		if leftRemain == 0 {
			i++
			if i < len(left) {
				leftRemain = leftWidths[i]
			}
		}
		if rightRemain == 0 {
			j++
			if j < len(right) {
				rightRemain = rightWidths[j]
			}
		}
	}
	return nil
}

func udocSliceBitVector(m *ast.ASTManager, expression ast.ASTHandle, high, low uint32) (ast.ASTHandle, error) {
	sort, err := m.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	width, err := m.BitVectorSize(sort)
	if err != nil || low > high || high >= width {
		return ast.ASTHandle{}, ErrDOC
	}
	if low == 0 && high+1 == width {
		return expression, nil
	}
	if value, numeralWidth, matched, e := m.BitVectorNumeralBigInt(expression); e != nil {
		return ast.ASTHandle{}, e
	} else if matched {
		if numeralWidth != width {
			return ast.ASTHandle{}, ErrDOC
		}
		value.Rsh(value, uint(low))
		mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(high-low+1)), big.NewInt(1))
		value.And(value, mask)
		return m.BitVectorNumeralRat(new(big.Rat).SetInt(value), high-low+1)
	}
	var extractLow, extractHigh uint32
	var base ast.ASTHandle
	if found, e := m.BitVectorExtractValue(expression, &extractLow, &extractHigh, &base); e != nil {
		return ast.ASTHandle{}, e
	} else if found {
		low += extractLow
		high += extractLow
		expression = base
	}
	return m.MkBitVectorExtract(high, low, expression)
}

func udocGuardRange(m *ast.ASTManager, expression ast.ASTHandle) (variable, low, high uint32, matched bool, err error) {
	kind, err := m.GetKind(expression)
	if err != nil {
		return
	}
	if kind == ast.ASTVar {
		variable, err = m.GetVarIndex(expression)
		if err != nil {
			return
		}
		sort, e := m.GetSort(expression)
		if e != nil {
			err = e
			return
		}
		width, e := m.BitVectorSize(sort)
		if e != nil {
			return 0, 0, 0, false, nil
		}
		return variable, 0, width - 1, true, nil
	}
	var base ast.ASTHandle
	var found bool
	found, err = m.BitVectorExtractValue(expression, &low, &high, &base)
	if err != nil || !found {
		return
	}
	kind, err = m.GetKind(base)
	if err != nil || kind != ast.ASTVar {
		return 0, 0, 0, false, err
	}
	variable, err = m.GetVarIndex(base)
	if err != nil {
		return
	}
	sort, err := m.GetSort(base)
	if err != nil {
		return
	}
	width, e := m.BitVectorSize(sort)
	if e != nil {
		return 0, 0, 0, false, nil
	}
	matched = low <= high && high < width
	return
}
