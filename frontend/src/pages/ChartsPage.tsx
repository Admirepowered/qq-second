// 榜单页：榜单列表 + 榜单详情。

import { useEffect, useState } from 'react';
import { QQCharts, QQChartDetail } from '../api';
import { ChartInfo, PlaylistDetail } from '../types';
import { usePlayerStore } from '../store/player';
import { useNavStore } from '../store/nav';
import { TrackList } from '../components/TrackRow';
import { Loading, ErrorHint } from './HomePage';

export function ChartsPage() {
  const [charts, setCharts] = useState<ChartInfo[]>([]);
  const navigate = useNavStore((s) => s.navigate);

  useEffect(() => {
    QQCharts()
      .then((c) => setCharts(c as ChartInfo[]))
      .catch(() => {});
  }, []);

  return (
    <div>
      <div className="page-header">
        <div className="page-header-info">
          <div className="page-header-title">排行榜</div>
          <div className="page-header-sub">QQ 音乐官方榜单，每日更新</div>
        </div>
      </div>
      <div className="card-grid">
        {charts.map((c) => (
          <button key={c.id} className="card" onClick={() => navigate({ page: 'chart', id: c.id, name: c.name })}>
            <div
              className="card-cover"
              style={{
                background: `linear-gradient(135deg, var(--accent) 0%, transparent 140%), rgba(255,255,255,0.06)`,
                display: 'grid',
                placeItems: 'center',
                fontSize: 30,
                fontWeight: 800,
                color: '#fff',
              }}
            >
              {c.name.slice(0, 2)}
            </div>
            <div className="card-title">{c.name}</div>
            {c.updateTime ? <div className="card-sub">{c.updateTime}</div> : null}
          </button>
        ))}
      </div>
    </div>
  );
}

export function ChartDetailPage({ id, name }: { id: number; name: string }) {
  const [detail, setDetail] = useState<PlaylistDetail | null>(null);
  const [error, setError] = useState('');
  const playTracks = usePlayerStore((s) => s.playTracks);

  useEffect(() => {
    let cancelled = false;
    QQChartDetail(id)
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
        <div
          className="page-header-cover"
          style={{ background: `linear-gradient(135deg, var(--accent) 0%, transparent 140%), rgba(255,255,255,0.06)`, display: 'grid', placeItems: 'center', fontSize: 40, fontWeight: 800, color: '#fff' }}
        >
          {(detail.info.title || name).slice(0, 2)}
        </div>
        <div className="page-header-info">
          <div className="page-header-title">{detail.info.title || name}</div>
          <div className="page-header-sub">共 {detail.tracks.length} 首{detail.info.updateTime ? ` · 更新于 ${detail.info.updateTime}` : ''}</div>
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
