// 主题系统：主色预设 + 运行时 CSS 变量生成（参考 YAQMC theme-tokens 思路）。

export interface PalettePreset {
  id: string;
  label: string;
  primary: string;
  secondary: string;
}

export const PALETTES: PalettePreset[] = [
  { id: 'default', label: '经典绿', primary: '#31c27c', secondary: '#1e9e63' },
  { id: 'ocean', label: '海洋', primary: '#3d8bd4', secondary: '#2b6ba8' },
  { id: 'violet', label: '紫罗兰', primary: '#8b6bd4', secondary: '#6a4bb0' },
  { id: 'sakura', label: '樱花', primary: '#e07a97', secondary: '#b85a76' },
  { id: 'ember', label: '琥珀', primary: '#e09a3d', secondary: '#b87a2a' },
  { id: 'mint', label: '薄荷', primary: '#3dc4a0', secondary: '#2a9e80' },
  { id: 'mono', label: '石墨', primary: '#8a8f98', secondary: '#6b7078' },
];

// 颜色工具
function hexToRgb(hex: string): [number, number, number] {
  const clean = hex.replace('#', '');
  const full = clean.length === 3 ? clean.split('').map((c) => c + c).join('') : clean;
  const num = parseInt(full, 16);
  return [(num >> 16) & 255, (num >> 8) & 255, num & 255];
}

function rgbToHex(r: number, g: number, b: number): string {
  return '#' + [r, g, b].map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
}

export function mixColor(a: string, b: string, ratio: number): string {
  const [r1, g1, b1] = hexToRgb(a);
  const [r2, g2, b2] = hexToRgb(b);
  return rgbToHex(r1 + (r2 - r1) * ratio, g1 + (g2 - g1) * ratio, b1 + (b2 - b1) * ratio);
}

function luminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255;
}

// 生成主题 CSS 变量并写到 documentElement
export function applyThemeTokens(opts: {
  dark: boolean;
  primary: string;
  surfaceOpacity: number;
}) {
  const { dark, primary, surfaceOpacity } = opts;
  const root = document.documentElement;

  const accent = primary;
  const accentInk = luminance(primary) > 0.62 ? '#101210' : '#ffffff';
  const focus = mixColor(primary, dark ? '#ffffff' : '#000000', 0.25);

  // 背景
  const bg = dark ? '#17191c' : '#f4f6f8';
  const bgOpaque = dark ? '#1a1d21' : '#ffffff';
  const surface = dark ? '#20242a' : '#ffffff';
  const sidebar = dark ? '#1b1e23' : '#f0f3f5';
  const player = dark ? '#1d2126' : '#ffffff';
  const textPrimary = dark ? '#e8eaed' : '#1c1e22';
  const textSecondary = dark ? '#9aa0a8' : '#5c6470';
  const border = dark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.08)';

  const baseAlpha = surfaceOpacity / 100;

  // 毛玻璃面板底色：alpha 直接由"面板不透明度"滑杆驱动，
  // 使背景在整个窗口可见（面板越透，背景越明显）。
  const panelGlass = dark
    ? `rgba(18, 20, 24, ${baseAlpha})`
    : `rgba(255, 255, 255, ${baseAlpha})`;
  const panelStrong = dark
    ? `rgba(18, 20, 24, ${Math.min(1, baseAlpha + 0.1)})`
    : `rgba(255, 255, 255, ${Math.min(1, baseAlpha + 0.1)})`;
  const cardGlass = dark
    ? `rgba(24, 27, 31, ${baseAlpha * 0.6})`
    : `rgba(255, 255, 255, ${baseAlpha * 0.9})`;

  const vars: Record<string, string> = {
    '--bg': bg,
    '--bg-opaque': bgOpaque,
    '--surface': surface,
    '--sidebar': sidebar,
    '--player': player,
    '--accent': accent,
    '--accent-ink': accentInk,
    '--accent-dim': mixColor(accent, dark ? '#000000' : '#ffffff', 0.35),
    '--focus': focus,
    '--text-primary': textPrimary,
    '--text-secondary': textSecondary,
    '--border': border,
    '--panel-glass': panelGlass,
    '--panel-glass-strong': panelStrong,
    '--card-glass': cardGlass,
    '--surface-base-alpha': String(baseAlpha),
    '--surface-alpha': String(baseAlpha),
    '--surface-soft-alpha': String(Math.max(0.15, baseAlpha - 0.04)),
    '--surface-strong-alpha': String(Math.min(1, baseAlpha + 0.08)),
    '--surface-button-alpha': String(Math.max(0.12, baseAlpha - 0.2)),
    '--text-alpha': '1',
  };

  for (const [k, v] of Object.entries(vars)) {
    root.style.setProperty(k, v);
  }
  root.dataset.theme = dark ? 'dark' : 'light';
}
