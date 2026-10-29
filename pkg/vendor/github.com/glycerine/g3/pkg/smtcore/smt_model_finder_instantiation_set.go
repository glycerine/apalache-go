package smtcore

// Source: src/smt/smt_model_finder.cpp instantiation_set.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

type smtModelFinderInstantiationSet struct {
	manager     *ast.ASTManager
	elements    map[ast.ASTHandle]uint32
	references  map[ast.ASTHandle]ast.ASTRef
	inverse     map[ast.ASTHandle]ast.ASTHandle
	inverseRefs map[ast.ASTHandle]ast.ASTRef
	closed      bool
}

func newSMTModelFinderInstantiationSet(manager *ast.ASTManager) (*smtModelFinderInstantiationSet, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtModelFinderInstantiationSet{
		manager: manager, elements: make(map[ast.ASTHandle]uint32), references: make(map[ast.ASTHandle]ast.ASTRef),
		inverse: make(map[ast.ASTHandle]ast.ASTHandle), inverseRefs: make(map[ast.ASTHandle]ast.ASTRef),
	}, nil
}

func (s *smtModelFinderInstantiationSet) open() error {
	if s == nil || s.manager == nil || s.elements == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *smtModelFinderInstantiationSet) containsModelValue(root ast.ASTHandle) (bool, error) {
	return containsPrivateModelTerm(s.manager, root, false)
}

func containsPrivateModelTerm(manager *ast.ASTManager, root ast.ASTHandle, rejectFunctionArrays bool) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]struct{})
	for len(stack) != 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, found := seen[n]; found {
			continue
		}
		seen[n] = struct{}{}
		value, err := manager.IsModelValue(n)
		if err != nil {
			return false, err
		}
		if value {
			return true, nil
		}
		kind, err := manager.GetKind(n)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTQuantifier {
			body, err := manager.GetQuantifierBody(n)
			if err != nil {
				return false, err
			}
			stack = append(stack, body)
			patterns, err := manager.GetNumPatterns(n)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := manager.GetPattern(n, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, pattern)
			}
			noPatterns, err := manager.GetNumNoPatterns(n)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < noPatterns; i++ {
				pattern, err := manager.GetNoPattern(n, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, pattern)
			}
			continue
		}
		if kind != ast.ASTApp {
			continue
		}
		if rejectFunctionArrays {
			family, err := manager.GetFamilyID(n)
			if err != nil {
				return false, err
			}
			arrayFamily, err := manager.ArrayFamilyID()
			if err != nil {
				return false, err
			}
			if family == arrayFamily {
				op, err := manager.GetDeclKind(n)
				if err != nil {
					return false, err
				}
				if op == ast.ASTArrayOpAsArray {
					return true, nil
				}
			}
		}
		count, err := manager.GetNumArgs(n)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			child, err := manager.GetArg(n, i)
			if err != nil {
				return false, err
			}
			stack = append(stack, child)
		}
	}
	return false, nil
}

func (s *smtModelFinderInstantiationSet) Insert(term ast.ASTHandle, generation uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if _, found := s.elements[term]; found {
		return false, nil
	}
	contains, err := s.containsModelValue(term)
	if err != nil || contains {
		return false, err
	}
	ref, err := s.manager.NewRef(term)
	if err != nil {
		return false, err
	}
	s.elements[term] = generation
	s.references[term] = ref
	return true, nil
}

func (s *smtModelFinderInstantiationSet) Remove(term ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.inverse) != 0 {
		return ast.ErrASTDomain
	}
	ref, found := s.references[term]
	if !found {
		return ast.ErrASTDomain
	}
	if err := s.manager.DelRef(&ref); err != nil {
		return err
	}
	delete(s.references, term)
	delete(s.elements, term)
	return nil
}

func (s *smtModelFinderInstantiationSet) clearInverse() (result error) {
	for value, ref := range s.inverseRefs {
		copy := ref
		if err := s.manager.DelRef(&copy); err != nil {
			result = errors.Join(result, err)
			continue
		}
		delete(s.inverseRefs, value)
	}
	if result == nil {
		clear(s.inverse)
	}
	return result
}

func (s *smtModelFinderInstantiationSet) MakeInverse(model *protomodel.ProtoModel) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if model == nil || model.Core() == nil {
		return ast.ErrASTHandle
	}
	if err := s.clearInverse(); err != nil {
		return err
	}
	for term, generation := range s.elements {
		valueRef, err := s.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		if _, err = model.Eval(term, valueRef, true); err != nil {
			return errors.Join(err, s.manager.DelRef(&valueRef))
		}
		value, err := s.manager.RefGet(valueRef)
		if err != nil {
			return errors.Join(err, s.manager.DelRef(&valueRef))
		}
		old, found := s.inverse[value]
		if found {
			if generation < s.elements[old] {
				s.inverse[value] = term
			}
			if err = s.manager.DelRef(&valueRef); err != nil {
				return err
			}
			continue
		}
		s.inverse[value] = term
		s.inverseRefs[value] = valueRef
	}
	return nil
}

func (s *smtModelFinderInstantiationSet) Inverse(value ast.ASTHandle) (term ast.ASTHandle, generation uint32, found bool, err error) {
	if err = s.open(); err != nil {
		return term, 0, false, err
	}
	term, found = s.inverse[value]
	if found {
		generation = s.elements[term]
	}
	return term, generation, found, nil
}

func (s *smtModelFinderInstantiationSet) Values() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, 0, len(s.inverse))
	for value := range s.inverse {
		result = append(result, value)
	}
	return result, nil
}

func (s *smtModelFinderInstantiationSet) Close() (result error) {
	if s == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if s.manager == nil {
		return ast.ErrASTHandle
	}
	result = s.clearInverse()
	for term, ref := range s.references {
		copy := ref
		if err := s.manager.DelRef(&copy); err != nil {
			result = errors.Join(result, err)
			continue
		}
		delete(s.references, term)
	}
	if result == nil {
		s.closed = true
		s.elements = nil
		s.references = nil
		s.inverse = nil
		s.inverseRefs = nil
		s.manager = nil
	}
	return result
}
