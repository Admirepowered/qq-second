// 听歌识曲页：麦克风录 8 秒（或选本地音频文件）→ 后端 Shazam 指纹识别 → QQ 曲库候选。

import { useEffect, useRef, useState } from 'react';
import { AudioLines, Loader2, Mic, Square, Upload } from 'lucide-react';
import { IdentifyAudio, SendLog } from '../api';
import { Track } from '../types';
import { usePlayerStore } from '../store/player';
import { toast } from '../store/toast';
import { PcmCapture, decodeFilePcm, pcmToBase64, recordPcm } from '../recorder';

interface IdentifyResult {
  matched: boolean;
  title?: string;
  artist?: string;
  album?: string;
  cover?: string;
  message?: string;
  songs: Track[];
}

type Phase = 'idle' | 'recording' | 'identifying' | 'done';

const RECORD_SECONDS = 8;

export function IdentifyPage() {
  const playTracks = usePlayerStore((s) => s.playTracks);
  const [phase, setPhase] = useState<Phase>('idle');
  const [progress, setProgress] = useState(0);
  const [result, setResult] = useState<IdentifyResult | null>(null);
  const [error, setError] = useState('');
  const abortRef = useRef<AbortController | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(
    () => () => {
      abortRef.current?.abort();
    },
    [],
  );

  const identify = async (capture: PcmCapture) => {
    if (capture.pcm.length < 8000) {
      throw new Error('录到的声音太短，请再试一次');
    }
    setPhase('identifying');
    setResult(null);
    setError('');
    const res = (await IdentifyAudio(pcmToBase64(capture.pcm), capture.sampleRate)) as unknown as IdentifyResult;
    setResult(res);
    setPhase('done');
    if (!res.matched) toast.error(res.message || '没有识别到这首歌');
  };

  const startRecording = async () => {
    setResult(null);
    setError('');
    setProgress(0);
    setPhase('recording');
    const abort = new AbortController();
    abortRef.current = abort;
    try {
      void SendLog('info', 'identify: 开始录音');
      const capture = await recordPcm(RECORD_SECONDS, setProgress, abort.signal);
      void SendLog('info', `identify: 录音完成 samples=${capture.pcm.length} rate=${capture.sampleRate}`);
      await identify(capture);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      void SendLog('error', 'identify: 录音/识别失败 ' + msg);
      if (msg !== '已取消') {
        setError(
          msg.includes('Permission') || msg.includes('NotAllowed') || msg.includes('denied')
            ? '麦克风不可用（权限被拒绝）。可以改用下方「选择音频文件」识别。'
            : msg,
        );
        toast.error('录音失败');
      }
      setPhase('idle');
    } finally {
      abortRef.current = null;
    }
  };

  const stopRecording = () => {
    abortRef.current?.abort();
    setPhase('idle');
    setProgress(0);
  };

  const onPickFile = async (file: File | undefined) => {
    if (!file) return;
    setResult(null);
    setError('');
    setPhase('identifying');
    try {
      const capture = await decodeFilePcm(file);
      await identify(capture);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setPhase('idle');
    }
  };

  const recording = phase === 'recording';
  const busy = phase === 'identifying';
  const pct = Math.round(progress * 100);

  return (
    <div>
      <div className="page-header">
        <div className="page-header-info">
          <div className="page-header-title">听歌识曲</div>
          <div className="page-header-sub">
            录 {RECORD_SECONDS} 秒正在播放的音乐，识别出歌名后直接在 QQ 音乐里播放
          </div>
        </div>
      </div>

      <div className="identify-stage">
        <div className="identify-card">
          <button
            className={`identify-mic${recording ? ' recording' : ''}${busy ? ' busy' : ''}`}
            onClick={recording ? stopRecording : startRecording}
            disabled={busy}
            title={recording ? '停止' : '开始识别'}
          >
            {busy ? (
              <Loader2 size={34} className="spin" />
            ) : recording ? (
              <Square size={30} fill="currentColor" />
            ) : (
              <Mic size={34} />
            )}
          </button>

          <div className="identify-hint">
            {busy
              ? '正在识别…'
              : recording
                ? `正在聆听… ${pct}%（点一下停止）`
                : '点一下开始录音'}
          </div>

          <div className={`identify-progress${recording ? ' active' : ''}`}>
            <div className="identify-progress-bar" style={{ width: `${recording ? pct : 0}%` }} />
          </div>

          <div className="identify-actions">
            <button className="btn" disabled={busy || recording} onClick={() => fileRef.current?.click()}>
              <Upload size={14} /> 选择音频文件识别
            </button>
            <input
              ref={fileRef}
              type="file"
              accept="audio/*"
              style={{ display: 'none' }}
              onChange={(e) => {
                const f = e.target.files?.[0];
                e.target.value = '';
                void onPickFile(f);
              }}
            />
          </div>

          {error ? <div className="identify-error">{error}</div> : null}
        </div>

        {result ? (
          result.matched ? (
            <div className="identify-result">
              <div className="identify-result-head">
                {result.cover ? (
                  <img className="identify-cover" src={result.cover} alt="" />
                ) : (
                  <div className="identify-cover identify-cover-ph">
                    <AudioLines size={28} />
                  </div>
                )}
                <div style={{ minWidth: 0 }}>
                  <div className="identify-title">{result.title}</div>
                  <div className="identify-artist">{result.artist}</div>
                  {result.album ? <div className="identify-album">{result.album}</div> : null}
                </div>
              </div>

              {result.songs.length > 0 ? (
                <>
                  <div className="identify-subhead">QQ 音乐中找到的版本</div>
                  <div className="track-list">
                    {result.songs.map((s, i) => (
                      <div
                        key={`${s.id}-${i}`}
                        className="track-row"
                        onClick={() => playTracks(result.songs, i)}
                        title="播放"
                      >
                        <span className="track-index">{i + 1}</span>
                        <div className="track-cover-ph">
                          <AudioLines size={14} />
                        </div>
                        <div style={{ minWidth: 0 }}>
                          <div className="track-title">{s.title}</div>
                          <div className="track-artist">
                            {(s.artists || []).map((a) => a.name).join(' / ')}
                          </div>
                        </div>
                        <span className="track-album">{s.album || ''}</span>
                        <span className="track-duration" />
                        <div className="track-actions" />
                      </div>
                    ))}
                  </div>
                </>
              ) : (
                <div className="identify-empty">{result.message || 'QQ 音乐曲库没有找到这首歌'}</div>
              )}
            </div>
          ) : (
            <div className="identify-empty">{result.message || '没有识别到这首歌'}</div>
          )
        ) : null}
      </div>
    </div>
  );
}
