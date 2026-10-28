package apalache

import "fmt"

type TypeFoundCallback func(Type)
type TypeErrorCallback func(*Substitution, []Type)

// TypeClause is a unification constraint. Disjunction is required for
// overloaded TLA+ operators whose signatures cannot be chosen eagerly.
type TypeClause interface {
	typeClause()
	callbacks() (TypeFoundCallback, TypeErrorCallback)
	SetOnTypeFound(TypeFoundCallback) TypeClause
	SetOnTypeError(TypeErrorCallback) TypeClause
}

type EqClause struct {
	Unknown     VarType
	Term        Type
	OnTypeFound TypeFoundCallback
	OnTypeError TypeErrorCallback
}

func (*EqClause) typeClause() {}
func (clause *EqClause) callbacks() (TypeFoundCallback, TypeErrorCallback) {
	return clause.OnTypeFound, clause.OnTypeError
}
func (clause *EqClause) SetOnTypeFound(callback TypeFoundCallback) TypeClause {
	clause.OnTypeFound = callback
	return clause
}
func (clause *EqClause) SetOnTypeError(callback TypeErrorCallback) TypeClause {
	clause.OnTypeError = callback
	return clause
}

type OrClause struct {
	Clauses     []*EqClause
	OnTypeFound TypeFoundCallback
	OnTypeError TypeErrorCallback
}

func (*OrClause) typeClause() {}
func (clause *OrClause) callbacks() (TypeFoundCallback, TypeErrorCallback) {
	return clause.OnTypeFound, clause.OnTypeError
}
func (clause *OrClause) SetOnTypeFound(callback TypeFoundCallback) TypeClause {
	clause.OnTypeFound = callback
	return clause
}
func (clause *OrClause) SetOnTypeError(callback TypeErrorCallback) TypeClause {
	clause.OnTypeError = callback
	return clause
}

type reportedType struct {
	clause TypeClause
	typ    Type
}

// ConstraintSolver incrementally resolves conjunctive equality constraints
// and postpones ambiguous overload choices until later constraints make one
// alternative unique.
type ConstraintSolver struct {
	pool        *TypeVarPool
	approximate *Substitution
	solution    *Substitution
	constraints []TypeClause
	reports     []reportedType
}

func NewConstraintSolver(pool *TypeVarPool, approximate *Substitution) *ConstraintSolver {
	if pool == nil {
		panic("NewConstraintSolver requires a type variable pool")
	}
	if approximate == nil {
		approximate = EmptySubstitution()
	}
	return &ConstraintSolver{pool: pool, approximate: approximate, solution: approximate}
}

func (solver *ConstraintSolver) Add(clause TypeClause) error {
	if clause == nil {
		return fmt.Errorf("cannot add nil type constraint")
	}
	switch typed := clause.(type) {
	case *EqClause:
		if typed.Term == nil {
			return fmt.Errorf("equality constraint requires a term")
		}
	case *OrClause:
		if len(typed.Clauses) == 0 {
			return fmt.Errorf("disjunctive constraint requires alternatives")
		}
		for i, alternative := range typed.Clauses {
			if alternative == nil || alternative.Term == nil {
				return fmt.Errorf("disjunctive constraint alternative %d is invalid", i)
			}
		}
	default:
		return fmt.Errorf("unsupported type constraint %T", clause)
	}
	solver.constraints = append(solver.constraints, clause)
	return nil
}

// SolvePartially resolves every currently unique constraint. Its Boolean is
// false only for an immediate equality conflict; ambiguous disjunctions are
// retained for a later call.
func (solver *ConstraintSolver) SolvePartially() (*Substitution, bool, error) {
	progress := true
	for len(solver.constraints) > 0 && progress {
		postponed := make([]TypeClause, 0)
		progress = false
		for _, constraint := range solver.constraints {
			solution, typ, unique, err := solver.solveOne(solver.solution, constraint)
			if err != nil {
				return nil, false, err
			}
			if unique {
				progress = true
				solver.solution = solution
				reported, err := solution.ApplyRecursive(typ)
				if err != nil {
					return nil, false, err
				}
				solver.reports = append(solver.reports, reportedType{clause: constraint, typ: reported})
				continue
			}
			if _, disjunction := constraint.(*OrClause); disjunction {
				postponed = append(postponed, constraint)
				continue
			}
			solver.reportError(constraint)
			solver.constraints = nil
			return nil, false, nil
		}
		solver.constraints = postponed
	}
	return solver.solution, true, nil
}

func (solver *ConstraintSolver) Solve() (*Substitution, bool, error) {
	_, consistent, err := solver.SolvePartially()
	if err != nil || !consistent {
		return nil, false, err
	}
	if len(solver.constraints) != 0 {
		for _, constraint := range solver.constraints {
			solver.reportError(constraint)
		}
		return nil, false, nil
	}
	for _, report := range solver.reports {
		typ, err := solver.solution.ApplyRecursive(report.typ)
		if err != nil {
			return nil, false, err
		}
		if found, _ := report.clause.callbacks(); found != nil {
			found(typ)
		}
	}
	return solver.solution, true, nil
}

func (solver *ConstraintSolver) IsFreeVar(number int) bool {
	outside := func(substitution *Substitution) bool {
		for _, entry := range substitution.Entries() {
			for _, variable := range entry.Class.Variables() {
				if variable == number {
					return false
				}
			}
		}
		return true
	}
	return outside(solver.approximate) && outside(solver.solution)
}

func (solver *ConstraintSolver) solveOne(initial *Substitution, constraint TypeClause) (*Substitution, Type, bool, error) {
	switch typed := constraint.(type) {
	case *EqClause:
		return NewTypeUnifier(solver.pool).Unify(initial, typed.Unknown, typed.Term)
	case *OrClause:
		type candidate struct {
			solution *Substitution
			typ      Type
		}
		candidates := make([]candidate, 0, len(typed.Clauses))
		for _, alternative := range typed.Clauses {
			solution, typ, ok, err := solver.solveOne(initial, alternative)
			if err != nil {
				return nil, nil, false, err
			}
			if ok {
				candidates = append(candidates, candidate{solution: solution, typ: typ})
			}
		}
		if len(candidates) != 1 {
			return nil, nil, false, nil
		}
		return candidates[0].solution, candidates[0].typ, true, nil
	default:
		return nil, nil, false, fmt.Errorf("unsupported type constraint %T", constraint)
	}
}

func (solver *ConstraintSolver) reportError(constraint TypeClause) {
	_, callback := constraint.callbacks()
	if callback == nil {
		return
	}
	terms := make([]Type, 0)
	switch typed := constraint.(type) {
	case *EqClause:
		terms = append(terms, typed.Term)
	case *OrClause:
		for _, alternative := range typed.Clauses {
			terms = append(terms, alternative.Term)
		}
	}
	for i, term := range terms {
		if substituted, err := solver.solution.ApplyRecursive(term); err == nil {
			terms[i] = substituted
		}
	}
	callback(solver.solution, terms)
}
