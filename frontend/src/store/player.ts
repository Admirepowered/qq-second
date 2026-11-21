// 播放器 store：队列/状态机/audio 元素/token 解析/SMTC/会话持久化。

import { create } from 'zustand';
import { QQResolvePlayBinding, SaveSession, GetSession, ReadLocalCover } from '../api';
import { Track, PlaybackSource, SessionData } from '../types';
import { usePrefsStore } from './prefs';

export type PlaybackState = 'idle' | 'loading' | 'playing' | 'paused' | 'error';
export type Order = 'sequential' | 'shuffle';
export type Repeat = 'off' | 'all' | 'one';

interface PlayerState {
  audio: HTMLAudioElement | null;
  queue: Track[];
  currentIndex: number;
  current: Track | null;
  playbackState: PlaybackState;
  positionMs: number;
  durationMs: number;
  volume: number;
  muted: boolean;
  order: Order;
  repeat: Repeat;
  quality: string;
  error: string;
  shuffleHistory: number[];
  // 队列面板 / 歌词面板开关
  queueOpen: boolean;
  lyricsOpen: boolean;

  initAudio: () => void;
  restoreSession: () => Promise<void>;
  playTracks: (tracks: Track[], startIndex: number) => void;
  playFromQueue: (index: number) => void;
  addToQueue: (tracks: Track[]) => void;
  playNext: () => void;
  removeFromQueue: (index: number) => void;
  clearQueue: () => void;
  toggle: () => void;
  pause: () => void;
  resume: () => void;
  next: () => void;
  previous: () => void;
  seek: (positionMs: number) => void;
  setVolume: (v: number) => void;
  toggleMute: () => void;
  setOrder: (o: Order) => void;
  setRepeat: (r: Repeat) => void;
  setQueueOpen: (v: boolean) => void;
  setLyricsOpen: (v: boolean) => void;
  persist: () => void;
}

// audio 单例（模块级，避免 React 生命周期干扰）
let audioEl: HTMLAudioElement | null = null;
let resolveGeneration = 0;
let persistedTimer: ReturnType<typeof setTimeout> | null = null;

function computeStreamURL(track: Track): string {
  return track.kind === 'local' ? `/audio/local/${encodeURIComponent(track.id)}` : '';
}

async function resolveQQ(track: Track): Promise<string> {
  const preferred = usePrefsStore.getState().settings.preferredQuality;
  const source = (await QQResolvePlayBinding(track, preferred)) as PlaybackSource;
  usePlayerStore.setState({ quality: source.quality || '' });
  return source.streamPath;
}

// SMTC: navigator.mediaSession 元数据
function updateMediaSession(track: Track | null, artwork?: string) {
  if (!('mediaSession' in navigator) || !track) {
    if ('mediaSession' in navigator) navigator.mediaSession.metadata = null;
    return;
  }
  try {
    navigator.mediaSession.metadata = new MediaMetadata({
      title: track.title,
      artist: (track.artists || []).map((a) => a.name).join(' / '),
      album: track.album || '',
      artwork: artwork ? [{ src: artwork, sizes: '300x300', type: 'image/jpeg' }] : [],
    });
    navigator.mediaSession.setActionHandler('play', () => getStore().resume());
    navigator.mediaSession.setActionHandler('pause', () => getStore().pause());
    navigator.mediaSession.setActionHandler('previoustrack', () => getStore().previous());
    navigator.mediaSession.setActionHandler('nexttrack', () => getStore().next());
  } catch {
    // WebView2 支持 mediaSession；忽略异常
  }
}

// 模块级引用，供 mediaSession 回调使用
let storeRef: PlayerState | null = null;
function getStore(): PlayerState {
  return storeRef!;
}

function nextSequentialIndex(state: PlayerState, manual: boolean): number {
  const { currentIndex, queue, repeat } = state;
  if (repeat === 'one' && !manual) return currentIndex;
  if (currentIndex + 1 < queue.length) return currentIndex + 1;
  if (repeat === 'all') return 0;
  return -1;
}

function pickShuffled(state: PlayerState): number {
  // 确定性洗牌遍历：未播放过的优先
  const candidates: number[] = [];
  for (let i = 0; i < state.queue.length; i++) {
    if (i !== state.currentIndex && !state.shuffleHistory.includes(i)) candidates.push(i);
  }
  if (candidates.length === 0) {
    // 全部播过，重置历史
    if (state.repeat === 'all' || state.repeat === 'off') {
      state.shuffleHistory = [];
      candidates.push(...Array.from({ length: state.queue.length }, (_, i) => i).filter((i) => i !== state.currentIndex));
    }
    if (candidates.length === 0) return -1;
  }
  return candidates[Math.floor(Math.random() * candidates.length)];
}

export const usePlayerStore = create<PlayerState>((set, get) => {
  const store: PlayerState = {
    audio: null,
    queue: [],
    currentIndex: -1,
    current: null,
    playbackState: 'idle',
    positionMs: 0,
    durationMs: 0,
    volume: 0.85,
    muted: false,
    order: 'sequential',
    repeat: 'off',
    quality: '',
    error: '',
    shuffleHistory: [],
    queueOpen: false,
    lyricsOpen: false,

    initAudio: () => {
      if (audioEl) return;
      audioEl = new Audio();
      audioEl.preload = 'auto';
      audioEl.volume = get().volume;
      set({ audio: audioEl });

      audioEl.addEventListener('timeupdate', () => {
        if (!audioEl) return;
        set({ positionMs: Math.floor(audioEl.currentTime * 1000) });
        schedulePersist();
      });
      audioEl.addEventListener('durationchange', () => {
        if (!audioEl || !isFinite(audioEl.duration)) return;
        set({ durationMs: Math.floor(audioEl.duration * 1000) });
      });
      audioEl.addEventListener('ended', () => {
        const s = get();
        if (s.repeat === 'one') {
          audioEl!.currentTime = 0;
          void audioEl!.play().catch(() => {});
          return;
        }
        s.next();
      });
      audioEl.addEventListener('error', () => {
        const s = get();
        if (!s.current) return;
        set({ playbackState: 'error', error: '播放失败：音源可能已过期或无权限' });
      });
      audioEl.addEventListener('playing', () => set({ playbackState: 'playing', error: '' }));
      audioEl.addEventListener('pause', () => {
        const s = get();
        if (audioEl && !audioEl.ended && s.playbackState !== 'error') set({ playbackState: 'paused' });
      });

      storeRef = get();
    },

    restoreSession: async () => {
      try {
        const session = (await GetSession()) as SessionData;
        if (!session || !Array.isArray(session.queue) || session.queue.length === 0) return;
        set({
          queue: session.queue,
          currentIndex: Math.min(Math.max(0, session.currentIndex), session.queue.length - 1),
          volume: session.volume > 0 ? Math.min(1, session.volume) : 0.85,
          order: session.order === 'shuffle' ? 'shuffle' : 'sequential',
          repeat: ['off', 'all', 'one'].includes(session.repeat) ? session.repeat : 'off',
          current: session.queue[Math.min(Math.max(0, session.currentIndex), session.queue.length - 1)],
          playbackState: 'paused',
          positionMs: 0,
        });
        if (audioEl) {
          audioEl.volume = session.volume > 0 ? Math.min(1, session.volume) : 0.85;
        }
      } catch (e) {
        console.error('restore session failed', e);
      }
    },

    playTracks: (tracks, startIndex) => {
      if (tracks.length === 0) return;
      const idx = Math.max(0, Math.min(startIndex, tracks.length - 1));
      set({
        queue: [...tracks],
        currentIndex: idx,
        current: tracks[idx],
        shuffleHistory: [],
        playbackState: 'loading',
        error: '',
      });
      updateMediaSession(tracks[idx], tracks[idx].artwork);
      void loadAndPlay(get());
      schedulePersist();
    },

    playFromQueue: (index) => {
      const { queue } = get();
      if (index < 0 || index >= queue.length) return;
      set({ currentIndex: index, current: queue[index], playbackState: 'loading', error: '' });
      updateMediaSession(queue[index], queue[index].artwork);
      void loadAndPlay(get());
      schedulePersist();
    },

    addToQueue: (tracks) => {
      const { queue } = get();
      set({ queue: [...queue, ...tracks] });
      schedulePersist();
    },

    removeFromQueue: (index) => {
      const { queue, currentIndex } = get();
      if (index < 0 || index >= queue.length) return;
      const newQueue = queue.filter((_, i) => i !== index);
      let newCurrent = currentIndex;
      if (index < currentIndex) newCurrent -= 1;
      else if (index === currentIndex) {
        newCurrent = Math.min(currentIndex, newQueue.length - 1);
        set({ current: newQueue[newCurrent] ?? null });
        if (newQueue.length === 0) {
          stopPlayback();
          set({ queue: [], currentIndex: -1, current: null, playbackState: 'idle', positionMs: 0, durationMs: 0 });
          schedulePersist();
          return;
        }
      }
      set({ queue: newQueue, currentIndex: newCurrent });
      schedulePersist();
    },

    clearQueue: () => {
      stopPlayback();
      set({ queue: [], currentIndex: -1, current: null, playbackState: 'idle', positionMs: 0, durationMs: 0 });
      schedulePersist();
    },

    playNext: () => {
      const { queue, currentIndex, order, current } = get();
      if (currentIndex + 1 < queue.length) {
        get().playFromQueue(currentIndex + 1);
        return;
      }
      // 之后插入播放（如果有"下一首"需求，这里简化为顺序推进）
      if (queue.length > 0 && current) {
        get().playFromQueue(Math.min(currentIndex + 1, queue.length - 1));
      }
    },

    toggle: () => {
      const { playbackState } = get();
      if (playbackState === 'playing') get().pause();
      else get().resume();
    },

    pause: () => {
      audioEl?.pause();
    },

    resume: () => {
      const s = get();
      if (!audioEl || !s.current) return;
      if (!audioEl.src) {
        void loadAndPlay(s);
        return;
      }
      void audioEl.play().catch(async () => {
        // src 过期：重新解析
        await loadAndPlay(s);
      });
    },

    next: () => {
      const s = get();
      if (s.queue.length === 0) return;
      let idx: number;
      if (s.order === 'shuffle') {
        idx = pickShuffled(s);
        if (idx < 0) return;
        s.shuffleHistory.push(s.currentIndex);
      } else {
        idx = nextSequentialIndex(s, true);
        if (idx < 0) {
          set({ playbackState: 'idle', positionMs: 0 });
          return;
        }
      }
      get().playFromQueue(idx);
    },

    previous: () => {
      const s = get();
      if (!s.queue.length) return;
      // 进度超过 3 秒先回到开头
      if (s.positionMs > 3000 && audioEl) {
        audioEl.currentTime = 0;
        set({ positionMs: 0 });
        return;
      }
      let idx: number;
      if (s.order === 'shuffle') {
        const last = s.shuffleHistory.pop();
        idx = last ?? 0;
      } else {
        idx = s.currentIndex - 1;
        if (idx < 0) idx = s.repeat === 'all' ? s.queue.length - 1 : 0;
      }
      get().playFromQueue(idx);
    },

    seek: (positionMs) => {
      if (!audioEl) return;
      audioEl.currentTime = positionMs / 1000;
      set({ positionMs });
    },

    setVolume: (v) => {
      const vol = Math.max(0, Math.min(1, v));
      set({ volume: vol, muted: vol === 0 });
      if (audioEl) {
        audioEl.volume = vol;
        audioEl.muted = vol === 0;
      }
      schedulePersist();
    },

    toggleMute: () => {
      const { muted, volume } = get();
      const newMuted = !muted;
      set({ muted: newMuted });
      if (audioEl) audioEl.muted = newMuted && volume === 0 ? true : newMuted;
      if (!newMuted && volume === 0) get().setVolume(0.5);
    },

    setOrder: (o) => {
      set({ order: o, shuffleHistory: [] });
      schedulePersist();
    },

    setRepeat: (r) => {
      set({ repeat: r });
      schedulePersist();
    },

    setQueueOpen: (v) => set({ queueOpen: v }),
    setLyricsOpen: (v) => set({ lyricsOpen: v }),

    persist: () => {
      const s = get();
      const session: SessionData = {
        queue: s.queue,
        currentIndex: s.currentIndex,
        positionMs: 0,
        volume: s.volume,
        order: s.order,
        repeat: s.repeat,
      };
      void SaveSession(session).catch(() => {});
    },
  };
  return store;
});

// 暴露到 window 便于调试（生产无副作用）
if (typeof window !== 'undefined') {
  (window as unknown as Record<string, unknown>).__player = usePlayerStore;
}

function stopPlayback() {
  if (audioEl) {
    audioEl.pause();
    audioEl.removeAttribute('src');
    audioEl.load();
  }
}

// 加载并播放当前曲目
let consecutiveFailures = 0;

async function loadAndPlay(s: PlayerState) {
  const track = s.current;
  if (!track || !audioEl) return;
  const generation = ++resolveGeneration;
  usePlayerStore.setState({ playbackState: 'loading', error: '' });

  let url: string;
  try {
    if (track.kind === 'local') {
      url = computeStreamURL(track);
    } else {
      url = await resolveQQ(track);
    }
  } catch (e) {
    if (generation !== resolveGeneration) return;
    const msg = typeof e === 'string' ? e : e instanceof Error ? e.message : '解析播放源失败';
    // 无权限（104003 等）→ 自动尝试下一首，连续失败 3 次停止
    if (/104003|104009|101404|无权限/.test(msg) && s.queue.length > 1 && consecutiveFailures < 3) {
      consecutiveFailures++;
      const nextIdx = nextIndexAfterFailure(s);
      if (nextIdx >= 0) {
        usePlayerStore.getState().playFromQueue(nextIdx);
        return;
      }
    }
    usePlayerStore.setState({
      playbackState: 'error',
      error: humanizeResolveError(msg),
    });
    return;
  }
  if (generation !== resolveGeneration || !audioEl) return;
  consecutiveFailures = 0;
  audioEl.src = url;
  audioEl.currentTime = 0;
  try {
    await audioEl.play();
    usePlayerStore.setState({ playbackState: 'playing' });
  } catch {
    if (generation !== resolveGeneration) return;
    usePlayerStore.setState({ playbackState: 'error', error: '无法开始播放' });
  }
}

function nextIndexAfterFailure(s: PlayerState): number {
  if (s.order === 'shuffle') {
    return pickShuffled(s);
  }
  const idx = s.currentIndex + 1;
  if (idx < s.queue.length) return idx;
  return s.repeat === 'all' ? 0 : -1;
}

function humanizeResolveError(msg: string): string {
  if (/104003/.test(msg)) return '该歌曲需要登录或无播放权限，已尝试跳过';
  if (/104009|101404/.test(msg)) return '该歌曲暂无可播放音源';
  if (/cgi 2001/.test(msg) || /session/.test(msg)) return '请求被限流，请稍后重试';
  if (/no playable url/.test(msg)) return '该歌曲需要登录后播放';
  return `解析播放源失败：${msg}`;
}

function schedulePersist() {
  if (persistedTimer) return;
  persistedTimer = setTimeout(() => {
    persistedTimer = null;
    usePlayerStore.getState().persist();
  }, 2000);
}

// 本地封面按需加载辅助
export async function localCover(track: Track): Promise<string | undefined> {
  if (track.kind !== 'local') return track.artwork;
  try {
    const url = (await ReadLocalCover(track.id)) as string;
    return url || undefined;
  } catch {
    return undefined;
  }
}

export { updateMediaSession };
