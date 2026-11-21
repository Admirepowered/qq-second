// 播放条：三栏布局 + rAF 直写进度条。

import { MouseEvent, useEffect, useRef, useState } from 'react';
import {
  Disc3,
  ListMusic,
  Mic2,
  Pause,
  Play,
  Repeat,
  Repeat1,
  Shuffle,
  SkipBack,
  SkipForward,
  Volume1,
  Volume2,
  VolumeX,
} from 'lucide-react';
import { usePlayerStore } from '../store/player';
import { useNavStore } from '../store/nav';
import { fmtDuration } from '../types';
import { TrackCover } from './TrackRow';

function setRangeProgress(el: HTMLInputElement | null, pct: number) {
  if (el) el.style.setProperty('--range-progress', `${Math.max(0, Math.min(100, pct))}%`);
}

export function PlayerBar() {
  const player = usePlayerStore();
  const {
    current,
    playbackState,
    positionMs,
    durationMs,
    volume,
    muted,
    order,
    repeat,
    quality,
    queue,
    queueOpen,
  } = player;
  const navigate = useNavStore((s) => s.navigate);
  const progressRef = useRef<HTMLInputElement>(null);
  const volumeRef = useRef<HTMLInputElement>(null);
  const [scrubbing, setScrubbing] = useState(false);

  // rAF 循环直写 DOM（绕过 React 重渲染）
  useEffect(() => {
    let raf = 0;
    const tick = () => {
      if (!scrubbing) {
        const s = usePlayerStore.getState();
        const pct = s.durationMs > 0 ? (s.positionMs / s.durationMs) * 100 : 0;
        setRangeProgress(progressRef.current, pct);
        if (progressRef.current) {
          progressRef.current.value = String(s.positionMs / 1000);
        }
        setRangeProgress(volumeRef.current, s.muted ? 0 : s.volume * 100);
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [scrubbing]);

  const beginScrub = () => setScrubbing(true);
  const previewScrub = (e: MouseEvent<HTMLInputElement>) => {
    if (!scrubbing) return;
    const el = e.currentTarget;
    const pct = Number(el.value) * 1000 / Math.max(1, durationMs) * 100;
    setRangeProgress(el, pct);
  };
  const commitScrub = (e: MouseEvent<HTMLInputElement>) => {
    if (!scrubbing) return;
    setScrubbing(false);
    const ms = Math.floor(Number(e.currentTarget.value) * 1000);
    player.seek(ms);
  };

  const playing = playbackState === 'playing';
  const loading = playbackState === 'loading';
  const errorMsg = playbackState === 'error' ? player.error : '';

  const cycleMode = () => {
    // 顺序 → 随机 → 单曲循环 → 列表循环 → 顺序
    if (order === 'sequential' && repeat === 'off') {
      player.setOrder('shuffle');
    } else if (order === 'shuffle') {
      player.setOrder('sequential');
      player.setRepeat('one');
    } else if (repeat === 'one') {
      player.setRepeat('all');
    } else {
      player.setRepeat('off');
    }
  };
  const modeIcon =
    order === 'shuffle' ? <Shuffle size={16} /> : repeat === 'one' ? <Repeat1 size={16} /> : <Repeat size={16} />;
  const modeLabel = order === 'shuffle' ? '随机播放' : repeat === 'one' ? '单曲循环' : repeat === 'all' ? '列表循环' : '顺序播放';
  const modeActive = order === 'shuffle' || repeat !== 'off';

  const volIcon = muted || volume === 0 ? <VolumeX size={16} /> : volume < 0.5 ? <Volume1 size={16} /> : <Volume2 size={16} />;

  return (
    <footer className="player-bar">
      <div className="pb-track">
        {current ? (
          <>
            <TrackCover track={current} />
            <div className="pb-meta" onClick={() => player.setLyricsOpen(true)}>
              <div className="pb-title" title={current.title}>
                {current.title}
                {quality ? <span className="pb-quality-badge">{qualityLabel(quality)}</span> : null}
              </div>
              <div className="pb-artist">
                {errorMsg ? <span style={{ color: '#ff7b72' }}>{errorMsg}</span> : loading ? '正在解析播放源…' : (current.artists || []).map((a) => a.name).join(' / ')}
              </div>
            </div>
          </>
        ) : (
          <>
            <div className="pb-cover-placeholder">
              <Disc3 size={22} />
            </div>
            <div className="pb-meta">
              <div className="pb-title" style={{ color: 'var(--text-secondary)' }}>
                尚未播放
              </div>
              <div className="pb-artist">选择一首歌开始聆听</div>
            </div>
          </>
        )}
      </div>

      <div className="pb-center">
        <div className="pb-controls">
          <button
            className="icon-btn"
            onClick={cycleMode}
            title={modeLabel}
            style={modeActive ? { color: 'var(--accent)' } : undefined}
          >
            {modeIcon}
          </button>
          <button className="icon-btn" onClick={() => player.previous()} title="上一首">
            <SkipBack size={17} fill="currentColor" />
          </button>
          <button className="play-btn" onClick={() => player.toggle()} title={playing ? '暂停' : '播放'}>
            {playing ? <Pause size={18} fill="currentColor" /> : <Play size={18} fill="currentColor" />}
          </button>
          <button className="icon-btn" onClick={() => player.next()} title="下一首">
            <SkipForward size={17} fill="currentColor" />
          </button>
          <button
            className="icon-btn"
            onClick={() => player.setLyricsOpen(true)}
            title="歌词"
            style={{ color: player.lyricsOpen ? 'var(--accent)' : undefined }}
          >
            <Mic2 size={16} />
          </button>
        </div>
        <div className="pb-progress-row">
          <span className="pb-time">{fmtDuration(positionMs)}</span>
          <input
            ref={progressRef}
            type="range"
            className="progress-slider"
            min={0}
            max={Math.max(1, durationMs / 1000)}
            step={0.5}
            defaultValue={0}
            onMouseDown={beginScrub}
            onMouseMove={previewScrub}
            onMouseUp={commitScrub}
          />
          <span className="pb-time">{fmtDuration(durationMs)}</span>
        </div>
      </div>

      <div className="pb-tools">
        <button
          className="icon-btn"
          onClick={() => player.setQueueOpen(!queueOpen)}
          title={`播放队列（${queue.length}）`}
          style={queueOpen ? { color: 'var(--accent)' } : undefined}
        >
          <ListMusic size={16} />
        </button>
        <div className="volume-wrap">
          <button className="icon-btn" onClick={() => player.toggleMute()} title="静音">
            {volIcon}
          </button>
          <input
            ref={volumeRef}
            type="range"
            className="volume-slider"
            min={0}
            max={100}
            defaultValue={volume * 100}
            onMouseUp={(e) => player.setVolume(Number(e.currentTarget.value) / 100)}
            onTouchEnd={(e) => player.setVolume(Number(e.currentTarget.value) / 100)}
          />
        </div>
      </div>
    </footer>
  );
}

function qualityLabel(q: string): string {
  switch (q) {
    case 'lossless':
      return 'SQ';
    case 'high':
      return 'HQ';
    case 'standard':
      return 'MP3';
    default:
      return q.toUpperCase();
  }
}
