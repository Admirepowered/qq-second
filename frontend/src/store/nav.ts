// 轻量路由：AppRoute union + 历史栈（参考 YAQMC navigation 思路）。

import { create } from 'zustand';

export type AppRoute =
  | { page: 'home' }
  | { page: 'search'; query?: string }
  | { page: 'charts' }
  | { page: 'playlist'; id: string }
  | { page: 'chart'; id: number; name: string }
  | { page: 'album'; id: string }
  | { page: 'artist'; id: string }
  | { page: 'local' }
  | { page: 'settings' };

interface NavState {
  route: AppRoute;
  history: AppRoute[];
  future: AppRoute[];
  navigate: (route: AppRoute) => void;
  back: () => void;
  forward: () => void;
  canBack: () => boolean;
  canForward: () => boolean;
}

export const useNavStore = create<NavState>((set, get) => ({
  route: { page: 'home' },
  history: [],
  future: [],
  navigate: (route) => {
    const { route: cur, history } = get();
    if (sameRoute(cur, route)) return;
    set({ route, history: [...history, cur], future: [] });
    window.scrollTo(0, 0);
  },
  back: () => {
    const { history, route, future } = get();
    if (history.length === 0) return;
    const prev = history[history.length - 1];
    set({ route: prev, history: history.slice(0, -1), future: [route, ...future] });
  },
  forward: () => {
    const { future, route, history } = get();
    if (future.length === 0) return;
    const next = future[0];
    set({ route: next, future: future.slice(1), history: [...history, route] });
  },
  canBack: () => get().history.length > 0,
  canForward: () => get().future.length > 0,
}));

function sameRoute(a: AppRoute, b: AppRoute): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}
