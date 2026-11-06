package tab

// Source: src/muz/tab/tab_context.cpp, selection strategy and scoring.

import "github.com/glycerine/g3/pkg/ast"

type predicateSelection struct {
	manager         *ast.ASTManager
	strategy        string
	scores          map[ast.ASTHandle][]float64
	cache           map[ast.ASTHandle]float64
	refs            []ast.ASTRef
	multiplier      float64
	frequency, next uint32
}

func newPredicateSelection(m *ast.ASTManager, strategy string) (*predicateSelection, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	// The source basic-weight assignment falls through the following independent
	// if/else chain and is overwritten with WEIGHT_SELECT.
	if strategy != "first" && strategy != "var-use" {
		strategy = "weight"
	}
	return &predicateSelection{manager: m, strategy: strategy, scores: make(map[ast.ASTHandle][]float64), cache: make(map[ast.ASTHandle]float64), multiplier: 1, frequency: 20, next: 20}, nil
}
func (s *predicateSelection) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	for i := range s.refs {
		if s.refs[i] != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.refs[i]); err != nil {
				return err
			}
		}
	}
	s.refs = nil
	s.cache = nil
	s.scores = nil
	s.manager = nil
	return nil
}
func (s *predicateSelection) argumentScore(h ast.ASTHandle, score *uint32) error {
	if *score >= 20 {
		return nil
	}
	kind, err := s.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return err
	}
	_, constructor, err := s.manager.DatatypeConstructorApp(h)
	if err != nil {
		return err
	}
	if constructor {
		*score++
		n, err := s.manager.GetNumArgs(h)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			arg, e := s.manager.GetArg(h, i)
			if e != nil {
				return e
			}
			if e = s.argumentScore(arg, score); e != nil {
				return e
			}
		}
		return nil
	}
	value, err := s.manager.IsValue(h)
	if err == nil && value {
		*score++
	}
	return err
}
func (s *predicateSelection) predicateScores(h ast.ASTHandle) ([]float64, error) {
	n, err := s.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	scores := make([]float64, n)
	for i := range scores {
		arg, e := s.manager.GetArg(h, uint32(i))
		if e != nil {
			return nil, e
		}
		var count uint32
		if e = s.argumentScore(arg, &count); e != nil {
			return nil, e
		}
		scores[i] = float64(count)
	}
	return scores, nil
}
func (s *predicateSelection) Init(rules *RuleIndex) error {
	if s == nil || s.manager == nil || rules == nil || rules.closed || rules.manager != s.manager {
		return ast.ErrASTHandle
	}
	clear(s.scores)
	counts := make(map[ast.ASTHandle]uint32)
	for _, clause := range rules.clauses {
		head, err := clause.Head()
		if err != nil {
			return err
		}
		decl, err := s.manager.GetDecl(head)
		if err != nil {
			return err
		}
		scores, err := s.predicateScores(head)
		if err != nil {
			return err
		}
		existing := s.scores[decl]
		if existing == nil {
			existing = make([]float64, len(scores))
		}
		if len(existing) != len(scores) {
			return ast.ErrASTDomain
		}
		for i := range scores {
			existing[i] += scores[i]
		}
		s.scores[decl] = existing
		counts[decl]++
	}
	for decl, scores := range s.scores {
		for i := range scores {
			scores[i] /= float64(counts[decl])
		}
	}
	return nil
}
func (s *predicateSelection) Select(c *Clause) (uint32, error) {
	if s == nil || s.manager == nil || c == nil || c.closed || c.manager != s.manager {
		return 0, ast.ErrASTHandle
	}
	preds, err := c.Predicates()
	if err != nil {
		return 0, err
	}
	if s.strategy == "first" {
		return 0, nil
	}
	if s.strategy == "weight" {
		s.next--
		if s.next == 0 {
			if s.frequency >= (1 << 16) {
				s.frequency = 20
				s.multiplier = 1
			}
			s.frequency = s.frequency * 11 / 10
			s.next = s.frequency
			s.multiplier *= 1.1
		}
	}
	var varScores []float64
	if s.strategy == "var-use" {
		for _, pred := range preds {
			decl, e := s.manager.GetDecl(pred)
			if e != nil {
				return 0, e
			}
			scores, found := s.scores[decl]
			if !found {
				continue
			}
			for i, score := range scores {
				arg, e := s.manager.GetArg(pred, uint32(i))
				if e != nil {
					return 0, e
				}
				kind, e := s.manager.GetKind(arg)
				if e != nil {
					return 0, e
				}
				if kind != ast.ASTVar {
					continue
				}
				idx, e := s.manager.GetVarIndex(arg)
				if e != nil {
					return 0, e
				}
				for len(varScores) <= int(idx) {
					varScores = append(varScores, 0)
				}
				varScores[idx] += score
			}
		}
	}
	best := 0.0
	selected := uint32(0)
	for i, pred := range preds {
		var score float64
		if s.strategy == "weight" {
			if cached, ok := s.cache[pred]; ok {
				score = cached
			} else {
				parts, e := s.predicateScores(pred)
				if e != nil {
					return 0, e
				}
				score = 1
				for _, v := range parts {
					score += v
				}
				score /= s.multiplier
				ref, e := s.manager.NewRef(pred)
				if e != nil {
					return 0, e
				}
				s.cache[pred] = score
				s.refs = append(s.refs, ref)
			}
		} else {
			parts, e := s.predicateScores(pred)
			if e != nil {
				return 0, e
			}
			if s.strategy == "basic-weight" {
				score = 1
				for _, v := range parts {
					score += v
				}
			} else {
				decl, e := s.manager.GetDecl(pred)
				if e != nil {
					return 0, e
				}
				weights := s.scores[decl]
				for j, v := range parts {
					arg, e := s.manager.GetArg(pred, uint32(j))
					if e != nil {
						return 0, e
					}
					kind, e := s.manager.GetKind(arg)
					if e != nil {
						return 0, e
					}
					if kind == ast.ASTVar {
						idx, e := s.manager.GetVarIndex(arg)
						if e != nil {
							return 0, e
						}
						if int(idx) >= len(varScores) {
							return 0, ast.ErrASTDomain
						}
						score += varScores[idx]
					} else if j < len(weights) {
						score += weights[j] * v
					}
				}
			}
		}
		if score > best {
			best = score
			selected = uint32(i)
		}
	}
	return selected, nil
}
