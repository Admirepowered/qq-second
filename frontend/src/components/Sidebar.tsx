// 侧栏导航。

import { Disc3, Home, ListMusic, Search as SearchIcon, Settings, TrendingUp } from 'lucide-react';
import { useNavStore } from '../store/nav';
import { usePlayerStore } from '../store/player';

export function Sidebar() {
  const { route, navigate } = useNavStore();
  const setLyricsOpen = usePlayerStore((s) => s.setLyricsOpen);
  const lyricsOpen = usePlayerStore((s) => s.lyricsOpen);

  const go = (r: Parameters<typeof navigate>[0]) => {
    // 歌词页打开时导航会关闭歌词
    if (lyricsOpen) setLyricsOpen(false);
    navigate(r);
  };

  const item = (active: boolean) => `nav-item${active ? ' active' : ''}`;

  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-dot">
          <Disc3 size={16} />
        </span>
        QQ Second
      </div>

      <button className={item(route.page === 'home')} onClick={() => go({ page: 'home' })}>
        <Home size={17} /> 发现
      </button>
      <button className={item(route.page === 'search')} onClick={() => go({ page: 'search' })}>
        <SearchIcon size={17} /> 搜索
      </button>
      <button className={item(route.page === 'charts')} onClick={() => go({ page: 'charts' })}>
        <TrendingUp size={17} /> 排行榜
      </button>

      <div className="nav-section-label">你的音乐</div>
      <button className={item(route.page === 'local')} onClick={() => go({ page: 'local' })}>
        <ListMusic size={17} /> 本地音乐
      </button>

      <div className="sidebar-spacer" />
      <button className={item(route.page === 'settings')} onClick={() => go({ page: 'settings' })}>
        <Settings size={17} /> 设置
      </button>
    </aside>
  );
}
