package scheduler

// ScanObserver observes a whole scan, not individual task outcomes. Configure
// runners before Start; observers must be safe for concurrent metric scrapes.
type ScanObserver interface {
	StartScan() func(error)
}

func startScan(observer ScanObserver) func(error) {
	if observer == nil {
		return func(error) {}
	}
	return observer.StartScan()
}
