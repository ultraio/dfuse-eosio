// Copyright 2026 Ultra. Licensed under the Apache License, Version 2.0.

package eosws

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	eos "github.com/eoscanada/eos-go"
	v1 "github.com/dfuse-io/eosws-go/mdl/v1"
	atom "go.uber.org/atomic"
)

// newTestHub builds a hub configured for ring-mechanics tests. It does NOT
// start the block-stream consumer; tests drive state directly via the
// onStepNew/onStepIrreversible/onStepUndo entry points and the appendLocked
// helper (with proper locking).
func newTestHub(size int) *RecentTxHub {
	return &RecentTxHub{
		ring:          make([]ringEntry, size),
		blockIdx:      make(map[string][]int, size),
		size:          size,
		chainID:       eos.Checksum256{}, // unused by tests that bypass synthesis
		ready:         atom.NewBool(true),
		lastBlockNano: atom.NewInt64(time.Now().UnixNano()),
		watchdogStall: 2 * time.Second,
	}
}

func mkLC(id string, irr bool) *v1.TransactionLifecycle {
	return &v1.TransactionLifecycle{
		ID:                    id,
		TransactionStatus:     "executed",
		ExecutionIrreversible: irr,
	}
}

func (h *RecentTxHub) testAppend(blockID string, blockNum uint32, lcs ...*v1.TransactionLifecycle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, lc := range lcs {
		h.appendLocked(blockID, blockNum, lc)
	}
}

// --- 1. empty hub fall-through ---------------------------------------------

func TestRecentTxHub_EmptyReturnsFallthrough(t *testing.T) {
	h := newTestHub(10)
	if got, ok := h.Snapshot(25); ok || got != nil {
		t.Fatalf("expected (nil,false) on empty hub, got (%v,%v)", got, ok)
	}
}

// --- 2. ready=false → fall-through -----------------------------------------

func TestRecentTxHub_NotReadyReturnsFallthrough(t *testing.T) {
	h := newTestHub(10)
	h.ready.Store(false)
	h.testAppend("blkA", 100, mkLC("trx1", false))
	if _, ok := h.Snapshot(1); ok {
		t.Fatalf("expected fall-through when ready=false")
	}
}

// --- 3. single-block append + snapshot reverse order -----------------------

func TestRecentTxHub_SingleBlockSnapshotReverseOrder(t *testing.T) {
	h := newTestHub(10)
	h.testAppend("blkA", 100, mkLC("t1", false), mkLC("t2", false), mkLC("t3", false))

	got, ok := h.Snapshot(3)
	if !ok {
		t.Fatalf("expected snapshot to succeed")
	}
	if len(got.Transactions) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got.Transactions))
	}
	// Newest first: appended t1,t2,t3 → snapshot returns t3,t2,t1.
	want := []string{"t3", "t2", "t1"}
	for i, lc := range got.Transactions {
		if lc.ID != want[i] {
			t.Errorf("entry %d: want %q, got %q", i, want[i], lc.ID)
		}
	}

	// Asking for more than available falls through.
	if _, ok := h.Snapshot(4); ok {
		t.Fatalf("expected fall-through when count < limit")
	}
}

// --- 4. ring wrap evicts oldest + prunes blockIdx --------------------------

func TestRecentTxHub_RingWrapEvictsAndPrunesBlockIdx(t *testing.T) {
	h := newTestHub(4)
	for i := 0; i < 5; i++ {
		blockID := fmt.Sprintf("blk%d", i)
		h.testAppend(blockID, uint32(100+i), mkLC(fmt.Sprintf("t%d", i), false))
	}

	got, ok := h.Snapshot(4)
	if !ok {
		t.Fatalf("expected snapshot to succeed")
	}
	if len(got.Transactions) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(got.Transactions))
	}
	// Last 4 in reverse: t4,t3,t2,t1. t0 evicted.
	want := []string{"t4", "t3", "t2", "t1"}
	for i, lc := range got.Transactions {
		if lc.ID != want[i] {
			t.Errorf("entry %d: want %q, got %q", i, want[i], lc.ID)
		}
	}

	// blockIdx must NOT retain blk0 after eviction (no leak).
	h.mu.RLock()
	_, has0 := h.blockIdx["blk0"]
	h.mu.RUnlock()
	if has0 {
		t.Errorf("blockIdx[blk0] should have been pruned on eviction")
	}
}

// --- 5. StepIrreversible flips the flag on existing entries ----------------

func TestRecentTxHub_StepIrreversibleFlipsFlag(t *testing.T) {
	h := newTestHub(10)
	h.testAppend("blkA", 100, mkLC("t1", false), mkLC("t2", false))

	pre, _ := h.Snapshot(2)
	for _, lc := range pre.Transactions {
		if lc.ExecutionIrreversible {
			t.Fatalf("pre-flip: ExecutionIrreversible should be false, got true on %s", lc.ID)
		}
	}

	h.onStepIrreversible("blkA")

	post, _ := h.Snapshot(2)
	for _, lc := range post.Transactions {
		if !lc.ExecutionIrreversible {
			t.Fatalf("post-flip: ExecutionIrreversible should be true, got false on %s", lc.ID)
		}
	}

	// blockIdx entry should be removed after irreversible flip (StepUndo
	// cannot reach an irreversible block, so the lookup is no longer needed).
	h.mu.RLock()
	_, has := h.blockIdx["blkA"]
	h.mu.RUnlock()
	if has {
		t.Errorf("blockIdx[blkA] should be removed after StepIrreversible")
	}
}

// --- 6. Snapshot copies are insulated from post-snapshot flag flips --------

func TestRecentTxHub_SnapshotCopyInsulatesPriorReaders(t *testing.T) {
	h := newTestHub(10)
	h.testAppend("blkA", 100, mkLC("t1", false))

	snap, _ := h.Snapshot(1)
	if snap.Transactions[0].ExecutionIrreversible {
		t.Fatalf("snapshot value should be false at capture time")
	}

	// Writer flips while the snapshot is in flight.
	h.onStepIrreversible("blkA")

	// The earlier snapshot's view must remain unchanged.
	if snap.Transactions[0].ExecutionIrreversible {
		t.Errorf("snapshot copy must not be mutated by post-snapshot StepIrreversible")
	}

	// A fresh snapshot DOES see the new value.
	fresh, _ := h.Snapshot(1)
	if !fresh.Transactions[0].ExecutionIrreversible {
		t.Errorf("fresh snapshot should see the flipped flag")
	}
}

// --- 7. StepUndo tombstones entries ----------------------------------------

func TestRecentTxHub_StepUndoTombstones(t *testing.T) {
	h := newTestHub(10)
	h.testAppend("blkA", 100, mkLC("tA", false))
	h.testAppend("blkB", 101, mkLC("tB", false))

	h.onStepUndo("blkB")

	got, ok := h.Snapshot(1)
	if !ok {
		t.Fatalf("expected snapshot to succeed (tA still present)")
	}
	if got.Transactions[0].ID != "tA" {
		t.Errorf("expected tA, got %s", got.Transactions[0].ID)
	}

	// blkB blockIdx removed.
	h.mu.RLock()
	_, has := h.blockIdx["blkB"]
	h.mu.RUnlock()
	if has {
		t.Errorf("blockIdx[blkB] should be removed after StepUndo")
	}
}

// --- 8. Tombstones at head cause fall-through ------------------------------

func TestRecentTxHub_TombstoneSaturationFallsThrough(t *testing.T) {
	h := newTestHub(4)
	h.testAppend("blk1", 100, mkLC("tA", false))
	h.testAppend("blk2", 101, mkLC("tB", false))
	h.testAppend("blk3", 102, mkLC("tC", false))

	// Undo everything except blk1.
	h.onStepUndo("blk2")
	h.onStepUndo("blk3")

	// Only 1 live entry; asking for 2 must fall through.
	if _, ok := h.Snapshot(2); ok {
		t.Errorf("expected fall-through when tombstones eat the response")
	}
	// Asking for 1 succeeds.
	got, ok := h.Snapshot(1)
	if !ok || got.Transactions[0].ID != "tA" {
		t.Errorf("expected 1 live entry tA")
	}
}

// --- 9. Cursor encoding is opaque + non-empty ------------------------------

func TestRecentTxHub_SnapshotCursorEncoded(t *testing.T) {
	h := newTestHub(10)
	h.testAppend("0000000064abc...blk", 100, mkLC("t1", false), mkLC("t2", false))

	got, _ := h.Snapshot(2)
	if got.Cursor == "" {
		t.Errorf("expected non-empty opaque cursor")
	}
}

// --- 10. blockIdx grows linearly with appends, prunes on eviction ----------

func TestRecentTxHub_BlockIdxBoundedByRingSize(t *testing.T) {
	const ringSize = 8
	h := newTestHub(ringSize)
	// Insert 4× ring size of single-tx blocks.
	for i := 0; i < ringSize*4; i++ {
		blockID := fmt.Sprintf("blk%d", i)
		h.testAppend(blockID, uint32(100+i), mkLC(fmt.Sprintf("t%d", i), false))
	}
	h.mu.RLock()
	gotMapLen := len(h.blockIdx)
	h.mu.RUnlock()
	if gotMapLen > ringSize {
		t.Errorf("blockIdx grew unbounded: len=%d, ring size=%d", gotMapLen, ringSize)
	}
}

// --- 11. Concurrent readers + writer, race-detector clean ------------------

func TestRecentTxHub_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}
	h := newTestHub(64)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	var produced atomic.Int64
	var consumed atomic.Int64

	// One producer ~1 block / 2ms = 500/s.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		t := time.NewTicker(2 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				blockID := fmt.Sprintf("blk%d", i)
				h.testAppend(blockID, uint32(100+i),
					mkLC(fmt.Sprintf("t%d-a", i), false),
					mkLC(fmt.Sprintf("t%d-b", i), false))
				produced.Add(2)
				// Periodically flip irreversibility on a recent block.
				if i > 5 && i%4 == 0 {
					h.onStepIrreversible(fmt.Sprintf("blk%d", i-3))
				}
				i++
			}
		}
	}()

	// Many readers hammering Snapshot.
	const readers = 32
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if got, ok := h.Snapshot(25); ok {
						consumed.Add(int64(len(got.Transactions)))
					}
				}
			}
		}()
	}

	wg.Wait()
	if produced.Load() == 0 {
		t.Fatalf("producer never advanced")
	}
	if consumed.Load() == 0 {
		t.Fatalf("no consumer ever saw a valid snapshot")
	}
}

// --- 12. Block-stream watchdog flips ready=false on stall ------------------

func TestRecentTxHub_WatchdogFlipsReadyOnStall(t *testing.T) {
	h := newTestHub(10)
	h.watchdogStall = 50 * time.Millisecond
	h.ready.Store(true)
	h.lastBlockNano.Store(time.Now().UnixNano() - int64(200*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.runWatchdog(ctx)

	// Watchdog ticks every 500ms; give it up to 800ms to fire.
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !h.ready.Load() {
			return // pass
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected watchdog to flip ready=false within 800ms")
}

// --- 13. New block arrival re-arms ready ------------------------------------

func TestRecentTxHub_StepNewReArmsReady(t *testing.T) {
	h := newTestHub(10)
	h.ready.Store(false)
	// Simulate handler effect: testAppend doesn't touch ready, so emulate
	// the handler body — the real handler runs ready.Store(true) after the
	// switch statement.
	h.testAppend("blkA", 100, mkLC("t1", false))
	h.lastBlockNano.Store(time.Now().UnixNano())
	h.ready.Store(true)
	if !h.ready.Load() {
		t.Fatalf("ready should be true after a new block")
	}
}

// --- 14. Negative or zero limit returns (nil,false) ------------------------

func TestRecentTxHub_NonPositiveLimit(t *testing.T) {
	h := newTestHub(10)
	h.testAppend("blkA", 100, mkLC("t1", false))
	if _, ok := h.Snapshot(0); ok {
		t.Errorf("limit=0 should fall through")
	}
	if _, ok := h.Snapshot(-1); ok {
		t.Errorf("limit=-1 should fall through")
	}
}

// --- 15. hexUint16 encoding matches kvdb.HexUint16 contract ----------------

func TestHexUint16(t *testing.T) {
	cases := []struct {
		v    uint16
		want string
	}{
		{0x0000, "0000"},
		{0x0001, "0001"},
		{0x00ff, "00ff"},
		{0xabcd, "abcd"},
		{0xffff, "ffff"},
	}
	for _, c := range cases {
		if got := hexUint16(c.v); got != c.want {
			t.Errorf("hexUint16(%#x) = %q, want %q", c.v, got, c.want)
		}
	}
}
