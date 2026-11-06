package qe

// Source: src/qe/qe_datatype_plugin.cpp, datatype_plugin construction,
// recognizer scanning, get_num_branches_nonrec and assign_nonrec.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// DatatypeSolverPlugin implements constructor and equality branches for the
// QE solver plugin's datatype variables.
type DatatypeSolverPlugin struct {
	self          *DatatypeSolverPlugin
	context       *SolverContext
	manager       *ast.ASTManager
	family        ast.FamilyID
	constraints   []ArithmeticConstraint
	substitutions map[datatypeSubstitutionKey]*datatypeSubstitution
	equalities    map[arithmeticPluginKey]*datatypeEqualityAtoms
	closed        bool
}

func NewDatatypeSolverPlugin(context *SolverContext) (*DatatypeSolverPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := context.Manager()
	if err != nil {
		return nil, err
	}
	family, err := m.RegisterDatatypePlugin()
	if err != nil {
		return nil, err
	}
	p := &DatatypeSolverPlugin{context: context, manager: m, family: family, substitutions: make(map[datatypeSubstitutionKey]*datatypeSubstitution), equalities: make(map[arithmeticPluginKey]*datatypeEqualityAtoms)}
	p.self = p
	return p, nil
}

func (p *DatatypeSolverPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return p.context.open()
}
func (p *DatatypeSolverPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}
func (p *DatatypeSolverPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return ast.ASTHandle{}, false, nil
}
func (p *DatatypeSolverPlugin) IsUninterpreted(h ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	kind, err := p.manager.GetKind(h)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return true, nil
}
func (p *DatatypeSolverPlugin) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	for len(p.constraints) != 0 {
		i := len(p.constraints) - 1
		if err := p.manager.DelRef(&p.constraints[i].Formula); err != nil {
			return err
		}
		p.constraints = p.constraints[:i]
	}
	for key, entry := range p.substitutions {
		if err := entry.close(p.manager); err != nil {
			return err
		}
		delete(p.substitutions, key)
	}
	for key, entry := range p.equalities {
		if err := entry.close(p.manager); err != nil {
			return err
		}
		delete(p.equalities, key)
	}
	p.closed = true
	return nil
}

// GetNumBranches follows the source constructor cases. Recursive variables
// with a recognizer but no matching selector use prepared equality atoms.
func (p *DatatypeSolverPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, out *numeral.OwnedRational) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if variable == nil || out == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	count, supported, err := p.NonRecursiveBranches(target, formula)
	if err != nil {
		return false, err
	}
	if !supported {
		constructor, found, e := p.findRecognizer(target, formula)
		if e != nil {
			return false, e
		}
		if found {
			selector, e := p.hasSelector(target, formula, constructor)
			if e != nil {
				return false, e
			}
			if selector {
				return true, p.manager.NumeralService().SetUnsigned(out, 1)
			}
			entry := p.equalities[arithmeticPluginKey{variable: target, formula: formula}]
			if entry == nil {
				return false, nil
			}
			return true, p.manager.NumeralService().SetUnsigned(out, uint32(len(entry.positive)+1))
		}
		sort, e := p.manager.GetSort(target)
		if e != nil {
			return false, e
		}
		constructors, e := p.manager.DatatypeConstructors(sort)
		if e != nil {
			return false, e
		}
		count, supported = uint32(len(constructors)), true
	}
	return supported, p.manager.NumeralService().SetUnsigned(out, count)
}

// Assign publishes constructor recognizers as owned branch constraints.
func (p *DatatypeSolverPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) error {
	if err := p.open(); err != nil {
		return err
	}
	if variable == nil || value == nil {
		return ast.ErrASTHandle
	}
	service := p.manager.NumeralService()
	unsigned, err := service.IsUnsigned(value)
	if err != nil {
		return err
	}
	if !unsigned {
		return ast.ErrASTDomain
	}
	branch, err := service.Unsigned(value)
	if err != nil {
		return err
	}
	target, err := variable.Target()
	if err != nil {
		return err
	}
	sort, err := p.manager.GetSort(target)
	if err != nil {
		return err
	}
	recursive, err := p.manager.DatatypeIsRecursive(sort)
	if err != nil {
		return err
	}
	var ref ast.ASTRef
	if !recursive {
		ref, err = p.NonRecursiveBranchConstraint(target, formula, branch)
	} else {
		constructor, found, e := p.findRecognizer(target, formula)
		if e != nil {
			return e
		}
		if found {
			selector, e := p.hasSelector(target, formula, constructor)
			if e != nil {
				return e
			}
			if selector {
				if branch != 0 {
					return ast.ErrASTDomain
				}
				return nil
			}
			return p.assignEqualityBranch(target, formula, branch)
		}
		constructors, e := p.manager.DatatypeConstructors(sort)
		if e != nil {
			return e
		}
		if uint64(branch) >= uint64(len(constructors)) {
			return ast.ErrASTDomain
		}
		recognizer, e := p.manager.DatatypeConstructorRecognizer(constructors[branch])
		if e != nil {
			return e
		}
		application, e := p.manager.MkApp(recognizer, []ast.ASTHandle{target})
		if e != nil {
			return e
		}
		ref, err = p.manager.NewRef(application)
	}
	if err != nil {
		return err
	}
	if ref != (ast.ASTRef{}) {
		p.constraints = append(p.constraints, ArithmeticConstraint{UseVariable: true, Formula: ref})
	}
	return nil
}

// TakeConstraints transfers the generated formula references to the caller.
func (p *DatatypeSolverPlugin) TakeConstraints() ([]ArithmeticConstraint, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := p.constraints
	p.constraints = nil
	return result, nil
}

func (p *DatatypeSolverPlugin) findRecognizer(variable, formula ast.ASTHandle) (constructor ast.ASTHandle, found bool, err error) {
	conjunction, err := NewConjunctionEnum(p.manager, formula)
	if err != nil {
		return constructor, false, err
	}
	defer func() { err = errors.Join(err, conjunction.Close()) }()
	count, err := conjunction.Len()
	if err != nil {
		return constructor, false, err
	}
	for i := uint32(0); i < count; i++ {
		h, e := conjunction.At(i)
		if e != nil {
			return constructor, false, e
		}
		kind, e := p.manager.GetKind(h)
		if e != nil {
			return constructor, false, e
		}
		if kind != ast.ASTApp {
			continue
		}
		family, e := p.manager.GetFamilyID(h)
		if e != nil {
			return constructor, false, e
		}
		if family != p.family {
			continue
		}
		decl, e := p.manager.GetDecl(h)
		if e != nil {
			return constructor, false, e
		}
		candidate, matched, e := p.manager.DatatypeRecognizerConstructor(decl)
		if e != nil {
			return constructor, false, e
		}
		if !matched {
			continue
		}
		argument, e := p.manager.GetArg(h, 0)
		if e != nil {
			return constructor, false, e
		}
		if argument == variable {
			return candidate, true, nil
		}
	}
	return constructor, false, nil
}

func (p *DatatypeSolverPlugin) hasRecognizer(variable, formula ast.ASTHandle) (bool, error) {
	_, found, err := p.findRecognizer(variable, formula)
	return found, err
}

func (p *DatatypeSolverPlugin) hasSelector(variable, formula, constructor ast.ASTHandle) (found bool, err error) {
	check, err := ast.NewCheckPredicate(p.manager, func(h ast.ASTHandle) (bool, error) {
		kind, e := p.manager.GetKind(h)
		if e != nil || kind != ast.ASTApp {
			return false, e
		}
		family, e := p.manager.GetFamilyID(h)
		if e != nil || family != p.family {
			return false, e
		}
		decl, e := p.manager.GetDecl(h)
		if e != nil {
			return false, e
		}
		_, matched, e := p.manager.DatatypeAccessorIndex(decl, constructor)
		if e != nil || !matched {
			return false, e
		}
		arg, e := p.manager.GetArg(h, 0)
		return arg == variable, e
	})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, check.Close()) }()
	return check.Check(formula)
}

// NonRecursiveBranches returns the source branch count and whether the target
// is non-recursive. Recursive datatypes are left for the pending source path.
func (p *DatatypeSolverPlugin) NonRecursiveBranches(variable, formula ast.ASTHandle) (uint32, bool, error) {
	if err := p.open(); err != nil {
		return 0, false, err
	}
	kind, err := p.manager.GetKind(variable)
	if err != nil {
		return 0, false, err
	}
	if kind != ast.ASTApp {
		return 0, false, ast.ErrASTDomain
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return 0, false, err
	}
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return 0, false, err
	}
	if family != p.family {
		return 0, false, ast.ErrASTDomain
	}
	recursive, err := p.manager.DatatypeIsRecursive(sort)
	if err != nil || recursive {
		return 0, false, err
	}
	constructors, err := p.manager.DatatypeConstructors(sort)
	if err != nil {
		return 0, false, err
	}
	count := uint32(len(constructors))
	if count != 1 {
		found, e := p.hasRecognizer(variable, formula)
		if e != nil {
			return 0, false, e
		}
		if found {
			count = 1
		}
	}
	return count, true, nil
}

// NonRecursiveBranchConstraint returns an owned recognizer application when
// assign_nonrec would add one; a zero ref means no constraint is needed.
func (p *DatatypeSolverPlugin) NonRecursiveBranchConstraint(variable, formula ast.ASTHandle, branch uint32) (ast.ASTRef, error) {
	count, supported, err := p.NonRecursiveBranches(variable, formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !supported || branch >= count {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return ast.ASTRef{}, err
	}
	constructors, err := p.manager.DatatypeConstructors(sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(constructors) == 1 {
		return ast.ASTRef{}, nil
	}
	found, err := p.hasRecognizer(variable, formula)
	if err != nil || found {
		return ast.ASTRef{}, err
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructors[branch])
	if err != nil {
		return ast.ASTRef{}, err
	}
	application, err := p.manager.MkApp(recognizer, []ast.ASTHandle{variable})
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(application)
}
