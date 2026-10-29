package pdr

// Source: src/muz/pdr/pdr_reachable_cache.h and
// src/muz/pdr/pdr_reachable_cache.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type ReachableCacheMode uint32

const (
	ReachableNoCache ReachableCacheMode = iota
	ReachableHashCache
	ReachableConstraintCache
)

type ReachableCacheStatistics struct {
	Hits    uint32
	Misses  uint32
	Inserts uint32
}

type ReachableCache struct {
	manager   *ast.ASTManager
	context   *SMTContext
	mode      ReachableCacheMode
	connector ast.ASTRef
	cache     map[ast.ASTHandle]ast.ASTRef
	stats     ReachableCacheStatistics
	closed    bool
}

func NewReachableCache(manager *ast.ASTManager, contexts *SMTContextManager, background ast.ASTHandle, mode ReachableCacheMode) (*ReachableCache, error) {
	if manager == nil || contexts == nil || mode > ReachableConstraintCache {
		return nil, ErrSMTContextOwner
	}
	c := &ReachableCache{manager: manager, mode: mode, cache: make(map[ast.ASTHandle]ast.ASTRef)}
	if mode == ReachableConstraintCache {
		var err error
		c.context, err = contexts.Fresh()
		if err != nil {
			return nil, err
		}
		if err = c.context.Assert(background); err != nil {
			return nil, errors.Join(err, c.context.Close())
		}
	}
	return c, nil
}

func (c *ReachableCache) open() error {
	if c == nil || c.closed || c.manager == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (c *ReachableCache) addDisjunctedFormula(formula ast.ASTHandle) (resultErr error) {
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return err
	}
	newConnector, err := c.manager.MkFreshConst("disj_conn", boolean)
	if err != nil {
		return err
	}
	newRef, err := c.manager.NewRef(newConnector)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&newRef))
		}
	}()
	negated, err := c.manager.MkNot(newConnector)
	if err != nil {
		return err
	}
	arguments := []ast.ASTHandle{negated, formula}
	if c.connector != (ast.ASTRef{}) {
		previous, x := c.manager.RefGet(c.connector)
		if x != nil {
			return x
		}
		arguments = []ast.ASTHandle{previous, negated, formula}
	}
	extended, err := c.manager.MkOrArgs(arguments)
	if err != nil {
		return err
	}
	if err = c.context.Assert(extended); err != nil {
		return err
	}
	if c.connector != (ast.ASTRef{}) {
		if err = c.manager.DelRef(&c.connector); err != nil {
			return err
		}
	}
	c.connector = newRef
	committed = true
	return nil
}

func (c *ReachableCache) AddInit(formula ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.mode != ReachableConstraintCache {
		return nil
	}
	return c.addDisjunctedFormula(formula)
}

func (c *ReachableCache) AddReachable(cube ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	switch c.mode {
	case ReachableNoCache:
		return nil
	case ReachableHashCache:
		c.stats.Inserts++
		if _, found := c.cache[cube]; found {
			return nil
		}
		reference, err := c.manager.NewRef(cube)
		if err != nil {
			return err
		}
		c.cache[cube] = reference
		return nil
	case ReachableConstraintCache:
		c.stats.Inserts++
		return c.addDisjunctedFormula(cube)
	default:
		return fmt.Errorf("muz/pdr: unknown reachable-cache mode %d", c.mode)
	}
}

func (c *ReachableCache) IsReachable(cube ast.ASTHandle) (found bool, resultErr error) {
	if err := c.open(); err != nil {
		return false, err
	}
	switch c.mode {
	case ReachableNoCache:
		return false, nil
	case ReachableHashCache:
		_, found = c.cache[cube]
	case ReachableConstraintCache:
		if c.connector == (ast.ASTRef{}) {
			found = false
			break
		}
		connector, err := c.manager.RefGet(c.connector)
		if err != nil {
			return false, err
		}
		if err = c.context.Push(); err != nil {
			return false, err
		}
		defer func() { resultErr = errors.Join(resultErr, c.context.Pop()) }()
		if err = c.context.Assert(cube); err != nil {
			return false, err
		}
		status, err := c.context.Check([]ast.ASTHandle{connector})
		if err != nil {
			return false, err
		}
		found = status == lbool.True
	default:
		return false, fmt.Errorf("muz/pdr: unknown reachable-cache mode %d", c.mode)
	}
	if found {
		c.stats.Hits++
	} else {
		c.stats.Misses++
	}
	return found, nil
}

func (c *ReachableCache) Statistics() (ReachableCacheStatistics, error) {
	if err := c.open(); err != nil {
		return ReachableCacheStatistics{}, err
	}
	return c.stats, nil
}

func (c *ReachableCache) CollectStatistics(destination *statistics.Statistics) error {
	if err := c.open(); err != nil {
		return err
	}
	if destination == nil {
		return ErrSMTContextOwner
	}
	destination.UpdateUint("cache inserts", c.stats.Inserts)
	destination.UpdateUint("cache miss", c.stats.Misses)
	destination.UpdateUint("cache hits", c.stats.Hits)
	return nil
}

func (c *ReachableCache) ResetStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	c.stats = ReachableCacheStatistics{}
	return nil
}

func (c *ReachableCache) Close() (result error) {
	if c == nil || c.manager == nil {
		return ErrSMTContextOwner
	}
	if c.closed {
		return nil
	}
	if c.context != nil {
		result = errors.Join(result, c.context.Close())
	}
	if c.connector != (ast.ASTRef{}) {
		result = errors.Join(result, c.manager.DelRef(&c.connector))
	}
	for handle, reference := range c.cache {
		result = errors.Join(result, c.manager.DelRef(&reference))
		delete(c.cache, handle)
	}
	if result == nil {
		c.context = nil
		c.manager = nil
		c.closed = true
	}
	return result
}
