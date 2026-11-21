// 收藏 store：QQ"我喜欢"歌单的 mid 集合（登录后加载，切换乐观更新）。

import { create } from 'zustand';
import { QQListFavoriteMids, QQSetFavorite } from '../api';

interface FavState {
  mids: string[];
  loaded: boolean;
  loading: boolean;
  load: () => Promise<void>;
  has: (mid?: string) => boolean;
  toggle: (mid: string, songId?: string) => Promise<boolean>;
}

export const useFavStore = create<FavState>((set, get) => ({
  mids: [],
  loaded: false,
  loading: false,

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
}));
