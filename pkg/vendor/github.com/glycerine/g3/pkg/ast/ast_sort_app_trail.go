package ast

// Source: src/ast/ast_trail.h.

type astSortAppTrailEntry struct {
	sort, app       ASTHandle
	sortRef, appRef ASTRef
	mapRemoved      bool
	sortReleased    bool
}

// ASTSortAppTrailMap is the concrete ast2ast_trailmap<sort, app>
// instantiation used by the retained array theory.
type ASTSortAppTrailMap struct {
	self    *ASTSortAppTrailMap
	manager *ASTManager
	values  map[ASTHandle]ASTHandle
	entries []astSortAppTrailEntry
	closed  bool
}

func NewASTSortAppTrailMap(m *ASTManager) (*ASTSortAppTrailMap, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	t := &ASTSortAppTrailMap{manager: m, values: make(map[ASTHandle]ASTHandle)}
	t.self = t
	return t, nil
}

func (t *ASTSortAppTrailMap) open() error {
	if t == nil || t.self != t || t.closed || t.manager == nil {
		return ErrASTHandle
	}
	_, err := t.manager.ProofsEnabled()
	return err
}

func (t *ASTSortAppTrailMap) Find(sort ASTHandle) (ASTHandle, bool, error) {
	if err := t.open(); err != nil {
		return ASTHandle{}, false, err
	}
	if kind, err := t.manager.GetKind(sort); err != nil {
		return ASTHandle{}, false, err
	} else if kind != ASTSort {
		return ASTHandle{}, false, ErrASTDomain
	}
	app, found := t.values[sort]
	return app, found, nil
}

func (t *ASTSortAppTrailMap) Insert(sort, app ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	if len(t.entries) != 0 && t.entries[len(t.entries)-1].mapRemoved {
		return ErrASTPending
	}
	if kind, err := t.manager.GetKind(sort); err != nil {
		return err
	} else if kind != ASTSort {
		return ErrASTDomain
	}
	if kind, err := t.manager.GetKind(app); err != nil {
		return err
	} else if kind != ASTApp {
		return ErrASTDomain
	}
	if _, found := t.values[sort]; found {
		return ErrASTDomain
	}
	sortRef, err := t.manager.NewRef(sort)
	if err != nil {
		return err
	}
	appRef, err := t.manager.NewRef(app)
	if err != nil {
		cleanup := t.manager.DelRef(&sortRef)
		if cleanup != nil {
			t.entries = append(t.entries, astSortAppTrailEntry{sort: sort, sortRef: sortRef, mapRemoved: true})
		}
		if cleanup != nil {
			return cleanup
		}
		return err
	}
	t.values[sort] = app
	t.entries = append(t.entries, astSortAppTrailEntry{sort: sort, app: app, sortRef: sortRef, appRef: appRef})
	return nil
}

// Pop removes the newest mapping before releasing its owned domain and range,
// matching ast2ast_trailmap::pop.
func (t *ASTSortAppTrailMap) Pop() error {
	if err := t.open(); err != nil {
		return err
	}
	if len(t.entries) == 0 {
		return ErrASTDomain
	}
	e := &t.entries[len(t.entries)-1]
	if !e.mapRemoved {
		delete(t.values, e.sort)
		e.mapRemoved = true
	}
	if !e.sortReleased && e.sortRef != (ASTRef{}) {
		if err := t.manager.DelRef(&e.sortRef); err != nil {
			return err
		}
		e.sortReleased = true
	}
	if e.appRef != (ASTRef{}) {
		if err := t.manager.DelRef(&e.appRef); err != nil {
			return err
		}
	}
	t.entries[len(t.entries)-1] = astSortAppTrailEntry{}
	t.entries = t.entries[:len(t.entries)-1]
	return nil
}

func (t *ASTSortAppTrailMap) Close() error {
	if t == nil || t.self != t {
		return ErrASTHandle
	}
	if t.closed {
		return nil
	}
	for len(t.entries) != 0 {
		if err := t.Pop(); err != nil {
			return err
		}
	}
	t.closed = true
	return nil
}

// ASTSortAppTrail performs the concrete trail constructor insertion and one
// idempotent undo.
type ASTSortAppTrail struct {
	table *ASTSortAppTrailMap
	done  bool
}

func NewASTSortAppTrail(table *ASTSortAppTrailMap, sort, app ASTHandle) (*ASTSortAppTrail, error) {
	if table == nil {
		return nil, ErrASTHandle
	}
	if err := table.Insert(sort, app); err != nil {
		return nil, err
	}
	return &ASTSortAppTrail{table: table}, nil
}

func (t *ASTSortAppTrail) Undo() error {
	if t == nil || t.table == nil {
		return ErrASTHandle
	}
	if t.done {
		return nil
	}
	if err := t.table.Pop(); err != nil {
		return err
	}
	t.done = true
	return nil
}
