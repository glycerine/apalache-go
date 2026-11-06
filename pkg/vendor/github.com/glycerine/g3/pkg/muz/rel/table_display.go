package rel

// Source: src/muz/rel/dl_base.cpp table_base::display and
// src/muz/rel/dl_table_relation.h table_relation::display.

import (
	"fmt"
	"io"
)

// Display writes the source table signature and insertion-ordered rows.
func (t *HashTable) Display(out io.Writer) error {
	if err := t.open(); err != nil || out == nil {
		if err != nil {
			return err
		}
		return ErrTable
	}
	if _, err := io.WriteString(out, "table with signature ("); err != nil {
		return err
	}
	for index, domain := range t.signature.domains {
		if index != 0 {
			if _, err := io.WriteString(out, ","); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(out, domain); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(out, "):\n"); err != nil {
		return err
	}
	for _, fact := range t.rows.All() {
		if _, err := io.WriteString(out, "("); err != nil {
			return err
		}
		for index, value := range fact {
			if index != 0 {
				if _, err := io.WriteString(out, ","); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(out, value); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, "\n")
	return err
}

// Display forwards the table-backed relation's source display operation.
func (r *TableRelation) Display(out io.Writer) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.table.Display(out)
}
