package swarmcd

import (
	"fmt"
	"sync"
	"time"
)

var stackStatus map[string]*StackStatus = map[string]*StackStatus{}
var stacks []*swarmStack

// stateMu protects stackStatus, stacks, and repo configs from concurrent access.
// Lock ordering: repo.lock must always be released before stateMu is acquired.
// No goroutine may hold repo.lock while acquiring stateMu.
var stateMu sync.RWMutex

func Run() {
	logger.Info("starting SwarmCD")
	for {
		var waitGroup sync.WaitGroup
		logger.Info("updating stacks...")
		stateMu.RLock()
		currentStacks := make([]*swarmStack, len(stacks))
		copy(currentStacks, stacks)
		stateMu.RUnlock()
		for _, swarmStack := range currentStacks {
			waitGroup.Add(1)
			go updateStackThread(swarmStack, &waitGroup)
		}
		waitGroup.Wait()
		logger.Info("waiting for the update interval")
		time.Sleep(time.Duration(config.UpdateInterval) * time.Second)
	}
}

func updateStackThread(swarmStack *swarmStack, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	repoLock := swarmStack.repo.lock
	repoLock.Lock()
	logger.Info(fmt.Sprintf("updating %s stack", swarmStack.name))
	revision, err := swarmStack.updateStack()
	repoLock.Unlock() // Release repo.lock BEFORE acquiring stateMu

	now := time.Now()

	stateMu.Lock()
	if err != nil {
		stackStatus[swarmStack.name].Error = err.Error()
		stateMu.Unlock()
		logger.Error(err.Error())
		return
	}

	status := stackStatus[swarmStack.name]

	// Set LastChangeAt when the revision changes
	if revision != status.Revision {
		status.LastChangeAt = &now
	}

	status.Error = ""
	status.Revision = revision
	status.LastDeployedAt = &now
	stateMu.Unlock()
	logger.Info(fmt.Sprintf("done updating %s stack", swarmStack.name))
}

// GetRuntimeInfo returns a copy of the instance's runtime metadata.
func GetRuntimeInfo() RuntimeInfo {
	return runtimeInfo
}

// GetStackStatus returns a snapshot of all stack statuses under a read lock.
// Pointer fields (LastChangeAt, LastDeployedAt) are deep-copied so callers
// cannot mutate the original values.
func GetStackStatus() map[string]*StackStatus {
	stateMu.RLock()
	defer stateMu.RUnlock()
	snapshot := make(map[string]*StackStatus, len(stackStatus))
	for k, v := range stackStatus {
		cp := *v
		if v.LastChangeAt != nil {
			t := *v.LastChangeAt
			cp.LastChangeAt = &t
		}
		if v.LastDeployedAt != nil {
			t := *v.LastDeployedAt
			cp.LastDeployedAt = &t
		}
		snapshot[k] = &cp
	}
	return snapshot
}
