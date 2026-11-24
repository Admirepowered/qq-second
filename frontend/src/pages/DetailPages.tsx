// 歌单/专辑/歌手详情页。

import { useEffect, useState } from 'react';
import { QQPlaylist, QQAlbum, QQArtist } from '../api';
import { PlaylistDetail, AlbumDetail, ArtistDetail, artworkSmall } from '../types';
import { usePlayerStore } from '../store/player';
import { useNavStore } from '../store/nav';
import { TrackList } from '../components/TrackRow';
import { Loading, ErrorHint } from './HomePage';

export function PlaylistPage({ id }: { id: string }) {
  const [detail, setDetail] = useState<PlaylistDetail | null>(null);
  const [error, setError] = useState('');
  const playTracks = usePlayerStore((s) => s.playTracks);

  useEffect(() => {
    let cancelled = false;
    setDetail(null);
    setError('');
    QQPlaylist(id, 0)
      .then((d) => {
        if (!cancelled) setDetail(d as PlaylistDetail);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (error) return <ErrorHint message={`加载失败：${error}`} />;
  if (!detail) return <Loading />;

  return (
    <div>
      <div className="page-header">
        {detail.info.cover ? (
          <img className="page-header-cover" src={artworkSmall(detail.info.cover)} alt="" />
        ) : (
          <div className="page-header-cover-ph">🎵</div>
        )}
        <div className="page-header-info">
          <div className="page-header-title">{detail.info.title || '歌单'}</div>
          {detail.info.creator ? <div className="page-header-sub">创建者：{detail.info.creator}</div> : null}
          <div className="page-header-sub">
            共 {detail.total || detail.tracks.length} 首{detail.info.updateTime ? ` · ${detail.info.updateTime}` : ''}
          </div>
          {detail.info.desc ? <div className="page-header-sub" style={{ maxHeight: 60, overflow: 'hidden' }}>{detail.info.desc}</div> : null}
          <div className="page-header-actions">
            <button className="btn primary" disabled={detail.tracks.length === 0} onClick={() => playTracks(detail.tracks, 0)}>
              ▶ 播放全部
            </button>
          </div>
        </div>
      </div>
      <TrackList tracks={detail.tracks} onPlayIndex={(i) => playTracks(detail.tracks, i)} />
    </div>
  );
}

export function AlbumPage({ id }: { id: string }) {
  const [detail, setDetail] = useState<AlbumDetail | null>(null);
  const [error, setError] = useState('');
  const playTracks = usePlayerStore((s) => s.playTracks);
  const navigate = useNavStore((s) => s.navigate);

  useEffect(() => {
    let cancelled = false;
    setDetail(null);
    setError('');
    QQAlbum(id)
      .then((d) => {
        if (!cancelled) setDetail(d as AlbumDetail);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (error) return <ErrorHint message={`加载失败：${error}`} />;
  if (!detail) return <Loading />;

  return (
    <div>
      <div className="page-header">
        {detail.cover ? (
          <img className="page-header-cover" src={artworkSmall(detail.cover)} alt="" />
        ) : (
          <div className="page-header-cover-ph">💿</div>
        )}
        <div className="page-header-info">
          <div className="page-header-title">{detail.title || '专辑'}</div>
          <div className="page-header-sub">
            {detail.artists?.length ? (
              <button className="section-more" onClick={() => navigate({ page: 'artist', id: detail.artists[0].mid ?? '' })}>
                {detail.artists.map((a) => a.name).join(' / ')}
              </button>
            ) : null}
            {detail.releaseYear ? ` · ${detail.releaseYear}` : ''}
            {detail.company ? ` · ${detail.company}` : ''}
          </div>
          <div className="page-header-sub">共 {detail.tracks.length} 首</div>
          {detail.desc ? <div className="page-header-sub" style={{ maxHeight: 60, overflow: 'hidden' }}>{detail.desc}</div> : null}
          <div className="page-header-actions">
            <button className="btn primary" disabled={detail.tracks.length === 0} onClick={() => playTracks(detail.tracks, 0)}>
              ▶ 播放全部
            </button>
          </div>
        </div>
      </div>
      <TrackList tracks={detail.tracks} onPlayIndex={(i) => playTracks(detail.tracks, i)} />
    </div>
  );
}

export function ArtistPage({ id }: { id: string }) {
  const [detail, setDetail] = useState<ArtistDetail | null>(null);
  const [error, setError] = useState('');
  const playTracks = usePlayerStore((s) => s.playTracks);
  const navigate = useNavStore((s) => s.navigate);

  useEffect(() => {
    let cancelled = false;
    setDetail(null);
    setError('');
    QQArtist(id)
      .then((d) => {
        if (!cancelled) setDetail(d as ArtistDetail);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (error) return <ErrorHint message={`加载失败：${error}`} />;
  if (!detail) return <Loading />;

  return (
    <div>
      <div className="page-header">
        {detail.cover ? (
          <img className="page-header-cover" src={artworkSmall(detail.cover)} alt="" style={{ borderRadius: '50%' }} />
        ) : (
          <div className="page-header-cover-ph" style={{ borderRadius: '50%' }}>🎤</div>
        )}
        <div className="page-header-info">
          <div className="page-header-title">{detail.name || '歌手'}</div>
          <div className="page-header-actions">
            <button className="btn primary" disabled={detail.songs.length === 0} onClick={() => playTracks(detail.songs, 0)}>
              ▶ 播放热门 {detail.songs.length} 首
            </button>
          </div>
        </div>
      </div>

      {detail.songs.length > 0 ? (
        <section className="section">
          <div className="section-header">
            <span className="section-title">热门歌曲</span>
          </div>
          <TrackList tracks={detail.songs} onPlayIndex={(i) => playTracks(detail.songs, i)} />
        </section>
      ) : null}

      {detail.albums.length > 0 ? (
        <section className="section">
          <div className="section-header">
            <span className="section-title">专辑</span>
          </div>
          <div className="card-grid">
            {detail.albums.map((a) => (
              <button key={a.id} className="card" onClick={() => navigate({ page: 'album', id: a.id })}>
                {a.cover ? <img className="card-cover" src={artworkSmall(a.cover)} alt="" loading="lazy" /> : <div className="card-cover" />}
                <div className="card-title">{a.title}</div>
                {a.releaseYear ? <div className="card-sub">{a.releaseYear}</div> : null}
              </button>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
