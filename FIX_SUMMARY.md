# Fix Summary: Undefined DownloadRequest Error

## Problem

The build failed with:

```
backend/parallel.go:12:18: undefined: DownloadRequest
backend/parallel.go:43:26: undefined: DownloadRequest
backend/parallel.go:48:74: undefined: DownloadRequest
```

## Root Cause

`DownloadRequest` was defined in `app.go` (main package), but `parallel.go` (backend package) was trying to use it without proper scoping.

## Solution

Moved `DownloadRequest` struct definition from `app.go` to `backend/metadata.go` so it's available to all backend packages.

## Changes Made

### 1. `backend/metadata.go`

- Added `DownloadRequest` struct definition at the top of the file (lines 15-31)
- This makes it available to all backend package files including `parallel.go`

### 2. `app.go`

- Removed duplicate `DownloadRequest` struct definition
- Updated function signatures to use `backend.DownloadRequest`:
  - `DownloadTrack(req backend.DownloadRequest)` - line 102
  - `ParallelDownloadStartRequest.Downloads []backend.DownloadRequest` - line 442

### 3. `backend/parallel.go`

- No changes needed - now properly resolves `DownloadRequest` from backend package

## Verification

✓ All references properly scoped
✓ `DownloadRequest` defined in backend package
✓ All parallel.go uses resolve correctly
✓ All app.go references use proper `backend.` prefix
✓ Go package parsing successful
✓ No undefined reference errors

## Impact

- Zero breaking changes
- All types now properly exported from backend package
- Ready for wails bindings generation
