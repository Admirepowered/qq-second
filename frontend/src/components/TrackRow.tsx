// 曲目行组件：本地封面按需加载 + 右键菜单（播放/队列/收藏到指定歌单）+ QQ"我喜欢"状态。

import { MouseEvent as ReactMouseEvent, useEffect, useState } from 'react';
import { Heart, ListPlus, Play } from 'lucide-react';
import { Track, fmtDuration, artistText, artworkSmall } from '../types';
import { usePlayerStore, localCover } from '../store/player';
import { useFavStore } from '../store/fav';
import { usePrefsStore } from '../store/prefs';
import { useNavStore } from '../store/nav';
import { toast } from '../store/toast';
import { useContextMenu, MenuItemDef } from './ContextMenu';

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

function errText(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e ?? '未知错误');
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
  const loadPlaylists = useFavStore((s) => s.loadPlaylists);
  const addToPlaylist = useFavStore((s) => s.addToPlaylist);
  const credential = usePrefsStore((s) => s.credential);
  const navigate = useNavStore((s) => s.navigate);
  const openMenu = useContextMenu((s) => s.open);
  const isCurrent = current?.id === track.id && current?.kind === track.kind;
  const favorited = track.mid ? favMids.includes(track.mid) : false;

  // 主菜单（收藏选择器里的"返回"也复用它）
  const buildMainMenu = (x: number, y: number): MenuItemDef[] => {
    const items: MenuItemDef[] = [
      { label: '播放', onClick: () => onPlay(track, index) },
      { label: '加入队列', onClick: () => addToQueue([track]) },
      { separator: true },
    ];
    if (!credential?.uin) {
      items.push({ label: '收藏（需登录）', onClick: () => navigate({ page: 'settings' }) });
    } else {
      if (favorited) {
        items.push({
          label: '取消收藏（我喜欢）',
          onClick: () => {
            void (async () => {
              try {
                await toggleFav(track.mid!, track.songId);
                toast.info('已取消收藏');
              } catch (e) {
                toast.error(`取消收藏失败：${errText(e)}`);
              }
            })();
          },
        });
      }
      items.push({
        label: '收藏到歌单…',
        onClick: () => void openPlaylistPicker(x, y),
      });
    }
    items.push({ separator: true }, { label: `歌曲 ID：${track.mid}`, disabled: true });
    return items;
  };

  // 歌单选择器：在同一位置把菜单替换成歌单列表（含"我喜欢"），支持返回
  const openPlaylistPicker = async (x: number, y: number) => {
    openMenu(x, y, [{ label: '正在加载歌单…', disabled: true }]);
    const playlists = await loadPlaylists();
    const items: MenuItemDef[] = [
      { label: '← 返回', onClick: () => openMenu(x, y, buildMainMenu(x, y)) },
      { separator: true },
    ];
    if (playlists.length === 0) {
      items.push({ label: '没有可用歌单（需登录）', disabled: true });
    }
    for (const pl of playlists) {
      const isFav = pl.dirId === 201;
      const suffix = isFav ? '' : pl.songCount ? `（${pl.songCount} 首）` : '';
      items.push({
        label: `${isFav ? '♥ ' : ''}${pl.title}${suffix}`,
        onClick: () => {
          void (async () => {
            try {
              await addToPlaylist(track, pl);
              toast.info(`已收藏到「${pl.title}」`);
            } catch (e) {
              toast.error(`收藏失败：${errText(e)}`);
            }
          })();
        },
      });
    }
    openMenu(x, y, items);
  };

  const onContextMenu = (e: ReactMouseEvent) => {
    e.preventDefault();
    openMenu(e.clientX, e.clientY, buildMainMenu(e.clientX, e.clientY));
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
