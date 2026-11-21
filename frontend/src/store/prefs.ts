// 偏好设置 store：加载/保存（防抖）、系统主题监听。

import { create } from 'zustand';
import { GetSettings, SaveSettings, GetCredential } from '../api';
import { Settings, normalizeSettings, Credential, emptySettings } from '../types';
import { applyThemeTokens, PALETTES } from '../theme';

interface PrefsState {
  settings: Settings;
  credential: Credential | null;
  loaded: boolean;
  dark: boolean;
  systemDark: boolean;
  load: () => Promise<void>;
  update: (patch: Partial<Settings>) => void;
  commit: () => Promise<void>;
  refreshCredential: () => Promise<void>;
  recomputeDark: () => void;
  applyTheme: () => void;
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;

function matchSystemDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export const usePrefsStore = create<PrefsState>((set, get) => ({
  settings: emptySettings(),
  credential: null,
  loaded: false,
  dark: true,
  systemDark: matchSystemDark(),

  load: async () => {
    try {
      const [settings, credential] = await Promise.all([
        GetSettings(),
        GetCredential().catch(() => null),
      ]);
      set({
        settings: normalizeSettings(settings as unknown as Settings),
        credential: credential as Credential | null,
        loaded: true,
      });
    } catch (e) {
      console.error('load settings failed', e);
      set({ settings: normalizeSettings(null), loaded: true });
    }
    get().recomputeDark();

    // 系统主题监听
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      set({ systemDark: matchSystemDark() });
      get().recomputeDark();
    });
  },

  update: (patch) => {
    set({ settings: { ...get().settings, ...patch } });
    get().recomputeDark();
    // 200ms 防抖保存
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      void get().commit();
    }, 200);
  },

  commit: async () => {
    try {
      await SaveSettings(get().settings);
    } catch (e) {
      console.error('save settings failed', e);
    }
  },

  refreshCredential: async () => {
    try {
      const credential = await GetCredential();
      set({ credential: credential as Credential | null });
    } catch {
      set({ credential: null });
    }
  },

  recomputeDark: () => {
    const { settings, systemDark } = get();
    const dark = settings.colorMode === 'system' ? systemDark : settings.colorMode === 'dark';
    set({ dark });
    get().applyTheme();
  },

  applyTheme: () => {
    const { settings } = get();
    let primary = settings.primaryColor;
    if (settings.palette !== 'custom') {
      const preset = PALETTES.find((p) => p.id === settings.palette);
      if (preset) primary = preset.primary;
    }
    applyThemeTokens({
      dark: get().dark,
      primary,
      surfaceOpacity: settings.surfaceOpacity,
    });
  },
}));
