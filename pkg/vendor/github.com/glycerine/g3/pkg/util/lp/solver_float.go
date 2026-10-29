package lp

// Source: src/util/lp/lp_solver.h and lp_solver.hpp, floating common LP layer.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/util/z3exception"
	"github.com/glycerine/insdict"
)

type LPRelation uint32

const (
	RelationLessOrEqual LPRelation = iota
	RelationEqual
	RelationGreaterOrEqual
)

type FloatConstraint struct {
	RightSide float64
	Relation  LPRelation
}

var ErrLPFixedColumn = z3exception.NewDefaultException("found fixed column")
var ErrLPSolverMissingRow = z3exception.NewDefaultException("cannot find row")

// FloatLPSolver implements the common floating data preparation layer. Primal
// and dual optimization are connected by their concrete simplex types; exact/LAR variants remain partial. Ordered row
// dictionaries choose insertion order for source unordered-map traversals.
type FloatLPSolver struct {
	A                                                             *FloatStaticMatrix
	B, ColumnScale, Costs                                         []float64
	X, UpperBounds                                                []float64
	Basis, NBasis                                                 []uint32
	Heading                                                       []int32
	ColumnTypes                                                   []ColumnType
	AValues                                                       *insdict.Dict[uint32, *insdict.Dict[uint32, float64]]
	Constraints                                                   map[uint32]FloatConstraint
	Columns                                                       map[uint32]*FloatColumnInfo
	NamesToColumns                                                map[string]uint32
	ExternalRowsToCore, CoreRowsToExternal, CoreColumnsToExternal map[uint32]uint32
	Artificials, Slacks                                           uint32
	Status                                                        Status
	TotalIterations                                               uint32
	Settings                                                      *Settings
}

func NewFloatLPSolver() (*FloatLPSolver, error) {
	s, err := NewSettings()
	if err != nil {
		return nil, err
	}
	return &FloatLPSolver{Settings: s, AValues: insdict.NewDict[uint32, *insdict.Dict[uint32, float64]](), Constraints: make(map[uint32]FloatConstraint), Columns: make(map[uint32]*FloatColumnInfo), NamesToColumns: make(map[string]uint32), ExternalRowsToCore: make(map[uint32]uint32), CoreRowsToExternal: make(map[uint32]uint32), CoreColumnsToExternal: make(map[uint32]uint32)}, nil
}
func (m *FloatLPSolver) GetOrCreateColumnInfo(column uint32) *FloatColumnInfo {
	if c, ok := m.Columns[column]; ok {
		return c
	}
	c := NewFloatColumnInfo(InvalidColumnIndex)
	m.Columns[column] = c
	return c
}
func (m *FloatLPSolver) SetRowColumnCoefficient(row, column uint32, v float64) {
	r, ok := m.AValues.Get2(row)
	if !ok {
		r = insdict.NewDict[uint32, float64]()
		m.AValues.Set(row, r)
	}
	r.Set(column, v)
}
func (m *FloatLPSolver) AddConstraint(relation LPRelation, rightSide float64, row uint32) error {
	if _, ok := m.Constraints[row]; ok {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Constraints[row] = FloatConstraint{rightSide, relation}
	return nil
}
func (m *FloatLPSolver) GiveSymbolicNameToColumn(name string, column uint32) {
	m.GetOrCreateColumnInfo(column).Name = name
	m.NamesToColumns[name] = column
}
func (m *FloatLPSolver) ColumnName(core uint32) string {
	ext, ok := m.CoreColumnsToExternal[core]
	if !ok {
		return "x" + strconv.FormatUint(uint64(core), 10)
	}
	c, ok := m.Columns[ext]
	if !ok {
		return "x" + strconv.FormatUint(uint64(ext), 10)
	}
	return c.Name
}
func (m *FloatLPSolver) ProblemIsEmpty() bool {
	for _, row := range m.AValues.All() {
		if row.Len() != 0 {
			return false
		}
	}
	return true
}
func (m *FloatLPSolver) MapExternalRowsToCoreSolverRows() {
	var next uint32
	for row := range m.AValues.All() {
		m.ExternalRowsToCore[row] = next
		m.CoreRowsToExternal[next] = row
		next++
	}
}

// Undefined source map/pointer domains become an explicit Go guard after the
// source debug assertion. Valid inputs have identical publication order.
func lpSolverDomain(ok bool) error {
	if ok {
		return nil
	}
	if err := sourceAssertionFailure(); err != nil {
		return err
	}
	return ErrBounds
}
func (m *FloatLPSolver) MapExternalColumnsToCoreSolverColumns() error {
	var next uint32
	for _, row := range m.AValues.All() {
		for j, v := range row.All() {
			if v == 0 {
				return ErrLPFixedColumn
			}
			c := m.Columns[j]
			if c == nil {
				m.Columns[j] = nil
			}
			if c == nil {
				return ErrBounds
			}
			if c.Fixed {
				return ErrLPFixedColumn
			}
			if !IsValidColumnIndex(c.Index) {
				c.Index = next
				m.CoreColumnsToExternal[next] = j
				next++
			}
		}
	}
	return nil
}
func (m *FloatLPSolver) FillAFromAValues() error {
	m.A = NewFloatStaticMatrix(uint32(m.AValues.Len()), uint32(len(m.CoreColumnsToExternal)))
	for external, row := range m.AValues.All() {
		i, ok := m.ExternalRowsToCore[external]
		if err := lpSolverDomain(ok); err != nil {
			return err
		}
		for j, v := range row.All() {
			c := m.Columns[j]
			if err := lpSolverDomain(c != nil); err != nil {
				return err
			}
			if err := lpSolverDomain(IsValidColumnIndex(c.Index)); err != nil {
				return err
			}
			if c.IsFlipped() {
				v = -v
			}
			if err := m.A.Set(i, c.Index, v); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *FloatLPSolver) FillMatrixAAndInitRightSide() error {
	m.MapExternalRowsToCoreSolverRows()
	if err := m.MapExternalColumnsToCoreSolverColumns(); err != nil {
		return err
	}
	if m.A != nil {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err := m.FillAFromAValues(); err != nil {
		return err
	}
	n := int(m.A.RowCount())
	if len(m.B) > n {
		m.B = m.B[:n]
	}
	for len(m.B) < n {
		m.B = append(m.B, 0)
	}
	return nil
}
func (m *FloatLPSolver) LowBoundShiftForRow(row uint32) (float64, error) {
	r, ok := m.AValues.Get2(row)
	if !ok {
		return 0, ErrLPSolverMissingRow
	}
	var sum float64
	for j, v := range r.All() {
		c := m.Columns[j]
		if c == nil {
			m.Columns[j] = nil
		}
		if c == nil {
			return sum, ErrBounds
		}
		sum += v * c.Shift()
	}
	return sum, nil
}
func (m *FloatLPSolver) FillB() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		external := m.CoreRowsToExternal[i] // source operator[] inserts a zero on miss
		if _, ok := m.CoreRowsToExternal[i]; !ok {
			m.CoreRowsToExternal[i] = 0
		}
		c, ok := m.Constraints[external]
		if err := lpSolverDomain(ok); err != nil {
			return err
		}
		shift, err := m.LowBoundShiftForRow(external)
		if err != nil {
			return err
		}
		if uint64(i) >= uint64(len(m.B)) {
			return ErrBounds
		}
		m.B[i] = c.RightSide - shift
	}
	return nil
}
func (m *FloatLPSolver) RestoreColumnScalesToOne() {
	for i := range m.ColumnScale {
		m.ColumnScale[i] = 1
	}
}
func (m *FloatLPSolver) Unscale() error {
	if m.A != nil {
		m.A.Clear()
	}
	m.A = nil
	if err := m.FillAFromAValues(); err != nil {
		return err
	}
	m.RestoreColumnScalesToOne()
	return m.FillB()
}
func (m *FloatLPSolver) Scale() error {
	if !m.Settings.UseScaling {
		m.ColumnScale = nil
		for i := uint32(0); i < m.A.ColumnCount(); i++ {
			m.ColumnScale = append(m.ColumnScale, 1)
		}
		return nil
	}
	// Source snapshots thresholds in locals while the scaler borrows them.
	min, max := m.Settings.ScalingMinimum, m.Settings.ScalingMaximum
	s, err := NewFloatScaler(&m.B, m.A, &min, &max, &m.ColumnScale, m.Settings)
	if err != nil {
		return err
	}
	ok, err := s.Scale()
	if err != nil {
		return err
	}
	if !ok {
		return m.Unscale()
	}
	return nil
}
func (m *FloatLPSolver) CountSlacksAndArtificialsForRow(i uint32) error {
	external, exists := m.CoreRowsToExternal[i]
	if !exists {
		m.CoreRowsToExternal[i] = 0
	}
	c, ok := m.Constraints[external]
	if err := lpSolverDomain(ok); err != nil {
		return err
	}
	switch c.Relation {
	case RelationEqual:
		m.Artificials++
	case RelationGreaterOrEqual:
		m.Slacks++
		if uint64(i) >= uint64(len(m.B)) {
			return ErrBounds
		}
		if m.B[i] > 0 {
			m.Artificials++
		}
	case RelationLessOrEqual:
		m.Slacks++
		if uint64(i) >= uint64(len(m.B)) {
			return ErrBounds
		}
		if m.B[i] < 0 {
			m.Artificials++
		}
	}
	return nil
}
func (m *FloatLPSolver) CountSlacksAndArtificials() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		if err := m.CountSlacksAndArtificialsForRow(i); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatLPSolver) SetScaledCost(j uint32) error {
	if err := lpSolverDomain(uint64(j) < uint64(len(m.ColumnScale))); err != nil {
		return err
	}
	external, ok := m.CoreColumnsToExternal[j]
	if !ok {
		m.CoreColumnsToExternal[j] = 0
	}
	c := m.Columns[external]
	if c == nil {
		m.Columns[external] = nil
	}
	if c == nil {
		return ErrBounds
	}
	cost := c.Cost
	if c.IsFlipped() {
		cost = -cost
	}
	if err := lpSolverDomain(!c.Fixed); err != nil {
		return err
	}
	if uint64(j) >= uint64(len(m.Costs)) {
		return ErrBounds
	}
	m.Costs[j] = cost * m.ColumnScale[j]
	return nil
}

// ColumnValueFromCore specializes get_column_value_with_core_solver's actual
// get_var_value(j) accessor, which reads the concrete floating core's X slice.
func (m *FloatLPSolver) ColumnValueFromCore(column uint32, coreX []float64) (float64, error) {
	c, ok := m.Columns[column]
	if !ok {
		return 0, nil
	}
	if c.Fixed {
		return c.GetFixedValue()
	}
	if c.Index == InvalidColumnIndex {
		return 0, nil
	}
	j := c.Index
	if uint64(j) >= uint64(len(coreX)) || uint64(j) >= uint64(len(m.ColumnScale)) {
		return 0, ErrBounds
	}
	v := coreX[j] * m.ColumnScale[j]
	if c.IsFree() {
		return v, nil
	}
	if !c.IsFlipped() {
		return v + c.Low, nil
	}
	return -v + c.Upper, nil
}
