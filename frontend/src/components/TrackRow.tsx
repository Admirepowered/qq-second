// 曲目行组件：本地封面按需加载。

import { useEffect, useState } from 'react';
import { Heart, ListPlus, Play } from 'lucide-react';
import { Track, fmtDuration, artistText } from '../types';
import { usePlayerStore, localCover } from '../store/player';

export function TrackCover({ track }: { track: Track }) {
  const [src, setSrc] = useState<string | undefined>(track.kind === 'qq' ? track.artwork : undefined);
  useEffect(() => {
    let cancelled = false;
    if (track.kind === 'local') {
      void localCover(track).then((v) => {
        if (!cancelled) setSrc(v);
      });
    } else {
      setSrc(track.artwork);
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

// 心形收藏按钮（视觉占位，未接账号收藏）
export function FavButton() {
  return (
    <button className="icon-btn" title="收藏（需要登录）">
      <Heart size={15} />
    </button>
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
