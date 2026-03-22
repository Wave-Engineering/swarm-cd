package swarmcd

// SetStackStatusForTest replaces the package-level stackStatus map with the
// provided data. It is intended for use in tests only. The caller is
// responsible for restoring the original state after the test.
func SetStackStatusForTest(data map[string]*StackStatus) {
	stateMu.Lock()
	defer stateMu.Unlock()
	stackStatus = data
}
