package rel

import (
	"fmt"
	"io"
)

// Display writes the sparse table's domain signature and insertion-ordered
// rows, matching table_base::display.
func (t *SparseTable) Display(out io.Writer) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
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
	facts, err := t.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if _, err := io.WriteString(out, "("); err != nil {
			return err
		}
		for index, value := range fact {
			if index != 0 {
				if _, err = io.WriteString(out, ","); err != nil {
					return err
				}
			}
			if _, err = fmt.Fprint(out, value); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, "\n")
	return err
}
