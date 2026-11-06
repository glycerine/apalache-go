package rel

type sparseStorage struct {
	entrySize, uniqueSize int
	entries               [][]byte
	index                 map[string]int
}

func newSparseStorage(entry, functional uint32) *sparseStorage {
	return &sparseStorage{entrySize: int(entry), uniqueSize: int(entry - functional), index: make(map[string]int)}
}
func (s *sparseStorage) key(data []byte) string { return string(data[:s.uniqueSize]) }
func (s *sparseStorage) insert(data []byte) (bool, int) {
	k := s.key(data)
	if i, ok := s.index[k]; ok {
		return false, i
	}
	copyData := append([]byte(nil), data...)
	i := len(s.entries)
	s.entries = append(s.entries, copyData)
	s.index[k] = i
	return true, i
}
func (s *sparseStorage) find(data []byte) (int, bool) { i, ok := s.index[s.key(data)]; return i, ok }
func (s *sparseStorage) remove(data []byte) bool {
	i, ok := s.find(data)
	if !ok {
		return false
	}
	delete(s.index, s.key(s.entries[i]))
	last := len(s.entries) - 1
	if i != last {
		s.entries[i] = s.entries[last]
		s.index[s.key(s.entries[i])] = i
	}
	s.entries = s.entries[:last]
	return true
}
func (s *sparseStorage) reset() { s.entries = nil; s.index = make(map[string]int) }
func (s *sparseStorage) clone() *sparseStorage {
	r := newSparseStorage(uint32(s.entrySize), uint32(s.entrySize-s.uniqueSize))
	for _, e := range s.entries {
		r.insert(e)
	}
	return r
}
