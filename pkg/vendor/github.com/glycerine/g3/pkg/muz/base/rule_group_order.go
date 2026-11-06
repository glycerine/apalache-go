package base

// Source: rule_set::add_rule and obj_map's core_hashtable traversal.
import "github.com/glycerine/g3/pkg/ast"

type ruleGroupBucket struct {
	predicate ast.ASTHandle
	hash      uint32
}

// resetGroups follows core_hashtable::reset, preserving capacity unless the
// source's sparse-table condition shrinks it by one power of two.
func (s *RuleSet) resetGroups() {
	if s.groupCount == 0 {
		return
	}
	capacity := len(s.groupBuckets)
	if capacity > 16 && (capacity-s.groupCount)*4 > capacity*3 {
		capacity /= 2
	}
	s.groupBuckets = make([]ruleGroupBucket, capacity)
	s.groupCount = 0
}

// insertGroup mirrors the source's concrete predicate-group table. Entries
// remain after deleting the last rule, as they do in rule_set::del_rule.
func (s *RuleSet) insertGroup(predicate ast.ASTHandle, hash uint32) {
	if len(s.groupBuckets) == 0 {
		s.groupBuckets = make([]ruleGroupBucket, 8)
	}
	// The source checks occupancy even for an already present key.
	if s.groupCount*4 > len(s.groupBuckets)*3 {
		old := s.groupBuckets
		s.groupBuckets = make([]ruleGroupBucket, len(old)*2)
		for _, entry := range old {
			if !entry.predicate.IsNull() {
				placeRuleGroup(s.groupBuckets, entry)
			}
		}
	}
	if placeRuleGroup(s.groupBuckets, ruleGroupBucket{predicate, hash}) {
		s.groupCount++
	}
}

func placeRuleGroup(buckets []ruleGroupBucket, entry ruleGroupBucket) bool {
	for i := int(entry.hash) & (len(buckets) - 1); ; i = (i + 1) & (len(buckets) - 1) {
		if buckets[i].predicate.IsNull() {
			buckets[i] = entry
			return true
		}
		if buckets[i].predicate == entry.predicate {
			return false
		}
	}
}

// GroupedPredicates returns the source grouped-map traversal, including empty
// groups left by removal. The declarations are borrowed from the rule manager.
func (s *RuleSet) GroupedPredicates() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, 0, s.groupCount)
	for _, entry := range s.groupBuckets {
		if !entry.predicate.IsNull() {
			result = append(result, entry.predicate)
		}
	}
	return result, nil
}
