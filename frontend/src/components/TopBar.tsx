// 顶栏：返回/前进 + 页面标题 + 搜索框 + 窗口控制。

import { KeyboardEvent, useState } from 'react';
import { ArrowLeft, ArrowRight, Minus, Search as SearchIcon, X, Square } from 'lucide-react';
import { useNavStore } from '../store/nav';
import { MinimizeWindow, ToggleMaximizeWindow, CloseWindow } from '../api';
import { pageTitle } from '../App';

export function TopBar() {
  const { route, navigate, back, forward, canBack, canForward } = useNavStore();
  const [query, setQuery] = useState('');

  const submit = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && query.trim()) {
      navigate({ page: 'search', query: query.trim() });
    }
  };

  return (
    <header className="topbar">
      <button className="icon-btn" onClick={back} disabled={!canBack()} title="返回">
        <ArrowLeft size={17} />
      </button>
      <button className="icon-btn" onClick={forward} disabled={!canForward()} title="前进">
        <ArrowRight size={17} />
      </button>
      <span className="topbar-title" style={{ minWidth: 52 }}>{pageTitle(route)}</span>
      <SearchIcon size={15} color="var(--text-secondary)" />
      <input
        className="search-input"
        placeholder="搜索歌曲、歌手、专辑、歌单"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={submit}
      />
      <div className="topbar-spacer" />
      <button className="icon-btn" onClick={() => MinimizeWindow()} title="最小化">
        <Minus size={16} />
      </button>
      <button className="icon-btn" onClick={() => ToggleMaximizeWindow()} title="最大化/还原">
        <Square size={14} />
      </button>
      <button className="icon-btn" onClick={() => CloseWindow()} title="关闭">
        <X size={17} />
      </button>
    </header>
  );
}
