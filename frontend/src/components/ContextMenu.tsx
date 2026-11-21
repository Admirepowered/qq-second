// 全局右键菜单：单例 store + 定位组件（点击外部/Esc 关闭）。

import { useEffect } from 'react';
import { create } from 'zustand';

export interface MenuItemDef {
  label?: string;
  onClick?: () => void;
  separator?: boolean;
  disabled?: boolean;
  danger?: boolean;
}

interface ContextMenuState {
  x: number;
  y: number;
  items: MenuItemDef[];
  open: (x: number, y: number, items: MenuItemDef[]) => void;
  close: () => void;
}

export const useContextMenu = create<ContextMenuState>((set) => ({
  x: 0,
  y: 0,
  items: [],
  open: (x, y, items) => set({ x, y, items }),
  close: () => set({ items: [] }),
}));

export function ContextMenuHost() {
  const { x, y, items, close } = useContextMenu();
  const visible = items.length > 0;

  useEffect(() => {
    if (!visible) return;
    const onDown = (e: MouseEvent) => {
      const el = e.target as HTMLElement;
      if (!el.closest('.context-menu')) close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close();
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey);
    window.addEventListener('blur', close);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey);
      window.removeEventListener('blur', close);
    };
  }, [visible, close]);

  if (!visible) return null;

  // 视口越界钳制
  const menuH = items.length * 34 + 12;
  const menuW = 180;
  const px = Math.min(x, window.innerWidth - menuW - 8);
  const py = Math.min(y, window.innerHeight - menuH - 8);

  return (
    <div
      className="context-menu"
      style={{ left: px, top: py }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item, i) =>
        item.separator ? (
          <div key={i} className="context-menu-sep" />
        ) : (
          <button
            key={i}
            className={`context-menu-item${item.danger ? ' danger' : ''}`}
            disabled={item.disabled}
            onClick={() => {
              close();
              item.onClick?.();
            }}
          >
            {item.label}
          </button>
        )
      )}
    </div>
  );
}
