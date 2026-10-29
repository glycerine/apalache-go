package smtcore

// Source: src/smt/smt_model_generator.h model-value dependencies and
// procedures.

import "github.com/glycerine/g3/pkg/ast"

// SMTExtraFreshValue is a generator-owned placeholder for a fresh value of a
// sort. The sort is borrowed from the generator's pinned AST set.
type SMTExtraFreshValue struct {
	sort  ast.ASTHandle
	index uint32
	value ast.ASTHandle
}

func NewSMTExtraFreshValue(sort ast.ASTHandle, index uint32) (*SMTExtraFreshValue, error) {
	if sort.IsNull() {
		return nil, ast.ErrASTDomain
	}
	return &SMTExtraFreshValue{sort: sort, index: index}, nil
}

func (v *SMTExtraFreshValue) Sort() ast.ASTHandle {
	if v == nil {
		return ast.ASTHandle{}
	}
	return v.sort
}

func (v *SMTExtraFreshValue) Index() uint32 {
	if v == nil {
		return 0
	}
	return v.index
}

func (v *SMTExtraFreshValue) SetValue(value ast.ASTHandle) error {
	if v == nil || value.IsNull() || !v.value.IsNull() {
		return ast.ErrASTDomain
	}
	v.value = value
	return nil
}

func (v *SMTExtraFreshValue) Value() (ast.ASTHandle, error) {
	if v == nil || v.value.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return v.value, nil
}

// SMTModelValueDependency is the source tagged dependency union represented
// with explicit Go fields. Enode dependencies are canonicalized to their root.
type SMTModelValueDependency struct {
	fresh bool
	node  *SMTEnode
	value *SMTExtraFreshValue
}

func NewSMTEnodeValueDependency(node *SMTEnode) (SMTModelValueDependency, error) {
	if node == nil || node.root == nil {
		return SMTModelValueDependency{}, ast.ErrASTDomain
	}
	return SMTModelValueDependency{node: node.root}, nil
}

func NewSMTFreshValueDependency(value *SMTExtraFreshValue) (SMTModelValueDependency, error) {
	if value == nil {
		return SMTModelValueDependency{}, ast.ErrASTDomain
	}
	return SMTModelValueDependency{fresh: true, value: value}, nil
}

func (d SMTModelValueDependency) IsFreshValue() bool { return d.fresh }

func (d SMTModelValueDependency) Enode() (*SMTEnode, error) {
	if d.fresh || d.node == nil {
		return nil, ast.ErrASTDomain
	}
	return d.node, nil
}

func (d SMTModelValueDependency) FreshValue() (*SMTExtraFreshValue, error) {
	if !d.fresh || d.value == nil {
		return nil, ast.ErrASTDomain
	}
	return d.value, nil
}

// SMTModelValueProcedure builds one model value after its dependencies have
// been evaluated in the order returned by Dependencies.
type SMTModelValueProcedure interface {
	Dependencies() []SMTModelValueDependency
	MakeValue(*SMTModelGenerator, []ast.ASTHandle) (ast.ASTHandle, error)
	IsFresh() bool
}

// SMTModelInitializer is the independent source init_model hook. A theory may
// register a factory without opting into enode value construction.
type SMTModelInitializer interface {
	InitializeModel(*SMTModelGenerator) error
}

// SMTModelValueTheory is the retained model-generation surface implemented by
// theories that assign concrete values to their enode roots.
type SMTModelValueTheory interface {
	SMTModelInitializer
	ModelValue(*SMTEnode, *SMTModelGenerator) (SMTModelValueProcedure, error)
}

// SMTModelFunctionTheory is the optional source include_func_interp hook.
// The C++ theory base excludes its declarations unless a concrete theory opts in.
type SMTModelFunctionTheory interface {
	IncludeFunctionInterpretation(ast.ASTHandle) bool
}

// SMTModelFinalizer is the optional source finalize_model hook.
type SMTModelFinalizer interface {
	FinalizeModel(*SMTModelGenerator) error
}

type SMTExpressionValueProcedure struct{ value ast.ASTHandle }

func NewSMTExpressionValueProcedure(value ast.ASTHandle) (*SMTExpressionValueProcedure, error) {
	if value.IsNull() {
		return nil, ast.ErrASTDomain
	}
	return &SMTExpressionValueProcedure{value: value}, nil
}

func (*SMTExpressionValueProcedure) Dependencies() []SMTModelValueDependency { return nil }
func (p *SMTExpressionValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || generator == nil || p.value.IsNull() || len(values) != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return p.value, nil
}
func (*SMTExpressionValueProcedure) IsFresh() bool { return false }

type SMTFreshValueProcedure struct {
	dependency SMTModelValueDependency
}

func NewSMTFreshValueProcedure(value *SMTExtraFreshValue) (*SMTFreshValueProcedure, error) {
	dependency, err := NewSMTFreshValueDependency(value)
	if err != nil {
		return nil, err
	}
	return &SMTFreshValueProcedure{dependency: dependency}, nil
}

func (p *SMTFreshValueProcedure) Dependencies() []SMTModelValueDependency {
	if p == nil {
		return nil
	}
	return []SMTModelValueDependency{p.dependency}
}
func (p *SMTFreshValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || generator == nil || len(values) != 1 || values[0].IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return values[0], nil
}
func (*SMTFreshValueProcedure) IsFresh() bool { return true }
