# `--history.chain none` Implementation Summary

## Files modified:

1. **`core/history/historymode.go`** — Added `KeepNone HistoryMode = 2`, wired up `"none"` in `String()`, `MarshalText()`, `UnmarshalText()`, and updated `IsValid()`.

2. **`core/blockchain.go`** — Added `case history.KeepNone` in `initializeHistoryPruning()` (uses freezer tail on restart, leaves unset on fresh start). Added `SetHistoryPrunePoint()` public method for the downloader to update the prune point at runtime. Added nil check in `Stop()` for `GetBlockByNumber` to handle blocks without bodies.

3. **`eth/downloader/downloader.go`** — Added `historyMode` field to `Downloader`. Updated `New()` to accept optional `historyMode` parameter. Added `SetHistoryPrunePoint()` to the `BlockChain` interface. In `syncToHead()`, after the pivot is computed, dynamically sets `chainCutoffNumber` and `chainCutoffHash` to the pivot for `KeepNone` mode, and calls `blockchain.SetHistoryPrunePoint()`.

4. **`eth/downloader/beaconsync.go`** — Added `d.chainCutoffHash != (common.Hash{})` guard to the cutoff hash validation, so dynamically-set cutoffs don't fail against stale hashes.

5. **`eth/handler.go`** — Added `HistoryMode` to `handlerConfig`, passes it to `downloader.New()`. Added `history` import.

6. **`eth/backend.go`** — Passes `config.HistoryMode` to the handler config.

## Tests added:

7. **`core/blockchain_test.go`** — `TestInsertChainWithKeepNone` — verifies headers exist for all blocks, bodies/receipts are nil before cutoff, present after, and `HistoryPruningCutoff()` returns the dynamically-set value.

8. **`eth/downloader/downloader_test.go`** — `TestBeaconSync68SnapKeepNone` — verifies beacon sync with `KeepNone` sets the cutoff near the pivot (HEAD-64) and the blockchain's `HistoryPruningCutoff()` reflects it.

## Live smoke test on hoodi confirmed:
- ✅ `"KeepNone: set chain cutoff to pivot"` — cutoff=2,170,985
- ✅ `"Updated history prune point"` — block=2,170,985
- ✅ `"Skip chain segment before cutoff"` — origin=0 cutoff=2,170,985
