// 账号页：收藏的歌曲 + 我创建的歌单 + 收藏的歌单（需登录）。

import { useEffect, useState } from 'react';
import { QQUserAccount } from '../api';
import { UserAccount, artworkSmall } from '../types';
import { usePlayerStore } from '../store/player';
import { useNavStore } from '../store/nav';
import { TrackList } from '../components/TrackRow';
import { Loading, ErrorHint } from './HomePage';

export function AccountPage() {
  const [account, setAccount] = useState<UserAccount | null>(null);
  const [error, setError] = useState('');
  const playTracks = usePlayerStore((s) => s.playTracks);
  const navigate = useNavStore((s) => s.navigate);

  useEffect(() => {
    let cancelled = false;
    QQUserAccount()
      .then((a) => {
        if (!cancelled) setAccount(a as unknown as UserAccount);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <div>
        <div className="page-header">
          <div className="page-header-info">
            <div className="page-header-title">我的音乐</div>
          </div>
        </div>
        <ErrorHint message={`加载失败：${error}（需要登录后才能查看）`} />
      </div>
    );
  }
  if (!account) return <Loading />;

  return (
    <div>
      <div className="page-header">
        {account.avatar ? (
          <img className="page-header-cover" src={account.avatar} alt="" style={{ borderRadius: '50%', width: 120, height: 120 }} />
        ) : (
          <div className="page-header-cover-ph" style={{ borderRadius: '50%', width: 120, height: 120 }}>👤</div>
        )}
        <div className="page-header-info">
          <div className="page-header-title">{account.nickname || account.uin}</div>
          <div className="page-header-sub">
            收藏 {account.favSongs.length} 首 · 创建歌单 {account.myPlaylists.length} 个
          </div>
          <div className="page-header-actions">
            <button
              className="btn primary"
              disabled={account.favSongs.length === 0}
              onClick={() => playTracks(account.favSongs, 0)}
            >
              ▶ 播放收藏
            </button>
          </div>
        </div>
      </div>

      {account.myPlaylists.length > 0 ? (
        <section className="section">
          <div className="section-header">
            <span className="section-title">我创建的歌单</span>
          </div>
          <div className="card-grid">
            {account.myPlaylists.map((p) => (
              <PlaylistCard key={p.id} id={p.id} title={p.title} cover={p.cover} sub={`${p.songCount ?? 0} 首`} />
            ))}
          </div>
        </section>
      ) : null}

      {account.favSongs.length > 0 ? (
        <section className="section">
          <div className="section-header">
            <span className="section-title">收藏的歌曲（我喜欢的音乐）</span>
          </div>
          <TrackList tracks={account.favSongs} onPlayIndex={(i) => playTracks(account.favSongs, i)} />
        </section>
      ) : null}

      {account.favSongs.length === 0 && account.myPlaylists.length === 0 ? (
        <div className="center-hint">账号下暂无内容</div>
      ) : null}
    </div>
  );
}

function PlaylistCard({ id, title, cover, sub }: { id: string; title: string; cover?: string; sub: string }) {
  const navigate = useNavStore((s) => s.navigate);
  return (
    <button className="card" onClick={() => navigate({ page: 'playlist', id })}>
      <div className="card-cover-wrap">
        {cover ? <img className="card-cover" src={artworkSmall(cover)} alt="" loading="lazy" /> : <div className="card-cover" />}
        <div className="card-play-overlay">▶</div>
      </div>
      <div className="card-title">{title}</div>
      <div className="card-sub">{sub}</div>
    </button>
  );
}
