package rel

func AAGNegate(id uint32) uint32 { return id ^ 1 }
func AAGFalse() uint32           { return 0 }
func AAGTrue() uint32            { return 1 }
