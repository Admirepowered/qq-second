// 与 Go 端模型一一对应的前端类型。

export interface NameRef {
  mid?: string;
  name: string;
}

export interface Track {
  kind: 'qq' | 'local';
  id: string;
  mid?: string;
  mediaMid?: string;
  albumMid?: string;
  title: string;
  artists: NameRef[];
  album?: string;
  durationMs: number;
  artwork?: string;
  path?: string;
  ext?: string;
  trackNum?: number;
  quality?: string;
  capability?: 'full' | 'preview' | 'unavailable' | '';
}

export interface PlaylistInfo {
  id: string;
  title: string;
  cover?: string;
  creator?: string;
  songCount?: number;
  desc?: string;
  updateTime?: string;
}

export interface PlaylistDetail {
  info: PlaylistInfo;
  tracks: Track[];
  total: number;
  hasMore: boolean;
}

export interface AlbumDetail {
  id: string;
  title: string;
  artists: NameRef[];
  cover?: string;
  releaseYear?: string;
  company?: string;
  desc?: string;
  tracks: Track[];
}

export interface AlbumBrief {
  id: string;
  title: string;
  cover?: string;
  releaseYear?: string;
}

export interface ArtistBrief {
  id: string;
  name: string;
  cover?: string;
}

export interface ArtistDetail {
  id: string;
  name: string;
  cover?: string;
  description?: string;
  songs: Track[];
  albums: AlbumBrief[];
}

export interface SearchResult {
  songs: Track[];
  artists: ArtistBrief[];
  albums: AlbumBrief[];
  playlists: PlaylistInfo[];
  songTotal: number;
  more: boolean;
}

export interface HomeFeed {
  playlists: PlaylistInfo[];
  newSongs: Track[];
}

export interface ChartInfo {
  id: number;
  name: string;
  cover?: string;
  updateTime?: string;
}

export interface LyricWord {
  startMs: number;
  endMs: number;
  text: string;
}

export interface LyricLine {
  startMs: number;
  endMs: number;
  text: string;
  translation?: string;
  romanization?: string;
  words?: LyricWord[];
}

export interface LyricDocument {
  songId: string;
  syncMode: 'word' | 'line' | 'unsynchronized';
  lines: LyricLine[];
}

export interface PlaybackSource {
  token: string;
  quality: string;
  format: string;
  sizeBytes: number;
  streamPath: string;
}

export interface Settings {
  colorMode: 'system' | 'light' | 'dark';
  palette: string;
  primaryColor: string;
  backgroundMode: 'default' | 'image' | 'artwork' | 'color';
  backgroundColor: string;
  backgroundImage: string;
  backgroundFit: 'cover' | 'contain';
  backgroundBlur: number;
  artworkInfluence: number;
  surfaceOpacity: number;
  preferredQuality: 'standard' | 'high' | 'lossless';
  closeBehavior: 'hide' | 'exit';
  localFolders: string[];
}

export interface LocalLibraryView {
  folders: string[];
  tracks: Track[];
  scanning: boolean;
}

export interface SessionData {
  queue: Track[];
  currentIndex: number;
  positionMs: number;
  volume: number;
  order: 'sequential' | 'shuffle';
  repeat: 'off' | 'all' | 'one';
}

export interface Credential {
  uin: string;
  nickname: string;
  loggedAt: number;
}

export interface BackgroundImage {
  id: string;
  name: string;
}

export interface QRCodeImage {
  dataUrl: string;
  state: string;
}

export interface QRLoginPoll {
  status: 'waiting' | 'scanned' | 'expired' | 'done' | 'refuse' | 'error';
  message: string;
}

export const emptySettings = (): Settings => ({
  colorMode: 'dark',
  palette: 'default',
  primaryColor: '#31c27c',
  backgroundMode: 'default',
  backgroundColor: '#1d1f21',
  backgroundImage: '',
  backgroundFit: 'cover',
  backgroundBlur: 24,
  artworkInfluence: 60,
  surfaceOpacity: 88,
  preferredQuality: 'high',
  closeBehavior: 'hide',
  localFolders: [],
});

export function normalizeSettings(raw: Partial<Settings> | null | undefined): Settings {
  const def = emptySettings();
  if (!raw) return def;
  const s = { ...def, ...raw };
  if (!['system', 'light', 'dark'].includes(s.colorMode)) s.colorMode = def.colorMode;
  if (!['default', 'image', 'artwork', 'color'].includes(s.backgroundMode)) s.backgroundMode = def.backgroundMode;
  if (!['cover', 'contain'].includes(s.backgroundFit)) s.backgroundFit = def.backgroundFit;
  if (!['standard', 'high', 'lossless'].includes(s.preferredQuality)) s.preferredQuality = def.preferredQuality;
  if (s.closeBehavior !== 'hide' && s.closeBehavior !== 'exit') s.closeBehavior = 'hide';
  s.backgroundBlur = Math.max(0, Math.min(40, s.backgroundBlur || 0));
  s.artworkInfluence = Math.max(0, Math.min(100, s.artworkInfluence ?? 60));
  s.surfaceOpacity = Math.max(40, Math.min(100, s.surfaceOpacity ?? 88));
  if (!Array.isArray(s.localFolders)) s.localFolders = [];
  return s;
}

// 工具函数
export function fmtDuration(ms: number): string {
  if (!ms || ms <= 0) return '--:--';
  const totalSec = Math.floor(ms / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

export function artistText(t: Track): string {
  return (t.artists || []).map((a) => a.name).join(' / ') || '未知歌手';
}
