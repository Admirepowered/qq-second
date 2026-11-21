// 播放队列抽屉。

import { Trash2, X } from 'lucide-react';
import { usePlayerStore } from '../store/player';
import { fmtDuration } from '../types';
import { TrackCover } from './TrackRow';

export function QueuePanel() {
  const { queue, currentIndex, queueOpen, playFromQueue, removeFromQueue, clearQueue, setQueueOpen } =
    usePlayerStore();

  if (!queueOpen) return null;

  const upcoming = queue.slice(currentIndex + 1);

  return (
    <>
      <div className="drawer-scrim" onClick={() => setQueueOpen(false)} />
      <aside className="drawer">
        <div className="drawer-header">
          <span className="drawer-title">播放队列（{queue.length}）</span>
          <div style={{ display: 'flex', gap: 4 }}>
            <button className="icon-btn" onClick={clearQueue} title="清空队列">
              <Trash2 size={16} />
            </button>
            <button className="icon-btn" onClick={() => setQueueOpen(false)} title="关闭">
              <X size={16} />
            </button>
          </div>
        </div>
        <div className="drawer-body">
          {queue.length === 0 ? (
            <div className="center-hint">队列是空的</div>
          ) : (
            <>
              {currentIndex >= 0 && queue[currentIndex] ? (
                <>
                  <div className="nav-section-label">正在播放</div>
                  <QueueRow
                    id={queue[currentIndex].id}
                    title={queue[currentIndex].title}
                    artist={(queue[currentIndex].artists || []).map((a) => a.name).join(' / ')}
                    durationMs={queue[currentIndex].durationMs}
                    current
                    onClick={() => playFromQueue(currentIndex)}
                    cover={<TrackCover track={queue[currentIndex]} />}
                    onRemove={() => removeFromQueue(currentIndex)}
                  />
                </>
              ) : null}
              {upcoming.length > 0 ? (
                <>
                  <div className="nav-section-label">接下来播放（{upcoming.length}）</div>
                  {upcoming.map((t, i) => (
                    <QueueRow
                      key={`${t.kind}-${t.id}-${i}`}
                      id={t.id}
                      title={t.title}
                      artist={(t.artists || []).map((a) => a.name).join(' / ')}
                      durationMs={t.durationMs}
                      onClick={() => playFromQueue(currentIndex + 1 + i)}
                      cover={<TrackCover track={t} />}
                      onRemove={() => removeFromQueue(currentIndex + 1 + i)}
                    />
                  ))}
                </>
              ) : null}
            </>
          )}
        </div>
      </aside>
    </>
  );
}

function QueueRow({
  title,
  artist,
  durationMs,
  current,
  onClick,
  onRemove,
  cover,
}: {
  id: string;
  title: string;
  artist: string;
  durationMs: number;
  current?: boolean;
  onClick: () => void;
  onRemove: () => void;
  cover: React.ReactNode;
}) {
  return (
    <div className={`queue-row${current ? ' current' : ''}`} onClick={onClick}>
      {cover}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div className="track-title">{title}</div>
        <div className="track-artist">{artist}</div>
      </div>
      <span className="track-duration">{fmtDuration(durationMs)}</span>
      <button
        className="icon-btn queue-remove"
        title="移除"
        onClick={(e) => {
          e.stopPropagation();
          onRemove();
        }}
      >
        <X size={14} />
      </button>
    </div>
  );
}
