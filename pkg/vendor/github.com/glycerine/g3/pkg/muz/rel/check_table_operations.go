package rel

// Source: src/muz/rel/dl_check_table.cpp operation-forwarding functors.

import (
	"errors"
	"fmt"
)

func adoptCheckedTables(primary, checker *HashTable) (*CheckTable, error) {
	table, err := AdoptCheckTable(primary, checker)
	if err != nil {
		return nil, errors.Join(err, primary.Close(), checker.Close())
	}
	return table, nil
}

func (t *CheckTable) compatible(other *CheckTable) bool {
	return t != nil && other != nil && t.signature.equal(other.signature)
}

func (t *CheckTable) Join(other *CheckTable, leftColumns, rightColumns []int) (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	primary, err := t.primary.Join(other.primary, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	checker, err := t.checker.Join(other.checker, leftColumns, rightColumns)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedTables(primary, checker)
}

func (t *CheckTable) JoinProject(other *CheckTable, leftColumns, rightColumns, removed []int) (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	primary, err := t.primary.JoinProject(other.primary, leftColumns, rightColumns, removed)
	if err != nil {
		return nil, err
	}
	checker, err := t.checker.JoinProject(other.checker, leftColumns, rightColumns, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedTables(primary, checker)
}

func (t *CheckTable) Project(removed []int) (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	checker, err := t.checker.Project(removed)
	if err != nil {
		return nil, err
	}
	primary, err := t.primary.Project(removed)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return adoptCheckedTables(primary, checker)
}

func (t *CheckTable) SelectEqualAndProject(value Element, column int) (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	checker, err := t.checker.SelectEqualAndProject(value, column)
	if err != nil {
		return nil, err
	}
	primary, err := t.primary.SelectEqualAndProject(value, column)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return adoptCheckedTables(primary, checker)
}

func (t *CheckTable) RenameCycle(cycle []int) (*CheckTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	checker, err := t.checker.RenameCycle(cycle)
	if err != nil {
		return nil, err
	}
	primary, err := t.primary.RenameCycle(cycle)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return adoptCheckedTables(primary, checker)
}

func (t *CheckTable) UnionFrom(source, delta *CheckTable) error {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !t.compatible(source) {
		return fmt.Errorf("%w: union signatures differ", ErrCheckTable)
	}
	var primaryDelta, checkerDelta *HashTable
	if delta != nil {
		if delta.open() != nil || !t.compatible(delta) {
			return fmt.Errorf("%w: delta signatures differ", ErrCheckTable)
		}
		primaryDelta, checkerDelta = delta.primary, delta.checker
	}
	if err := t.primary.Union(source.primary, primaryDelta); err != nil {
		return err
	}
	if err := t.checker.Union(source.checker, checkerDelta); err != nil {
		return err
	}
	if err := t.Validate(); err != nil {
		return err
	}
	if delta != nil {
		return delta.Validate()
	}
	return nil
}

// RemoveFacts follows table_base::remove_facts, dispatching each fact through
// CheckTable.Remove so both backing tables are checked after each removal.
func (t *CheckTable) RemoveFacts(facts []Fact) error {
	for _, fact := range facts {
		if _, err := t.Remove(fact); err != nil {
			return err
		}
	}
	return nil
}

func (t *CheckTable) FilterEqual(value Element, column int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.checker.FilterEqual(value, column); err != nil {
		return err
	}
	if err := t.primary.FilterEqual(value, column); err != nil {
		return err
	}
	return t.Validate()
}

func (t *CheckTable) FilterIdentical(columns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.checker.FilterIdentical(columns); err != nil {
		return err
	}
	if err := t.primary.FilterIdentical(columns); err != nil {
		return err
	}
	return t.Validate()
}

func (t *CheckTable) FilterByNegation(source *CheckTable, targetColumns, sourceColumns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil {
		return ErrCheckTable
	}
	if err := t.checker.FilterByNegation(source.checker, targetColumns, sourceColumns); err != nil {
		return err
	}
	if err := t.primary.FilterByNegation(source.primary, targetColumns, sourceColumns); err != nil {
		return err
	}
	return t.Validate()
}
