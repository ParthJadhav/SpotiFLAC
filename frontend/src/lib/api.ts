import type {
  SpotifyMetadataResponse,
  DownloadRequest,
  DownloadResponse,
  HealthResponse,
  LyricsDownloadRequest,
  LyricsDownloadResponse,
} from "@/types/api";
import { GetSpotifyMetadata, DownloadTrack as DownloadTrackFn, DownloadLyrics, PreCheckDownloadFiles as PreCheckDownloadFilesFn } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";

export async function fetchSpotifyMetadata(
  url: string,
  batch: boolean = true,
  delay: number = 1.0,
  timeout: number = 300.0
): Promise<SpotifyMetadataResponse> {
  const req = new main.SpotifyMetadataRequest({
    url,
    batch,
    delay,
    timeout,
  });

  const jsonString = await GetSpotifyMetadata(req);
  return JSON.parse(jsonString);
}

export async function downloadTrack(
  request: DownloadRequest
): Promise<DownloadResponse> {
  return await DownloadTrackFn(request as any) as DownloadResponse;
}

export async function checkHealth(): Promise<HealthResponse> {
  // For Wails, we can just return a simple health check
  // since the app is running locally
  return {
    status: "ok",
    time: new Date().toISOString(),
  };
}

export async function downloadLyrics(
  request: LyricsDownloadRequest
): Promise<LyricsDownloadResponse> {
  const req = new main.LyricsDownloadRequest(request);
  return await DownloadLyrics(req) as LyricsDownloadResponse;
}

export async function preCheckDownloadFiles(
  tracks: Array<{ isrc: string; track_name: string; artist_name: string; album_name: string; position: number }>,
  outputDir: string,
  filenameFormat: string,
  trackNumber: boolean,
  useAlbumTrackNumber: boolean
): Promise<string> {
  const req = new main.PreCheckDownloadRequest({
    tracks,
    output_dir: outputDir,
    filename_format: filenameFormat,
    track_number: trackNumber,
    use_album_track_number: useAlbumTrackNumber,
  });
  return await PreCheckDownloadFilesFn(req);
}
