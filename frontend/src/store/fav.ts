// 收藏 store：QQ"我喜欢"歌单的 mid 集合 + 我创建的歌单缓存（供"收藏到歌单"选择器）。

import { create } from 'zustand';
import { QQAddToPlaylist, QQListFavoriteMids, QQMyPlaylists, QQSetFavorite } from '../api';
import { PlaylistInfo, Track } from '../types';

interface FavState {
  mids: string[];
  loaded: boolean;
  loading: boolean;
  /** 我创建的歌单（含"我喜欢"） */
  playlists: PlaylistInfo[];
  playlistsLoaded: boolean;
  playlistsLoading: boolean;
  load: () => Promise<void>;
  loadPlaylists: (force?: boolean) => Promise<PlaylistInfo[]>;
  has: (mid?: string) => boolean;
  toggle: (mid: string, songId?: string) => Promise<boolean>;
  addToPlaylist: (track: Track, playlist: PlaylistInfo) => Promise<void>;
}

export const useFavStore = create<FavState>((set, get) => ({
  mids: [],
  loaded: false,
  loading: false,
  playlists: [],
  playlistsLoaded: false,
  playlistsLoading: false,

  load: async () => {
    if (get().loading) return;
    set({ loading: true });
    try {
      const mids = (await QQListFavoriteMids()) as unknown as string[];
      set({ mids: mids || [], loaded: true, loading: false });
    } catch {
      // 未登录或网络失败：清空
      set({ mids: [], loaded: true, loading: false });
    }
  },

  loadPlaylists: async (force = false) => {
    if (!force && get().playlistsLoaded) return get().playlists;
    if (get().playlistsLoading) return get().playlists;
    set({ playlistsLoading: true });
    try {
      const list = ((await QQMyPlaylists()) || []) as unknown as PlaylistInfo[];
      // "我喜欢"排在最前，其余按服务端顺序
      const sorted = [...list].sort((a, b) => (a.dirId === 201 ? -1 : b.dirId === 201 ? 1 : 0));
      set({ playlists: sorted, playlistsLoaded: true, playlistsLoading: false });
      return sorted;
    } catch {
      set({ playlists: [], playlistsLoaded: true, playlistsLoading: false });
      return [];
    }
  },

  has: (mid) => (mid ? get().mids.includes(mid) : false),

  toggle: async (mid, songId) => {
    const had = get().mids.includes(mid);
    // 乐观更新
    set({ mids: had ? get().mids.filter((m) => m !== mid) : [...get().mids, mid] });
    try {
      const track = { mid, songId: songId || '' };
      await QQSetFavorite(track as never, !had);
      return !had; // 返回新状态
    } catch (e) {
      // 失败回滚
      set({ mids: had ? [...get().mids, mid] : get().mids.filter((m) => m !== mid) });
      throw e;
    }
  },

  addToPlaylist: async (track, playlist) => {
    if (!track.mid) throw new Error('该曲目缺少 mid');
    if (playlist.dirId === 201) {
      // "我喜欢"走收藏接口（同时维护心形状态）
      if (!get().mids.includes(track.mid)) {
        await get().toggle(track.mid, track.songId);
      }
      return;
    }
    if (typeof playlist.dirId !== 'number' || playlist.dirId <= 0) {
      throw new Error('歌单目录 ID 缺失');
    }
    await QQAddToPlaylist(track, playlist.id, playlist.dirId);
  },
}));
