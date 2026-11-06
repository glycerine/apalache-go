package realclosure

// Source: src/math/realclosure/realclosure.cpp:724-727, 1808-1835.

import "github.com/glycerine/g3/pkg/arena"

func (o *algebraicRootOwner) registerAlgebraicRoot(root *algebraicRoot) error {
	index, err := o.nextAlgebraicIndex()
	if err != nil {
		return err
	}
	root.index = index
	o.roots = append(o.roots, root)
	return nil
}

func (o *algebraicRootOwner) registerExtension(root *algebraicRoot, kind realExtensionKind) error {
	if o == nil || root == nil || root.owner != o || kind == realExtensionAlgebraic {
		return arena.ErrInvalidSlot
	}
	registry := o.registry(kind)
	for len(*registry) > 0 && (*registry)[len(*registry)-1] == nil {
		*registry = (*registry)[:len(*registry)-1]
	}
	root.kind = kind
	root.index = len(*registry)
	*registry = append(*registry, root)
	return nil
}
