// 曲目行组件：本地封面按需加载 + 右键菜单（收藏/加入队列）+ QQ"我喜欢"状态。

import { MouseEvent as ReactMouseEvent, useEffect, useState } from 'react';
import { Heart, ListPlus, Play } from 'lucide-react';
import { Track, fmtDuration, artistText, artworkSmall } from '../types';
import { usePlayerStore, localCover } from '../store/player';
import { useFavStore } from '../store/fav';
import { usePrefsStore } from '../store/prefs';
import { useNavStore } from '../store/nav';
import { useContextMenu } from './ContextMenu';

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
  const favMids = useFavStore((s) => s.mids);
  const toggleFav = useFavStore((s) => s.toggle);
  const credential = usePrefsStore((s) => s.credential);
  const navigate = useNavStore((s) => s.navigate);
  const openMenu = useContextMenu((s) => s.open);
  const isCurrent = current?.id === track.id && current?.kind === track.kind;
  const favorited = track.mid ? favMids.includes(track.mid) : false;

  const onContextMenu = (e: ReactMouseEvent) => {
    e.preventDefault();
    openMenu(e.clientX, e.clientY, [
      { label: '播放', onClick: () => onPlay(track, index) },
      { label: '加入队列', onClick: () => addToQueue([track]) },
      { separator: true },
      credential?.uin
        ? {
            label: favorited ? '取消收藏' : '收藏到我喜欢',
            onClick: async () => {
              if (!track.mid) return;
              try {
                await toggleFav(track.mid, track.songId);
              } catch {
                /* 失败静默回滚 */
              }
            },
          }
        : { label: '收藏（需登录）', onClick: () => navigate({ page: 'settings' }) },
      { separator: true },
      { label: `歌曲 ID：${track.mid}`, disabled: true },
    ]);
  };

  return (
    <div
      className={`track-row${isCurrent ? ' current' : ''}`}
      onClick={() => onPlay(track, index)}
      onDoubleClick={() => onPlay(track, index)}
      onContextMenu={onContextMenu}
    >
      <span className="track-index">{isCurrent ? <Play size={13} /> : index + 1}</span>
      <TrackCover track={track} />
      <div style={{ minWidth: 0 }}>
        <div className="track-title">
          {track.title}
          {favorited ? <Heart size={11} fill="var(--accent)" color="var(--accent)" style={{ marginLeft: 5, verticalAlign: -1 }} /> : null}
        </div>
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
