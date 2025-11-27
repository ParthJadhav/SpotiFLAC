# TypeScript Build Error Fixes

## Issues Resolved

### Error 1: PreCheckDownloadRequest Missing convertValues

**Original Error:**

```
src/hooks/useDownload.ts(51,56): error TS2345: Argument of type '{ tracks: ...; output_dir: ...; ... }' is not assignable to parameter of type 'PreCheckDownloadRequest'.
Property 'convertValues' is missing in type ...
```

**Solution:**

- Added proper `PreCheckDownloadRequest` interface definition in `frontend/src/types/api.ts`
- Created `TrackCheckRequest` and `ExistingTrackInfo` interfaces for type safety
- Updated `useDownload.ts` to use typed interface instead of inline object

### Error 2: main.DownloadRequest Doesn't Exist

**Original Error:**

```
src/lib/api.ts(32,24): error TS2339: Property 'DownloadRequest' does not exist on type 'typeof main'.
```

**Solution:**

- `DownloadRequest` is now defined in the backend package, not exposed through wails main
- Updated `downloadTrack()` in `api.ts` to accept `DownloadRequest` as a plain object
- Removed `new main.DownloadRequest(request)` wrapper - pass request directly to wails function
- Wails will handle JSON serialization automatically

## Files Modified

### 1. `frontend/src/types/api.ts`

Added three new interfaces:

```typescript
export interface TrackCheckRequest {
  isrc: string;
  track_name: string;
  artist_name: string;
  album_name: string;
  position: number;
}

export interface ExistingTrackInfo {
  isrc: string;
  filename: string;
  path: string;
  size_gb: number;
}

export interface PreCheckDownloadRequest {
  tracks: TrackCheckRequest[];
  output_dir: string;
  filename_format: string;
  track_number: boolean;
  use_album_track_number: boolean;
}
```

### 2. `frontend/src/lib/api.ts`

- Added `PreCheckDownloadRequest` import from types
- Added `PreCheckDownloadFiles` import from wails
- Updated `downloadTrack()` to pass request object directly (no wrapper)
- Changed: `new main.DownloadRequest(request)` → direct object

### 3. `frontend/src/hooks/useDownload.ts`

- Updated imports to include `PreCheckDownloadRequest` and `ExistingTrackInfo`
- Imported `PreCheckDownloadFiles` directly from wails
- Updated `preCheckExistingFiles()` to use typed `PreCheckDownloadRequest`
- Changed response type from `ExistingFile[]` to `ExistingTrackInfo[]`
- Removed dynamic import of PreCheckDownloadFiles

## Key Changes

1. **Type Safety**: All request/response objects now properly typed
2. **Consistency**: Types match backend struct definitions
3. **Wails Integration**: Leveraging wails auto-marshaling instead of manual constructors
4. **No Runtime Changes**: Backend functionality unchanged, only type safety improved

## Verification

✓ All TypeScript interfaces properly defined
✓ Type imports correct
✓ Wails bindings properly referenced
✓ No circular dependencies
✓ Matches backend struct definitions
