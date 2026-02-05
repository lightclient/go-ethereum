# Prompt: Add `--history.chain none` mode to geth (prototype)

## Goal

Add a new `HistoryMode` called `KeepNone` (exposed as `--history.chain none`) that skips downloading block bodies and receipts entirely during snap sync, except the ~64 blocks near the head that snap sync requires to function. This is a prototype to minimize initial sync time by focusing purely on state sync.

## How sync works today (relevant parts only)

### Engine API triggers sync
When the CL sends `forkchoiceUpdated` and geth doesn't have the block, `forkchoiceUpdated()` in `eth/catalyst/api.go` (line ~262) calls `Downloader().BeaconSync(header, finalized)` to kick off sync. No new waiting logic is needed — this is already the entry point.

### Skeleton sync (`eth/downloader/skeleton.go`)
Downloads headers backward from the beacon head to genesis. The `linked()` check (line 367) requires `HasHeader + HasBody + HasReceipts` — on a fresh node, only genesis satisfies this. Once linked, it triggers the backfiller.

### Backfiller (`eth/downloader/downloader.go`, line ~440)
Fetches bodies and receipts starting from `chainOffset`:
```
chainOffset = max(origin+1, chainCutoffNumber)
```
Headers before `chainCutoffNumber` are inserted via `InsertHeadersBeforeCutoff()` with nil bodies/receipts. Bodies+receipts are only fetched from `chainOffset` onward.

### Snap sync pivot
Snap sync downloads state at the **pivot block** (`HEAD - 64`), then fully executes the last 64 blocks. This means bodies+receipts for `[HEAD-64, HEAD]` are mandatory — everything else is optional.

### `chainCutoffNumber` mechanism
Set from `blockchain.HistoryPruningCutoff()` in the `Downloader` constructor. Already used by `KeepPostMerge` to skip pre-merge body/receipt downloads. Controls:
- `chainOffset` — starting point for body/receipt fetching (line ~578)
- `processHeaders()` — splits headers around cutoff, calls `InsertHeadersBeforeCutoff` for pre-cutoff (line ~750)
- `ancientLimit` — extended to at least `chainCutoffNumber` (line ~547)
- `fetchHeaders()` in `beaconsync.go` — validates hash at cutoff block (line ~274)

### Where cutoff is currently set
`initializeHistoryPruning()` in `core/blockchain.go` (line 716) runs at startup. For `KeepPostMerge`, it sets a static prune point from `history.PrunePoints` (hardcoded merge block). `HistoryPruningCutoff()` returns this, and the `Downloader` constructor reads it.

**Problem for `KeepNone`**: At startup on a fresh node, `latest` is 0 — we don't know the chain head yet. The head is only known after the first engine API message triggers skeleton sync. So we can't compute the cutoff at `initializeHistoryPruning()` time.

### Who reads `HistoryPruningCutoff()` / `historyPrunePoint` at runtime
The prune point isn't just used for initial sync. It's read continuously:
- **Downloader constructor** (`eth/downloader/downloader.go:233`) — sets `chainCutoffNumber` once
- **API backends** (`eth/api_backend.go`) — returns `PrunedHistoryError` (code 4444) for blocks before cutoff
- **Log filters** (`eth/filters/filter.go`, `filter_system.go`) — rejects log queries before cutoff
- **Tx indexer** (`core/txindexer.go:71`) — won't index below cutoff
- **Peer block range announcements** (`eth/handler.go:632`) — tells peers our earliest available block

If `historyPrunePoint` is never set, all of these think full history is available. APIs will try to serve old blocks (returning empty/nil results instead of 4444), peers will be told we have history we don't have, etc.

## What to implement

### 1. Add `KeepNone` history mode

**File: `core/history/historymode.go`**
- Add `KeepNone HistoryMode = 2` after `KeepPostMerge`
- Wire up `"none"` in `String()`, `MarshalText()`, `UnmarshalText()`
- Update `IsValid()` to include `KeepNone`

### 2. Handle `KeepNone` at startup and expose a setter

**File: `core/blockchain.go`**

Add a case in `initializeHistoryPruning()`:
```go
case history.KeepNone:
    // On restart (freezer tail > 0), set the prune point from the freezer tail
    // so APIs/filters/peers know our actual data availability.
    if freezerTail > 0 {
        hash := rawdb.ReadCanonicalHash(bc.db, freezerTail)
        bc.historyPrunePoint.Store(&history.PrunePoint{
            BlockNumber: freezerTail,
            BlockHash:   hash,
        })
    }
    // On fresh start (freezerTail == 0), leave prune point unset.
    // The downloader will set it dynamically once the sync target is known.
    return nil
```

Add a public method so the downloader can update the prune point after computing the cutoff:
```go
// SetHistoryPrunePoint updates the history prune point at runtime.
// Used by the downloader in KeepNone mode once the sync target is known.
func (bc *BlockChain) SetHistoryPrunePoint(number uint64, hash common.Hash) {
    bc.historyPrunePoint.Store(&history.PrunePoint{
        BlockNumber: number,
        BlockHash:   hash,
    })
    log.Info("Updated history prune point", "block", number, "hash", hash)
}
```

### 3. Compute the cutoff dynamically in the downloader

**File: `eth/downloader/downloader.go`**

Add a `historyMode history.HistoryMode` field to `Downloader`. Set it in `New()`.

In the backfill function (around line 500, after `latest` and `pivot` are known), add:

```go
// For KeepNone mode, set the cutoff to just before the pivot block so only
// the minimum blocks needed for snap sync are downloaded with bodies/receipts.
if d.historyMode == history.KeepNone && mode == ethconfig.SnapSync && pivot != nil {
    cutoff := pivot.Number.Uint64()
    if cutoff > d.chainCutoffNumber {
        d.chainCutoffNumber = cutoff
        log.Info("KeepNone: set chain cutoff to pivot", "cutoff", cutoff)

        // Update the blockchain's prune point so APIs, filters, tx indexer,
        // and peer announcements reflect the actual data availability.
        d.blockchain.SetHistoryPrunePoint(cutoff, pivot.Hash())
    }
}
```

This must be inserted **before** the `ancientLimit` calculation (line ~530) and the `chainOffset` calculation (line ~578), so both pick up the new cutoff.

The `BlockChain` interface in the downloader needs `SetHistoryPrunePoint` added:
```go
type BlockChain interface {
    // ... existing methods ...
    SetHistoryPrunePoint(number uint64, hash common.Hash)
}
```

### 4. Skip cutoff hash validation for dynamic cutoffs

**File: `eth/downloader/beaconsync.go`**

In `fetchHeaders()` (line ~274), the cutoff block's hash is validated against `chainCutoffHash`. For `KeepNone`, we don't have a predefined hash. Skip the validation when `chainCutoffHash` is zero:

```go
if d.chainCutoffNumber != 0 && d.chainCutoffHash != (common.Hash{}) && d.chainCutoffNumber >= from && d.chainCutoffNumber <= head.Number.Uint64() {
```

This is safe because the header chain is still fully verified via parent hash linkage.

### 5. Pass history mode to the downloader

**File: `eth/downloader/downloader.go`**

Update the `BlockChain` interface to expose the history mode, or simply add `historyMode` as a parameter to `New()`.

**File: `eth/handler.go`** (or wherever `New()` is called)

Pass `blockchain.Config().ChainHistoryMode` (or equivalent) to the downloader constructor.

### 6. Wire up the CLI flag

**File: `cmd/utils/flags.go`** — The existing `ChainHistoryFlag` already accepts a string and calls `UnmarshalText`. Just adding `"none"` to the `UnmarshalText` switch (step 1) is sufficient.

## Files to modify (summary)

1. `core/history/historymode.go` — add `KeepNone` constant, wire up `"none"` string
2. `core/blockchain.go` — add `case history.KeepNone` in `initializeHistoryPruning()`, add `SetHistoryPrunePoint()` method
3. `eth/downloader/downloader.go` — add `historyMode` field, compute dynamic cutoff in backfill, call `SetHistoryPrunePoint()` on the blockchain
4. `eth/downloader/beaconsync.go` — skip hash validation when `chainCutoffHash` is zero
5. `eth/handler.go` or `eth/backend.go` — pass history mode to downloader

## What happens at runtime

### Fresh start with `geth --syncmode snap --history.chain none`:

1. Geth starts, `initializeHistoryPruning()` sees freezerTail==0, does nothing (no head known yet)
2. `HistoryPruningCutoff()` returns `(0, genesisHash)` → downloader sets `chainCutoffNumber = 0`
3. CL sends `forkchoiceUpdated` → triggers `BeaconSync()` → skeleton starts
4. Skeleton downloads all headers backward to genesis (~600MB, ~30-60 min on mainnet)
5. Skeleton links at genesis, triggers backfiller
6. Backfiller computes `pivot = HEAD - 64`, sets `chainCutoffNumber = pivot`
7. Backfiller calls `blockchain.SetHistoryPrunePoint(pivot, pivotHash)` — this immediately updates:
   - API responses (return 4444 for blocks before pivot)
   - Log filters (reject queries before pivot)
   - Peer block range announcements (advertise correct earliest block)
8. `InsertHeadersBeforeCutoff` stores all pre-pivot headers with nil bodies/receipts (fast)
9. Bodies + receipts fetched only for `[pivot, HEAD]` — 64 blocks, negligible
10. Snap sync downloads state at pivot root (this is the real bottleneck, hours)
11. Last 64 blocks executed on top of snapped state
12. Node is synced and follows the chain normally

### Restart with `geth --syncmode snap --history.chain none`:

1. Geth starts, `initializeHistoryPruning()` sees freezerTail > 0 (set during initial sync)
2. Sets `historyPrunePoint` from the freezer tail → APIs/filters/peers immediately correct
3. `HistoryPruningCutoff()` returns the stored cutoff → downloader inherits it
4. Node resumes normal operation

**Total body/receipt download: ~64 blocks instead of ~21M blocks.**

## What this does NOT do

- Skip header download (impossible — headers must be verified contiguously to genesis)
- Speed up state sync (that's the bottleneck, unchanged)
- Prune old data over time (not needed for prototype — bodies were never downloaded)
- Work with `--syncmode full` (only meaningful with snap sync)

## Verification (required)

After implementation, run both of these verification steps before considering the task done.

### Step 1: Unit tests

Add a test modeled on `TestInsertChainWithCutoff` in `core/blockchain_test.go` (line 4296). That test already exercises the exact `InsertHeadersBeforeCutoff` + `InsertReceiptChain` pattern with a cutoff. Clone it, but use `history.KeepNone` instead of `history.KeepPostMerge` and verify:
- Headers exist for all blocks (0 to HEAD)
- Bodies/receipts are nil for blocks before the cutoff
- Bodies/receipts exist for blocks at and after the cutoff
- `chain.HistoryPruningCutoff()` returns the expected cutoff
- `db.Tail()` equals the cutoff block number

Also add a downloader-level test modeled on `TestBeaconSync68Snap` in `eth/downloader/downloader_test.go` (line 639). Set `historyMode = history.KeepNone` on the downloader and verify that after beacon sync completes, the blockchain's `HistoryPruningCutoff()` is near the pivot.

Run all tests and make sure nothing is broken:
```bash
go test ./core/... ./eth/downloader/...
```

### Step 2: Live smoke test on hoodi

Build geth, run it against hoodi with a CL, and watch the logs long enough to confirm the cutoff is applied and state sync begins. Then kill the process and clean up.

```bash
# Build
go build ./cmd/geth

# Start geth (in background or a separate terminal)
./geth --syncmode snap --history.chain none --datadir /tmp/geth-none-test \
       --verbosity 4 --hoodi

# In another terminal, start a CL (e.g. Lighthouse) pointed at geth's engine API
```

Watch the logs and confirm these messages appear in order:

1. `"KeepNone: set chain cutoff to pivot"` — the dynamic cutoff was computed
2. `"Updated history prune point"` — the blockchain's prune point was set
3. `"Skip chain segment before cutoff"` — body/receipt fetching was skipped for old blocks
4. `"Inserted headers before cutoff"` — headers are being stored with nil bodies
5. `"Syncing: chain download in progress"` — check that `bodies` and `receipts` sizes are tiny (KB range) while `headers` is growing (MB range). This confirms bodies/receipts are not being downloaded.

Once you see all five confirmations, the implementation is working. Kill both processes and clean up:

```bash
rm -rf /tmp/geth-none-test
```

## Edge cases (already handled)

- **Pivot staleness**: During long state syncs, the pivot can move forward (line ~972). The cutoff we set is at the original pivot, which is lower — so we download slightly more bodies than strictly needed. This is fine; the `if cutoff > d.chainCutoffNumber` guard prevents moving it backward.
- **Backfill retries**: If backfill is retried (sync errors, reorgs), the cutoff logic runs again but the guard ensures it only moves forward.
- **Post-sync new blocks**: New blocks arrive via engine API, get executed, go into the KV store, then the freezer background thread moves them to ancients. The freezer tail stays where `InsertHeadersBeforeCutoff` set it. Bodies/receipts accumulate normally from the pivot onward.
- **`--syncmode full` incompatibility**: Full sync needs every body to execute every block. `KeepNone` should error at startup if `syncmode != snap`.

## Constraints

- Do NOT modify skeleton sync — headers always download fully
- Do NOT break `KeepAll` or `KeepPostMerge` modes
- No periodic tail pruning needed — bodies/receipts were never stored
- This is a prototype — no need for graceful mode switching or migration logic
