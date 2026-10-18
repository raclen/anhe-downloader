export type MusicSource = 'kw' | 'wy' | 'kg' | 'tx' | 'mg';

export type MusicQuality = '128k' | '320k' | 'flac';

export interface SongEntity {
  id: string;
  source: MusicSource;
  name: string;
  singer: string;
  album: string;
  duration: number; // 秒
  cover: string;
  songmid: string;
  hash?: string;
  types: MusicQuality[];
  lrcUrl?: string;
}

export type TaskStatus =
  | 'pending'
  | 'parsing'
  | 'downloading'
  | 'tagging'
  | 'completed'
  | 'error'
  | 'paused';

export interface DownloadTask {
  id: string;
  song: SongEntity;
  quality: MusicQuality;
  status: TaskStatus;
  progress: number;
  downloadedBytes: number;
  totalBytes: number;
  speed: string;
  savePath: string;
  errorMsg?: string;
  resolverNote?: string;
  createdAt: number;
}

export interface AppSettings {
  downloadDir: string;
  preferredQuality: MusicQuality;
  autoFallback: boolean;
  fileNamePattern: string; // {singer} - {name}
  embedCover: boolean;
  embedLyrics: boolean;
  saveLrcFile: boolean;
  maxConcurrent: number;
  activeSourceScript: string;
}

export interface SoundSourceMeta {
  name: string;
  version: string;
  author: string;
  description?: string;
  supportedSources: MusicSource[];
  supportedQualities: MusicQuality[];
  status: 'active' | 'error' | 'loading';
  error?: string;
}
