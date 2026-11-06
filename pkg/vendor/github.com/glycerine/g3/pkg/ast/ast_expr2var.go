package ast

// Source: src/ast/expr2var.cpp:27-92; src/ast/expr2var.h:32-77.

import (
	"errors"
	"io"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Expr2Var owns one AST reference for each mapped expression. The source
// variable type is unsigned, represented concretely by uint32.
type Expr2Var struct {
	self        *Expr2Var
	manager     *ASTManager
	mapping     map[ASTHandle]uint32
	recent      []ASTHandle
	scopeLimits []int
	interpreted bool
	closed      bool
}

// ExprVarBinding is one borrowed expression-to-variable entry. Entry order is
// unspecified, matching the source obj_map iterator.
type ExprVarBinding struct {
	Expr     ASTHandle
	Variable uint32
}

func NewExpr2Var(m *ASTManager) (*Expr2Var, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if err := m.lock(); err != nil {
		return nil, err
	}
	m.mu.Unlock()
	x := &Expr2Var{manager: m, mapping: make(map[ASTHandle]uint32)}
	x.self = x
	return x, nil
}

func (x *Expr2Var) open() error {
	if x == nil || x.self != x || x.closed || x.manager == nil {
		return ErrASTHandle
	}
	if x.manager.Closed {
		return arena.ErrClosed
	}
	return nil
}

// Manager returns the borrowed manager used by the mapping.
func (x *Expr2Var) Manager() (*ASTManager, error) {
	if err := x.open(); err != nil {
		return nil, err
	}
	return x.manager, nil
}

func (x *Expr2Var) isUninterpretedConstant(h ASTHandle) (bool, error) {
	m := x.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ASTApp {
		return false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	d, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(d)
	return family == NullFamilyID, err
}

func (x *Expr2Var) traceInsert(h ASTHandle) error {
	tags := x.manager.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("expr2var") {
		return nil
	}
	return tags.EmitConditional("expr2var", "insert", "src/ast/expr2var.cpp", 29,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "interpreted:\n"); err != nil {
				return err
			}
			g := uparams.BorrowProcessGlobalParamsService()
			if g == nil || g.Context == nil {
				return uparams.ErrGlobalParamsOwner
			}
			format, err := x.manager.SMT2PPExprTree(h, nil)
			if err != nil {
				return err
			}
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: x.manager.BudgetField})
			if err != nil {
				return err
			}
			defer out.Close()
			empty := g.Context.EmptyValue()
			if err = x.manager.PrintFormat(out, format, g.Context, &empty, g); err != nil {
				return err
			}
			body, err := out.String()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body+"\n")
			return err
		})
}

func (x *Expr2Var) Insert(h ASTHandle, variable uint32) error {
	if err := x.open(); err != nil {
		return err
	}
	if _, present := x.mapping[h]; present {
		return ErrASTDomain
	}
	uninterpreted, err := x.isUninterpretedConstant(h)
	if err != nil {
		return err
	}
	if !uninterpreted {
		if err = x.traceInsert(h); err != nil {
			return err
		}
		x.interpreted = true
	}
	if err = x.manager.IncRef(h); err != nil {
		return err
	}
	x.mapping[h] = variable
	x.recent = append(x.recent, h)
	return nil
}

func (x *Expr2Var) ToVar(h ASTHandle) (uint32, error) {
	if err := x.open(); err != nil {
		return math.MaxUint32, err
	}
	if _, err := x.manager.GetKind(h); err != nil {
		return math.MaxUint32, err
	}
	if variable, present := x.mapping[h]; present {
		return variable, nil
	}
	return math.MaxUint32, nil
}

func (x *Expr2Var) IsVar(h ASTHandle) (bool, error) {
	if err := x.open(); err != nil {
		return false, err
	}
	if _, err := x.manager.GetKind(h); err != nil {
		return false, err
	}
	_, present := x.mapping[h]
	return present, nil
}

func (x *Expr2Var) InterpretedVars() (bool, error) {
	if err := x.open(); err != nil {
		return false, err
	}
	return x.interpreted, nil
}

// Recent returns a borrowed expression snapshot in insertion order.
func (x *Expr2Var) Recent() ([]ASTHandle, error) {
	if err := x.open(); err != nil {
		return nil, err
	}
	return append([]ASTHandle(nil), x.recent...), nil
}

// Entries returns a borrowed snapshot of the source begin/end range.
func (x *Expr2Var) Entries() ([]ExprVarBinding, error) {
	if err := x.open(); err != nil {
		return nil, err
	}
	result := make([]ExprVarBinding, 0, len(x.mapping))
	for expression, variable := range x.mapping {
		result = append(result, ExprVarBinding{Expr: expression, Variable: variable})
	}
	return result, nil
}

func (x *Expr2Var) ResetRecent() error {
	if err := x.open(); err != nil {
		return err
	}
	if len(x.scopeLimits) != 0 {
		return ErrASTDomain
	}
	x.recent = nil
	return nil
}

func (x *Expr2Var) Push() error {
	if err := x.open(); err != nil {
		return err
	}
	x.scopeLimits = append(x.scopeLimits, len(x.recent))
	return nil
}

func (x *Expr2Var) Pop(scopes uint32) error {
	if err := x.open(); err != nil {
		return err
	}
	if scopes == 0 {
		return nil
	}
	if uint64(scopes) > uint64(len(x.scopeLimits)) {
		return ErrASTDomain
	}
	newScopeCount := len(x.scopeLimits) - int(scopes)
	start := x.scopeLimits[newScopeCount]
	for i := start; i < len(x.recent); i++ {
		h := x.recent[i]
		if _, present := x.mapping[h]; !present {
			continue
		}
		consumed, err := x.manager.DecRefConsumed(h)
		if consumed {
			delete(x.mapping, h)
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ErrASTPending
		}
	}
	x.recent = x.recent[:start]
	x.scopeLimits = x.scopeLimits[:newScopeCount]
	return nil
}

func (x *Expr2Var) reset() error {
	for h := range x.mapping {
		consumed, err := x.manager.DecRefConsumed(h)
		if consumed {
			delete(x.mapping, h)
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ErrASTPending
		}
	}
	x.recent = nil
	x.scopeLimits = nil
	x.interpreted = false
	return nil
}

func (x *Expr2Var) Reset() error {
	if err := x.open(); err != nil {
		return err
	}
	return x.reset()
}

// Inverse returns an owning sparse vector. Each nonzero ASTRef must be released
// by the caller with ASTManager.DelRef.
func (x *Expr2Var) Inverse() (result []ASTRef, err error) {
	if err = x.open(); err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		for i := range result {
			if result[i] != (ASTRef{}) {
				err = errors.Join(err, x.manager.DelRef(&result[i]))
			}
		}
		result = nil
	}()
	for h, variable := range x.mapping {
		if uint64(variable)+1 > uint64(len(result)) {
			result = append(result, make([]ASTRef, int(uint64(variable)+1)-len(result))...)
		}
		if result[variable] != (ASTRef{}) {
			if err = x.manager.DelRef(&result[variable]); err != nil {
				return result, err
			}
		}
		result[variable], err = x.manager.NewRef(h)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (x *Expr2Var) Display(w io.Writer) (result error) {
	if err := x.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	for h, variable := range x.mapping {
		format, err := x.manager.SMT2PPExprTree(h, nil)
		if err != nil {
			return err
		}
		g := uparams.BorrowProcessGlobalParamsService()
		if g == nil || g.Context == nil {
			return uparams.ErrGlobalParamsOwner
		}
		out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: x.manager.BudgetField})
		if err != nil {
			return err
		}
		empty := g.Context.EmptyValue()
		if err = x.manager.PrintFormat(out, format, g.Context, &empty, g); err != nil {
			return errors.Join(err, out.Close())
		}
		body, err := out.String()
		if err != nil {
			return errors.Join(err, out.Close())
		}
		if _, err = io.WriteString(w, body+" -> "); err == nil {
			_, err = io.WriteString(w, strconv.FormatUint(uint64(variable), 10)+"\n")
		}
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (x *Expr2Var) Close() error {
	if x == nil || x.self != x {
		return ErrASTHandle
	}
	if x.closed {
		return nil
	}
	if x.manager.Closed {
		x.mapping = nil
		x.recent = nil
		x.scopeLimits = nil
		x.closed = true
		return nil
	}
	if err := x.reset(); err != nil {
		return err
	}
	x.mapping = nil
	x.closed = true
	return nil
}
