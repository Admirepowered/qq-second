// 全屏歌词页：逐字渐变填充 + 翻译/罗马音 + 点击行 seek。

import { CSSProperties, useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, Pause, Play, SkipBack, SkipForward } from 'lucide-react';
import { usePlayerStore } from '../store/player';
import { QQLyrics } from '../api';
import { LyricDocument, LyricLine, fmtDuration } from '../types';

export function LyricsPanel() {
  const { lyricsOpen, current, positionMs, durationMs, seek, setLyricsOpen, playbackState } =
    usePlayerStore();
  const toggle = usePlayerStore((s) => s.toggle);
  const next = usePlayerStore((s) => s.next);
  const previous = usePlayerStore((s) => s.previous);
  const [doc, setDoc] = useState<LyricDocument | null>(null);
  const [showTrans, setShowTrans] = useState(true);
  const [showRoma, setShowRoma] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const lineRefs = useRef<(HTMLDivElement | null)[]>([]);
  const activeLineRef = useRef(-1);

  // 加载歌词
  useEffect(() => {
    if (!lyricsOpen || !current || current.kind !== 'qq' || !current.mid) {
      setDoc(null);
      return;
    }
    let cancelled = false;
    setDoc(null);
    QQLyrics(current.mid)
      .then((d) => {
        if (!cancelled) setDoc(d as LyricDocument);
      })
      .catch(() => setDoc(null));
    return () => {
      cancelled = true;
    };
  }, [lyricsOpen, current?.id]);

  // 当前活动行
  const lines = doc?.lines ?? [];
  const activeIndex = useMemo(() => {
    // 只对有同步时间轴的歌词计算活动行；首行 startMs=0 是合法值，不能用真值守卫
    if (!doc || doc.syncMode === 'unsynchronized' || lines.length === 0) return -1;
    let idx = -1;
    for (let i = 0; i < lines.length; i++) {
      if (lines[i].startMs <= positionMs) idx = i;
      else break;
    }
    return idx;
  }, [lines, positionMs]);

  // 滚动到当前行
  useEffect(() => {
    if (activeIndex < 0 || activeIndex === activeLineRef.current) return;
    activeLineRef.current = activeIndex;
    const el = lineRefs.current[activeIndex];
    const scroller = scrollRef.current;
    if (el && scroller) {
      const target = el.offsetTop - scroller.clientHeight / 2 + el.clientHeight / 2;
      scroller.scrollTo({ top: Math.max(0, target), behavior: 'smooth' });
    }
  }, [activeIndex]);

  if (!lyricsOpen) return null;

  const artwork = current?.kind === 'qq' ? current.artwork : undefined;

  return (
    <div className="lyrics-stage">
      {artwork ? <div className="lyrics-bg" style={{ backgroundImage: `url("${artwork}")` } as CSSProperties} /> : null}

      <div className="lyrics-topbar">
        <button className="icon-btn" onClick={() => setLyricsOpen(false)} title="收起歌词">
          <ChevronDown size={20} />
        </button>
        <span className="topbar-title">{current?.title ?? ''}</span>
        <div className="lyrics-topbar-spacer" />
        {doc && doc.syncMode !== 'unsynchronized' ? (
          <div className="lyrics-toggle-group">
            <button
              className={`lyrics-chip${showTrans ? ' active' : ''}`}
              onClick={() => setShowTrans(!showTrans)}
            >
              译文
            </button>
            <button
              className={`lyrics-chip${showRoma ? ' active' : ''}`}
              onClick={() => setShowRoma(!showRoma)}
            >
              罗马音
            </button>
          </div>
        ) : null}
      </div>

      <div className="lyrics-body">
        <div className="lyrics-left">
          {artwork ? (
            <img className="lyrics-cover" src={artwork} alt="" />
          ) : (
            <div className="lyrics-cover-ph">♪</div>
          )}
          <div>
            <div className="lyrics-now-title">{current?.title ?? '尚未播放'}</div>
            <div className="lyrics-now-artist">
              {(current?.artists || []).map((a) => a.name).join(' / ')}
            </div>
          </div>
        </div>

        <div className="lyrics-scroll" ref={scrollRef}>
          {!current ? (
            <div className="lyrics-line-empty">选择一首歌开始聆听</div>
          ) : lines.length === 0 ? (
            <div className="lyrics-line-empty">
              {doc ? '暂无歌词' : current.kind === 'local' ? '本地曲目暂不支持歌词' : '歌词加载中…'}
            </div>
          ) : (
            lines.map((line, i) => (
              <div
                key={i}
                ref={(el) => {
                  lineRefs.current[i] = el;
                }}
                className={`lyrics-line${i === activeIndex ? ' current' : ''}`}
                onClick={() => seek(Math.max(0, line.startMs))}
              >
                {i === activeIndex && doc?.syncMode === 'word' && wordFillActive(line, positionMs) ? (
                  <WordFillLine text={line.text} words={line.words!} positionMs={positionMs} />
                ) : (
                  <span>{line.text}</span>
                )}
                {showTrans && line.translation ? <div className="lyrics-sub">{line.translation}</div> : null}
                {showRoma && line.romanization ? <div className="lyrics-sub">{line.romanization}</div> : null}
              </div>
            ))
          )}
        </div>
      </div>

      <div className="lyrics-bottombar">
        <button className="icon-btn" onClick={previous} title="上一首">
          <SkipBack size={17} fill="currentColor" />
        </button>
        <span className="lyrics-time">{fmtDuration(positionMs)}</span>
        <input
          type="range"
          className="lyrics-progress"
          min={0}
          max={Math.max(1, durationMs / 1000)}
          step={0.5}
          value={positionMs / 1000}
          style={{ '--range-progress': `${durationMs > 0 ? (positionMs / durationMs) * 100 : 0}%` } as CSSProperties}
          onChange={(e) => seek(Math.floor(Number(e.target.value) * 1000))}
        />
        <span className="lyrics-time">{fmtDuration(durationMs)}</span>
        <button className="icon-btn" onClick={toggle} title="播放/暂停">
          {playbackState === 'playing' ? <Pause size={17} fill="currentColor" /> : <Play size={17} fill="currentColor" />}
        </button>
        <button className="icon-btn" onClick={next} title="下一首">
          <SkipForward size={17} fill="currentColor" />
        </button>
      </div>
    </div>
  );
}

// 逐字填充行：每个字按时间推进白色渐变比例
function WordFillLine({ text, words, positionMs }: { text: string; words: { startMs: number; endMs: number; text: string }[]; positionMs: number }) {
  return (
    <span>
      {words.map((w, i) => {
        let fill = 0;
        const dur = w.endMs - w.startMs;
        if (positionMs >= w.endMs) fill = 100;
        else if (positionMs > w.startMs) fill = dur > 0 ? ((positionMs - w.startMs) / dur) * 100 : 100;
        return (
          <span key={i} className="word-fill" style={{ '--fill': `${fill}%` } as CSSProperties}>
            {w.text}
          </span>
        );
      })}
    </span>
  );
}

// 本行最后一个词的结束时间：超过它以后整句直接显示为已高亮纯文本，
// 避免「逐字数据提前结束 → 整句看起来一直没亮」。
function lastWordEnd(line: LyricLine): number {
  const words = line.words;
  if (!words || words.length === 0) return line.endMs;
  return words[words.length - 1].endMs;
}

// 逐字渐变只在「句子正在播放」的区间启用；句首前/句末后整句保持高亮，
// 保证一句话从头到尾都是亮的（历史问题：只在开头约 1s 内可见高亮）。
function wordFillActive(line: LyricLine, positionMs: number): boolean {
  const words = line.words;
  if (!words || words.length === 0) return false;
  if (positionMs < words[0].startMs) return false;
  return positionMs < lastWordEnd(line);
}
