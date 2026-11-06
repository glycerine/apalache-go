package pattern

// Source: src/ast/pattern/pattern_inference.cpp:31-93.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type expressionPair struct{ left, right ast.ASTHandle }

// smallerPattern is the concrete borrowed expression-pair cache used while
// pattern inference compares two candidate patterns. The AST manager owns the
// expressions; this matcher retains no references after a call.
type smallerPattern struct {
	manager   *ast.ASTManager
	bindings  []ast.ASTHandle
	cache     map[expressionPair]struct{}
	todo      []expressionPair
	traceTags *diagnostic.TraceTags
}

func newSmallerPattern(m *ast.ASTManager) (*smallerPattern, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &smallerPattern{manager: m, cache: make(map[expressionPair]struct{})}, nil
}

func (s *smallerPattern) save(left, right ast.ASTHandle) error {
	pair := expressionPair{left, right}
	if _, seen := s.cache[pair]; seen {
		return nil
	}
	trace := s.traceTags
	if trace == nil {
		trace = diagnostic.BorrowProcessTraceTags()
	}
	if trace.ShouldTrace("smaller_pattern_proc") {
		leftID, err := s.manager.GetASTID(left)
		if err != nil {
			return err
		}
		rightID, err := s.manager.GetASTID(right)
		if err != nil {
			return err
		}
		if err := trace.Emit("smaller_pattern_proc", "save", "src/ast/pattern/pattern_inference.cpp", 34,
			fmt.Sprintf("saving: %d %d\n", leftID, rightID)); err != nil {
			return err
		}
	}
	s.cache[pair] = struct{}{}
	s.todo = append(s.todo, pair)
	return nil
}

// smaller implements smaller_pattern::operator(). Pattern variables below
// numBindings bind once to any instance expression; external variables must
// retain their exact AST identity.
func (s *smallerPattern) smaller(numBindings uint32, left, right ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil {
		return false, ast.ErrASTHandle
	}
	if uint64(numBindings) > uint64(^uint(0)>>1) {
		return false, ast.ErrASTDomain
	}
	if cap(s.bindings) < int(numBindings) {
		s.bindings = make([]ast.ASTHandle, numBindings)
	} else {
		s.bindings = s.bindings[:numBindings]
		clear(s.bindings)
	}
	clear(s.cache)
	s.todo = s.todo[:0]
	if err := s.save(left, right); err != nil {
		return false, err
	}
	for len(s.todo) != 0 {
		i := len(s.todo) - 1
		pair := s.todo[i]
		s.todo[i] = expressionPair{}
		s.todo = s.todo[:i]
		leftKind, err := s.manager.GetKind(pair.left)
		if err != nil {
			return false, err
		}
		if leftKind != ast.ASTVar {
			rightKind, err := s.manager.GetKind(pair.right)
			if err != nil {
				return false, err
			}
			if leftKind != rightKind {
				return false, nil
			}
		}
		switch leftKind {
		case ast.ASTApp:
			leftCount, err := s.manager.GetNumArgs(pair.left)
			if err != nil {
				return false, err
			}
			rightCount, err := s.manager.GetNumArgs(pair.right)
			if err != nil {
				return false, err
			}
			leftDecl, err := s.manager.GetDecl(pair.left)
			if err != nil {
				return false, err
			}
			rightDecl, err := s.manager.GetDecl(pair.right)
			if err != nil {
				return false, err
			}
			if leftCount != rightCount || leftDecl != rightDecl {
				return false, nil
			}
			for j := uint32(0); j < leftCount; j++ {
				childLeft, err := s.manager.GetArg(pair.left, j)
				if err != nil {
					return false, err
				}
				childRight, err := s.manager.GetArg(pair.right, j)
				if err != nil {
					return false, err
				}
				if err := s.save(childLeft, childRight); err != nil {
					return false, err
				}
			}
		case ast.ASTVar:
			index, err := s.manager.GetVarIndex(pair.left)
			if err != nil {
				return false, err
			}
			if index < numBindings {
				if s.bindings[index].IsNull() {
					s.bindings[index] = pair.right
				} else if s.bindings[index] != pair.right {
					return false, nil
				}
			} else if pair.left != pair.right {
				return false, nil
			}
		default:
			if pair.left != pair.right {
				return false, nil
			}
		}
	}
	return true, nil
}
