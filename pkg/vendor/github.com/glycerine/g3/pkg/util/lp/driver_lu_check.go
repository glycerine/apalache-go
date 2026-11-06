package lp

// Source: src/test/lp.cpp:2568-2620,2639-2668,3126-3130.

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// LUCheckFileDriver owns the file and factorization of check_lu_from_file.
// It always solves entering column 26. A failed Run retains its completed
// prefix until Close; successful Run closes the file before factorization.
// Matrix, basis and solved column remain available as native Go values.
type LUCheckFileDriver struct {
	Name            string
	A               *FloatStaticMatrix
	Basis           []uint32
	Column, Work    *IndexedFloatVector
	LU              *FloatLU
	Settings        *Settings
	file            *os.File
	input           *bufio.Reader
	out             io.Writer
	started, closed bool
}

func NewLUCheckFileDriver(name string, out io.Writer) *LUCheckFileDriver {
	if out == nil {
		out = os.Stdout
	}
	return &LUCheckFileDriver{Name: name, out: out}
}

func (d *LUCheckFileDriver) line() (string, error) {
	if d.input == nil {
		return "", ErrBounds
	}
	line, err := d.input.ReadString('\n')
	if err == io.EOF && len(line) > 0 {
		err = nil
	}
	return strings.TrimSuffix(line, "\n"), err
}

// Strict signed32 parsing replaces source atoi's malformed/overflow domains;
// conversion to unsigned still preserves valid negative input modulo 2^32.
func luCheckIndex(text string) (uint32, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 32)
	return uint32(n), err
}

func (d *LUCheckFileDriver) dimension() (uint32, error) {
	line, err := d.line()
	if err != nil {
		return 0, err
	}
	words := mpsWords(line)
	if len(words) < 2 {
		return 0, ErrBounds
	}
	return luCheckIndex(words[1])
}

func (d *LUCheckFileDriver) readRows() error {
	for {
		line, err := d.line()
		if err != nil {
			return err
		}
		// The first line without substring "row" is consumed, not put back.
		if !strings.Contains(line, "row") {
			return nil
		}
		words := mpsWords(line)
		if len(words) == 0 {
			return ErrBounds
		}
		if words[0] != "row" {
			if err = writeDriverLine(d.out, "wrong row line"+line); err != nil {
				return err
			}
		}
		if len(words) < 2 {
			return ErrBounds
		}
		i, err := luCheckIndex(words[1])
		if err != nil {
			return err
		}
		if err = d.readRowColumns(i); err != nil {
			return err
		}
	}
}

func (d *LUCheckFileDriver) readRowColumns(i uint32) error {
	for {
		line, err := d.line()
		if err != nil {
			return err
		}
		if line == "row_end" {
			return nil
		}
		words := mpsWords(line)
		if len(words) != 4 {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if len(words) < 4 {
			return ErrBounds
		}
		j, err := luCheckIndex(words[1])
		if err != nil {
			return err
		}
		value, err := mpsNumber(words[3])
		if err != nil {
			return err
		}
		if err = d.A.Set(i, j, value); err != nil {
			return err
		}
	}
}

func (d *LUCheckFileDriver) readBasis() error {
	if err := writeDriverLine(d.out, "reading basis"); err != nil {
		return err
	}
	line, err := d.line()
	if err != nil {
		return err
	}
	if line != "basis_start" {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for {
		line, err = d.line()
		if err != nil {
			return err
		}
		if line == "basis_end" {
			return nil
		}
		j, err := luCheckIndex(line)
		if err != nil {
			return err
		}
		d.Basis = append(d.Basis, j)
	}
}

func (d *LUCheckFileDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	var err error
	d.file, err = os.Open(d.Name)
	if err != nil {
		if e := writeDriverLine(d.out, "cannot open file "+d.Name); e != nil {
			return e
		}
		// Source continues into invalid empty token accesses after failed open.
		return err
	}
	d.input = bufio.NewReader(d.file)
	if _, err = d.line(); err != nil {
		return err
	} // source discards the first line
	rows, err := d.dimension()
	if err != nil {
		return err
	}
	cols, err := d.dimension()
	if err != nil {
		return err
	}
	if err = writeDriverLine(d.out, "init matrix "+strconv.FormatUint(uint64(rows), 10)+" by "+strconv.FormatUint(uint64(cols), 10)); err != nil {
		return err
	}
	d.A = NewFloatStaticMatrix(rows, cols)
	if err = d.readRows(); err != nil {
		return err
	}
	if err = d.readBasis(); err != nil {
		return err
	}
	d.Work = NewIndexedFloatVector(rows)
	// read_indexed_vector is commented out in source: remaining bytes are ignored.
	if err = d.file.Close(); err != nil {
		return err
	}
	d.file = nil
	d.input = nil
	d.Settings, err = NewSettings()
	if err != nil {
		return err
	}
	d.LU, err = NewFloatLU(d.A, d.Basis, d.Settings)
	if err != nil {
		return err
	}
	d.Column = NewIndexedFloatVector(d.A.RowCount())
	if err = d.LU.SolveBd(26, d.Column, d.Work); err != nil {
		return err
	}
	if err = d.checkResidual(); err != nil {
		return err
	}
	return d.Close()
}

func (d *LUCheckFileDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.LU != nil {
		if err := d.LU.Close(); err != nil {
			return err
		}
		d.LU = nil
	}
	if d.file != nil {
		if err := d.file.Close(); err != nil {
			return err
		}
		d.file = nil
		d.input = nil
	}
	d.closed = true
	return nil
}
