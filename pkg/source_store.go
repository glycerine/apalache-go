package apalache

import (
	"fmt"
	"math"
	"sync"
)

type regionNode struct {
	index    int
	region   SourceRegion
	children []*regionNode
}

// RegionTree stores regions according to proper nesting. Regions may be
// disjoint or contain one another, but partial overlap is rejected.
type RegionTree struct {
	root    *regionNode
	regions []SourceRegion
}

func NewRegionTree() *RegionTree {
	rootRegion := NewSourceRegion(0, 0, math.MaxInt, math.MaxInt)
	return &RegionTree{
		root:    &regionNode{index: 0, region: rootRegion},
		regions: []SourceRegion{rootRegion},
	}
}

func (tree *RegionTree) Add(region SourceRegion) (int, error) {
	newIndex := -1
	var add func(*regionNode) (*regionNode, error)
	add = func(node *regionNode) (*regionNode, error) {
		if region.IsInside(node.region) {
			contains := false
			for _, child := range node.children {
				if child.region.Contains(region) {
					contains = true
					break
				}
			}
			if contains {
				children := append([]*regionNode(nil), node.children...)
				for index, child := range children {
					updated, err := add(child)
					if err != nil {
						return nil, err
					}
					children[index] = updated
				}
				return &regionNode{index: node.index, region: node.region, children: children}, nil
			}
			for _, child := range node.children {
				if child.region.Intersects(region) && !child.region.IsInside(region) {
					return nil, fmt.Errorf("region %s intersects with %s", region, child.region)
				}
			}
			if region == node.region {
				newIndex = node.index
				return node, nil
			}
			newIndex = len(tree.regions)
			tree.regions = append(tree.regions, region)
			adopted := make([]*regionNode, 0)
			remaining := make([]*regionNode, 0)
			for _, child := range node.children {
				if child.region.IsInside(region) {
					adopted = append(adopted, child)
				} else {
					remaining = append(remaining, child)
				}
			}
			newChild := &regionNode{index: newIndex, region: region, children: adopted}
			children := append([]*regionNode{newChild}, remaining...)
			return &regionNode{index: node.index, region: node.region, children: children}, nil
		}
		if region.Intersects(node.region) {
			return nil, fmt.Errorf("region %s intersects with %s", region, node.region)
		}
		return node, nil
	}
	updated, err := add(tree.root)
	if err != nil {
		return 0, err
	}
	if newIndex <= 0 {
		return 0, fmt.Errorf("region %s is outside the source coordinate space", region)
	}
	tree.root = updated
	return newIndex, nil
}

func (tree *RegionTree) Region(index int) (SourceRegion, bool) {
	if index < 0 || index >= len(tree.regions) {
		return SourceRegion{}, false
	}
	return tree.regions[index], true
}
func (tree *RegionTree) Size() int { return len(tree.regions) - 1 }

type SourceMap map[UID]SourceLocation

type SourceStore struct {
	mutex           sync.RWMutex
	filenameToIndex map[string]int
	filenames       []string
	trees           []*RegionTree
	idToRegion      []map[UID]int
}

func NewSourceStore() *SourceStore {
	return &SourceStore{filenameToIndex: make(map[string]int)}
}

func (store *SourceStore) Add(id UID, location SourceLocation) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	filenameIndex, regionIndex, err := store.addRegionLocked(location)
	if err != nil {
		return err
	}
	store.idToRegion[filenameIndex][id] = regionIndex
	return nil
}

func (store *SourceStore) AddRecursive(root Expr, location SourceLocation) error {
	if root == nil {
		return fmt.Errorf("cannot locate a nil expression")
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	filenameIndex, regionIndex, err := store.addRegionLocked(location)
	if err != nil {
		return err
	}
	locations := store.idToRegion[filenameIndex]
	var add func(Expr)
	add = func(expr Expr) {
		if _, exists := locations[expr.ID()]; exists {
			return
		}
		locations[expr.ID()] = regionIndex
		if operation, ok := expr.(*OperExpr); ok {
			for _, argument := range operation.Arguments() {
				add(argument)
			}
		}
	}
	add(root)
	return nil
}

func (store *SourceStore) addRegionLocked(location SourceLocation) (int, int, error) {
	filenameIndex, exists := store.filenameToIndex[location.Filename]
	if !exists {
		filenameIndex = len(store.trees)
		store.filenameToIndex[location.Filename] = filenameIndex
		store.filenames = append(store.filenames, location.Filename)
		store.trees = append(store.trees, NewRegionTree())
		store.idToRegion = append(store.idToRegion, make(map[UID]int))
	}
	regionIndex, err := store.trees[filenameIndex].Add(location.Region)
	return filenameIndex, regionIndex, err
}

func (store *SourceStore) Find(id UID) (SourceLocation, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	for filenameIndex, locations := range store.idToRegion {
		regionIndex, exists := locations[id]
		if !exists {
			continue
		}
		region, ok := store.trees[filenameIndex].Region(regionIndex)
		if !ok {
			return SourceLocation{}, false
		}
		return SourceLocation{Filename: store.filenames[filenameIndex], Region: region}, true
	}
	return SourceLocation{}, false
}

func (store *SourceStore) Contains(id UID) bool {
	_, ok := store.Find(id)
	return ok
}

func (store *SourceStore) SourceMap() SourceMap {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	result := make(SourceMap)
	for filenameIndex, locations := range store.idToRegion {
		for id, regionIndex := range locations {
			region, ok := store.trees[filenameIndex].Region(regionIndex)
			if ok {
				result[id] = SourceLocation{Filename: store.filenames[filenameIndex], Region: region}
			}
		}
	}
	return result
}

func (store *SourceStore) OnExpressionTransformation(original, transformed Expr) {
	if original.ID() == transformed.ID() {
		return
	}
	if location, ok := store.Find(original.ID()); ok {
		if err := store.AddRecursive(transformed, location); err != nil {
			panic(err)
		}
	}
}
func (store *SourceStore) OnDeclarationTransformation(original, transformed Decl) {
	if original.ID() == transformed.ID() {
		return
	}
	if location, ok := store.Find(original.ID()); ok {
		if err := store.Add(transformed.ID(), location); err != nil {
			panic(err)
		}
	}
}

// StoreCopyListener copies only the transformed node's location. It exists for
// importer paths that perform their own recursive source bookkeeping.
type StoreCopyListener struct{ Store *SourceStore }

func (listener StoreCopyListener) OnExpressionTransformation(original, transformed Expr) {
	if original.ID() == transformed.ID() || listener.Store == nil {
		return
	}
	if location, ok := listener.Store.Find(original.ID()); ok {
		if err := listener.Store.Add(transformed.ID(), location); err != nil {
			panic(err)
		}
	}
}
func (listener StoreCopyListener) OnDeclarationTransformation(original, transformed Decl) {
	if original.ID() == transformed.ID() || listener.Store == nil {
		return
	}
	if location, ok := listener.Store.Find(original.ID()); ok {
		if err := listener.Store.Add(transformed.ID(), location); err != nil {
			panic(err)
		}
	}
}
