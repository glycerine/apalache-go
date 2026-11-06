package rel

// Source: src/muz/rel/dl_table_relation.cpp
// table_relation_plugin::mk_filter_by_intersection_fn and the table manager
// intersection-filter contract.

import "fmt"

func validateTableIntersection(target, source *HashTable, targetColumns, sourceColumns []int) error {
	if target == nil || source == nil {
		return fmt.Errorf("%w: nil intersection table", ErrTable)
	}
	if err := target.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	return validateColumns(target.signature, source.signature, targetColumns, sourceColumns)
}
