package lp

// Source: src/test/smt_reader.h:50-70,114-176.

import "strings"

// SMTLispElement is the retained test reader's small, single-line syntax tree.
// Only ASCII space and parentheses delimit tokens; this is not SMT-LIB syntax.
type SMTLispElement struct {
	Head     string
	Elements []SMTLispElement
}

func smtSeparator(line string) int {
	return strings.IndexAny(line, " ()")
}

func smtFillElement(line *string, out *SMTLispElement) error {
	if *line == "" {
		return ErrBounds
	}
	if (*line)[0] != '(' {
		i := smtSeparator(*line)
		if i <= 0 {
			if err := sourceAssertionFailure(); err != nil {
				return err
			}
			return ErrBounds // source invalid substr/nonprogress domain
		}
		out.Head, *line = (*line)[:i], (*line)[i:]
		return nil
	}
	*line = (*line)[1:]
	i := smtSeparator(*line)
	if i < 0 {
		i = len(*line)
	}
	out.Head, *line = (*line)[:i], (*line)[i:]
	*line = strings.TrimLeft(*line, " ")
	for *line != "" {
		if (*line)[0] == ')' {
			*line = (*line)[1:]
			break
		}
		var child SMTLispElement
		if err := smtFillElement(line, &child); err != nil {
			return err
		}
		out.Elements = append(out.Elements, child)
		*line = strings.TrimLeft(*line, " ")
	}
	return nil
}
