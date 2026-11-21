// 应用外壳：初始化偏好/音频/会话 → 布局（背景 + 侧栏 + 顶栏 + 页面 + 播放条 + 队列 + 歌词）。

import { useEffect } from 'react';
import { AppBackground } from './components/AppBackground';
import { Sidebar } from './components/Sidebar';
import { TopBar } from './components/TopBar';
import { PlayerBar } from './components/PlayerBar';
import { QueuePanel } from './components/QueuePanel';
import { LyricsPanel } from './components/LyricsPanel';
import { HomePage } from './pages/HomePage';
import { SearchPage } from './pages/SearchPage';
import { ChartsPage, ChartDetailPage } from './pages/ChartsPage';
import { PlaylistPage, AlbumPage, ArtistPage } from './pages/DetailPages';
import { LocalMusicPage } from './pages/LocalMusicPage';
import { SettingsPage } from './pages/SettingsPage';
import { useNavStore, AppRoute } from './store/nav';
import { usePrefsStore } from './store/prefs';
import { usePlayerStore } from './store/player';
import { EventsOn } from './api';

const TITLES: Record<AppRoute['page'], string> = {
  home: '发现',
  search: '搜索',
  charts: '排行榜',
  playlist: '歌单',
  chart: '排行榜',
  album: '专辑',
  artist: '歌手',
  local: '本地音乐',
  settings: '设置',
};

export default function App() {
  const route = useNavStore((s) => s.route);
  const { load, loaded } = usePrefsStore();
  const { initAudio, restoreSession } = usePlayerStore();

  useEffect(() => {
    initAudio();
    void load();
    void restoreSession();
    // 托盘菜单 → 播放控制
    const offPlayback = EventsOn('tray:playback', (cmd: string) => {
      const p = usePlayerStore.getState();
      switch (cmd) {
        case 'toggle':
          p.toggle();
          break;
        case 'previous':
          p.previous();
          break;
        case 'next':
          p.next();
          break;
        case 'lyrics':
          p.setLyricsOpen(!p.lyricsOpen);
          break;
      }
    });
    return () => {
      offPlayback?.();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  let page: React.ReactNode = null;
  switch (route.page) {
    case 'home':
      page = <HomePage />;
      break;
    case 'search':
      page = <SearchPage key={route.query ?? '_'} initialQuery={route.query} />;
      break;
    case 'charts':
      page = <ChartsPage />;
      break;
    case 'chart':
      page = <ChartDetailPage id={route.id} name={route.name} />;
      break;
    case 'playlist':
      page = <PlaylistPage id={route.id} />;
      break;
    case 'album':
      page = <AlbumPage id={route.id} />;
      break;
    case 'artist':
      page = <ArtistPage id={route.id} />;
      break;
    case 'local':
      page = <LocalMusicPage />;
      break;
    case 'settings':
      page = <SettingsPage />;
      break;
  }

  return (
    <>
      <AppBackground />
      <div className="app-shell">
        <Sidebar />
        <div className="content-shell">
          <TopBar />
          <main className="main-content">{loaded ? page : null}</main>
        </div>
        <PlayerBar />
      </div>
      <QueuePanel />
      <LyricsPanel />
    </>
  );
}

// 页面标题（顶栏显示当前页名）
export function pageTitle(route: AppRoute): string {
  if (route.page === 'chart') return route.name;
  return TITLES[route.page];
}
