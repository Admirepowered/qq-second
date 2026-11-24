// 录音 / 音频文件 → 16kHz 单声道 PCM（供听歌识曲使用）。
// 采样率不做前端重采样：把真实采样率一并交给 Go 端统一处理。

export interface PcmCapture {
  pcm: Int16Array;
  sampleRate: number;
}

function floatToInt16(input: Float32Array): Int16Array {
  const out = new Int16Array(input.length);
  for (let i = 0; i < input.length; i++) {
    const s = Math.max(-1, Math.min(1, input[i]));
    out[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
  }
  return out;
}

/** 麦克风录音，录满 seconds 秒（或手动 stop）后返回 PCM。 */
export async function recordPcm(
  seconds: number,
  onProgress?: (ratio: number) => void,
  abort?: AbortSignal,
): Promise<PcmCapture> {
  if (!navigator.mediaDevices?.getUserMedia) {
    throw new Error('当前环境不支持录音（缺少麦克风接口）');
  }
  let stream: MediaStream;
  try {
    stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
      },
    });
  } catch (e) {
    const err = e as { name?: string; message?: string };
    throw new Error(`麦克风打开失败：${err.name || 'Error'} ${err.message || ''}`.trim());
  }
  const ctx = new AudioContext({ sampleRate: 16000 });
  if (ctx.state === 'suspended') await ctx.resume();

  const source = ctx.createMediaStreamSource(stream);
  const processor = ctx.createScriptProcessor(4096, 1, 1);
  const sink = ctx.createGain();
  sink.gain.value = 0; // 静音，避免把麦克风回放出来

  const chunks: Float32Array[] = [];
  let collected = 0;
  let stopped = false;
  let fullTimer: number | undefined;

  const cleanup = async () => {
    stopped = true;
    if (fullTimer) window.clearInterval(fullTimer);
    processor.onaudioprocess = null;
    try {
      processor.disconnect();
      source.disconnect();
      sink.disconnect();
    } catch {
      /* ignore */
    }
    stream.getTracks().forEach((t) => t.stop());
    if (ctx.state !== 'closed') await ctx.close();
  };

  return new Promise<PcmCapture>((resolve, reject) => {
    processor.onaudioprocess = (e) => {
      if (stopped) return;
      const data = e.inputBuffer.getChannelData(0);
      chunks.push(new Float32Array(data));
      collected += data.length;
      onProgress?.(Math.min(1, collected / (seconds * ctx.sampleRate)));
    };

    source.connect(processor);
    processor.connect(sink);
    sink.connect(ctx.destination);

    const finish = () => {
      if (stopped) return;
      void cleanup().then(() => {
        const total = chunks.reduce((n, c) => n + c.length, 0);
        const merged = new Float32Array(total);
        let offset = 0;
        for (const c of chunks) {
          merged.set(c, offset);
          offset += c.length;
        }
        resolve({ pcm: floatToInt16(merged), sampleRate: ctx.sampleRate });
      });
    };

    const timer = window.setTimeout(finish, seconds * 1000 + 250);
    const onAbort = () => {
      window.clearTimeout(timer);
      void cleanup().then(() => reject(new Error('已取消')));
    };
    abort?.addEventListener('abort', onAbort, { once: true });
    // 提前录满（进度到 1）就结束，避免多录静音
    fullTimer = window.setInterval(() => {
      if (collected >= seconds * ctx.sampleRate) {
        window.clearTimeout(timer);
        finish();
      }
    }, 120);
  });
}

/** 本地音频文件 → PCM（用于没有麦克风权限时验证/识别）。 */
export async function decodeFilePcm(file: File): Promise<PcmCapture> {
  const buf = await file.arrayBuffer();
  const ctx = new AudioContext();
  try {
    const audio = await ctx.decodeAudioData(buf);
    const data = audio.getChannelData(0);
    return { pcm: floatToInt16(data), sampleRate: audio.sampleRate };
  } finally {
    if (ctx.state !== 'closed') await ctx.close();
  }
}

/** Int16 PCM → base64（分块避免超长参数栈）。 */
export function pcmToBase64(pcm: Int16Array): string {
  const bytes = new Uint8Array(pcm.buffer, pcm.byteOffset, pcm.byteLength);
  const chunk = 0x8000;
  let binary = '';
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}
