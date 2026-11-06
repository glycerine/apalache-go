package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocSimilarityQueryScope struct {
	owner             *datalogUDocContext
	program           *rel.UDocSimilarityProgram
	previousProducts  *udocProductProgram
	installed         []ast.ASTHandle
	installedProducts []ast.ASTHandle
}

func (e *PropositionalEngine) udocSimilaritySettings() (bool, uint32, error) {
	enabled, threshold := true, uint32(11)
	if e.params == nil || e.globals == nil {
		return enabled, threshold, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, 0, err
	}
	enabled, err = view.Bool("datalog.similarity_compressor", true)
	if err == nil && enabled {
		threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
	}
	return enabled, threshold, errors.Join(err, view.Close())
}

func installUDocSimilaritySeeds(owner *datalogUDocContext, program *rel.UDocSimilarityProgram) (*udocSimilarityQueryScope, error) {
	if owner == nil || program == nil {
		return nil, ErrUDocDatalogFragment
	}
	scope := &udocSimilarityQueryScope{owner: owner, program: program}
	seeds := program.Seeds()
	products := program.ProductSeeds()
	for _, declaration := range program.SeedOrder() {
		if product := products[declaration]; product != nil {
			if owner.products == nil || owner.products.seeds[declaration] != nil || owner.seeds[declaration] != nil {
				return nil, errors.Join(ErrUDocDatalogFragment, scope.Close())
			}
			owner.products.seeds[declaration] = product
			scope.installedProducts = append(scope.installedProducts, declaration)
			continue
		}
		if seeds[declaration] == nil || owner.seeds[declaration] != nil {
			return nil, errors.Join(ErrUDocDatalogFragment, scope.Close())
		}
		owner.seeds[declaration] = seeds[declaration]
		owner.order = append(owner.order, declaration)
		scope.installed = append(scope.installed, declaration)
	}
	return scope, nil
}

func (s *udocSimilarityQueryScope) Close() (result error) {
	if s == nil {
		return nil
	}
	for _, declaration := range s.installed {
		if relation := s.owner.result[declaration]; relation != nil {
			result = errors.Join(result, relation.Close())
			if result == nil {
				delete(s.owner.result, declaration)
			}
		}
		delete(s.owner.seeds, declaration)
	}
	if s.owner != nil && s.owner.products != nil {
		for _, declaration := range s.installedProducts {
			if product := s.owner.products.result[declaration]; product != nil {
				result = errors.Join(result, product.Close())
				if result == nil {
					delete(s.owner.products.result, declaration)
				}
			}
			delete(s.owner.products.seeds, declaration)
		}
	}
	if s.owner != nil {
		if s.previousProducts != nil && s.owner.products != nil {
			s.owner.products.program = s.previousProducts
			s.previousProducts = nil
		}
		installed := make(map[ast.ASTHandle]bool, len(s.installed))
		for _, declaration := range s.installed {
			installed[declaration] = true
		}
		order := s.owner.order[:0]
		for _, declaration := range s.owner.order {
			if !installed[declaration] {
				order = append(order, declaration)
			}
		}
		s.owner.order = order
	}
	if s.program != nil {
		result = errors.Join(result, s.program.Close())
	}
	if result == nil {
		s.owner, s.program, s.installed, s.installedProducts = nil, nil, nil, nil
	}
	return result
}

func similarityProductSeedSorts(program *rel.UDocSimilarityProgram) map[ast.ASTHandle][]ast.ASTHandle {
	result := make(map[ast.ASTHandle][]ast.ASTHandle)
	if program == nil {
		return result
	}
	sorts, products := program.SeedSorts(), program.ProductSeeds()
	for declaration := range products {
		result[declaration] = sorts[declaration]
	}
	return result
}
