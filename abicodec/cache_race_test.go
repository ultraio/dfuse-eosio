// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package abicodec

import (
	"sync"
	"testing"
)

// TestDefaultCache_ConcurrentReadWrite reproduces the abicodec ABI-cache data race.
//
// The decode hot path (decodeAction/decodeTable/getABI -> Cache.ABIAtBlockNum) read
// the Abis map and iterated the per-account slice with NO lock, while setabi
// processing writes the same map/slice (SetABIAtBlockNum) under c.lock. On the
// single-replica abicodec pod this surfaces as a fatal "concurrent map read and map
// write" runtime crash. Must be run under `-race`.
func TestDefaultCache_ConcurrentReadWrite(t *testing.T) {
	cache := &DefaultCache{Abis: make(map[string][]*ABICacheItem)}
	abi := NewTestABI("v1")

	const account = "eosio.token"
	cache.SetABIAtBlockNum(account, 1, abi)

	const iterations = 5000
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: append/replace ABI versions (mutates the map and reslices the slice).
	go func() {
		defer wg.Done()
		for i := uint32(2); i < iterations; i++ {
			cache.SetABIAtBlockNum(account, i, abi)
		}
	}()

	// Reader: the lock-free path used by every decode/getABI RPC.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = cache.ABIAtBlockNum(account, ^uint32(0))
		}
	}()

	wg.Wait()
}
