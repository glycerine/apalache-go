package params

func catalogInsert(d *ParamDescrs, key string, kind ParamKind, descr, def, module string) error {
	if module == "" {
		return d.InsertName(key, kind, &descr, &def, nil)
	}
	return d.InsertName(key, kind, &descr, &def, &module)
}
func catalogCollectEnvParams(d *ParamDescrs) error {
	if e := catalogInsert(d, "verbose", ParamUInt, "be verbose, where the value is the verbosity level", "0", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "warning", ParamBool, "enable/disable warning messages", "true", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "memory_max_size", ParamUInt, "set hard upper limit for memory consumption (in megabytes), if 0 then there is no limit", "0", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "memory_max_alloc_count", ParamUInt, "set hard upper limit for memory allocations, if 0 then there is no limit", "0", ""); e != nil {
		return e
	}
	if e := catalogInsert(d, "memory_high_watermark", ParamUInt, "set high watermark for memory consumption (in megabytes), if 0 then there is no limit", "0", ""); e != nil {
		return e
	}
	return nil
}
