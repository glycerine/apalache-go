package nlsat

// Source: src/nlsat/nlsat_solver.cpp var_info_collector, reorder_lt,
// heuristic_reorder, and can_reorder.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/math/polynomial"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type variableOrderScore struct {
	degree      uint32
	occurrences uint32
}

// HeuristicVariablePermutation returns the source old-variable-to-new-position
// permutation. Root atoms disable reordering because their indexed-root
// invariants depend on the existing variable order.
func (s *Solver) HeuristicVariablePermutation() (_ []uint32, supported bool, resultErr error) {
	if err := s.open(); err != nil {
		return nil, false, err
	}
	count := len(s.clauses.vars.isInt)
	if len(s.patches) != 0 {
		return nil, false, nil
	}
	for _, atom := range s.atoms.bools.atoms {
		if root, ok := atom.(*RootAtom); ok && root != nil {
			return nil, false, nil
		}
	}
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return nil, false, err
	}
	degrees, err := polynomial.NewPolynomialMaxDegrees(core)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, degrees.CloseChecked()) }()
	scores := make([]variableOrderScore, count)
	collect := func(value polynomial.Polynomial) error {
		if err := degrees.Reset(); err != nil {
			return err
		}
		if err := degrees.Init(value); err != nil {
			return err
		}
		n, err := degrees.NumVariables()
		if err != nil {
			return err
		}
		variables, err := degrees.Vars()
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			x, xErr := variables.At(i)
			if xErr != nil {
				return xErr
			}
			if uint64(x) >= uint64(len(scores)) {
				return ErrSearchOwner
			}
			degree, xErr := degrees.Degree(x)
			if xErr != nil {
				return xErr
			}
			scores[x].occurrences++
			if degree > scores[x].degree {
				scores[x].degree = degree
			}
		}
		return nil
	}
	for _, clauses := range [][]*Clause{s.clauses.originals, s.clauses.learned} {
		for _, clause := range clauses {
			for _, literal := range clause.Literals() {
				atom, atomErr := s.clauses.literalAtom(literal)
				if atomErr != nil {
					return nil, false, atomErr
				}
				switch atom := atom.(type) {
				case *InequalityAtom:
					for _, factor := range atom.factors {
						if err := collect(factor.Polynomial); err != nil {
							return nil, false, err
						}
					}
				case *RootAtom:
					return nil, false, nil
				}
			}
		}
	}
	order := make([]uint32, count)
	for i := range order {
		order[i] = uint32(i)
	}
	sort.Slice(order, func(i, j int) bool {
		left, right := scores[order[i]], scores[order[j]]
		if left.degree != right.degree {
			return left.degree > right.degree
		}
		if left.occurrences != right.occurrences {
			return left.occurrences > right.occurrences
		}
		return order[i] < order[j]
	})
	permutation := make([]uint32, count)
	for position, variable := range order {
		permutation[variable] = uint32(position)
	}
	return permutation, true, nil
}

// ShuffledVariablePermutation returns the source shuffle_vars permutation and
// advances the solver seed once per attempted order, before constructing the
// local random generator.
func (s *Solver) ShuffledVariablePermutation() ([]uint32, bool, error) {
	if err := s.open(); err != nil {
		return nil, false, err
	}
	if len(s.patches) != 0 {
		return nil, false, nil
	}
	for _, atom := range s.atoms.bools.atoms {
		if root, ok := atom.(*RootAtom); ok && root != nil {
			return nil, false, nil
		}
	}
	count := len(s.clauses.vars.isInt)
	order := make([]uint32, count)
	for i := range order {
		order[i] = uint32(i)
	}
	s.randomSeed++
	z3rand.ShuffleUint32(order, z3rand.New(s.randomSeed))
	permutation := make([]uint32, count)
	for position, variable := range order {
		permutation[variable] = uint32(position)
	}
	return permutation, true, nil
}

func validateVariablePermutation(permutation []uint32, count int) error {
	if len(permutation) != count {
		return ErrSearchOwner
	}
	seen := make([]bool, count)
	for _, target := range permutation {
		if uint64(target) >= uint64(count) || seen[target] {
			return ErrSearchOwner
		}
		seen[target] = true
	}
	return nil
}

// ReorderVariables applies an old-position-to-new-position permutation to the
// solver. It preserves the current arithmetic model and
// composes the external/internal variable maps exactly as the source does.
func (s *Solver) ReorderVariables(permutation []uint32) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	count := len(s.clauses.vars.isInt)
	if len(s.patches) != 0 {
		return ErrSearchOwner
	}
	if err := validateVariablePermutation(permutation, count); err != nil {
		return err
	}
	nextAssignment, err := NewAssignment(s.config.Algebraic)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, nextAssignment.Close()) }()
	for old := 0; old < count; old++ {
		assigned, x := s.assignment.Contains(uint32(old))
		if x != nil {
			return x
		}
		if !assigned {
			continue
		}
		value, x := s.assignment.Value(uint32(old))
		if x != nil {
			return x
		}
		if x = nextAssignment.Set(permutation[old], value); x != nil {
			return x
		}
	}
	// Reordering changes arithmetic stages but must leave Boolean decisions in
	// the model, as source reorder's undo_until_stage(null_var) does.
	if err = s.search.undoUntilStage(NullVariable); err != nil {
		return err
	}
	if err = s.cache.Reset(); err != nil {
		return err
	}
	for i := range s.clauses.vars.watches {
		s.clauses.vars.watches[i] = nil
	}
	for i := range s.atoms.bools.watches {
		s.atoms.bools.watches[i] = nil
	}
	variables := &s.clauses.vars
	oldInteger := append([]bool(nil), variables.isInt...)
	oldInverse := append([]uint32(nil), variables.inv...)
	newInteger := make([]bool, count)
	newInverse := make([]uint32, count)
	newPermutation := make([]uint32, count)
	for old := 0; old < count; old++ {
		newInteger[permutation[old]] = oldInteger[old]
	}
	for external := 0; external < count; external++ {
		newInverse[external] = permutation[oldInverse[external]]
		newPermutation[newInverse[external]] = uint32(external)
	}
	if err = s.config.Polynomial.Rename(permutation); err != nil {
		return err
	}
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	recache := func(p polynomial.Polynomial) error {
		unique, e := s.cache.MkUnique(p)
		if e != nil {
			return e
		}
		if unique != p {
			return ErrSearchOwner
		}
		return nil
	}
	// A root atom's distinguished variable must follow its polynomial through
	// the same permutation. Update the existing interning table as well.
	for _, atom := range s.atoms.bools.atoms {
		root, ok := atom.(*RootAtom)
		if !ok || root == nil {
			continue
		}
		owner := s.atoms.ownedRoots[root]
		if owner == nil || uint64(root.x) >= uint64(len(permutation)) {
			return ErrSearchOwner
		}
		s.atoms.eraseRoot(owner)
		root.x = permutation[root.x]
		owner.hash, err = root.Hash(core)
		if err != nil {
			return err
		}
		s.atoms.roots[owner.hash] = append(s.atoms.roots[owner.hash], owner)
		owner.inTable = true
	}
	// Source reorder removes learned clauses whose root atoms are ill-formed
	// in the new variable order.
	for _, clause := range append([]*Clause(nil), s.clauses.learned...) {
		illFormed := false
		for _, literal := range clause.Literals() {
			atom, e := s.clauses.literalAtom(literal)
			if e != nil {
				return e
			}
			if root, ok := atom.(*RootAtom); ok && root != nil {
				maximum, e := core.MaxVariable(root.polynomial)
				if e != nil {
					return e
				}
				if maximum != NullVariable && root.x < maximum {
					illFormed = true
					break
				}
			}
		}
		if illFormed {
			if err = s.clauses.deleteClause(clause); err != nil {
				return err
			}
		}
	}
	for _, atom := range s.atoms.bools.atoms {
		switch a := atom.(type) {
		case *InequalityAtom:
			if a == nil {
				continue
			}
			maximum := uint32(NullVariable)
			for _, factor := range a.factors {
				if err = recache(factor.Polynomial); err != nil {
					return err
				}
				x, xErr := core.MaxVariable(factor.Polynomial)
				if xErr != nil {
					return xErr
				}
				if x != NullVariable && (maximum == NullVariable || x > maximum) {
					maximum = x
				}
			}
			a.maxVar = maximum
		case *RootAtom:
			if a == nil {
				continue
			}
			if err = recache(a.polynomial); err != nil {
				return err
			}
			a.maxVar, err = core.MaxVariable(a.polynomial)
			if err != nil {
				return err
			}
		}
	}
	variables.isInt = newInteger
	variables.inv = newInverse
	variables.perm = newPermutation
	if err = s.assignment.Swap(nextAssignment); err != nil {
		return err
	}
	for _, clauses := range [][]*Clause{s.clauses.originals, s.clauses.learned} {
		for _, clause := range clauses {
			if err = s.clauses.attach(clause); err != nil {
				return err
			}
		}
	}
	return nil
}

// RestoreVariableOrder maps the current internal variables back to their
// original externally visible positions.
func (s *Solver) RestoreVariableOrder() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.ReorderVariables(append([]uint32(nil), s.clauses.vars.perm...))
}
