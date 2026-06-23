// Copyright 2026 Ultra. Licensed under the Apache License, Version 2.0.

package eosws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dfuse-io/dfuse-eosio/codec"
	codec_eosio "github.com/dfuse-io/dfuse-eosio/codec/eosio"
	"github.com/dfuse-io/dfuse-eosio/eosws/mdl"
	pbcodec "github.com/dfuse-io/dfuse-eosio/pb/dfuse/eosio/codec/v1"
	v1 "github.com/dfuse-io/eosws-go/mdl/v1"
	eos "github.com/eoscanada/eos-go"
	"github.com/streamingfast/bstream"
	"github.com/streamingfast/bstream/forkable"
	"github.com/streamingfast/bstream/hub"
	"github.com/streamingfast/dmetrics"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// RecentTxHub maintains an in-memory ring buffer of the most recently observed
// transactions, populated from the same live block stream eosws already
// subscribes to. It exists to short-circuit the heavy Bigtable reverse-scan
// served today by db.ListMostRecentTransactions when answering the eosq home
// page's /v0/transactions?cursor=&limit=N request.
//
// The hub stores already-converted *v1.TransactionLifecycle values, built via
// the same pbcodec.MergeTransactionEvents + mdl.ToV1TransactionLifecycle
// pipeline the Bigtable path uses. For canonical (non-deferred) Ultra
// transactions this guarantees byte-identical JSON output.
//
// Concurrency: a sync.RWMutex protects the ring storage. The block-stream
// consumer takes the write lock for a sub-millisecond append; REST readers
// take the read lock for the duration of a shallow per-entry copy. Expensive
// proto→v1 conversion runs OUTSIDE the lock so the producer never blocks
// on slow readers.
type RecentTxHub struct {
	mu       sync.RWMutex
	ring     []ringEntry
	head     int // next write index (modular)
	count    int // number of populated entries; <= size after warmup
	blockIdx map[string][]int

	size              int
	chainID           eos.Checksum256
	initialStartBlock string
	initialLIB        string
	subscriptionHub   *hub.SubscriptionHub

	ready         *atomic.Bool
	lastBlockNano *atomic.Int64 // unix nano of last observed block; watchdog gate
	watchdogStall time.Duration
}

type ringEntry struct {
	blockID  string
	blockNum uint32
	lc       *v1.TransactionLifecycle
}

// Prometheus counters scoped to the recent-tx hub. Registered in metrics.Metricset
// so they appear alongside the existing eosws metrics on the eosws Prometheus
// endpoint.
var (
	recentTxRingHits      = recentTxMetricset.NewCounter("eosws_recent_tx_ring_hits", "Number of /v0/transactions requests served from the in-memory ring buffer")
	recentTxRingMisses    = recentTxMetricset.NewCounter("eosws_recent_tx_ring_misses", "Number of /v0/transactions requests that fell through to Bigtable")
	recentTxRingUndos     = recentTxMetricset.NewCounter("eosws_recent_tx_ring_undos", "Number of ring entries tombstoned because their block was reverted")
	recentTxRingEvictions = recentTxMetricset.NewCounter("eosws_recent_tx_ring_evictions", "Number of ring entries evicted by ring wrap")
	recentTxRingSize      = recentTxMetricset.NewGauge("eosws_recent_tx_ring_size", "Current populated entry count in the recent-tx ring buffer")
)

// recentTxMetricset is registered alongside the existing eosws metricset in
// app.Run() via dmetrics.Register. We use a dedicated set rather than the
// shared eosws metricset so the counters live with the rest of this feature.
var recentTxMetricset = dmetrics.NewSet()

// RecentTxMetricset is exported so app.Run() can register it with the global
// dmetrics registry alongside the existing eosws metricset.
func RecentTxMetricset() *dmetrics.Set { return recentTxMetricset }

// NewRecentTxHub constructs a hub of the given ring size. The chainID is used
// to recover signing public keys; it is typically obtained at boot via the
// eosws nodeos RPC client's GetInfo call. A size of zero or negative
// disables the hub entirely; callers should pass nil to ListTransactionsHandler
// in that case.
func NewRecentTxHub(size int, chainID eos.Checksum256, initialStartBlock, initialLIB string, subscriptionHub *hub.SubscriptionHub) *RecentTxHub {
	if size <= 0 {
		panic("recent-tx hub size must be positive")
	}
	return &RecentTxHub{
		ring:              make([]ringEntry, size),
		blockIdx:          make(map[string][]int, size),
		size:              size,
		chainID:           chainID,
		initialStartBlock: initialStartBlock,
		initialLIB:        initialLIB,
		subscriptionHub:   subscriptionHub,
		ready:             atomic.NewBool(false),
		lastBlockNano:     atomic.NewInt64(0),
		watchdogStall:     2 * time.Second, // 2× nominal 0.5 s/block
	}
}

// IsReady returns true once the hub has processed at least one StepNew block
// AND the watchdog has not flagged a stalled block stream. REST callers gate
// the fast path on this.
func (h *RecentTxHub) IsReady() bool { return h.ready.Load() }

// Launch starts the long-running goroutine that consumes the block stream.
// It mirrors HeadInfoHub.Launch in shape (joining source + forkable +
// eternal source) but uses no block-num gate filter — we need StepNew,
// StepRedo, StepIrreversible AND StepUndo.
func (h *RecentTxHub) Launch(ctx context.Context) {
	libRef := bstream.NewBlockRefFromID(h.initialLIB)

	handler := bstream.HandlerFunc(func(block *bstream.Block, obj interface{}) error {
		// Stamp the watchdog NOW (block received) rather than after the
		// switch so a slow onStepNew on a heavy block doesn't trigger a
		// false stall flip (R1 #4). Watchdog measures stream silence, not
		// per-block processing time.
		h.lastBlockNano.Store(time.Now().UnixNano())

		fObj := obj.(*forkable.ForkableObject)
		blk := block.ToNative().(*pbcodec.Block)

		switch fObj.Step {
		case forkable.StepNew, forkable.StepRedo:
			h.onStepNew(blk)
		case forkable.StepIrreversible:
			h.onStepIrreversible(blk.Id)
		case forkable.StepUndo:
			h.onStepUndo(blk.Id)
		}
		h.ready.Store(true)
		return nil
	})

	// Bound the reversible buffer: this hub is long-lived (per-pod) and runs on real
	// chain LIB, so a LIB stall would grow it unbounded -> OOM. Fail fast instead
	// (clean restart) — see ultraOS-doc dfuse-deep-dive/17 (B1). Tune per pod memory.
	forkableHandler := forkable.New(handler, forkable.WithLogger(zlog), forkable.WithExclusiveLIB(libRef), forkable.WithMaxReversibleBlocks(10_000))

	joiningSourceFactory := bstream.SourceFromRefFactory(func(blockRef bstream.BlockRef, sh bstream.Handler) bstream.Source {
		if blockRef.ID() == "" {
			blockRef = libRef
		}
		gate := bstream.NewBlockIDGate(blockRef.ID(), bstream.GateInclusive, forkableHandler)
		return h.subscriptionHub.NewSourceFromBlockRef(blockRef, gate)
	})

	eternalSource := bstream.NewEternalSource(joiningSourceFactory, forkableHandler, bstream.EternalSourceWithLogger(zlog))
	eternalSource.OnTerminating(func(e error) {
		zlog.Error("recent-tx hub block stream failed and quit", zap.Error(e))
		h.ready.Store(false)
	})

	// Clean-shutdown: propagate ctx cancel to the bstream source so it
	// drops its relayer connection and exits its goroutines within the
	// pod's terminationGracePeriodSeconds window (R4 #4).
	go func() {
		<-ctx.Done()
		eternalSource.Shutdown(nil)
	}()
	go h.runWatchdog(ctx)
	eternalSource.Run()
}

// runWatchdog flips ready=false when the block stream has been silent longer
// than watchdogStall. The next received block re-arms via lastBlockNano +
// ready.Store(true) in the handler.
func (h *RecentTxHub) runWatchdog(ctx context.Context) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			last := h.lastBlockNano.Load()
			if last == 0 {
				continue // not yet started
			}
			if now.UnixNano()-last > h.watchdogStall.Nanoseconds() {
				if h.ready.Load() {
					zlog.Warn("recent-tx hub: block stream stalled, flipping ready=false",
						zap.Duration("stall", time.Since(time.Unix(0, last))))
				}
				h.ready.Store(false)
			}
		}
	}
}

// onStepNew converts each canonical-chain transaction in blk into a
// *v1.TransactionLifecycle via the same pipeline the Bigtable path uses,
// then appends each to the ring under a brief write lock. The conversion
// runs outside the lock; only the append touches shared state.
//
// Both EXPLICIT transactions (`blk.Transactions()` — user-submitted, have a
// PackedTransaction in their receipt) and IMPLICIT transactions
// (`blk.ImplicitTransactionOps()` — system-generated like `onblock`, no
// PackedTransaction) are surfaced, matching the Bigtable read path which
// walks both `ImplicitTransactionRefs.Hashes` + `TransactionTraceRefs.Hashes`
// (db.go:277). Without this, low-traffic windows show only sparse explicit
// txs with non-contiguous block numbers in the eosq home page, whereas
// Bigtable returns the onblock entries that bridge the gaps.
//
// Deferred trxs (separate from implicit) are disabled at Ultra protocol
// level and are skipped if encountered as a defensive measure.
func (h *RecentTxHub) onStepNew(blk *pbcodec.Block) {
	// Index explicit receipts by trx id — these have PackedTransaction and
	// resolve to TransactionEvent_Addition events on the Bigtable path.
	receiptByID := make(map[string]*pbcodec.TransactionReceipt, len(blk.Transactions()))
	for _, r := range blk.Transactions() {
		if r == nil || r.PackedTransaction == nil {
			continue
		}
		receiptByID[r.Id] = r
	}

	// Index implicit trx ops by trx id — these are system-generated (onblock,
	// etc.) and resolve to TransactionEvent_InternalAddition events on the
	// Bigtable path. They appear as traces in blk.TransactionTraces() but
	// have NO receipt in blk.Transactions() because PackedTransaction is
	// nil for system trxs.
	implicitByID := make(map[string]*pbcodec.TrxOp, len(blk.ImplicitTransactionOps()))
	for _, op := range blk.ImplicitTransactionOps() {
		if op == nil {
			continue
		}
		implicitByID[op.TransactionId] = op
	}

	type prepared struct {
		trxID string
		lc    *v1.TransactionLifecycle
	}
	prep := make([]prepared, 0, len(blk.TransactionTraces()))

	for _, trxTrace := range blk.TransactionTraces() {
		if trxTrace == nil {
			continue
		}

		// Make sure the trace is in the same "reduplicated" shape the
		// Bigtable read path produces (idempotent if already reduplicated).
		codec.ReduplicateTransactionTrace(trxTrace)

		var events []*pbcodec.TransactionEvent

		if receipt, ok := receiptByID[trxTrace.Id]; ok {
			// Explicit transaction: synthesize Addition (with PubKeys) + Execution.
			signedTx, err := codec.ExtractEOSSignedTransactionFromReceipt(receipt)
			if err != nil {
				zlog.Warn("recent-tx hub: extract signed tx failed; skipping",
					zap.String("trx_id", trxTrace.Id),
					zap.String("block_id", blk.Id),
					zap.Error(err))
				continue
			}
			deosSignedTx := codec_eosio.SignedTransactionToDEOS(signedTx)
			pubKeys := codec_eosio.GetPublicKeysFromSignedTransaction(h.chainID, signedTx)

			events = []*pbcodec.TransactionEvent{
				{
					Id:           trxTrace.Id,
					BlockId:      blk.Id,
					BlockNum:     blk.Number,
					Irreversible: false,
					Event: &pbcodec.TransactionEvent_Addition{
						Addition: &pbcodec.TransactionEvent_Added{
							Receipt:     receipt,
							Transaction: deosSignedTx,
							PublicKeys:  &pbcodec.PublicKeys{PublicKeys: pubKeys},
						},
					},
				},
				{
					Id:           trxTrace.Id,
					BlockId:      blk.Id,
					BlockNum:     blk.Number,
					Irreversible: false,
					Event: &pbcodec.TransactionEvent_Execution{
						Execution: &pbcodec.TransactionEvent_Executed{
							Trace:       trxTrace,
							BlockHeader: blk.Header,
						},
					},
				},
			}
		} else if op, ok := implicitByID[trxTrace.Id]; ok {
			// Implicit transaction (onblock, etc.): synthesize InternalAddition
			// + Execution. No PubKeys (system-signed), no Receipt.
			events = []*pbcodec.TransactionEvent{
				{
					Id:           trxTrace.Id,
					BlockId:      blk.Id,
					BlockNum:     blk.Number,
					Irreversible: false,
					Event: &pbcodec.TransactionEvent_InternalAddition{
						InternalAddition: &pbcodec.TransactionEvent_AddedInternally{
							Transaction: op.Transaction,
						},
					},
				},
				{
					Id:           trxTrace.Id,
					BlockId:      blk.Id,
					BlockNum:     blk.Number,
					Irreversible: false,
					Event: &pbcodec.TransactionEvent_Execution{
						Execution: &pbcodec.TransactionEvent_Executed{
							Trace:       trxTrace,
							BlockHeader: blk.Header,
						},
					},
				},
			}
		} else {
			// Neither explicit nor implicit — most likely a deferred-trx
			// execution whose scheduling was in an older block. Ultra
			// disables deferred trxs; skip defensively.
			continue
		}

		lifecycle := pbcodec.MergeTransactionEvents(events, alwaysCanonical)
		if lifecycle == nil {
			continue
		}
		v1tx, err := mdl.ToV1TransactionLifecycle(lifecycle)
		if err != nil {
			zlog.Warn("recent-tx hub: ToV1TransactionLifecycle failed; skipping",
				zap.String("trx_id", trxTrace.Id),
				zap.Error(err))
			continue
		}
		prep = append(prep, prepared{trxID: trxTrace.Id, lc: v1tx})
	}

	if len(prep) == 0 {
		return
	}

	// Single short critical section: append all prepared entries.
	h.mu.Lock()
	for _, p := range prep {
		h.appendLocked(blk.Id, blk.Number, p.lc)
	}
	recentTxRingSize.SetFloat64(float64(h.count))
	h.mu.Unlock()
}

// appendLocked must be called with h.mu held. Inserts entry at h.head,
// evicts the previous occupant's blockIdx entry if any, and updates
// blockIdx for the new entry.
func (h *RecentTxHub) appendLocked(blockID string, blockNum uint32, lc *v1.TransactionLifecycle) {
	idx := h.head

	// Evict prior occupant from blockIdx, if any.
	if prior := h.ring[idx]; prior.lc != nil {
		h.removeBlockIdxLocked(prior.blockID, idx)
		recentTxRingEvictions.Inc()
	}

	h.ring[idx] = ringEntry{blockID: blockID, blockNum: blockNum, lc: lc}
	h.blockIdx[blockID] = append(h.blockIdx[blockID], idx)

	h.head = (h.head + 1) % h.size
	if h.count < h.size {
		h.count++
	}
}

// removeBlockIdxLocked drops `idx` from blockIdx[blockID]; deletes the slice
// if it becomes empty. Must be called with h.mu held.
func (h *RecentTxHub) removeBlockIdxLocked(blockID string, idx int) {
	indices := h.blockIdx[blockID]
	for i, v := range indices {
		if v == idx {
			indices = append(indices[:i], indices[i+1:]...)
			break
		}
	}
	if len(indices) == 0 {
		delete(h.blockIdx, blockID)
	} else {
		h.blockIdx[blockID] = indices
	}
}

// onStepIrreversible flips ExecutionIrreversible=true on every ring entry
// belonging to blockID. After flipping, the block's indices are removed
// from blockIdx — they will never need lookup again because StepUndo
// cannot reach an irreversible block.
//
// The mutation is a single bool flip on a struct pointer. Snapshot readers
// take a shallow-struct copy under RLock, so they observe either the
// pre-flip or post-flip value but never a torn state.
func (h *RecentTxHub) onStepIrreversible(blockID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	indices, ok := h.blockIdx[blockID]
	if !ok {
		// Block already evicted past the ring boundary; no-op.
		return
	}
	for _, idx := range indices {
		if h.ring[idx].lc != nil {
			h.ring[idx].lc.ExecutionIrreversible = true
		}
	}
	delete(h.blockIdx, blockID)
}

// onStepUndo tombstones every ring entry belonging to blockID. Snapshot
// skips tombstoned entries; if too many tombstones bunch up at the head,
// Snapshot falls through to Bigtable instead of returning an under-sized
// response.
func (h *RecentTxHub) onStepUndo(blockID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	indices, ok := h.blockIdx[blockID]
	if !ok {
		return
	}
	for _, idx := range indices {
		h.ring[idx] = ringEntry{}
		recentTxRingUndos.Inc()
	}
	delete(h.blockIdx, blockID)
}

// Snapshot returns the most recent `limit` transactions from the ring, in
// reverse-chronological order (newest first), or (nil, false) if the hub
// cannot satisfy the request — in which case the caller must fall through
// to the Bigtable path. Reasons for fall-through:
//   - hub not ready (cold start or watchdog tripped)
//   - ring has fewer than `limit` complete entries (incl. tombstones)
//
// The returned *v1.TransactionLifecycle pointers are shallow copies of the
// ring's storage. The nested fields (Transaction, ExecutionTrace, …) alias
// the originals, which are immutable after construction; only the three
// irreversibility bool fields ever mutate, and Snapshot captures them at
// copy time under the read lock.
func (h *RecentTxHub) Snapshot(limit int) (*mdl.TransactionList, bool) {
	if limit <= 0 {
		return nil, false
	}
	if !h.ready.Load() {
		recentTxRingMisses.Inc()
		return nil, false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count < limit {
		recentTxRingMisses.Inc()
		return nil, false
	}

	out := &mdl.TransactionList{Transactions: make([]*v1.TransactionLifecycle, 0, limit)}
	var lastBlockNum uint32
	for i := 0; i < h.count && len(out.Transactions) < limit; i++ {
		idx := (h.head - 1 - i + h.size) % h.size
		entry := h.ring[idx]
		if entry.lc == nil {
			continue // tombstoned
		}
		// Shallow-copy the v1.TransactionLifecycle under the read lock so
		// concurrent writers flipping ExecutionIrreversible don't race with
		// the JSON encoder which runs after we release the lock.
		cp := *entry.lc
		out.Transactions = append(out.Transactions, &cp)
		lastBlockNum = entry.blockNum
	}

	if len(out.Transactions) < limit {
		// Tombstones ate the tail; fall through.
		recentTxRingMisses.Inc()
		return nil, false
	}

	// Build the next-page cursor pointing one block earlier than the ring's
	// oldest returned tx. We synthesize a 32-byte block-id whose first 4
	// bytes encode (lastBlockNum - 1) — db.ListMostRecentTransactions only
	// uses eos.BlockNum(blockID) to start the scan and never matches the
	// synthesized id against any real blk.Id, so the in-block skip predicate
	// (`blk.Id == startBlockID && trxIndex > startTrxIndex`) never fires
	// and the Bigtable fall-through resumes cleanly at lastBlockNum-1,
	// returning all txs from there backward without overlap.
	//
	// Trade-off: any txs that lived in the ring's last block but weren't
	// returned in this page (because limit landed mid-block) are NOT
	// surfaced on the next page. With Ultra's home-page limit=25 vs the
	// chain's typical 1-3 tx/block this case is rare; when it happens the
	// missing txs remain discoverable via /v0/transactions/{id} or the
	// search endpoints.
	out.Cursor = opaqueCursor(syntheticPrevBlockID(lastBlockNum) + ":" + hexUint16(0xffff))

	recentTxRingHits.Inc()
	return out, true
}

// syntheticPrevBlockID returns a 64-hex-char string whose first 8 chars
// encode (blockNum - 1). The remaining 56 chars are zeros — they are never
// matched against a real blk.Id by the Bigtable fall-through, so collisions
// are not a concern.
func syntheticPrevBlockID(blockNum uint32) string {
	if blockNum == 0 {
		return "00000000" + zero56
	}
	return fmt.Sprintf("%08x", blockNum-1) + zero56
}

const zero56 = "00000000000000000000000000000000000000000000000000000000"

// alwaysCanonical is the inCanonicalChain discriminator passed to
// MergeTransactionEvents. The forkable handler only fires StepNew/StepRedo
// for canonical-chain blocks, so every event we synthesize is on chain.
// StepUndo blocks are tombstoned out of the ring instead of being marked
// off-chain after the fact.
func alwaysCanonical(_ string) bool { return true }

// hexUint16 is the cursor encoding mirror of kvdb.HexUint16 (db.go:331);
// we re-implement it here to avoid pulling in the kvdb package just for
// a 4-byte hex format string.
func hexUint16(v uint16) string {
	const hex = "0123456789abcdef"
	return string([]byte{
		hex[(v>>12)&0xf],
		hex[(v>>8)&0xf],
		hex[(v>>4)&0xf],
		hex[v&0xf],
	})
}
