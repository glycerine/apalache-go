package rel

import "errors"

func lazySparseJoinMetadata(left, right *lazySparseNode) (Signature, int, error) {
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	domains := make([]uint64, 0, left.signature.Len()+right.signature.Len())
	domains = append(domains, left.signature.domains[:leftFirst]...)
	domains = append(domains, right.signature.domains[:rightFirst]...)
	domains = append(domains, left.signature.domains[leftFirst:]...)
	domains = append(domains, right.signature.domains[rightFirst:]...)
	signature, err := NewSignature(domains)
	return signature, left.functional + right.functional, err
}

func lazySparseProjectMetadata(source *lazySparseNode, removed []int) (Signature, int, error) {
	signature, err := projectedSignature(source.signature, removed)
	if err != nil {
		return Signature{}, 0, err
	}
	functional := source.functional
	if len(removed) != 0 && removed[0] < source.signature.Len()-source.functional {
		functional = 0
	} else {
		functional -= len(removed)
	}
	return signature, functional, nil
}

func (t *LazySparseTable) Join(other *LazySparseTable, leftColumns, rightColumns []int) (*LazySparseTable, error) {
	if err := t.open(); err != nil || other == nil || other.open() != nil || t.engine != other.engine {
		return nil, ErrLazySparse
	}
	if err := validateColumns(t.root.signature, other.root.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	signature, functional, err := lazySparseJoinMetadata(t.root, other.root)
	if err != nil {
		return nil, err
	}
	return t.engine.adopt(&lazySparseNode{kind: LazyTableJoin, signature: signature, functional: functional, left: t.root, right: other.root, columns1: append([]int(nil), leftColumns...), columns2: append([]int(nil), rightColumns...)}), nil
}

func (t *LazySparseTable) Project(removed []int) (*LazySparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, functional, err := lazySparseProjectMetadata(t.root, removed)
	if err != nil {
		return nil, err
	}
	return t.engine.adopt(&lazySparseNode{kind: LazyTableProject, signature: signature, functional: functional, left: t.root, columns1: append([]int(nil), removed...)}), nil
}

func (e *LazySparseEngine) eval(node *lazySparseNode) (*SparseTable, error) {
	if err := e.open(); err != nil || node == nil {
		return nil, ErrLazySparse
	}
	if node.evaluated != nil && !node.retryForce {
		return node.evaluated, nil
	}
	e.stats.Forces++
	var result *SparseTable
	var err error
	switch node.kind {
	case LazyTableJoin:
		if node.left.signature.Len() == 0 || node.right.signature.Len() == 0 {
			result, err = e.evalSparseJoinWithNullary(node)
			break
		}
		left, leftErr := e.eval(node.left)
		if leftErr != nil {
			return nil, leftErr
		}
		right, rightErr := e.eval(node.right)
		if rightErr != nil {
			return nil, rightErr
		}
		e.stats.Joins++
		result, err = left.Join(right, node.columns1, node.columns2)
	case LazyTableProject:
		if node.left != nil && node.left.kind == LazyTableJoin && node.left.left.signature.Len() != 0 && node.left.right.signature.Len() != 0 {
			join := node.left
			left, leftErr := e.eval(join.left)
			if leftErr != nil {
				return nil, leftErr
			}
			right, rightErr := e.eval(join.right)
			if rightErr != nil {
				return nil, rightErr
			}
			e.stats.JoinProjects++
			result, err = left.JoinProject(right, join.columns1, join.columns2, node.columns1)
		} else if node.left != nil && node.left.kind == LazyTableFilterEqual && len(node.columns1) == 1 && node.columns1[0] == node.left.column {
			result, err = evalLazySparseEqualProject(e, node.left)
			if err == nil {
				e.stats.SelectEqualProject++
			}
		} else if node.left != nil && node.left.kind == LazyTableFilterInterpreted {
			result, err = evalLazySparseInterpretedProject(e, node.left, node.columns1)
			if err == nil {
				e.stats.InterpretedProjects++
			}
		} else {
			source, sourceErr := e.eval(node.left)
			if sourceErr != nil {
				return nil, sourceErr
			}
			e.stats.Projects++
			result, err = source.Project(node.columns1)
		}
	case LazyTableRename:
		source, sourceErr := e.eval(node.left)
		if sourceErr != nil {
			return nil, sourceErr
		}
		result, err = source.Rename(node.columns1)
	case LazyTableFilterEqual:
		result, err = evalLazySparseEqual(e, node)
		if err == nil {
			e.stats.EqualFilters++
		}
	case LazyTableFilterIdentical:
		result, err = evalLazySparseIdentical(e, node)
		if err == nil {
			e.stats.IdenticalFilters++
		}
	case LazyTableFilterInterpreted:
		result, err = evalLazySparseInterpreted(e, node)
		if err == nil {
			e.stats.InterpretedFilters++
		}
	case LazyTableFilterByNegation:
		result, err = evalLazySparseNegation(e, node)
		if err == nil {
			e.stats.NegationFilters++
		}
	default:
		return nil, ErrLazySparse
	}
	if err != nil {
		return nil, err
	}
	node.evaluated = result
	node.retryForce = false
	return result, nil
}

func (t *LazySparseTable) eval() (*SparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.engine.eval(t.root)
}

func (t *LazySparseTable) finiteEval() (FiniteTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.engine.evalFiniteNode(t.root)
}

func (e *LazySparseEngine) evalSparseJoinWithNullary(node *lazySparseNode) (*SparseTable, error) {
	var zero *lazySparseNode
	var other *lazySparseNode
	if node.left.signature.Len() == 0 {
		zero, other = node.left, node.right
	} else {
		zero, other = node.right, node.left
	}
	truth, err := e.evalFiniteNode(zero)
	if err != nil {
		return nil, err
	}
	length, err := truth.Len()
	if err != nil {
		return nil, err
	}
	if length == 0 {
		e.stats.Joins++
		return NewSparseTable(node.signature, node.functional)
	}
	source, err := e.eval(other)
	if err != nil {
		return nil, err
	}
	e.stats.Joins++
	return source.Clone()
}

func (e *LazySparseEngine) evalFiniteNode(node *lazySparseNode) (FiniteTable, error) {
	if err := e.open(); err != nil || node == nil {
		return nil, ErrLazySparse
	}
	if node.nullary != nil {
		return node.nullary, nil
	}
	if node.signature.Len() != 0 {
		return e.eval(node)
	}
	e.stats.Forces++
	nonempty := false
	switch node.kind {
	case LazyTableBase:
		return nil, ErrLazySparse
	case LazyTableJoin:
		left, err := e.evalFiniteNode(node.left)
		if err != nil {
			return nil, err
		}
		right, err := e.evalFiniteNode(node.right)
		if err != nil {
			return nil, err
		}
		leftLength, err := left.Len()
		if err != nil {
			return nil, err
		}
		rightLength, err := right.Len()
		if err != nil {
			return nil, err
		}
		nonempty = leftLength != 0 && rightLength != 0
		e.stats.Joins++
	case LazyTableProject:
		if node.left == nil {
			return nil, ErrLazySparse
		}
		if node.left.signature.Len() == 0 {
			source, err := e.evalFiniteNode(node.left)
			if err != nil {
				return nil, err
			}
			length, err := source.Len()
			if err != nil {
				return nil, err
			}
			nonempty = length != 0
			e.stats.Projects++
		} else if node.left.kind == LazyTableJoin && node.left.left.signature.Len() != 0 && node.left.right.signature.Len() != 0 {
			join := node.left
			left, err := e.eval(join.left)
			if err != nil {
				return nil, err
			}
			right, err := e.eval(join.right)
			if err != nil {
				return nil, err
			}
			joined, err := left.JoinProject(right, join.columns1, join.columns2, node.columns1)
			if err != nil {
				return nil, err
			}
			length, lenErr := joined.Len()
			closeErr := joined.Close()
			if lenErr != nil || closeErr != nil {
				return nil, errors.Join(lenErr, closeErr)
			}
			nonempty = length != 0
			e.stats.JoinProjects++
		} else {
			source, err := e.evalFiniteNode(node.left)
			if err != nil {
				return nil, err
			}
			length, err := source.Len()
			if err != nil {
				return nil, err
			}
			nonempty = length != 0
			e.stats.Projects++
		}
	case LazyTableRename:
		source, err := e.evalFiniteNode(node.left)
		if err != nil {
			return nil, err
		}
		length, err := source.Len()
		if err != nil {
			return nil, err
		}
		nonempty = length != 0
	case LazyTableFilterIdentical, LazyTableFilterInterpreted, LazyTableFilterByNegation:
		result, err := evalLazySparseNullaryFilter(e, node)
		if err != nil {
			return nil, err
		}
		node.nullary = result
		switch node.kind {
		case LazyTableFilterIdentical:
			e.stats.IdenticalFilters++
		case LazyTableFilterInterpreted:
			e.stats.InterpretedFilters++
		case LazyTableFilterByNegation:
			e.stats.NegationFilters++
		}
		return result, nil
	default:
		return nil, ErrLazySparse
	}
	signature, _ := NewSignature(nil)
	result := NewHashTable(signature)
	if nonempty {
		if _, err := result.Add(Fact{}); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	node.nullary = result
	return result, nil
}
