package rel

// Source: the selected bound_relation, interval_relation, and karr_relation
// concrete abstract relations used as the ordinary side of a product.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

type relationExplanationNumeric struct {
	manager  *RelationManager
	sorts    []ast.ASTHandle
	kind     string
	bound    *BoundRelation
	interval *IntervalRelation
	karr     *KarrRelation
	checker  *smtcore.SMTSolverConfig
}

// RelationExplanationNumericSeed carries one concrete typed fixedpoint seed
// into the selected relation-level explanation backing.
type RelationExplanationNumericSeed struct {
	Kind     string
	Bound    *BoundRelation
	Interval *IntervalRelation
	Karr     *KarrRelation
}

func relationExplanationNumericCanHandle(manager *RelationManager, kind string, sorts []ast.ASTHandle) (bool, error) {
	if manager == nil || manager.open() != nil {
		return false, ErrTableRelation
	}
	switch kind {
	case "bound_relation":
		plugin, err := NewBoundRelationPlugin(manager.manager)
		if err != nil {
			return false, err
		}
		return plugin.CanHandleSignature(sorts)
	case "interval_relation":
		for _, sort := range sorts {
			kind, err := manager.manager.GetKind(sort)
			if err != nil || kind != ast.ASTSort {
				return false, errors.Join(err, ast.ErrASTDomain)
			}
		}
		return CanHandleIntervalSorts(manager.manager, sorts), nil
	case "karr_relation":
		family, err := manager.manager.RegisterArithmeticPlugin()
		if err != nil {
			return false, err
		}
		integer, err := manager.manager.MkFamilySort(family, 1, nil)
		if err != nil {
			return false, err
		}
		for _, sort := range sorts {
			if sort != integer {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, ErrTableRelation
	}
}

func RelationExplanationNumericCanHandle(manager *RelationManager, kind string, sorts []ast.ASTHandle) (bool, error) {
	return relationExplanationNumericCanHandle(manager, kind, sorts)
}

func newRelationExplanationNumeric(manager *RelationManager, sorts []ast.ASTHandle, kind string, checker *smtcore.SMTSolverConfig) (*relationExplanationNumeric, error) {
	if manager == nil || manager.open() != nil || checker != nil && checker.Params == nil {
		return nil, ErrTableRelation
	}
	b := &relationExplanationNumeric{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), kind: kind, checker: checker}
	switch kind {
	case "bound_relation":
		accepted, err := relationExplanationNumericCanHandle(manager, kind, sorts)
		if err != nil || !accepted {
			return nil, errors.Join(err, ErrTableRelation)
		}
		b.bound = NewBoundRelation(uint32(len(sorts)), true)
	case "interval_relation":
		accepted, err := relationExplanationNumericCanHandle(manager, kind, sorts)
		if err != nil || !accepted {
			return nil, ErrTableRelation
		}
		b.interval = NewIntervalEmpty(uint32(len(sorts)))
	case "karr_relation":
		family, err := manager.manager.RegisterArithmeticPlugin()
		if err != nil {
			return nil, err
		}
		integer, err := manager.manager.MkFamilySort(family, 1, nil)
		if err != nil {
			return nil, err
		}
		for _, sort := range sorts {
			if sort != integer {
				return nil, ErrTableRelation
			}
		}
		var formula *KarrRelationFormulaChecker
		if checker != nil {
			formula, err = NewKarrRelationFormulaChecker(manager.manager, *checker)
			if err != nil {
				return nil, err
			}
		}
		b.karr = NewKarrEmpty(uint32(len(sorts)), nil, formula)
	default:
		return nil, ErrTableRelation
	}
	if checker != nil {
		if err := b.verifyConstant(false); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *relationExplanationNumeric) open() error {
	if b == nil || b.manager == nil || b.manager.open() != nil {
		return ErrTableRelation
	}
	switch b.kind {
	case "bound_relation":
		if b.bound == nil || uint32(len(b.sorts)) != uint32(len(b.bound.columns)) {
			return ErrTableRelation
		}
	case "interval_relation":
		if b.interval == nil || b.interval.values == nil || uint32(len(b.sorts)) != uint32(len(b.interval.values.columns)) {
			return ErrTableRelation
		}
	case "karr_relation":
		if b.karr == nil || uint32(len(b.sorts)) != b.karr.Dimension() {
			return ErrTableRelation
		}
	default:
		return ErrTableRelation
	}
	return nil
}

func (b *relationExplanationNumeric) Close() error {
	if b == nil {
		return nil
	}
	b.manager, b.sorts, b.bound, b.interval, b.karr, b.checker = nil, nil, nil, nil, nil, nil
	return nil
}

func (b *relationExplanationNumeric) Empty() (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	switch b.kind {
	case "bound_relation":
		return b.bound.Empty(), nil
	case "interval_relation":
		return b.interval.Empty(), nil
	default:
		return b.karr.Empty(), nil
	}
}

func (b *relationExplanationNumeric) Contains(values []ast.ASTHandle) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	if len(values) != len(b.sorts) {
		return false, ast.ErrASTDomain
	}
	switch b.kind {
	case "bound_relation":
		return b.bound.ContainsFact(values), nil
	case "interval_relation":
		return b.interval.ContainsFactExpressions(b.manager.manager, values)
	default:
		// Karr exposes an abstract formula rather than a tuple membership
		// operation. A nonempty result relation can explain every tuple query
		// routed through this global relation-level product.
		return !b.karr.Empty(), nil
	}
}

func (b *relationExplanationNumeric) Clone() (*relationExplanationNumeric, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	copy := &relationExplanationNumeric{manager: b.manager, sorts: append([]ast.ASTHandle(nil), b.sorts...), kind: b.kind, checker: b.checker}
	switch b.kind {
	case "bound_relation":
		copy.bound = b.bound.Clone()
	case "interval_relation":
		var err error
		copy.interval, err = b.interval.Clone()
		if err != nil {
			return nil, err
		}
	case "karr_relation":
		copy.karr = b.karr.Clone()
	}
	if b.checker != nil && b.kind == "bound_relation" {
		if err := VerifyBoundRelationClone(b.manager.manager, b.sorts, *b.checker, b.bound, copy.bound); err != nil {
			copy.Close()
			return nil, err
		}
	}
	if b.checker != nil && b.kind == "interval_relation" {
		if err := VerifyIntervalRelationClone(b.manager.manager, b.sorts, *b.checker, b.interval, copy.interval); err != nil {
			copy.Close()
			return nil, err
		}
	}
	return copy, nil
}

func (b *relationExplanationNumeric) Join(other *relationExplanationNumeric, left, right []uint32) (*relationExplanationNumeric, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	if err := other.open(); err != nil {
		return nil, err
	}
	if b.manager != other.manager || b.kind != other.kind || len(left) != len(right) {
		return nil, ErrTableRelation
	}
	sorts := append(append([]ast.ASTHandle(nil), b.sorts...), other.sorts...)
	checker := b.checker
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		// C++ joins the concrete backings before constructing the checked result.
		checker = nil
	}
	out, err := newRelationExplanationNumeric(b.manager, sorts, b.kind, checker)
	if err != nil {
		return nil, err
	}
	switch b.kind {
	case "bound_relation":
		out.bound, err = JoinBoundRelations(b.bound, other.bound, left, right)
	case "interval_relation":
		out.interval, err = JoinIntervalRelations(b.interval, other.interval, left, right)
	case "karr_relation":
		out.karr, err = KarrJoin(b.karr, other.karr, left, right)
	}
	if err != nil {
		out.Close()
		return nil, err
	}
	if b.checker != nil && b.kind == "bound_relation" {
		err = VerifyBoundRelationJoin(b.manager.manager, b.sorts, other.sorts, *b.checker, b.bound, other.bound, left, right, out.bound)
	}
	if b.checker != nil && b.kind == "interval_relation" {
		err = VerifyIntervalRelationJoin(b.manager.manager, b.sorts, other.sorts, *b.checker, b.interval, other.interval, left, right, out.interval)
	}
	if err != nil {
		out.Close()
		return nil, err
	}
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		out.checker = b.checker
	}
	return out, nil
}

func (b *relationExplanationNumeric) Project(removed []uint32) (*relationExplanationNumeric, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, 0, len(b.sorts)-len(removed))
	removedSet := make(map[uint32]bool, len(removed))
	for _, col := range removed {
		if col >= uint32(len(b.sorts)) || removedSet[col] {
			return nil, ast.ErrASTDomain
		}
		removedSet[col] = true
	}
	for i, s := range b.sorts {
		if !removedSet[uint32(i)] {
			sorts = append(sorts, s)
		}
	}
	checker := b.checker
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		// C++ wraps the transformed concrete relation directly; it does not
		// construct and validate an intermediate empty backing.
		checker = nil
	}
	out, err := newRelationExplanationNumeric(b.manager, sorts, b.kind, checker)
	if err != nil {
		return nil, err
	}
	switch b.kind {
	case "bound_relation":
		out.bound, err = b.bound.Project(removed)
	case "interval_relation":
		out.interval, err = b.interval.Project(removed)
	case "karr_relation":
		out.karr, err = b.karr.Project(removed)
	}
	if err != nil {
		out.Close()
		return nil, err
	}
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		var sourceRef ast.ASTRef
		if b.kind == "bound_relation" {
			sourceRef, err = b.bound.ToFormula(b.manager.manager, b.sorts)
		} else {
			sourceRef, err = b.interval.ToFormula(b.manager.manager, b.sorts)
		}
		if err != nil {
			out.Close()
			return nil, err
		}
		defer b.manager.manager.DelRef(&sourceRef)
		sourceHandle, err := b.manager.manager.RefGet(sourceRef)
		if err != nil {
			out.Close()
			return nil, err
		}
		removedColumns := make([]int, len(removed))
		for i, column := range removed {
			removedColumns[i] = int(column)
		}
		switch b.kind {
		case "bound_relation":
			err = VerifyBoundRelationProject(b.manager.manager, b.sorts, sorts, *b.checker, sourceHandle, removedColumns, out.bound)
		case "interval_relation":
			err = VerifyIntervalRelationProject(b.manager.manager, b.sorts, sorts, *b.checker, sourceHandle, removedColumns, out.interval)
		}
		if err != nil {
			out.Close()
			return nil, err
		}
		out.checker = b.checker
	}
	return out, nil
}

func (b *relationExplanationNumeric) Rename(cycle []uint32) (*relationExplanationNumeric, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	checker := b.checker
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		checker = nil
	}
	out, err := newRelationExplanationNumeric(b.manager, b.sorts, b.kind, checker)
	if err != nil {
		return nil, err
	}
	switch b.kind {
	case "bound_relation":
		out.bound, err = b.bound.Rename(cycle)
	case "interval_relation":
		out.interval, err = b.interval.Rename(cycle)
	case "karr_relation":
		out.karr, err = b.karr.Rename(cycle)
	}
	if err != nil {
		out.Close()
		return nil, err
	}
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		var sourceRef ast.ASTRef
		if b.kind == "bound_relation" {
			sourceRef, err = b.bound.ToFormula(b.manager.manager, b.sorts)
		} else {
			sourceRef, err = b.interval.ToFormula(b.manager.manager, b.sorts)
		}
		if err != nil {
			out.Close()
			return nil, err
		}
		defer b.manager.manager.DelRef(&sourceRef)
		sourceHandle, err := b.manager.manager.RefGet(sourceRef)
		if err != nil {
			out.Close()
			return nil, err
		}
		switch b.kind {
		case "bound_relation":
			err = VerifyBoundRelationRename(b.manager.manager, b.sorts, out.sorts, *b.checker, sourceHandle, cycle, out.bound)
		case "interval_relation":
			err = VerifyIntervalRelationRename(b.manager.manager, b.sorts, out.sorts, *b.checker, sourceHandle, cycle, out.interval)
		}
		if err != nil {
			out.Close()
			return nil, err
		}
		out.checker = b.checker
	}
	return out, nil
}

func (b *relationExplanationNumeric) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := b.open(); err != nil {
		return err
	}
	if column >= uint32(len(b.sorts)) {
		return ast.ErrASTDomain
	}
	var before ast.ASTRef
	var condition ast.ASTRef
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		var err error
		if b.kind == "bound_relation" {
			before, err = b.bound.ToFormula(b.manager.manager, b.sorts)
		} else {
			before, err = b.interval.ToFormula(b.manager.manager, b.sorts)
		}
		if err != nil {
			return err
		}
		defer b.manager.manager.DelRef(&before)
		variable, err := b.manager.manager.MkVar(column, b.sorts[column])
		if err != nil {
			return err
		}
		equality, err := b.manager.manager.MkEq(variable, value)
		if err != nil {
			return err
		}
		condition, err = b.manager.manager.NewRef(equality)
		if err != nil {
			return err
		}
		defer b.manager.manager.DelRef(&condition)
	}
	switch b.kind {
	case "bound_relation":
		b.bound.FilterEqual(0, column)
	case "interval_relation":
		if err := b.interval.FilterEqualExpression(b.manager.manager, column, value); err != nil {
			return err
		}
	case "karr_relation":
		text, _, matched, err := b.manager.manager.ArithmeticNumeralText(value)
		if err != nil {
			return err
		}
		_, integer := new(big.Rat).SetString(text)
		return b.karr.FilterEqual(column, matched && integer)
	}
	if b.checker != nil {
		beforeHandle, err := b.manager.manager.RefGet(before)
		if err != nil {
			return err
		}
		conditionHandle, err := b.manager.manager.RefGet(condition)
		if err != nil {
			return err
		}
		switch b.kind {
		case "bound_relation":
			return VerifyBoundRelationFilter(b.manager.manager, b.sorts, *b.checker, beforeHandle, conditionHandle, b.bound, false)
		case "interval_relation":
			return VerifyIntervalRelationFilter(b.manager.manager, b.sorts, *b.checker, beforeHandle, conditionHandle, b.interval, false)
		}
	}
	return nil
}

func (b *relationExplanationNumeric) FilterIdentical(columns []uint32) error {
	if err := b.open(); err != nil {
		return err
	}
	switch b.kind {
	case "bound_relation":
		var before, condition ast.ASTRef
		if b.checker != nil {
			if len(columns) == 0 {
				return ast.ErrASTDomain
			}
			var err error
			before, err = b.bound.ToFormula(b.manager.manager, b.sorts)
			if err != nil {
				return err
			}
			defer b.manager.manager.DelRef(&before)
			condition, err = numericIdenticalCondition(b.manager.manager, b.sorts, columns)
			if err != nil {
				return err
			}
			defer b.manager.manager.DelRef(&condition)
		}
		if err := b.bound.FilterIdentical(columns); err != nil {
			return err
		}
		if b.checker != nil {
			beforeHandle, err := b.manager.manager.RefGet(before)
			if err != nil {
				return err
			}
			conditionHandle, err := b.manager.manager.RefGet(condition)
			if err != nil {
				return err
			}
			return VerifyBoundRelationFilter(b.manager.manager, b.sorts, *b.checker, beforeHandle, conditionHandle, b.bound, false)
		}
		return nil
	case "interval_relation":
		var before, condition ast.ASTRef
		if b.checker != nil {
			if len(columns) == 0 {
				return ast.ErrASTDomain
			}
			var err error
			before, err = b.interval.ToFormula(b.manager.manager, b.sorts)
			if err != nil {
				return err
			}
			defer b.manager.manager.DelRef(&before)
			condition, err = numericIdenticalCondition(b.manager.manager, b.sorts, columns)
			if err != nil {
				return err
			}
			defer b.manager.manager.DelRef(&condition)
		}
		if err := b.interval.FilterIdenticalColumns(columns); err != nil {
			return err
		}
		if b.checker != nil {
			beforeHandle, err := b.manager.manager.RefGet(before)
			if err != nil {
				return err
			}
			conditionHandle, err := b.manager.manager.RefGet(condition)
			if err != nil {
				return err
			}
			return VerifyIntervalRelationFilter(b.manager.manager, b.sorts, *b.checker, beforeHandle, conditionHandle, b.interval, false)
		}
		return nil
	default:
		return b.karr.FilterIdentical(columns)
	}
}

func numericIdenticalCondition(manager *ast.ASTManager, sorts []ast.ASTHandle, columns []uint32) (result ast.ASTRef, resultErr error) {
	if manager == nil || len(columns) == 0 {
		return result, ast.ErrASTDomain
	}
	first := columns[0]
	if first >= uint32(len(sorts)) {
		return result, ast.ErrASTDomain
	}
	condition := make([]ast.ASTHandle, 0, len(columns)-1)
	for _, column := range columns[1:] {
		if column >= uint32(len(sorts)) {
			return result, ast.ErrASTDomain
		}
		left, err := manager.MkVar(first, sorts[first])
		if err != nil {
			return result, err
		}
		right, err := manager.MkVar(column, sorts[column])
		if err != nil {
			return result, err
		}
		equality, err := manager.MkEq(left, right)
		if err != nil {
			return result, err
		}
		condition = append(condition, equality)
	}
	formula, err := manager.MkAndArgs(condition)
	if err != nil {
		return result, err
	}
	return manager.NewRef(formula)
}

func (b *relationExplanationNumeric) FilterInterpreted(condition ast.ASTHandle) error {
	if err := b.open(); err != nil {
		return err
	}
	var before ast.ASTRef
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		var err error
		if b.kind == "bound_relation" {
			before, err = b.bound.ToFormula(b.manager.manager, b.sorts)
		} else {
			before, err = b.interval.ToFormula(b.manager.manager, b.sorts)
		}
		if err != nil {
			return err
		}
		defer b.manager.manager.DelRef(&before)
	}
	switch b.kind {
	case "bound_relation":
		_, err := b.bound.FilterInterpreted(b.manager.manager, condition)
		if err != nil {
			return err
		}
	case "interval_relation":
		if err := b.interval.FilterInterpreted(b.manager.manager, condition); err != nil {
			return err
		}
	default:
		return ErrCompilerUnsupported
	}
	if b.checker == nil {
		return nil
	}
	beforeHandle, err := b.manager.manager.RefGet(before)
	if err != nil {
		return err
	}
	switch b.kind {
	case "bound_relation":
		return VerifyBoundRelationFilter(b.manager.manager, b.sorts, *b.checker, beforeHandle, condition, b.bound, false)
	case "interval_relation":
		return VerifyIntervalRelationFilter(b.manager.manager, b.sorts, *b.checker, beforeHandle, condition, b.interval, false)
	default:
		return ErrCompilerUnsupported
	}
}

func (b *relationExplanationNumeric) UnionFrom(source, delta *relationExplanationNumeric) error {
	if err := b.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if b.kind != source.kind || b.manager != source.manager {
		return ErrTableRelation
	}
	if delta != nil {
		if err := delta.open(); err != nil {
			return err
		}
		if delta.kind != b.kind || delta.manager != b.manager || delta == b || delta == source {
			return ErrTableRelation
		}
	}
	var before ast.ASTRef
	var deltaBefore ast.ASTRef
	if b.checker != nil && (b.kind == "bound_relation" || b.kind == "interval_relation") {
		var err error
		if b.kind == "bound_relation" {
			before, err = b.bound.ToFormula(b.manager.manager, b.sorts)
		} else {
			before, err = b.interval.ToFormula(b.manager.manager, b.sorts)
		}
		if err != nil {
			return err
		}
		defer b.manager.manager.DelRef(&before)
		if delta != nil {
			if b.kind == "bound_relation" {
				deltaBefore, err = delta.bound.ToFormula(b.manager.manager, delta.sorts)
			} else {
				deltaBefore, err = delta.interval.ToFormula(b.manager.manager, delta.sorts)
			}
			if err != nil {
				return err
			}
			defer b.manager.manager.DelRef(&deltaBefore)
		}
	}
	var changed bool
	var err error
	switch b.kind {
	case "bound_relation":
		if delta != nil {
			changed, err = b.bound.UnionWithDelta(source.bound, delta.bound, false)
		} else {
			changed, err = b.bound.Union(source.bound)
		}
	case "interval_relation":
		if delta != nil {
			changed, err = b.interval.UnionWithDelta(source.interval, delta.interval, false)
		} else {
			changed, err = b.interval.Union(source.interval, false)
		}
	default:
		changed, err = b.karr.Union(source.karr)
		if err == nil && changed && delta != nil {
			_, err = delta.karr.Union(source.karr)
		}
	}
	if err != nil || b.checker == nil || (b.kind != "bound_relation" && b.kind != "interval_relation") {
		return err
	}
	beforeHandle, err := b.manager.manager.RefGet(before)
	if err != nil {
		return err
	}
	if delta != nil {
		sourceFormula, formulaErr := source.ToFormula()
		if formulaErr != nil {
			return formulaErr
		}
		defer b.manager.manager.DelRef(&sourceFormula)
		sourceHandle, formulaErr := b.manager.manager.RefGet(sourceFormula)
		if formulaErr != nil {
			return formulaErr
		}
		destinationFormula, formulaErr := b.ToFormula()
		if formulaErr != nil {
			return formulaErr
		}
		defer b.manager.manager.DelRef(&destinationFormula)
		destinationHandle, formulaErr := b.manager.manager.RefGet(destinationFormula)
		if formulaErr != nil {
			return formulaErr
		}
		deltaFormula, formulaErr := delta.ToFormula()
		if formulaErr != nil {
			return formulaErr
		}
		defer b.manager.manager.DelRef(&deltaFormula)
		deltaHandle, formulaErr := b.manager.manager.RefGet(deltaFormula)
		if formulaErr != nil {
			return formulaErr
		}
		deltaBeforeHandle, formulaErr := b.manager.manager.RefGet(deltaBefore)
		if formulaErr != nil {
			return formulaErr
		}
		return verifyRelationUnionDelta(b.manager.manager, b.sorts, *b.checker, beforeHandle, sourceHandle, destinationHandle, deltaBeforeHandle, deltaHandle)
	}
	switch b.kind {
	case "bound_relation":
		return VerifyBoundRelationUnion(b.manager.manager, b.sorts, *b.checker, beforeHandle, source.bound, b.bound)
	case "interval_relation":
		return VerifyIntervalRelationUnion(b.manager.manager, b.sorts, *b.checker, beforeHandle, source.interval, b.interval)
	default:
		return nil
	}
}

func (p *RelationExplanationProduct) UnionNumericSeed(seed RelationExplanationNumericSeed) error {
	if err := p.open(); err != nil {
		return err
	}
	if p.numeric == nil || seed.Kind != p.numeric.kind {
		return ErrTableRelation
	}
	switch seed.Kind {
	case "bound_relation":
		if seed.Bound == nil || uint32(len(seed.Bound.columns)) != uint32(len(p.finiteSorts)) {
			return ErrTableRelation
		}
		_, err := p.numeric.bound.Union(seed.Bound)
		return err
	case "interval_relation":
		if seed.Interval == nil || uint32(len(seed.Interval.values.columns)) != uint32(len(p.finiteSorts)) {
			return ErrTableRelation
		}
		_, err := p.numeric.interval.Union(seed.Interval, false)
		return err
	case "karr_relation":
		if seed.Karr == nil || seed.Karr.Dimension() != uint32(len(p.finiteSorts)) {
			return ErrTableRelation
		}
		_, err := p.numeric.karr.Union(seed.Karr)
		return err
	default:
		return ErrTableRelation
	}
}

func (b *relationExplanationNumeric) ToFormula() (ast.ASTRef, error) {
	if err := b.open(); err != nil {
		return ast.ASTRef{}, err
	}
	switch b.kind {
	case "bound_relation":
		return b.bound.ToFormula(b.manager.manager, b.sorts)
	case "interval_relation":
		return b.interval.ToFormula(b.manager.manager, b.sorts)
	default:
		return b.karr.ToFormula(b.manager.manager, b.sorts)
	}
}

func (p *RelationExplanationProduct) toFormulaWithNumeric() (result ast.ASTRef, resultErr error) {
	m := p.manager.manager
	base, err := p.numeric.ToFormula()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&base)) }()
	proofEmpty, err := p.explanation.Empty()
	if err != nil {
		return result, err
	}
	var proof []ast.ASTHandle
	if !proofEmpty {
		proof, err = p.explanation.Data()
		if err != nil {
			return result, err
		}
	}
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle) error {
		ref, cause := m.NewRef(value)
		if cause == nil {
			roots = append(roots, ref)
		}
		return cause
	}
	conjuncts := make([]ast.ASTHandle, 0, len(proof))
	for column, value := range proof {
		if value.IsNull() {
			continue
		}
		proofSort, cause := m.GetSort(value)
		if cause != nil {
			return result, cause
		}
		variable, cause := m.MkVar(uint32(len(p.finiteSorts)+column), proofSort)
		if cause != nil {
			return result, cause
		}
		if cause = hold(variable); cause != nil {
			return result, cause
		}
		equality, cause := m.MkEq(variable, value)
		if cause != nil {
			return result, cause
		}
		if cause = hold(equality); cause != nil {
			return result, cause
		}
		conjuncts = append(conjuncts, equality)
	}
	proofRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	roots = append(roots, proofRef)
	proofGate := ast.ASTOpAnd
	if proofEmpty {
		proofGate = ast.ASTOpOr
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, proofGate, conjuncts, proofRef); err != nil {
		return result, err
	}
	baseHandle, err := m.RefGet(base)
	if err != nil {
		return result, err
	}
	proofHandle, err := m.RefGet(proofRef)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	return result, rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{baseHandle, proofHandle}, result)
}

func (b *relationExplanationNumeric) verifyConstant(full bool) error {
	if b.checker == nil {
		return nil
	}
	switch b.kind {
	case "bound_relation":
		return VerifyBoundRelationConstant(b.manager.manager, b.sorts, *b.checker, b.bound, full)
	case "interval_relation":
		return VerifyIntervalRelationConstant(b.manager.manager, b.sorts, *b.checker, b.interval, full)
	default:
		return VerifyKarrRelationConstant(b.manager.manager, b.sorts, *b.checker, b.karr, full)
	}
}
