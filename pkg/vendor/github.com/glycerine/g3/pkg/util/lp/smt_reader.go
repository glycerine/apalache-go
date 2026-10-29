package lp

// Source: src/test/smt_reader.h:88-112,178-310,371-406.

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// SMTReader implements the retained test reader's :formula/and grammar. It
// owns constraints and partial lexical/population work in a borrowed manager.
// A failed CreateLARSolver returns its partial owner; close that solver first.
type SMTReader struct {
	Name                string
	LineNumber          uint32
	Formula             SMTLispElement
	Constraints         []*SMTFormulaConstraint
	Messages            io.Writer
	Names               map[string]uint32
	input               *bufio.Reader
	file                *os.File
	manager             *numeral.RationalManager
	pending             *SMTFormulaConstraint
	pendingCopy         *SMTFormulaConstraint
	population          *SparseRationalVector
	populationWork      *IndexedRationalVector
	ok, stopped, closed bool
}

func NewSMTReader(m *numeral.RationalManager, input io.Reader) *SMTReader {
	r := &SMTReader{manager: m, ok: true, Messages: os.Stdout, Names: make(map[string]uint32)}
	if input != nil {
		r.input = bufio.NewReader(input)
	}
	return r
}

func OpenSMTReader(m *numeral.RationalManager, name string) *SMTReader {
	f, err := os.Open(name)
	r := NewSMTReader(m, nil)
	r.Name = name
	if err == nil {
		r.file, r.input = f, bufio.NewReader(f)
	}
	return r
}

func (r *SMTReader) IsOK() bool { return r.ok }
func (r *SMTReader) ready() error {
	if r.closed || r.stopped || r.pending != nil || r.pendingCopy != nil || r.population != nil || r.populationWork != nil {
		return ErrExactVectorPending
	}
	if r.manager == nil {
		return ErrExactVectorOwner
	}
	return nil
}

func (r *SMTReader) setError() error {
	if err := writeDriverLine(r.Messages, "setting error"); err != nil {
		return err
	}
	r.ok = false
	return nil
}

func (r *SMTReader) parseLine(line string) error {
	if !strings.HasPrefix(line, ":formula") {
		return nil
	}
	i := strings.IndexByte(line, '(')
	if i < 0 {
		return writeDriverLine(r.Messages, "empty formula")
	}
	line = line[i:]
	return smtFillElement(&line, &r.Formula)
}

func (r *SMTReader) Read() (err error) {
	if err = r.ready(); err != nil {
		return err
	}
	defer func() { r.stopped = err != nil }()
	if r.input == nil {
		if err = writeDriverLine(r.Messages, "cannot open file "+r.Name); err != nil {
			return err
		}
		return r.setError()
	}
	for r.ok {
		line, e := r.input.ReadString('\n')
		if e != nil && e != io.EOF {
			return e
		}
		if e == io.EOF && line == "" {
			break
		}
		if err = r.parseLine(strings.TrimSuffix(line, "\n")); err != nil {
			return err
		}
		r.LineNumber++
		if e == io.EOF {
			break
		}
	}
	if r.file != nil {
		f := r.file
		r.file = nil
		r.input = nil
		if err = f.Close(); err != nil {
			return err
		}
	}
	if r.Formula.Head != "and" {
		if err = writeDriverLine(r.Messages, "unexpected top element "+r.Formula.Head); err != nil {
			return err
		}
		return r.setError()
	}
	for _, el := range r.Formula.Elements {
		if err = r.createConstraint(el); err != nil {
			return err
		}
	}
	return nil
}

func (r *SMTReader) createConstraint(el SMTLispElement) error {
	c := &SMTFormulaConstraint{Values: NewIndexedRationalVector(r.manager, 1)}
	r.pending = c
	switch el.Head {
	case "=":
		c.Kind = LConstraintEQ
	case ">=":
		c.Kind = LConstraintGE
	case "<=":
		c.Kind = LConstraintLE
	case ">":
		c.Kind = LConstraintGT
	case "<":
		c.Kind = LConstraintLT
	default:
		if err := writeDriverLine(r.Messages, "kind "+el.Head+" is not supported "); err != nil {
			return err
		}
		if err := r.setError(); err != nil {
			return err
		}
		return ErrBounds // source would continue with uninitialized m_kind
	}
	if err := smtAssert(len(el.Elements) == 2); err != nil {
		return err
	}
	if len(el.Elements) < 2 {
		return ErrBounds
	}
	lhs := el.Elements[0] // source adjust_rigth_side deliberately ignores RHS
	if lhs.Head == "*" {
		if err := c.addProduct(lhs); err != nil {
			return err
		}
	} else if lhs.Head == "+" {
		for _, term := range lhs.Elements {
			if err := r.addSumElement(c, term); err != nil {
				return err
			}
		}
	} else if err := sourceAssertionFailure(); err != nil {
		return err
	}
	// Source push_back(c) copies coefficient pairs before the RHS member, then
	// destroys the lexical original. Retain both partial owners on refusal.
	stored := &SMTFormulaConstraint{Kind: c.Kind, Values: NewIndexedRationalVector(r.manager, uint32(len(c.Names)+1))}
	r.pendingCopy = stored
	for i, name := range c.Names {
		if err := stored.Values.copySlot(&stored.Values.Data[i+1], c.Coefficient(i)); err != nil {
			return err
		}
		stored.Names = append(stored.Names, name)
	}
	if err := stored.Values.copySlot(&stored.Values.Data[0], &c.Values.Data[0]); err != nil {
		return err
	}
	r.Constraints = append(r.Constraints, stored)
	r.pendingCopy = nil
	if err := c.Close(); err != nil {
		return err
	}
	r.pending = nil
	return nil
}

func (r *SMTReader) addSumElement(c *SMTFormulaConstraint, el SMTLispElement) error {
	if len(el.Elements) == 0 {
		n, err := smtInteger(el.Head)
		if err != nil {
			return err
		}
		return c.addConstant(n, false)
	}
	if el.Head == "*" {
		return c.addProduct(el)
	}
	if el.Head == "~" {
		if err := smtAssert(len(el.Elements[0].Elements) == 0); err != nil {
			return err
		}
		n, err := smtInteger(el.Elements[0].Head)
		if err != nil {
			return err
		}
		return c.addConstant(n, true)
	}
	if err := writeDriverLine(r.Messages, "unexpected input "+el.Head); err != nil {
		return err
	}
	return r.setError()
}

func (r *SMTReader) CreateLARSolver() (s *LARSolver, err error) {
	if err = r.ready(); err != nil {
		return nil, err
	}
	defer func() { r.stopped = err != nil }()
	s, err = NewLARSolver(r.manager)
	if err != nil {
		return s, err
	}
	for _, c := range r.Constraints {
		r.population = NewSparseRationalVector(r.manager)
		for i, name := range c.Names {
			j, exists := r.Names[name]
			if !exists {
				j = uint32(len(r.Names))
				r.Names[name] = j
			}
			column, e := s.AddVar(j)
			if e != nil {
				return s, e
			}
			// make_pair copies its lvalue coefficient once; vector::push_back
			// selects the rvalue overload and moves that pair into storage.
			r.populationWork = NewIndexedRationalVector(r.manager, 1)
			if err = r.populationWork.copySlot(&r.populationWork.Data[0], c.Coefficient(i)); err != nil {
				return s, err
			}
			r.population.Data = append(r.population.Data, RationalSparseEntry{Index: column, Value: r.populationWork.Data[0]})
			r.populationWork.Data[0] = numeral.NewRational()
			if err = r.populationWork.Close(); err != nil {
				return s, err
			}
			r.populationWork = nil
		}
		if _, err = s.AddConstraint(r.population.Data, c.Kind, &c.Values.Data[0]); err != nil {
			return s, err
		}
		if err = r.population.Close(); err != nil {
			return s, err
		}
		r.population = nil
	}
	return s, nil
}

func (r *SMTReader) Close() error {
	if r.populationWork != nil {
		if err := r.populationWork.Close(); err != nil {
			return err
		}
		r.populationWork = nil
	}
	if r.population != nil {
		if err := r.population.Close(); err != nil {
			return err
		}
		r.population = nil
	}
	if r.pendingCopy != nil {
		if err := r.pendingCopy.Close(); err != nil {
			return err
		}
		r.pendingCopy = nil
	}
	if r.pending != nil {
		if err := r.pending.Close(); err != nil {
			return err
		}
		r.pending = nil
	}
	for len(r.Constraints) > 0 {
		last := len(r.Constraints) - 1
		if err := r.Constraints[last].Close(); err != nil {
			return err
		}
		r.Constraints = r.Constraints[:last]
	}
	r.closed = true
	if r.file != nil {
		f := r.file
		r.file = nil
		return f.Close()
	}
	return nil
}
