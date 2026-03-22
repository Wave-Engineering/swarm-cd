package swarmcd

// SetStackStatusForTest replaces the internal stackStatus map with the given
// test data and returns a cleanup function that restores the original state.
// This is intended for use in tests only.
func SetStackStatusForTest(testStatus map[string]*StackStatus) func() {
	stateMu.Lock()
	oldStatus := stackStatus
	oldStacks := stacks
	stackStatus = testStatus
	stacks = nil
	stateMu.Unlock()

	return func() {
		stateMu.Lock()
		stackStatus = oldStatus
		stacks = oldStacks
		stateMu.Unlock()
	}
}

// SetRuntimeInfoForTest replaces the internal runtimeInfo with the given
// test data and returns a cleanup function that restores the original state.
// This is intended for use in tests only.
func SetRuntimeInfoForTest(info RuntimeInfo) func() {
	old := runtimeInfo
	runtimeInfo = info
	return func() {
		runtimeInfo = old
	}
}
