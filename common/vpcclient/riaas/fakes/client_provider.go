// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/IBM/ibmcloud-volume-file-vpc/common/vpcclient/riaas"
)

type RegionalAPIClientProvider struct {
	NewStub        func(riaas.Config) (riaas.RegionalAPI, error)
	newMutex       sync.RWMutex
	newArgsForCall []struct {
		arg1 riaas.Config
	}
	newReturns struct {
		result1 riaas.RegionalAPI
		result2 error
	}
	newReturnsOnCall map[int]struct {
		result1 riaas.RegionalAPI
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *RegionalAPIClientProvider) New(arg1 riaas.Config) (riaas.RegionalAPI, error) {
	fake.newMutex.Lock()
	ret, specificReturn := fake.newReturnsOnCall[len(fake.newArgsForCall)]
	fake.newArgsForCall = append(fake.newArgsForCall, struct {
		arg1 riaas.Config
	}{arg1})
	stub := fake.NewStub
	fakeReturns := fake.newReturns
	fake.recordInvocation("New", []interface{}{arg1})
	fake.newMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *RegionalAPIClientProvider) NewCallCount() int {
	fake.newMutex.RLock()
	defer fake.newMutex.RUnlock()
	return len(fake.newArgsForCall)
}

func (fake *RegionalAPIClientProvider) NewCalls(stub func(riaas.Config) (riaas.RegionalAPI, error)) {
	fake.newMutex.Lock()
	defer fake.newMutex.Unlock()
	fake.NewStub = stub
}

func (fake *RegionalAPIClientProvider) NewArgsForCall(i int) riaas.Config {
	fake.newMutex.RLock()
	defer fake.newMutex.RUnlock()
	argsForCall := fake.newArgsForCall[i]
	return argsForCall.arg1
}

func (fake *RegionalAPIClientProvider) NewReturns(result1 riaas.RegionalAPI, result2 error) {
	fake.newMutex.Lock()
	defer fake.newMutex.Unlock()
	fake.NewStub = nil
	fake.newReturns = struct {
		result1 riaas.RegionalAPI
		result2 error
	}{result1, result2}
}

func (fake *RegionalAPIClientProvider) NewReturnsOnCall(i int, result1 riaas.RegionalAPI, result2 error) {
	fake.newMutex.Lock()
	defer fake.newMutex.Unlock()
	fake.NewStub = nil
	if fake.newReturnsOnCall == nil {
		fake.newReturnsOnCall = make(map[int]struct {
			result1 riaas.RegionalAPI
			result2 error
		})
	}
	fake.newReturnsOnCall[i] = struct {
		result1 riaas.RegionalAPI
		result2 error
	}{result1, result2}
}

func (fake *RegionalAPIClientProvider) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.newMutex.RLock()
	defer fake.newMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *RegionalAPIClientProvider) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ riaas.RegionalAPIClientProvider = new(RegionalAPIClientProvider)
