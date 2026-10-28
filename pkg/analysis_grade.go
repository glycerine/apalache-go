package apalache

import "sync"

// ExprGrade classifies expressions by the environments on which their
// symbolic rewrite can depend. The ordering is not purely linear: combining
// an action-free expression with a state-bound expression is action-bound.
type ExprGrade uint8

const (
	ExprGradeConstant ExprGrade = iota
	ExprGradeStateFree
	ExprGradeStateBound
	ExprGradeActionFree
	ExprGradeActionBound
	ExprGradeHigher
	ExprGradeNonCacheable
)

func JoinExprGrades(left, right ExprGrade) ExprGrade {
	if left == ExprGradeNonCacheable || right == ExprGradeNonCacheable {
		return ExprGradeNonCacheable
	}
	if left == ExprGradeHigher || right == ExprGradeHigher {
		return ExprGradeHigher
	}
	if left == ExprGradeActionBound || right == ExprGradeActionBound {
		return ExprGradeActionBound
	}
	if left == ExprGradeActionFree && right == ExprGradeStateBound ||
		right == ExprGradeActionFree && left == ExprGradeStateBound {
		return ExprGradeActionBound
	}
	if left == ExprGradeActionFree || right == ExprGradeActionFree {
		return ExprGradeActionFree
	}
	if left == ExprGradeStateBound || right == ExprGradeStateBound {
		return ExprGradeStateBound
	}
	if left == ExprGradeStateFree || right == ExprGradeStateFree {
		return ExprGradeStateFree
	}
	return ExprGradeConstant
}

// ExprGradeStore records grades by stable expression identity.
type ExprGradeStore struct {
	mu     sync.RWMutex
	grades map[UID]ExprGrade
}

func NewExprGradeStore() *ExprGradeStore {
	return &ExprGradeStore{grades: make(map[UID]ExprGrade)}
}

func (store *ExprGradeStore) put(id UID, grade ExprGrade) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.grades[id] = grade
}

func (store *ExprGradeStore) Get(id UID) (ExprGrade, bool) {
	if store == nil {
		return 0, false
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	grade, found := store.grades[id]
	return grade, found
}

// LabelExpressionGrades computes and stores the grade of every visited
// subexpression, returning the root grade.
func LabelExpressionGrades(store *ExprGradeStore, constants, variables map[string]struct{}, expression Expr) ExprGrade {
	if store == nil {
		store = NewExprGradeStore()
	}
	var visit func(Expr) ExprGrade
	update := func(expression Expr, grade ExprGrade) ExprGrade {
		store.put(expression.ID(), grade)
		return grade
	}
	visit = func(expression Expr) ExprGrade {
		switch typed := expression.(type) {
		case *ValueExpr:
			return update(expression, ExprGradeConstant)
		case *NameExpr:
			if _, found := constants[typed.Name()]; found {
				return update(expression, ExprGradeConstant)
			}
			if _, found := variables[typed.Name()]; found {
				return update(expression, ExprGradeStateFree)
			}
			return update(expression, ExprGradeStateBound)
		case *OperExpr:
			arguments := typed.Arguments()
			switch typed.Operator().Name() {
			case "Apalache!Gen":
				return update(expression, ExprGradeNonCacheable)
			case "PRIME":
				if len(arguments) == 1 {
					return update(expression, JoinExprGrades(ExprGradeActionFree, visit(arguments[0])))
				}
			}
			if typed.Operator().Category() == CategoryTemporal {
				for _, argument := range arguments {
					visit(argument)
				}
				return update(expression, ExprGradeHigher)
			}
			if len(arguments) == 0 {
				return update(expression, ExprGradeConstant)
			}
			grade := visit(arguments[0])
			for _, argument := range arguments[1:] {
				grade = JoinExprGrades(grade, visit(argument))
			}
			return update(expression, grade)
		default:
			return update(expression, ExprGradeHigher)
		}
	}
	return visit(expression)
}
