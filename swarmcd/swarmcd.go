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

	stateMu.Lock()
	if err != nil {
		stackStatus[swarmStack.name].Error = err.Error()
		stateMu.Unlock()
		logger.Error(err.Error())
		return
	}

	stackStatus[swarmStack.name].Error = ""
	stackStatus[swarmStack.name].Revision = revision
	stateMu.Unlock()
	logger.Info(fmt.Sprintf("done updating %s stack", swarmStack.name))
}

// GetStackStatus returns a snapshot of all stack statuses under a read lock.
func GetStackStatus() map[string]*StackStatus {
	stateMu.RLock()
	defer stateMu.RUnlock()
	snapshot := make(map[string]*StackStatus, len(stackStatus))
	for k, v := range stackStatus {
		cp := *v
		snapshot[k] = &cp
	}
	return snapshot
}
