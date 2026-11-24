// 轻量 toast：全局单例，2.6s 自动消失（收藏/识曲等操作的即时反馈）。

import { create } from 'zustand';

type ToastKind = 'info' | 'error';

interface ToastState {
  message: string;
  kind: ToastKind;
  show: (message: string, kind?: ToastKind) => void;
  hide: () => void;
}

let timer: number | undefined;

export const useToast = create<ToastState>((set) => ({
  message: '',
  kind: 'info',
  show: (message, kind = 'info') => {
    if (timer) window.clearTimeout(timer);
    set({ message, kind });
    timer = window.setTimeout(() => set({ message: '' }), 2600);
  },
  hide: () => {
    if (timer) window.clearTimeout(timer);
    set({ message: '' });
  },
}));

export const toast = {
  info: (message: string) => useToast.getState().show(message, 'info'),
  error: (message: string) => useToast.getState().show(message, 'error'),
};
