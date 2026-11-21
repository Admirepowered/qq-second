// 应用背景层：四种模式 default/image/artwork/color（Pulse 模式移植）。

import { CSSProperties, useEffect, useState } from 'react';
import { ReadBackgroundImageDataURL } from '../api';
import { usePrefsStore } from '../store/prefs';
import { usePlayerStore } from '../store/player';

export function AppBackground() {
  const { settings, dark } = usePrefsStore();
  const current = usePlayerStore((s) => s.current);
  const [bgDataUrl, setBgDataUrl] = useState('');

  useEffect(() => {
    const id = settings.backgroundImage;
    if (settings.backgroundMode !== 'image' || !id) {
      setBgDataUrl('');
      return;
    }
    let cancelled = false;
    ReadBackgroundImageDataURL(id)
      .then((v) => {
        if (!cancelled) setBgDataUrl(v as string);
      })
      .catch(() => setBgDataUrl(''));
    return () => {
      cancelled = true;
    };
  }, [settings.backgroundImage, settings.backgroundMode]);

  const mode = settings.backgroundMode;
  let imageSrc = '';
  let color = '';
  if (mode === 'image' && bgDataUrl) imageSrc = bgDataUrl;
  if (mode === 'artwork' && current?.artwork) imageSrc = current.artwork;
  if (mode === 'color') color = settings.backgroundColor;

  const style: CSSProperties = {
    backgroundImage: imageSrc ? `url("${imageSrc}")` : 'none',
    backgroundColor: color || undefined,
    backgroundSize: settings.backgroundFit === 'contain' ? 'contain' : 'cover',
    filter: mode === 'artwork' ? `blur(${Math.min(30, settings.backgroundBlur + 8)}px) saturate(0.9)` : settings.backgroundBlur > 0 ? `blur(${settings.backgroundBlur}px)` : undefined,
    opacity: mode === 'image' || mode === 'artwork' ? settings.artworkInfluence / 100 : 1,
  };

  return (
    <>
      {mode !== 'default' ? (
        <>
          <div className="backgroundLayer" aria-hidden="true" style={style} />
          <div
            className="backgroundOverlay"
            aria-hidden="true"
            style={{ background: dark ? 'rgba(0,0,0,0.25)' : 'rgba(255,255,255,0.25)' }}
          />
        </>
      ) : null}
    </>
  );
}
