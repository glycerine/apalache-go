package fp

// Source: dl_instruction.cpp filter values and interpreted-tail conditions
// stay rooted in app_ref fields throughout execution_context and program reuse.

import "github.com/glycerine/g3/pkg/ast"

func (p *externalIntervalDatalogCartesianRule) pin(manager *ast.ASTManager) error {
	if p == nil {
		return nil
	}
	for i := range p.constants {
		constant := &p.constants[i]
		if constant.root == (ast.ASTRef{}) {
			root, err := manager.NewRef(constant.value)
			if err != nil {
				return err
			}
			constant.root = root
		}
	}
	for i := range p.headConstants {
		constant := &p.headConstants[i]
		if constant.root == (ast.ASTRef{}) {
			root, err := manager.NewRef(constant.value)
			if err != nil {
				return err
			}
			constant.root = root
		}
	}
	return p.condition.prepare(manager)
}

func (p *externalIntervalDatalogCartesianRule) close(manager *ast.ASTManager) error {
	if p == nil {
		return nil
	}
	if err := p.condition.close(manager); err != nil {
		return err
	}
	for i := len(p.headConstants) - 1; i >= 0; i-- {
		if p.headConstants[i].root != (ast.ASTRef{}) {
			if err := manager.DelRef(&p.headConstants[i].root); err != nil {
				return err
			}
		}
	}
	for i := len(p.constants) - 1; i >= 0; i-- {
		if p.constants[i].root != (ast.ASTRef{}) {
			if err := manager.DelRef(&p.constants[i].root); err != nil {
				return err
			}
		}
	}
	return nil
}
