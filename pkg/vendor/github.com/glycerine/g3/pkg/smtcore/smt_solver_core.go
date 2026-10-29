package smtcore

// Source: src/smt/smt_solver.cpp unsat-core pattern extension.

import "github.com/glycerine/g3/pkg/ast"

type smtDeclSet map[ast.ASTHandle]struct{}

func smtDeclSetsIntersect(left, right smtDeclSet) bool {
	for declaration := range left {
		if _, found := right[declaration]; found {
			return true
		}
	}
	return false
}

func (s *SMTSolver) collectUninterpretedDecls(root ast.ASTHandle, declarations smtDeclSet) error {
	visited := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if _, found := visited[expression]; found {
			continue
		}
		visited[expression] = struct{}{}
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := s.manager.GetDecl(expression)
			if err != nil {
				return err
			}
			family, err := s.manager.GetFamilyID(declaration)
			if err != nil {
				return err
			}
			if family == ast.NullFamilyID {
				declarations[declaration] = struct{}{}
			}
			count, err := s.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := s.manager.GetArg(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, err := s.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			stack = append(stack, body)
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (s *SMTSolver) collectPatternDecls(root ast.ASTHandle, declarations smtDeclSet) error {
	visited := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if _, found := visited[expression]; found {
			continue
		}
		visited[expression] = struct{}{}
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			count, err := s.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := s.manager.GetArg(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			patterns, err := s.manager.GetNumPatterns(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := s.manager.GetPattern(expression, i)
				if err != nil {
					return err
				}
				if err = s.collectUninterpretedDecls(pattern, declarations); err != nil {
					return err
				}
			}
			noPatterns, err := s.manager.GetNumNoPatterns(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < noPatterns; i++ {
				pattern, err := s.manager.GetNoPattern(expression, i)
				if err != nil {
					return err
				}
				if err = s.collectUninterpretedDecls(pattern, declarations); err != nil {
					return err
				}
			}
			body, err := s.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			stack = append(stack, body)
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (s *SMTSolver) collectQuantifierBodyDecls(root ast.ASTHandle, declarations smtDeclSet) error {
	visited := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		expression := stack[last]
		stack = stack[:last]
		if _, found := visited[expression]; found {
			continue
		}
		visited[expression] = struct{}{}
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			count, err := s.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := s.manager.GetArg(expression, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, err := s.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			if err = s.collectUninterpretedDecls(body, declarations); err != nil {
				return err
			}
			stack = append(stack, body)
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (s *SMTSolver) trackedFormula(tag ast.ASTHandle) (ast.ASTHandle, bool, error) {
	for _, assertion := range s.tracked {
		if assertion.tag != tag {
			continue
		}
		formula, err := s.manager.RefGet(assertion.formula)
		return formula, true, err
	}
	return ast.ASTHandle{}, false, nil
}

func (s *SMTSolver) addPatternLiteralsToCore(core []ast.ASTHandle, maxDistance uint32) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), core...)
	inCore := make(map[ast.ASTHandle]struct{}, len(result))
	for _, tag := range result {
		inCore[tag] = struct{}{}
	}
	patternDecls := make(smtDeclSet)
	for distance := uint32(0); distance < maxDistance; distance++ {
		for _, tag := range result {
			formula, found, err := s.trackedFormula(tag)
			if err != nil {
				return nil, err
			}
			if found {
				if err = s.collectPatternDecls(formula, patternDecls); err != nil {
					return nil, err
				}
			}
		}
		added := false
		for _, assertion := range s.tracked {
			if _, found := inCore[assertion.tag]; found {
				continue
			}
			formula, err := s.manager.RefGet(assertion.formula)
			if err != nil {
				return nil, err
			}
			assertionDecls := make(smtDeclSet)
			if err = s.collectUninterpretedDecls(formula, assertionDecls); err != nil {
				return nil, err
			}
			if smtDeclSetsIntersect(patternDecls, assertionDecls) {
				result = append(result, assertion.tag)
				inCore[assertion.tag] = struct{}{}
				added = true
			}
		}
		if !added {
			break
		}
	}
	return result, nil
}

func (s *SMTSolver) addNonlocalPatternLiteralsToCore(core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), core...)
	inCore := make(map[ast.ASTHandle]struct{}, len(result))
	for _, tag := range result {
		inCore[tag] = struct{}{}
	}
	for _, assertion := range s.tracked {
		if _, found := inCore[assertion.tag]; found {
			continue
		}
		formula, err := s.manager.RefGet(assertion.formula)
		if err != nil {
			return nil, err
		}
		patternDecls := make(smtDeclSet)
		if err = s.collectPatternDecls(formula, patternDecls); err != nil {
			return nil, err
		}
		bodyDecls := make(smtDeclSet)
		if err = s.collectQuantifierBodyDecls(formula, bodyDecls); err != nil {
			return nil, err
		}
		for declaration := range patternDecls {
			if _, local := bodyDecls[declaration]; !local {
				result = append(result, assertion.tag)
				inCore[assertion.tag] = struct{}{}
				break
			}
		}
	}
	return result, nil
}

func (s *SMTSolver) extendUnsatCore(core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	name := "core.extend_patterns"
	extend, err := s.config.Params.GetBoolName(&s.params, &name, false)
	if err != nil {
		return nil, err
	}
	if extend {
		name = "core.extend_patterns.max_distance"
		maxDistance, err := s.config.Params.GetUIntName(&s.params, &name, ^uint32(0))
		if err != nil {
			return nil, err
		}
		core, err = s.addPatternLiteralsToCore(core, maxDistance)
		if err != nil {
			return nil, err
		}
	}
	name = "core.extend_nonlocal_patterns"
	extend, err = s.config.Params.GetBoolName(&s.params, &name, false)
	if err != nil {
		return nil, err
	}
	if extend {
		return s.addNonlocalPatternLiteralsToCore(core)
	}
	return core, nil
}
