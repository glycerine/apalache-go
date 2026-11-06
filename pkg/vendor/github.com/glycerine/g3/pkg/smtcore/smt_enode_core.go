package smtcore

// Source: src/smt/smt_enode.h and src/smt/smt_enode.cpp. This file contains
// the context-independent enode representation and algorithms.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/approx"
)

type SMTTransJustification struct {
	Target        *SMTEnode
	Justification SMTEqualityJustification
}

type SMTEnodeOptions struct {
	Generation           uint32
	SuppressArguments    bool
	MergeTruthValue      bool
	InternalScopeLevel   uint32
	CongruenceEnabled    bool
	UpdateChildrenParent bool
}

// SMTEnode is the concrete congruence-closure node. Owner and argument nodes
// are borrowed from the enclosing SMT context.
type SMTEnode struct {
	manager                               *ast.ASTManager
	owner, declaration                    ast.ASTHandle
	root, next, congruence                *SMTEnode
	classSize, generation, functionDeclID uint32
	internalScopeLevel                    uint32
	mark, mark2, interpreted              bool
	suppressArguments, equality           bool
	commutative, boolean, mergeTruthValue bool
	congruenceEnabled                     bool
	parents, arguments                    []*SMTEnode
	theoryVariables                       SMTTheoryVarList
	trans                                 SMTTransJustification
	labelHash                             int8
	labels, propagatedLabels              approx.UintSet
}

func NewSMTEnode(m *ast.ASTManager, owner ast.ASTHandle, byApp map[ast.ASTHandle]*SMTEnode, options SMTEnodeOptions) (*SMTEnode, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	kind, err := m.GetKind(owner)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTApp {
		return nil, ast.ErrASTDomain
	}
	declaration, err := m.GetDecl(owner)
	if err != nil {
		return nil, err
	}
	sort, err := m.GetSort(owner)
	if err != nil {
		return nil, err
	}
	booleanSort, err := m.MkBoolSort()
	if err != nil {
		return nil, err
	}
	boolean := sort == booleanSort
	if options.MergeTruthValue && !boolean {
		return nil, ast.ErrASTDomain
	}
	family, err := m.GetFamilyID(declaration)
	if err != nil {
		return nil, err
	}
	op, err := m.GetDeclKind(declaration)
	if err != nil {
		return nil, err
	}
	flags, err := m.GetFuncDeclFlags(declaration)
	if err != nil {
		return nil, err
	}
	n := &SMTEnode{
		manager: m, owner: owner, declaration: declaration,
		classSize: 1, generation: options.Generation, functionDeclID: math.MaxUint32,
		internalScopeLevel: options.InternalScopeLevel, suppressArguments: options.SuppressArguments,
		equality: family == m.BasicFamilyID() && op == ast.ASTOpEq,
		boolean:  boolean, mergeTruthValue: options.MergeTruthValue,
		congruenceEnabled: options.CongruenceEnabled, labelHash: -1,
		theoryVariables: *NewDefaultSMTTheoryVarList(),
		trans:           SMTTransJustification{Justification: NullSMTEqualityJustification()},
	}
	n.root, n.next = n, n
	if !options.SuppressArguments {
		count, err := m.GetNumArgs(owner)
		if err != nil {
			return nil, err
		}
		n.arguments = make([]*SMTEnode, count)
		for i := uint32(0); i < count; i++ {
			argument, err := m.GetArg(owner, i)
			if err != nil {
				return nil, err
			}
			child := byApp[argument]
			if child == nil || child.root == nil {
				return nil, ast.ErrASTDomain
			}
			n.arguments[i] = child
			if options.UpdateChildrenParent {
				child.root.parents = append(child.root.parents, n)
			}
		}
	}
	n.commutative = len(n.arguments) == 2 && flags.Commutative
	return n, nil
}

func (n *SMTEnode) valid() error {
	if n == nil || n.manager == nil || n.root == nil || n.next == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (n *SMTEnode) Owner() ast.ASTHandle              { return n.owner }
func (n *SMTEnode) Declaration() ast.ASTHandle        { return n.declaration }
func (n *SMTEnode) Root() *SMTEnode                   { return n.root }
func (n *SMTEnode) Next() *SMTEnode                   { return n.next }
func (n *SMTEnode) ClassSize() uint32                 { return n.classSize }
func (n *SMTEnode) Generation() uint32                { return n.generation }
func (n *SMTEnode) InternalScopeLevel() uint32        { return n.internalScopeLevel }
func (n *SMTEnode) FunctionDeclID() uint32            { return n.functionDeclID }
func (n *SMTEnode) SetFunctionDeclID(id uint32)       { n.functionDeclID = id }
func (n *SMTEnode) IsBool() bool                      { return n.boolean }
func (n *SMTEnode) IsEquality() bool                  { return n.equality }
func (n *SMTEnode) IsMarked() bool                    { return n.mark }
func (n *SMTEnode) IsMarked2() bool                   { return n.mark2 }
func (n *SMTEnode) IsInterpreted() bool               { return n.interpreted }
func (n *SMTEnode) IsCongruenceRoot() bool            { return n.congruence == n }
func (n *SMTEnode) Congruence() *SMTEnode             { return n.congruence }
func (n *SMTEnode) CongruenceEnabled() bool           { return n.congruenceEnabled }
func (n *SMTEnode) IsCommutative() bool               { return n.commutative }
func (n *SMTEnode) MergeTruthValue() bool             { return n.mergeTruthValue }
func (n *SMTEnode) NumArgs() uint32                   { return uint32(len(n.arguments)) }
func (n *SMTEnode) NumParents() uint32                { return uint32(len(n.parents)) }
func (n *SMTEnode) Parents() []*SMTEnode              { return append([]*SMTEnode(nil), n.parents...) }
func (n *SMTEnode) Arguments() []*SMTEnode            { return append([]*SMTEnode(nil), n.arguments...) }
func (n *SMTEnode) Labels() *approx.UintSet           { return &n.labels }
func (n *SMTEnode) PropagatedLabels() *approx.UintSet { return &n.propagatedLabels }

func (n *SMTEnode) Arg(index uint32) (*SMTEnode, error) {
	if err := n.valid(); err != nil {
		return nil, err
	}
	if index >= uint32(len(n.arguments)) {
		return nil, ast.ErrASTDomain
	}
	return n.arguments[index], nil
}

func (n *SMTEnode) OwnerID() (uint32, error)       { return n.manager.GetASTID(n.owner) }
func (n *SMTEnode) DeclarationID() (uint32, error) { return n.manager.GetASTID(n.declaration) }
func (n *SMTEnode) Hash() (uint32, error)          { return n.manager.GetHash(n.owner) }

func (n *SMTEnode) IsTrueEquality() bool {
	return n.equality && len(n.arguments) == 2 && n.arguments[0].root == n.arguments[1].root
}

func (n *SMTEnode) SetMark(value bool) error {
	if err := n.valid(); err != nil || n.mark == value {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	n.mark = value
	return nil
}
func (n *SMTEnode) SetMark2(value bool) error {
	if err := n.valid(); err != nil || n.mark2 == value {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	n.mark2 = value
	return nil
}

func (n *SMTEnode) MarkAsInterpreted() error {
	if err := n.valid(); err != nil {
		return err
	}
	if n.interpreted || len(n.arguments) != 0 || n.classSize != 1 {
		return ast.ErrASTDomain
	}
	n.interpreted = true
	return nil
}

func (n *SMTEnode) NumTheoryVars() uint32 {
	var count uint32
	if n.theoryVariables.variable == SMTNullTheoryVar {
		return 0
	}
	for current := &n.theoryVariables; current != nil; current = current.next {
		count++
	}
	return count
}

func (n *SMTEnode) TheoryVar(theory SMTTheoryID) SMTTheoryVar {
	if n.theoryVariables.variable == SMTNullTheoryVar {
		return SMTNullTheoryVar
	}
	for current := &n.theoryVariables; current != nil; current = current.next {
		if current.theory == theory {
			return current.variable
		}
	}
	return SMTNullTheoryVar
}

func (n *SMTEnode) AddTheoryVar(theory SMTTheoryID, variable SMTTheoryVar) error {
	if err := n.valid(); err != nil {
		return err
	}
	if theory == SMTNullTheoryID || variable == SMTNullTheoryVar || n.TheoryVar(theory) != SMTNullTheoryVar {
		return ast.ErrASTDomain
	}
	if n.theoryVariables.variable == SMTNullTheoryVar {
		n.theoryVariables = SMTTheoryVarList{theory: theory, variable: variable}
		return nil
	}
	current := &n.theoryVariables
	for current.next != nil {
		current = current.next
	}
	current.next = NewSMTTheoryVarList(theory, variable, nil)
	return nil
}

func (n *SMTEnode) ReplaceTheoryVar(theory SMTTheoryID, variable SMTTheoryVar) error {
	if variable == SMTNullTheoryVar {
		return ast.ErrASTDomain
	}
	for current := &n.theoryVariables; current != nil && current.variable != SMTNullTheoryVar; current = current.next {
		if current.theory == theory {
			current.variable = variable
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (n *SMTEnode) DeleteTheoryVar(theory SMTTheoryID) error {
	if n.theoryVariables.variable == SMTNullTheoryVar {
		return ast.ErrASTDomain
	}
	if n.theoryVariables.theory == theory {
		if n.theoryVariables.next == nil {
			n.theoryVariables = *NewDefaultSMTTheoryVarList()
		} else {
			n.theoryVariables = *n.theoryVariables.next
		}
		return nil
	}
	previous := &n.theoryVariables
	for current := previous.next; current != nil; current = current.next {
		if current.theory == theory {
			previous.next = current.next
			return nil
		}
		previous = current
	}
	return ast.ErrASTDomain
}

func (n *SMTEnode) EqualityClassMinGeneration() *SMTEnode {
	best := n
	if n.generation == 0 {
		return best
	}
	for current := n.next; current != n; current = current.next {
		if current.generation < best.generation {
			best = current
			if best.generation == 0 {
				return best
			}
		}
	}
	return best
}

func SMTEnodesCongruent(left, right *SMTEnode) (congruent, commuted bool, err error) {
	if left == nil || right == nil || left.declaration.IsNull() || right.declaration.IsNull() {
		return false, false, ast.ErrASTDomain
	}
	if left.declaration != right.declaration || len(left.arguments) != len(right.arguments) {
		return false, false, nil
	}
	if left.commutative {
		if len(left.arguments) != 2 {
			return false, false, ast.ErrASTDomain
		}
		l0, l1 := left.arguments[0].root, left.arguments[1].root
		r0, r1 := right.arguments[0].root, right.arguments[1].root
		if l0 == r0 && l1 == r1 {
			return true, false, nil
		}
		return l0 == r1 && l1 == r0, l0 == r1 && l1 == r0, nil
	}
	for i := range left.arguments {
		if left.arguments[i].root != right.arguments[i].root {
			return false, false, nil
		}
	}
	return true, false, nil
}

func SMTEnodesMaxGeneration(nodes []*SMTEnode) uint32 {
	var maximum uint32
	for _, node := range nodes {
		if node != nil && node.generation > maximum {
			maximum = node.generation
		}
	}
	return maximum
}

func SMTUnmarkEnodes(nodes []*SMTEnode) error {
	for _, node := range nodes {
		if node == nil || !node.mark {
			return ast.ErrASTDomain
		}
		node.mark = false
	}
	return nil
}

func SMTUnmarkEnodes2(nodes []*SMTEnode) error {
	for _, node := range nodes {
		if node == nil || !node.mark2 {
			return ast.ErrASTDomain
		}
		node.mark2 = false
	}
	return nil
}
