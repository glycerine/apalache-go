package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp:264-333,450-551.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func (s *Solver) internalizeVariable(v ast.ASTHandle, variables *[]sat.BoolVar) (complete bool, err error) {
	start := len(*variables)
	bitvector := false
	defer func() {
		if err == nil {
			err = s.emitVariable(v, (*variables)[start:], complete, bitvector)
		}
	}()
	kind, err := s.manager.GetKind(v)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := s.manager.GetNumArgs(v)
	if err != nil || n != 0 {
		return false, err
	}
	family, err := s.manager.GetFamilyID(v)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	sort, err := s.manager.GetSort(v)
	if err != nil {
		return false, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	if sort == boolean {
		index, e := s.encoder.atoms.ToVar(v)
		if e != nil {
			return false, e
		}
		if index == ^uint32(0) {
			return false, nil
		}
		*variables = append(*variables, sat.BoolVar(index))
		return true, nil
	}
	bindings, err := s.encoder.BitVectorBindings()
	if err != nil {
		return false, err
	}
	decl, err := s.manager.GetDecl(v)
	if err != nil {
		return false, err
	}
	bits, found := bindings[decl]
	if !found {
		return false, nil
	}
	count, err := s.manager.GetNumArgs(bits)
	if err != nil {
		return false, err
	}
	complete = true
	bitvector = true
	for i := uint32(0); i < count; i++ {
		bit, e := s.manager.GetArg(bits, i)
		if e != nil {
			return false, e
		}
		index, e := s.encoder.atoms.ToVar(bit)
		if e != nil {
			return false, e
		}
		if index == ^uint32(0) {
			complete = false
		} else {
			*variables = append(*variables, sat.BoolVar(index))
		}
	}
	return complete, nil
}

func (s *Solver) fixedVariable(variable ast.ASTHandle, byVariable map[sat.BoolVar]int, rows [][]sat.Literal, dependencies map[uint32]ast.ASTHandle) (ast.ASTHandle, bool, error) {
	var bits []sat.BoolVar
	complete, err := s.internalizeVariable(variable, &bits)
	if err != nil || !complete {
		return ast.ASTHandle{}, false, err
	}
	var values []sat.Literal
	var premises []uint32
	seen := make(map[uint32]bool)
	for _, bit := range bits {
		index, found := byVariable[bit]
		if !found {
			if e := s.emit("extract_fixed_variable", 510, func(out io.Writer) error {
				if _, e := io.WriteString(out, "variable is not bound "); e != nil {
					return e
				}
				if e := s.writeExpression(out, variable); e != nil {
					return e
				}
				_, e := io.WriteString(out, "\n")
				return e
			}); e != nil {
				return ast.ASTHandle{}, false, e
			}
			return ast.ASTHandle{}, false, nil
		}
		row := rows[index]
		if len(row) == 0 {
			return ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		values = append(values, row[0])
		for _, literal := range row[1:] {
			i := literal.Index()
			if !seen[i] {
				seen[i] = true
				premises = append(premises, i)
			}
		}
	}
	sort, err := s.manager.GetSort(variable)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	var value ast.ASTHandle
	if sort == boolean {
		if len(values) != 1 {
			return ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		value = variable
		if values[0].Sign() {
			value, err = s.manager.MkNot(variable)
		}
	} else {
		var integer big.Int
		for i, literal := range values {
			if !literal.Sign() {
				integer.SetBit(&integer, i, 1)
			}
		}
		var numeral ast.ASTHandle
		numeral, err = s.manager.BitVectorNumeralRat(new(big.Rat).SetInt(&integer), uint32(len(values)))
		if err == nil {
			value, err = s.manager.MkEq(variable, numeral)
		}
	}
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	// sat::literal_set::pop removes the most recently inserted distinct literal.
	var conjunction []ast.ASTHandle
	for i := len(premises) - 1; i >= 0; i-- {
		f, found := dependencies[premises[i]]
		if !found {
			return ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		conjunction = append(conjunction, f)
	}
	antecedent, err := s.manager.MkAndArray(conjunction)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := s.manager.MkImplies(antecedent, value)
	return result, err == nil, err
}

// GetConsequences returns expressions borrowed until the next consequence
// call or solver destruction. It preserves the specialized SAT/BV algorithm,
// including its omission of absent or only partly fixed bit-vector variables.
func (s *Solver) GetConsequences(asms, variables []ast.ASTHandle) (status lbool.LBool, result []ast.ASTHandle, err error) {
	if err = s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	defer func() {
		s.retainFailure(err)
		err = errors.Join(err, s.drainGoals())
		if err != nil {
			canceled, e := s.manager.Canceled()
			if e == nil && canceled {
				s.unknown = messages.Canceled
				status = lbool.Undef
				err = nil
			} else {
				s.unknown = err.Error()
			}
		}
	}()
	if err = releaseRoots(s.manager, &s.consequences, 0); err != nil {
		return lbool.Undef, nil, err
	}
	if err = s.encoder.initPreprocess(); err != nil {
		return lbool.Undef, nil, err
	}
	if err = s.emit("get_consequences_core", 266, func(out io.Writer) error {
		for _, values := range [][]ast.ASTHandle{asms, variables} {
			text, e := s.formatExpressions(values)
			if e != nil {
				return e
			}
			if _, e = io.WriteString(out, text+"\n"); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		return lbool.Undef, nil, err
	}
	if err = s.core.PopToBaseLevel(); err != nil {
		return lbool.Undef, nil, err
	}
	status, err = s.internalizeFormulas()
	if err != nil || status != lbool.True {
		return status, nil, err
	}
	var bvars []sat.BoolVar
	for _, v := range variables {
		if _, err = s.internalizeVariable(v, &bvars); err != nil {
			return lbool.Undef, nil, err
		}
	}
	status, deps, err := s.internalizeAssumptions(asms)
	if err != nil || status != lbool.True {
		return status, nil, err
	}
	dimacs, err := s.prepareDIMACS()
	if err != nil {
		return lbool.Undef, nil, err
	}
	status, rows, err := s.core.GetConsequences(s.assumptions, bvars)
	err = errors.Join(err, s.finishDIMACS(dimacs, false))
	if err != nil {
		return status, nil, err
	}
	if status == lbool.False {
		if len(s.assumptions) > 0 {
			err = s.extractCore(deps, nil)
		}
		return status, nil, err
	}
	byVariable := make(map[sat.BoolVar]int)
	for i, row := range rows {
		if e := s.emit("get_consequences_core", 291, func(out io.Writer) error {
			for j, l := range row {
				if j > 0 {
					if _, e := io.WriteString(out, " "); e != nil {
						return e
					}
				}
				if _, e := fmt.Fprint(out, l.String()); e != nil {
					return e
				}
			}
			_, e := io.WriteString(out, "\n")
			return e
		}); e != nil {
			return status, nil, e
		}
		if len(row) == 0 {
			return status, nil, ast.ErrASTDomain
		}
		byVariable[row[0].Variable()] = i
	}
	inverse := make(map[uint32]ast.ASTHandle)
	for f, l := range deps {
		inverse[l.Index()] = f
	}
	for _, v := range variables {
		consequence, found, e := s.fixedVariable(v, byVariable, rows, inverse)
		if e != nil {
			return status, result, e
		}
		if found {
			if e = s.hold(&s.consequences, consequence); e != nil {
				return status, result, e
			}
			result = append(result, consequence)
		}
	}
	return status, result, nil
}

// FindMutexes queries only literals already in the persistent atom map. Source
// intentionally neither compiles pending assertions nor propagates SAT status.
func (s *Solver) FindMutexes(variables []ast.ASTHandle) (status lbool.LBool, result [][]ast.ASTHandle, err error) {
	if err = s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { s.retainFailure(err) }()
	var literals []sat.Literal
	inverse := make(map[uint32]ast.ASTHandle)
	for _, v := range variables {
		negative, inner, e := s.manager.IsNot(v)
		if e != nil {
			return lbool.Undef, nil, e
		}
		if !negative {
			inner = v
		}
		index, e := s.encoder.atoms.ToVar(inner)
		if e != nil {
			return lbool.Undef, nil, e
		}
		if index == ^uint32(0) {
			continue
		}
		literal := sat.MakeLiteral(sat.BoolVar(index), negative)
		literals = append(literals, literal)
		inverse[literal.Index()] = v
	}
	_, rows, err := s.core.FindMutexes(literals)
	if err != nil {
		return lbool.Undef, nil, err
	}
	for _, row := range rows {
		var expressions []ast.ASTHandle
		for _, l := range row {
			v, found := inverse[l.Index()]
			if !found {
				return lbool.Undef, result, ast.ErrASTDomain
			}
			expressions = append(expressions, v)
		}
		result = append(result, expressions)
	}
	return lbool.True, result, nil
}
