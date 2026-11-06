package api

// Source: generated api_commands.cpp and src/api/api_stats.cpp.

import (
	"errors"
	"reflect"
)

type replayStatistics struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIStatistics
}

func (s *replayStatistics) apiDestroy() error { return s.value.Close() }
func registerZ3GeneratedStatistics(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{435, "Z3_solver_get_statistics"}, {437, "Z3_stats_to_string"}, {438, "Z3_stats_inc_ref"}, {439, "Z3_stats_dec_ref"},
		{440, "Z3_stats_size"}, {441, "Z3_stats_get_key"}, {442, "Z3_stats_is_uint"}, {443, "Z3_stats_is_double"},
		{444, "Z3_stats_get_uint_value"}, {445, "Z3_stats_get_double_value"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayStatisticsCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreStatistics(r *Z3Replayer, owner *replayContextOwner, value *APIStatistics) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayStatistics{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	s := &replayStatistics{owner: owner, value: value}
	s.object = apiObject{context: owner.results, destructor: s, ownerBytes: n}
	s.object.self = &s.object
	id, err := owner.results.addObject(&s.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	s.object.id = id
	if err = owner.results.saveObject(&s.object); err != nil {
		return err
	}
	if r.statistics == nil {
		r.statistics = make(map[uint64]*replayStatistics)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.statistics[replayID] = s
	owner.statisticIDs = append(owner.statisticIDs, replayID)
	return r.StoreResult(replayID)
}
func replayStatisticsCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 435 {
		solver, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		value, e := solver.value.Statistics()
		if e != nil {
			return e
		}
		return replayStoreStatistics(r, owner, value)
	}
	object, err := r.Object(1)
	if err != nil {
		return err
	}
	if object == 0 && (id == 438 || id == 439) {
		return nil
	}
	stats := r.statistics[object]
	if stats == nil || stats.owner != owner || stats.object.open() != nil {
		return ErrZ3ReplayReference
	}
	if id == 438 {
		return stats.object.incRef()
	}
	if id == 439 {
		return stats.object.decRef()
	}
	switch id {
	case 437:
		_, err = stats.value.String()
	case 440:
		_, err = stats.value.Size()
	default:
		var index uint32
		index, err = r.Uint(2)
		if err != nil {
			return err
		}
		switch id {
		case 441:
			_, err = stats.value.Key(index)
		case 442:
			_, err = stats.value.IsUint(index)
		case 443:
			_, err = stats.value.IsDouble(index)
		case 444:
			_, err = stats.value.UintValue(index)
		case 445:
			_, err = stats.value.DoubleValue(index)
		}
	}
	return err
}
