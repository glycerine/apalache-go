package lp

// Source: src/util/lp/mps_reader.h:41-671,741-794,800-897 (mpq/LAR consumer).

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/insdict"
)

type rationalMPSBound struct {
	low, upper, fixed                *numeral.Rational
	lowSet, upperSet, fixedSet, free bool
}
type rationalMPSColumn struct {
	name  string
	index uint32
	bound *rationalMPSBound
}
type rationalMPSRow struct {
	name         string
	index        uint32
	typ          mpsRowType
	rhs, span    *numeral.Rational
	coefficients *insdict.Dict[string, *numeral.Rational]
}

// RationalMPSReader parses exact numerals in a borrowed manager. Go text and
// insertion-ordered dictionaries replace source string/unordered containers;
// iteration order is input order. Close after returned solver owners and before
// the numeral manager, except that a successful CreateLARSolver leaves no
// borrowed reader numerals and permits the source file driver's earlier Close.
// Failed lexical arithmetic retains its reached prefix
// until Close; do not copy the reader or mutate its owning numeral pointers.
type RationalMPSReader struct {
	input                 *bufio.Reader
	file                  *os.File
	rows                  *insdict.Dict[string, *rationalMPSRow]
	columns               *insdict.Dict[string, *rationalMPSColumn]
	namesToVarIndex       map[string]uint32
	line, Name, costName  string
	LineNumber, costLines uint32
	ok, stopped, closed   bool
	Messages              io.Writer
	manager               *numeral.RationalManager
	calls                 exactEtaCalls
	values                []*numeral.Rational
	work                  *IndexedRationalVector
	population            *SparseRationalVector
}

func NewRationalMPSReader(manager *numeral.RationalManager, in io.Reader) *RationalMPSReader {
	m := &RationalMPSReader{manager: manager, ok: true, Messages: os.Stdout, rows: insdict.NewDict[string, *rationalMPSRow](), columns: insdict.NewDict[string, *rationalMPSColumn](), namesToVarIndex: make(map[string]uint32)}
	if in != nil {
		m.input = bufio.NewReader(in)
	}
	return m
}
func OpenRationalMPSReader(manager *numeral.RationalManager, name string) *RationalMPSReader {
	f, err := os.Open(name)
	if err != nil {
		return NewRationalMPSReader(manager, nil)
	}
	m := NewRationalMPSReader(manager, f)
	m.file = f
	return m
}
func (m *RationalMPSReader) ready() error {
	if m.closed || m.stopped || m.work != nil || m.population != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	if m.manager == nil {
		return ErrExactVectorOwner
	}
	return nil
}
func (m *RationalMPSReader) newValue() *numeral.Rational {
	v := numeral.NewRational()
	m.values = append(m.values, &v)
	return &v
}
func (m *RationalMPSReader) newBound() *rationalMPSBound {
	return &rationalMPSBound{low: m.newValue(), upper: m.newValue(), fixed: m.newValue(), lowSet: true}
}
func (m *RationalMPSReader) copyValue(dst, src *numeral.Rational) error {
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	err = m.manager.Copy(dst, src)
	return m.calls.after(m.manager, b, err)
}
func (m *RationalMPSReader) setSmall(dst *numeral.Rational, n int32) error {
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	err = m.manager.SetInt32(dst, n)
	return m.calls.after(m.manager, b, err)
}
func (m *RationalMPSReader) parseValue(text string) (*numeral.Rational, error) {
	m.work = NewIndexedRationalVector(m.manager, 1)
	v := &m.work.Data[0]
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return nil, err
	}
	err = m.manager.SetString(v, text)
	if err = m.calls.after(m.manager, b, err); err != nil {
		return nil, err
	}
	return v, nil
}
func (m *RationalMPSReader) finishWork() error {
	if err := m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
func (m *RationalMPSReader) Close() error {
	if m.manager != nil {
		if err := m.calls.cleanupRaw(m.manager); err != nil {
			return err
		}
		if m.population != nil {
			if err := m.population.Close(); err != nil {
				return err
			}
			m.population = nil
		}
		if m.work != nil {
			if err := m.finishWork(); err != nil {
				return err
			}
		}
		for len(m.values) > 0 {
			last := len(m.values) - 1
			if err := m.manager.Del(m.values[last]); err != nil {
				return err
			}
			m.values = m.values[:last]
		}
		if err := m.calls.cleanupRoots(m.manager); err != nil {
			return err
		}
	}
	m.closed = true
	if m.file != nil {
		f := m.file
		m.file = nil
		m.input = nil
		return f.Close()
	}
	return nil
}
func (m *RationalMPSReader) expandRangeEndpoint(row, other *rationalMPSRow, delta *numeral.Rational, wasEqual bool) error {
	if !wasEqual {
		negative, err := m.manager.IsNeg(delta)
		if err != nil {
			return err
		}
		if negative {
			b, err := m.calls.begin(m.manager)
			if err != nil {
				return err
			}
			err = m.manager.Neg(delta)
			if err = m.calls.after(m.manager, b, err); err != nil {
				return err
			}
		}
	}
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	if !wasEqual && row.typ == mpsLess {
		err = m.manager.Sub(row.rhs, delta, &m.work.Data[1])
	} else {
		err = m.manager.Add(row.rhs, delta, &m.work.Data[1])
	}
	if err = m.calls.after(m.manager, b, err); err != nil {
		return err
	}
	if err = m.copyValue(other.rhs, &m.work.Data[1]); err != nil {
		return err
	}
	if err = m.finishWork(); err != nil {
		return err
	}
	for col, v := range row.coefficients.All() {
		// Source iterates by value: the lexical pair's coefficient copy
		// precedes its column assertion and destination-map publication.
		m.work = NewIndexedRationalVector(m.manager, 1)
		if err = m.copyValue(&m.work.Data[0], v); err != nil {
			return err
		}
		if _, ok := m.columns.Get2(col); sourceDebugEnabled && !ok {
			return ErrAssertion
		}
		dst := m.newValue()
		other.coefficients.Set(col, dst)
		if err = m.copyValue(dst, &m.work.Data[0]); err != nil {
			return err
		}
		if err = m.finishWork(); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalMPSReader) IsOK() bool { return m.ok }
func (m *RationalMPSReader) SetMessageStream(out io.Writer) error {
	if out == nil {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Messages = out
	return nil
}
func (m *RationalMPSReader) ColumnNames() []string {
	v := make([]string, 0, m.columns.Len())
	for name := range m.columns.All() {
		v = append(v, name)
	}
	return v
}
func (m *RationalMPSReader) message(format string, args ...any) error {
	if m.Messages == nil {
		return ErrBounds
	}
	_, err := fmt.Fprintf(m.Messages, format, args...)
	return err
}
func (m *RationalMPSReader) fail() error {
	if err := m.message("setting m_is_OK to false\n"); err != nil {
		return err
	}
	m.ok = false
	return nil
}
func (m *RationalMPSReader) readLine() error {
	for m.ok {
		if m.input == nil {
			return ErrBounds
		}
		line, err := m.input.ReadString('\n')
		m.line = strings.TrimSuffix(line, "\n")
		if err != nil && !(err == io.EOF && len(line) > 0) {
			m.LineNumber++
			if e := m.fail(); e != nil {
				return e
			}
			if e := m.message("cannot read from file\n"); e != nil {
				return e
			}
			if err != io.EOF {
				return err
			}
		}
		m.LineNumber++
		if m.line != "" && m.line[0] != '*' && strings.Trim(m.line, " \t") != "" {
			break
		}
	}
	return nil
}
func (m *RationalMPSReader) readName() error {
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "NAME") {
			m.line = strings.Trim(m.line[4:], " \t")
			m.Name = m.line
			return nil
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *RationalMPSReader) newRow(name string, index uint32, typ mpsRowType) *rationalMPSRow {
	return &rationalMPSRow{name: name, index: index, typ: typ, rhs: m.newValue(), span: m.newValue(), coefficients: insdict.NewDict[string, *numeral.Rational]()}
}
func (m *RationalMPSReader) addRow() error {
	if len(m.line) < 2 {
		return nil
	}
	m.line = strings.Trim(m.line, " \t")
	if len(m.line) == 0 {
		return ErrBounds
	}
	c := m.line[0]
	m.line = strings.Trim(m.line[1:], " \t")
	index := uint32(m.rows.Len()) - m.costLines
	var typ mpsRowType
	switch c {
	case 'E':
		typ = mpsEqual
	case 'L':
		typ = mpsLess
	case 'G':
		typ = mpsGreater
	case 'N':
		typ = mpsCost
	default:
		return nil
	}
	m.rows.Set(m.line, m.newRow(m.line, index, typ))
	if c == 'N' {
		m.costName = m.line
		m.costLines++
	}
	return nil
}
func (m *RationalMPSReader) readRows() error {
	if err := m.readLine(); err != nil {
		return err
	}
	// The source header-search loop does not read another line.
	if !strings.Contains(m.line, "ROWS") && m.ok {
		return ErrMPSStalled
	}
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "COLUMNS") {
			return nil
		}
		if err := m.addRow(); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}

func (m *RationalMPSReader) assignRow(row *rationalMPSRow, column, value string, field mpsRowField) error {
	n, err := m.parseValue(value)
	if err != nil {
		return err
	}
	var dst *numeral.Rational
	switch field {
	case mpsCoefficient:
		dst, _ = row.coefficients.Get2(column)
		if dst == nil {
			dst = m.newValue()
			row.coefficients.Set(column, dst)
		}
	case mpsRHS:
		dst = row.rhs
	case mpsRange:
		dst = row.span
	default:
		return ErrBounds
	}
	if err = m.copyValue(dst, n); err != nil {
		return err
	}
	if err = m.finishWork(); err != nil {
		return err
	}
	if field == mpsRange {
		return m.expandRange(row)
	}
	return nil
}
func (m *RationalMPSReader) cannotUnderstand() error {
	if err := m.fail(); err != nil {
		return err
	}
	if err := m.message("cannot understand this line\n"); err != nil {
		return err
	}
	return m.message("line = %s, line number is %d\n", m.line, m.LineNumber)
}
func (m *RationalMPSReader) shortLine(column bool) error {
	if column {
		if err := m.message("line is too short for a column\n"); err != nil {
			return err
		}
	} else if err := m.message("line is too short\n"); err != nil {
		return err
	}
	if err := m.message("%s\n", m.line); err != nil {
		return err
	}
	if err := m.message("line number is %d\n", m.LineNumber); err != nil {
		return err
	}
	return m.fail()
}

// Fixed pairs retain the source 8-byte name and 25-byte recursive stride.
func (m *RationalMPSReader) fixedRowPairs(data, column string, field mpsRowField) error {
	if field == mpsRange && len(m.line) < 14 {
		return m.shortLine(false)
	}
	for {
		if len(data) < 22 {
			return m.cannotUnderstand()
		}
		name := strings.Trim(data[:8], " \t")
		row, ok := m.rows.Get2(name)
		if !ok {
			if err := m.message("cannot find %s\n", name); err != nil {
				return err
			}
			return m.cannotUnderstand()
		}
		if err := m.assignRow(row, column, data[8:], field); err != nil {
			return err
		}
		if len(data) <= 24 {
			return nil
		}
		data = data[25:]
		if len(data) < 22 {
			return nil
		}
	}
}
func (m *RationalMPSReader) rowPairs(data, column string, start int, field mpsRowField) error {
	words := mpsWords(data)
	if len(words) == 0 {
		return ErrBounds
	} // unsigned tokens.size()-1 source underflow
	for i := start; i < len(words)-1; i += 2 {
		name := words[i]
		if field == mpsCoefficient && name == "'MARKER'" {
			return nil
		}
		row, ok := m.rows.Get2(name)
		if !ok {
			if field == mpsRange {
				if len(m.line) < 14 {
					return m.shortLine(false)
				}
				data = m.line[14:]
			}
			return m.fixedRowPairs(data, column, field)
		}
		if err := m.assignRow(row, column, words[i+1], field); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalMPSReader) readColumns() error {
	name := ""
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "RHS") {
			return nil
		}
		if len(m.line) < 22 {
			return m.shortLine(true)
		}
		if n := strings.Trim(m.line[4:12], " \t"); n != "" {
			name = n
		}
		if _, ok := m.columns.Get2(name); !ok {
			m.columns.Set(name, &rationalMPSColumn{name: name, index: uint32(m.columns.Len())})
		}
		if err := m.rowPairs(m.line[14:], name, 0, mpsCoefficient); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *RationalMPSReader) readRHS() error {
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "BOUNDS") || strings.HasPrefix(m.line, "ENDATA") || strings.HasPrefix(m.line, "RANGES") {
			return nil
		}
		if len(m.line) < 14 {
			return m.shortLine(false)
		}
		if err := m.rowPairs(m.line[14:], "", 0, mpsRHS); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *RationalMPSReader) expandRange(row *rationalMPSRow) error {
	index := uint32(m.rows.Len()) - m.costLines
	name := row.name + "_range"
	var typ mpsRowType
	wasEqual := row.typ == mpsEqual
	switch row.typ {
	case mpsGreater:
		typ = mpsLess
	case mpsLess:
		typ = mpsGreater
	case mpsEqual:
		positive, err := m.manager.IsPos(row.span)
		if err != nil {
			return err
		}
		if positive {
			row.typ = mpsGreater
			typ = mpsLess
		} else {
			row.typ = mpsLess
			typ = mpsGreater
		}
	default:
		if err := m.message("unexpected bound type %d at line %d\n", row.typ, m.LineNumber); err != nil {
			return err
		}
		if err := m.fail(); err != nil {
			return err
		}
		return ErrMPSUnhandled
	}
	other := m.newRow(name, index, typ)
	m.rows.Set(name, other) // source publishes before computing the endpoint.
	m.work = NewIndexedRationalVector(m.manager, 2)
	delta := row.span
	if !wasEqual {
		delta = &m.work.Data[0]
		if err := m.copyValue(delta, row.span); err != nil {
			return err
		}
	}
	// Equal rows use the signed span; inequality rows use its magnitude.
	return m.expandRangeEndpoint(row, other, delta, wasEqual)
}
func (m *RationalMPSReader) readRanges() error {
	if !strings.HasPrefix(m.line, "RANGES") {
		return nil
	}
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if len(mpsWords(m.line)) < 2 {
			return nil
		}
		if err := m.rowPairs(m.line, "", 1, mpsRange); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *RationalMPSReader) updateBound(b *rationalMPSBound, words []string) error {
	if len(m.line) < 1 {
		return ErrBounds
	}
	i := len(m.line)
	if k := strings.IndexByte(m.line[1:], ' '); k >= 0 {
		i = k + 1
	}
	// Keep the source's literal shift assertion, including its LIMITMAX failure.
	if sourceDebugEnabled && uint(len(m.line))>>uint(i) == 0 {
		return ErrAssertion
	}
	typ := m.line[1:i]
	if typ == "BV" {
		b.upperSet = true
		return m.setSmall(b.upper, 1)
	}
	switch typ {
	case "UP", "UI", "LIMITMAX", "LO", "LI", "FX":
		if len(words) <= 1 {
			return m.fail()
		}
		// Source publishes the matching flag before numeric conversion.
		switch typ {
		case "UP", "UI", "LIMITMAX":
			b.upperSet = true
		case "LO", "LI":
			b.lowSet = true
		case "FX":
			b.fixedSet = true
		}
		v, err := m.parseValue(words[1])
		if err != nil {
			return err
		}
		dst := b.fixed
		switch typ {
		case "UP", "UI", "LIMITMAX":
			dst = b.upper
		case "LO", "LI":
			dst = b.low
		}
		if err = m.copyValue(dst, v); err != nil {
			return err
		}
		return m.finishWork()
	case "FR":
		b.free = true
	case "PL":
		b.lowSet = true
		return m.setSmall(b.low, 0)
	case "MI":
		b.upperSet = true
		return m.setSmall(b.upper, 0) // source retains its default lower flag
	default:
		if err := m.message("unexpected bound type %s at line %d\n", typ, m.LineNumber); err != nil {
			return err
		}
		if err := m.fail(); err != nil {
			return err
		}
		return ErrMPSUnhandled
	}
	return nil
}
func (m *RationalMPSReader) createBound() error {
	if len(m.line) < 14 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	data := m.line[14:]
	words := mpsWords(data)
	if len(words) == 0 {
		if err := m.fail(); err != nil {
			return err
		}
		if err := m.message("error at line %d\n", m.LineNumber); err != nil {
			return err
		}
		return ErrMPSUnhandled
	}
	col, ok := m.columns.Get2(words[0])
	if !ok {
		if len(data) < 14 {
			return m.shortLine(false)
		}
		if len(data) < 22 {
			return m.cannotUnderstand()
		}
		name := strings.Trim(data[:8], " \t")
		col, ok = m.columns.Get2(name)
		if !ok {
			if err := m.message("cannot find %s\n", name); err != nil {
				return err
			}
			return m.cannotUnderstand()
		}
		words = []string{name, data[14:]}
	}
	if col.bound == nil {
		col.bound = m.newBound()
	}
	return m.updateBound(col.bound, words)
}
func (m *RationalMPSReader) readBounds() error {
	if !strings.HasPrefix(m.line, "BOUNDS") {
		return nil
	}
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if len(m.line) == 0 {
			return ErrBounds
		}
		if m.line[0] != ' ' {
			return nil
		}
		if err := m.createBound(); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *RationalMPSReader) Read() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.input == nil {
		return m.fail()
	}
	if err := m.readName(); err != nil {
		return err
	}
	if err := m.readRows(); err != nil {
		return err
	}
	if err := m.readColumns(); err != nil {
		return err
	}
	if err := m.readRHS(); err != nil {
		return err
	}
	if strings.HasPrefix(m.line, "BOUNDS") {
		if err := m.readBounds(); err != nil {
			return err
		}
		return m.readRanges()
	}
	if strings.HasPrefix(m.line, "RANGES") {
		if err := m.readRanges(); err != nil {
			return err
		}
		return m.readBounds()
	}
	return nil
}
func (m *RationalMPSReader) larRelation(typ mpsRowType) (LConstraintKind, error) {
	switch typ {
	case mpsLess:
		return LConstraintLE, nil
	case mpsGreater:
		return LConstraintGE, nil
	case mpsEqual:
		return LConstraintEQ, nil
	}
	if err := m.message("Unexpected rt %d\n", typ); err != nil {
		return 0, err
	}
	if err := m.fail(); err != nil {
		return 0, err
	}
	return 0, ErrMPSUnhandled
}

func (m *RationalMPSReader) getVarIndex(name string) uint32 {
	if j, ok := m.namesToVarIndex[name]; ok {
		return j
	}
	j := uint32(len(m.namesToVarIndex))
	m.namesToVarIndex[name] = j
	return j
}

// CreateLARSolver preserves source column-before-row population, including the
// early return from column filling on the first absent bound. General row and
// column constraints reach AddConstraint's source debug assertion/native no-op.
// Return the partial solver on failure; close it before closing this reader.
// On success no reader numerals remain borrowed by the solver.
func (m *RationalMPSReader) CreateLARSolver() (solver *LARSolver, err error) {
	if err = m.ready(); err != nil {
		return nil, err
	}
	defer func() { m.stopped = err != nil }()
	solver, err = NewLARSolver(m.manager)
	if err != nil {
		return solver, err
	}
	if err = m.fillLARColumns(solver); err != nil {
		return solver, err
	}
	return solver, m.fillLARRows(solver)
}

func (m *RationalMPSReader) fillLARColumns(s *LARSolver) error {
	for _, col := range m.columns.All() {
		if col == nil {
			return ErrBounds
		}
		if _, err := s.AddVar(col.index); err != nil {
			return err
		}
		b := col.bound
		if b == nil {
			return nil // source returns from the entire column pass.
		}
		if b.free {
			continue
		}
		for _, bound := range []struct {
			set   bool
			kind  LConstraintKind
			value *numeral.Rational
		}{{b.lowSet, LConstraintGE, b.low}, {b.upperSet, LConstraintLE, b.upper}, {b.fixedSet, LConstraintEQ, b.fixed}} {
			if !bound.set {
				continue
			}
			j, err := s.AddVar(col.index)
			if err != nil {
				return err
			}
			// numeric_traits<mpq>::one has an inline, nonallocating value.
			one := numeral.NewRational()
			one.Num = numeral.NewInt(1)
			if _, err = s.AddConstraint([]RationalSparseEntry{{Index: j, Value: one}}, bound.kind, bound.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *RationalMPSReader) fillLARRows(s *LARSolver) error {
	for _, row := range m.rows.All() {
		if row.name == m.costName {
			continue
		}
		kind, err := m.larRelation(row.typ)
		if err != nil {
			return err
		}
		m.population = NewSparseRationalVector(m.manager)
		for name, value := range row.coefficients.All() {
			m.work = NewIndexedRationalVector(m.manager, 1)
			if err := m.copyValue(&m.work.Data[0], value); err != nil {
				return err
			}
			j, err := s.AddVar(m.getVarIndex(name))
			if err != nil {
				return err
			}
			if err = m.appendPopulation(j, &m.work.Data[0]); err != nil {
				return err
			}
			if err = m.finishWork(); err != nil {
				return err
			}
		}
		if _, err := s.AddConstraint(m.population.Data, kind, row.rhs); err != nil {
			return err
		}
		if err := m.population.Close(); err != nil {
			return err
		}
		m.population = nil
	}
	return nil
}

// Source fill_lar_solver_on_row constructs a pair and copies it into ls after
// copying the by-value map iteration entry. That first copy is owned by work.
func (m *RationalMPSReader) appendPopulation(j uint32, value *numeral.Rational) error {
	v := m.population
	v.appendValues = NewIndexedRationalVector(m.manager, 2)
	w := v.appendValues
	if err := w.copySlot(&w.Data[0], value); err != nil {
		return err
	}
	if err := w.copySlot(&w.Data[1], &w.Data[0]); err != nil {
		return err
	}
	v.Data = append(v.Data, RationalSparseEntry{Index: j, Value: w.Data[1]})
	w.Data[1] = numeral.NewRational()
	if err := w.Close(); err != nil {
		return err
	}
	v.appendValues = nil
	return nil
}
