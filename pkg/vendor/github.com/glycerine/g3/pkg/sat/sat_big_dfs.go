package sat

// Source: src/sat/sat_big.cpp:94-158.

type satBIGFrame struct{ parent, child Literal }

func (b *satBIG) initDFSNumbers() error {
	n := int(b.numLiterals())
	b.left = make([]int32, n)
	b.right = make([]int32, n)
	b.root = make([]Literal, n)
	b.parent = make([]Literal, n)
	for i := 0; i < n; i++ {
		b.right[i] = -1
		b.root[i] = Literal(i)
		b.parent[i] = Literal(i)
	}
	todo := make([]satBIGFrame, 0, n)
	for i, root := range b.roots {
		if root {
			todo = append(todo, satBIGFrame{DefaultLiteral(), Literal(i)})
		}
	}
	for i := len(todo) - 1; i > 0; i-- {
		j := int(b.random.Next()) % (i + 1)
		todo[i], todo[j] = todo[j], todo[i]
	}
	var dfs int32
	for len(todo) != 0 {
		last := len(todo) - 1
		f := todo[last]
		u := f.child
		if b.left[u.index()] > 0 {
			if b.right[u.index()] < 0 {
				dfs++
				b.right[u.index()] = dfs
			}
			todo = todo[:last]
			continue
		}
		dfs++
		b.left[u.index()] = dfs
		if f.parent != DefaultLiteral() {
			b.root[u.index()] = b.root[f.parent.index()]
			b.parent[u.index()] = f.parent
		}
		for _, v := range b.dag[u.index()] {
			if b.left[v.index()] == 0 {
				todo = append(todo, satBIGFrame{u, v})
			}
		}
	}
	for i := 0; i < n; i++ {
		if b.right[i] < 0 {
			dfs++
			b.left[i] = dfs
			dfs++
			b.right[i] = dfs
		}
	}
	return nil
}
