package subscriber

// notLocker is a sync.Locker whose Lock and Unlock methods are nops.
type notLocker struct{}

func (n notLocker) Lock()   {}
func (n notLocker) Unlock() {}
