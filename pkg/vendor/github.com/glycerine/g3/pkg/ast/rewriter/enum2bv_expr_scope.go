package rewriter

// Source: enum2bv_rewriter.cpp:54-81,132-148 concrete lexical expr_ref owners.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source null expr_ref locals acquire no reference slot until assignment.
// These concrete owners use NewRef on their first nonnull value and SetRef
// for subsequent INC/new, DEC/old, publication assignments.
func enum2BVAssign(m *ast.ASTManager, out *ast.ASTRef, value ast.ASTHandle) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	if *out == (ast.ASTRef{}) {
		if value.IsNull() {
			return nil
		}
		ref, err := m.NewRef(value)
		if err == nil {
			*out = ref
		}
		return err
	}
	return m.SetRef(*out, value)
}

type enum2BVAppScope struct {
	manager *ast.ASTManager
	a0, a1  ast.ASTRef
	args    []ast.ASTRef
	values  []ast.ASTHandle
}

func (s *enum2BVAppScope) close() error {
	for i := range s.args {
		if s.args[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.args[i]); err != nil {
				return err
			}
		}
	}
	if s.a1 != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.a1); err != nil {
			return err
		}
	}
	if s.a0 != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.a0); err != nil {
			return err
		}
	}
	return nil
}

type enum2BVScalarScope struct {
	manager *ast.ASTManager
	ref     ast.ASTRef
}

func (s *enum2BVScalarScope) close() error {
	if s.ref != (ast.ASTRef{}) {
		return s.manager.DelRef(&s.ref)
	}
	return nil
}

// The only scalar shapes are reduce_args' tmp and reduce_arg's f_def.
// Queue cleanup in the observed innermost-first unwind order.
type enum2BVPending struct {
	app        *enum2BVAppScope
	scalar     *enum2BVScalarScope
	quantifier *enum2BVQuantifierScope
	printer    *enum2BVErrorPrinter
}

func (r *Enum2BVRewriter) finishApp(s *enum2BVAppScope, result *error) {
	err := s.close()
	if err != nil {
		r.pending = append(r.pending, enum2BVPending{app: s})
		r.closing = true
	}
	*result = errors.Join(*result, err)
}
func (r *Enum2BVRewriter) finishScalar(s *enum2BVScalarScope, result *error) {
	err := s.close()
	if err != nil {
		r.pending = append(r.pending, enum2BVPending{scalar: s})
		r.closing = true
	}
	*result = errors.Join(*result, err)
}
func enum2BVAppAssertion(kind ast.ASTKind) error {
	return enum2BVSourceAssert(103, "is_app(a)", func() bool { return kind == ast.ASTApp })
}
func enum2BVSourceAssert(line int32, condition string, evaluate func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	if evaluate() {
		return nil
	}
	return diagnostic.DebugAssertionFailure(d, "src/ast/rewriter/enum2bv_rewriter.cpp", line, condition)
}

func (r *Enum2BVRewriter) reduceArguments(args ast.AppArguments, scope *enum2BVAppScope) (complete bool, err error) {
	tmp := &enum2BVScalarScope{manager: r.manager}
	defer r.finishScalar(tmp, &err)
	for i := uint32(0); i < args.Count(); i++ {
		arg, err := args.At(i)
		if err != nil {
			return false, err
		}
		yes, err := r.reduceArgument(arg, &tmp.ref)
		if err != nil || !yes {
			return false, err
		}
		value, err := r.manager.RefGet(tmp.ref)
		if err != nil {
			return false, err
		}
		ref, err := r.manager.NewRef(value)
		if err != nil {
			return false, err
		}
		scope.args = append(scope.args, ref)
		scope.values = append(scope.values, value)
	}
	return true, nil
}
