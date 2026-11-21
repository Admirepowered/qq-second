// 首页：推荐歌单 + 新歌 + 榜单入口。

import { useEffect, useState } from 'react';
import { QQHome, QQCharts } from '../api';
import { HomeFeed, ChartInfo, Track, PlaylistInfo, artworkSmall } from '../types';
import { usePlayerStore } from '../store/player';
import { useNavStore } from '../store/nav';
import { TrackList } from '../components/TrackRow';

export function HomePage() {
  const [feed, setFeed] = useState<HomeFeed | null>(null);
  const [charts, setCharts] = useState<ChartInfo[]>([]);
  const [error, setError] = useState('');
  const playTracks = usePlayerStore((s) => s.playTracks);
  const navigate = useNavStore((s) => s.navigate);

  useEffect(() => {
    let cancelled = false;
    QQHome()
      .then((f) => {
        if (!cancelled) setFeed(f as HomeFeed);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    QQCharts()
      .then((c) => {
        if (!cancelled) setCharts(c as ChartInfo[]);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) return <div className="center-hint">加载失败：{error}</div>;
  if (!feed) return <Loading />;

  const playlists: PlaylistInfo[] = feed.playlists || [];
  const newSongs: Track[] = feed.newSongs || [];

  return (
    <div>
      <section className="section">
        <div className="section-header">
          <span className="section-title">排行榜</span>
        </div>
        <div className="card-grid">
          {charts.map((c) => (
            <button key={c.id} className="card" onClick={() => navigate({ page: 'chart', id: c.id, name: c.name })}>
              <div className="card-cover-wrap">
                <div
                  className="card-cover"
                  style={{
                    background: `linear-gradient(135deg, var(--accent) 0%, transparent 140%), rgba(255,255,255,0.06)`,
                    display: 'grid',
                    placeItems: 'center',
                    fontSize: 26,
                    fontWeight: 800,
                    color: '#fff',
                  }}
                >
                  {c.name.slice(0, 2)}
                </div>
              </div>
              <div className="card-title">{c.name}</div>
            </button>
          ))}
        </div>
      </section>

      {newSongs.length > 0 ? (
        <section className="section">
          <div className="section-header">
            <span className="section-title">新歌速递</span>
            <button
              className="section-more"
              onClick={() => playTracks(newSongs, 0)}
            >
              播放全部
            </button>
          </div>
          <TrackList tracks={newSongs.slice(0, 10)} onPlayIndex={(i) => playTracks(newSongs.slice(0, 10), i)} />
        </section>
      ) : null}

      {playlists.length > 0 ? (
        <section className="section">
          <div className="section-header">
            <span className="section-title">推荐歌单</span>
          </div>
          <div className="card-grid">
            {playlists.map((p) => (
              <button key={p.id} className="card" onClick={() => navigate({ page: 'playlist', id: p.id })}>
                <div className="card-cover-wrap">
                  {p.cover ? <img className="card-cover" src={artworkSmall(p.cover)} alt="" loading="lazy" /> : <div className="card-cover" />}
                  <div className="card-play-overlay">▶</div>
                </div>
                <div className="card-title">{p.title}</div>
                {p.creator ? <div className="card-sub">{p.creator}</div> : null}
              </button>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}

export function Loading() {
  return (
    <div className="center-hint">
      <div className="spinner" />
      加载中…
    </div>
  );
}

export function ErrorHint({ message }: { message: string }) {
  return <div className="center-hint">{message}</div>;
}
