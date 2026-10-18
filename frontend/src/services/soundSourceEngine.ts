import { MusicQuality, MusicSource, SongEntity, SoundSourceMeta } from '../types/music';

type RequestHandler = (params: {
  source: MusicSource;
  action: 'musicUrl';
  info: {
    type: MusicQuality;
    musicInfo: {
      songmid: string;
      hash?: string;
      name?: string;
      singer?: string;
    };
  };
}) => Promise<string>;

export interface ResolvedAudio {
  url: string;
  actualQuality: MusicQuality;
  format: 'mp3' | 'flac' | 'm4a';
  resolver: string;
}

export class SoundSourceEngine {
  private static instance: SoundSourceEngine;
  private requestHandler: RequestHandler | null = null;
  public meta: SoundSourceMeta = {
    name: '长青SVIP音源',
    version: '1.3.0',
    author: 'SVIP',
    description: '支持酷狗/QQ/网易/酷我/咪咕各平台直链解析',
    supportedSources: ['kg', 'tx', 'wy', 'kw', 'mg'],
    supportedQualities: ['128k', '320k', 'flac'],
    status: 'loading'
  };
  private scriptRawCode = '';

  private constructor() {}

  public static getInstance(): SoundSourceEngine {
    if (!SoundSourceEngine.instance) {
      SoundSourceEngine.instance = new SoundSourceEngine();
    }
    return SoundSourceEngine.instance;
  }

  public initFromScript(code: string): boolean {
    try {
      this.scriptRawCode = code;
      // 重置上一次注册的处理器，避免旧音源残留导致「假成功」
      this.requestHandler = null;
      this.meta.status = 'loading';
      this.meta.error = undefined;

      const nameMatch = code.match(/@name\s+([^\r\n]+)/);
      const versionMatch = code.match(/@version\s+([^\r\n]+)/);
      const authorMatch = code.match(/@author\s+([^\r\n]+)/);
      const descMatch = code.match(/@description\s+([^\r\n]+)/);

      this.meta.name = nameMatch ? nameMatch[1].trim() : '自定义音源';
      this.meta.version = versionMatch ? versionMatch[1].trim() : '1.0.0';
      this.meta.author = authorMatch ? authorMatch[1].trim() : '未知';
      this.meta.description = descMatch ? descMatch[1].trim() : '';

      // 构造洛雪环境沙箱
      const sandboxLx = {
        EVENT_NAMES: {
          inited: 'inited',
          request: 'request',
          updateAlert: 'updateAlert'
        },
        request: (url: string, options: any, callback: (err: any, resp: any, body: any) => void) => {
          fetch(url, {
            method: options?.method || 'GET',
            headers: options?.headers,
            body: options?.body
          })
            .then(async (res) => {
              const text = await res.text();
              callback(null, { statusCode: res.status, headers: Object.fromEntries(res.headers) }, text);
            })
            .catch((err) => callback(err, null, null));
        },
        on: (event: string, handler: any) => {
          if (event === 'request') {
            this.requestHandler = handler;
          }
        },
        send: (event: string, data: any) => {
          if (event === 'inited') {
            this.meta.status = 'active';
            if (data?.sources) {
              this.meta.supportedSources = Object.keys(data.sources) as MusicSource[];
            }
          }
        }
      };

      // 在当前窗口上下文中执行
      const runner = new Function('globalThis', `
        const oldLx = globalThis.lx;
        globalThis.lx = arguments[1];
        try {
          ${code}
        } finally {
          // preserve handler
        }
      `);

      runner(window, sandboxLx);

      if (!this.requestHandler) {
        this.meta.status = 'error';
        this.meta.error = '脚本未注册 lx.on("request") 处理器，无法解析音源';
        return false;
      }

      this.meta.status = 'active';
      return true;
    } catch (e: any) {
      console.error('音源沙箱初始化失败:', e);
      this.meta.status = 'error';
      this.meta.error = e.message;
      return false;
    }
  }

  /** 当前音源是否已成功注册解析处理器 */
  public isReady(): boolean {
    return this.requestHandler !== null;
  }

  /**
   * 核心解析方法：优先走后端统一解析（无 CORS、支持跨源兜底），其次长青音源，最后浏览器直连
   */
  public async resolveAudioUrl(
    song: SongEntity,
    targetQuality: MusicQuality
  ): Promise<ResolvedAudio> {
    // 0. 后端统一解析：酷我/网易直链 + QQ/酷狗跨源匹配
    const serverResult = await this.resolveFromBackend(song, targetQuality);
    if (serverResult) {
      return serverResult;
    }

    // 1. 尝试使用长青音源解析
    if (this.requestHandler) {
      try {
        const customUrl = await this.requestHandler({
          source: song.source,
          action: 'musicUrl',
          info: {
            type: targetQuality,
            musicInfo: {
              songmid: song.songmid,
              hash: song.hash,
              name: song.name,
              singer: song.singer
            }
          }
        });

        if (customUrl && typeof customUrl === 'string' && customUrl.startsWith('http')) {
          // 快速验证直链有效性 (带超时)
          const valid = await this.probeUrl(customUrl);
          if (valid) {
            return {
              url: customUrl,
              actualQuality: targetQuality,
              format: targetQuality === 'flac' ? 'flac' : 'mp3',
              resolver: `${this.meta.name} (直链)`
            };
          }
        }
      } catch (err) {
        console.warn('长青音源解析未命中，进入备用解析:', err);
      }
    }

    // 2. 内置备用高速直链引擎 (KuWo / NetEase / KuGou)
    const fallbackResult = await this.resolveFallback(song, targetQuality);
    if (fallbackResult) {
      return fallbackResult;
    }

    throw new Error(`歌曲《${song.name}》暂未找到可播放或下载的音源直链`);
  }

  /** 后端统一解析（/api/resolve） */
  private async resolveFromBackend(
    song: SongEntity,
    quality: MusicQuality
  ): Promise<ResolvedAudio | null> {
    try {
      const params = new URLSearchParams({
        source: song.source || '',
        songmid: song.songmid || '',
        title: song.name || '',
        singer: song.singer || '',
        quality
      });
      const resp = await fetch(`/api/resolve?${params.toString()}`);
      if (!resp.ok) return null;
      const data = await resp.json();
      if (!data || !data.url) return null;

      const format = data.format === 'flac' ? 'flac' : data.format === 'm4a' ? 'm4a' : 'mp3';
      return {
        url: data.url,
        actualQuality: (data.quality as MusicQuality) || quality,
        format,
        resolver: data.resolver || '后端统一解析'
      };
    } catch (err) {
      console.warn('后端解析接口不可用，使用本地兜底:', err);
      return null;
    }
  }

  private async probeUrl(url: string): Promise<boolean> {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 2500);
      const res = await fetch(url, {
        method: 'GET',
        headers: { Range: 'bytes=0-1' },
        signal: controller.signal
      });
      clearTimeout(timeoutId);
      return res.status === 200 || res.status === 206 || res.status === 302;
    } catch {
      return false;
    }
  }

  private async resolveFallback(
    song: SongEntity,
    quality: MusicQuality
  ): Promise<ResolvedAudio | null> {
    // 酷我平台备用源
    if (song.source === 'kw' || song.songmid) {
      try {
        const kwRid = song.songmid.replace(/^MUSIC_/, '');
        const url = `http://antiserver.kuwo.cn/anti.s?type=convert_url&rid=${kwRid}&format=mp3&response=url`;
        const res = await fetch(url);
        const playUrl = await res.text();
        if (playUrl && playUrl.startsWith('http')) {
          return {
            url: playUrl,
            actualQuality: quality === 'flac' ? '320k' : quality,
            format: 'mp3',
            resolver: '内置高速直链源 (酷我)'
          };
        }
      } catch (e) {
        console.warn('酷我备用源解析失败:', e);
      }
    }

    // 网易云备用源
    if (song.source === 'wy' || /^\d+$/.test(song.songmid)) {
      try {
        const wyId = song.songmid;
        const testUrl = `https://music.163.com/song/media/outer/url?id=${wyId}.mp3`;
        return {
          url: testUrl,
          actualQuality: '128k',
          format: 'mp3',
          resolver: '内置直链源 (网易云)'
        };
      } catch (e) {
        console.warn('网易云备用源解析失败:', e);
      }
    }

    return null;
  }
}
