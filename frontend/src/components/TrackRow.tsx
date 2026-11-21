// 曲目行组件：本地封面按需加载 + QQ"我喜欢"收藏切换。

import { useEffect, useState } from 'react';
import { Heart, ListPlus, Play } from 'lucide-react';
import { Track, fmtDuration, artistText, artworkSmall } from '../types';
import { usePlayerStore, localCover } from '../store/player';
import { useFavStore } from '../store/fav';
import { usePrefsStore } from '../store/prefs';
import { useNavStore } from '../store/nav';

export function TrackCover({ track }: { track: Track }) {
  const [src, setSrc] = useState<string | undefined>(track.kind === 'qq' ? artworkSmall(track.artwork) : undefined);
  useEffect(() => {
    let cancelled = false;
    if (track.kind === 'local') {
      void localCover(track).then((v) => {
        if (!cancelled) setSrc(v);
      });
    } else {
      setSrc(artworkSmall(track.artwork));
    }
    return () => {
      cancelled = true;
    };
  }, [track.id]);
  return src ? (
    <img className="track-cover" src={src} alt="" loading="lazy" />
  ) : (
    <div className="track-cover-ph">
      <Play size={14} />
    </div>
  );
}

// 心形收藏按钮：收藏到 QQ"我喜欢"（需登录；未登录跳转设置页）
export function FavButton({ track }: { track: Track }) {
  const favMids = useFavStore((s) => s.mids);
  const toggle = useFavStore((s) => s.toggle);
  const credential = usePrefsStore((s) => s.credential);
  const navigate = useNavStore((s) => s.navigate);
  const favorited = track.mid ? favMids.includes(track.mid) : false;

  return (
    <button
      className="icon-btn"
      title={favorited ? '取消收藏' : '收藏到我喜欢'}
      style={favorited ? { color: 'var(--accent)' } : undefined}
      onClick={async (e) => {
        e.stopPropagation();
        if (!credential?.uin) {
          navigate({ page: 'settings' });
          return;
        }
        if (!track.mid) return;
        try {
          await toggle(track.mid, track.songId);
        } catch {
          /* 失败静默回滚 */
        }
      }}
    >
      <Heart size={15} fill={favorited ? 'var(--accent)' : 'none'} />
    </button>
  );
}

export function TrackRow({
  track,
  index,
  onPlay,
}: {
  track: Track;
  index: number;
  onPlay: (t: Track, i: number) => void;
}) {
  const current = usePlayerStore((s) => s.current);
  const addToQueue = usePlayerStore((s) => s.addToQueue);
  const isCurrent = current?.id === track.id && current?.kind === track.kind;

  return (
    <div
      className={`track-row${isCurrent ? ' current' : ''}`}
      onClick={() => onPlay(track, index)}
      onDoubleClick={() => onPlay(track, index)}
    >
      <span className="track-index">{isCurrent ? <Play size={13} /> : index + 1}</span>
      <TrackCover track={track} />
      <div style={{ minWidth: 0 }}>
        <div className="track-title">{track.title}</div>
        <div className="track-artist">{artistText(track)}</div>
      </div>
      <span className="track-album">{track.album || (track.kind === 'local' ? '本地音乐' : '')}</span>
      <span className="track-duration">{fmtDuration(track.durationMs)}</span>
      <div className="track-actions">
        {track.kind === 'qq' ? <FavButton track={track} /> : null}
        <button
          className="icon-btn"
          title="加入队列"
          onClick={(e) => {
            e.stopPropagation();
            addToQueue([track]);
          }}
        >
          <ListPlus size={15} />
        </button>
      </div>
    </div>
  );
}

export function TrackList({
  tracks,
  onPlayIndex,
}: {
  tracks: Track[];
  onPlayIndex: (index: number) => void;
}) {
  return (
    <div className="track-list">
      {tracks.map((t, i) => (
        <TrackRow key={`${t.kind}-${t.id}-${i}`} track={t} index={i} onPlay={(_, idx) => onPlayIndex(idx)} />
      ))}
    </div>
  );
}
