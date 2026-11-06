package smtcore

// Source: src/smt/watch_list.h and src/smt/watch_list.cpp.

import "github.com/glycerine/g3/pkg/ast"

// SMTWatchList stores the two source watch-list regions as typed Go slices.
// Clause pointers are in insertion order. Literals are in iterator order,
// which is reverse insertion order in the packed source representation.
type SMTWatchList struct {
	clauses  []*SMTClause
	literals []SMTLiteral
}

func (w *SMTWatchList) Size() int {
	if w == nil {
		return 0
	}
	return len(w.clauses) + len(w.literals)
}

func (w *SMTWatchList) Reset() {
	if w == nil {
		return
	}
	w.clauses = w.clauses[:0]
	w.literals = w.literals[:0]
}

func (w *SMTWatchList) ResetAndReleaseMemory() {
	if w == nil {
		return
	}
	w.clauses = nil
	w.literals = nil
}

// Clauses and Literals return the live mutable views used by propagation.
func (w *SMTWatchList) Clauses() []*SMTClause {
	if w == nil {
		return nil
	}
	return w.clauses
}

func (w *SMTWatchList) Literals() []SMTLiteral {
	if w == nil {
		return nil
	}
	return w.literals
}

func (w *SMTWatchList) FindClause(clause *SMTClause) int {
	if w == nil {
		return -1
	}
	for i, candidate := range w.clauses {
		if candidate == clause {
			return i
		}
	}
	return -1
}

func (w *SMTWatchList) FindLiteral(literal SMTLiteral) int {
	if w == nil {
		return -1
	}
	for i, candidate := range w.literals {
		if candidate == literal {
			return i
		}
	}
	return -1
}

func (w *SMTWatchList) InsertClause(clause *SMTClause) error {
	if w == nil {
		return ast.ErrASTHandle
	}
	w.clauses = append(w.clauses, clause)
	return nil
}

func (w *SMTWatchList) InsertLiteral(literal SMTLiteral) error {
	if w == nil {
		return ast.ErrASTHandle
	}
	w.literals = append(w.literals, 0)
	copy(w.literals[1:], w.literals[:len(w.literals)-1])
	w.literals[0] = literal
	return nil
}

func (w *SMTWatchList) RemoveClause(clause *SMTClause) bool {
	i := w.FindClause(clause)
	if i < 0 {
		return false
	}
	copy(w.clauses[i:], w.clauses[i+1:])
	w.clauses[len(w.clauses)-1] = nil
	w.clauses = w.clauses[:len(w.clauses)-1]
	return true
}

func (w *SMTWatchList) RemoveLiteral(literal SMTLiteral) bool {
	i := w.FindLiteral(literal)
	if i < 0 {
		return false
	}
	copy(w.literals[i:], w.literals[i+1:])
	w.literals = w.literals[:len(w.literals)-1]
	return true
}

// SetClauseCount is the source set_end_clause operation used after a caller
// compacts the mutable Clauses view in place.
func (w *SMTWatchList) SetClauseCount(count int) error {
	if w == nil {
		return ast.ErrASTHandle
	}
	if count < 0 || count > len(w.clauses) {
		return ast.ErrASTDomain
	}
	for i := count; i < len(w.clauses); i++ {
		w.clauses[i] = nil
	}
	w.clauses = w.clauses[:count]
	return nil
}
