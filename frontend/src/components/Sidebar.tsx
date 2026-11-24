// 侧栏导航。

import { AudioLines, Database, Disc3, Heart, Home, ListMusic, Search as SearchIcon, Settings, TrendingUp } from 'lucide-react';
import { useNavStore } from '../store/nav';
import { usePlayerStore } from '../store/player';
import { usePrefsStore } from '../store/prefs';

export function Sidebar() {
  const { route, navigate } = useNavStore();
  const setLyricsOpen = usePlayerStore((s) => s.setLyricsOpen);
  const lyricsOpen = usePlayerStore((s) => s.lyricsOpen);
  const credential = usePrefsStore((s) => s.credential);

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
      <button className={item(route.page === 'identify')} onClick={() => go({ page: 'identify' })}>
        <AudioLines size={17} /> 听歌识曲
      </button>

      <div className="nav-section-label">你的音乐</div>
      {credential?.uin ? (
        <button className={item(route.page === 'account')} onClick={() => go({ page: 'account' })}>
          <Heart size={17} /> 我的收藏
        </button>
      ) : null}
      <button className={item(route.page === 'cache')} onClick={() => go({ page: 'cache' })}>
        <Database size={17} /> 缓存歌曲
      </button>
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
