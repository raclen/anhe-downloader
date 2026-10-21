import CryptoJS from 'crypto-js';
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

/** 把各种输入统一转成字节数组（模拟 Node Buffer.from 的常用编码） */
function toBytes(data: any, encoding?: string): Uint8Array {
  if (typeof data === 'string') {
    const enc = (encoding || 'utf8').toLowerCase();
    if (enc === 'base64') {
      const binary = atob(data);
      return Uint8Array.from(binary, (c) => c.charCodeAt(0));
    }
    if (enc === 'hex') {
      const bytes: number[] = [];
      for (let i = 0; i + 1 < data.length; i += 2) bytes.push(parseInt(data.slice(i, i + 2), 16));
      return Uint8Array.from(bytes);
    }
    return new TextEncoder().encode(data);
  }
  if (data instanceof Uint8Array) return data;
  if (data instanceof ArrayBuffer) return new Uint8Array(data);
  if (Array.isArray(data)) return Uint8Array.from(data);
  if (data && typeof data === 'object' && data.buffer instanceof ArrayBuffer) {
    return new Uint8Array(data.buffer, data.byteOffset || 0, data.byteLength ?? data.length ?? 0);
  }
  return new TextEncoder().encode(String(data));
}

/** 字节数组按指定编码转字符串（对应 Buffer#toString） */
function bytesToString(bytes: Uint8Array, format?: string): string {
  const fmt = (format || 'utf8').toLowerCase();
  if (fmt === 'hex') return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  if (fmt === 'base64') {
    let binary = '';
    for (const b of bytes) binary += String.fromCharCode(b);
    return btoa(binary);
  }
  return new TextDecoder().decode(bytes);
}

/** Uint8Array ↔ crypto-js WordArray */
function bytesToWordArray(bytes: Uint8Array): any {
  const words: number[] = [];
  for (let i = 0; i < bytes.length; i++) words[i >>> 2] |= bytes[i] << (24 - (i % 4) * 8);
  return CryptoJS.lib.WordArray.create(words as any, bytes.length);
}

function wordArrayToBytes(wordArray: any): Uint8Array {
  const words: number[] = wordArray.words;
  const { sigBytes } = wordArray;
  const bytes = new Uint8Array(sigBytes);
  for (let i = 0; i < sigBytes; i++) {
    bytes[i] = (words[i >>> 2] >>> (24 - (i % 4) * 8)) & 0xff;
  }
  return bytes;
}

/** 洛雪 lx.utils 的浏览器兼容实现（crypto-js 提供同步的 MD5/AES） */
const lxUtils = {
  buffer: {
    from: (data: any, encoding?: string) => toBytes(data, encoding),
    bufToString: (buffer: any, format?: string) => bytesToString(toBytes(buffer), format),
  },
  crypto: {
    md5: (value: any) => CryptoJS.MD5(bytesToWordArray(toBytes(value))).toString(CryptoJS.enc.Hex),
    randomBytes: (size: number) => {
      const bytes = new Uint8Array(size);
      globalThis.crypto.getRandomValues(bytes);
      return bytes;
    },
    aesEncrypt: (buffer: any, mode = 'aes-128-cbc', key: any, iv?: any) => {
      const isEcb = /ecb/i.test(mode);
      const config: any = {
        mode: isEcb ? CryptoJS.mode.ECB : CryptoJS.mode.CBC,
        // 与洛雪保持一致：不做自动填充，由音源自行补齐块大小
        padding: CryptoJS.pad.NoPadding,
      };
      if (!isEcb) config.iv = bytesToWordArray(toBytes(iv ?? ''));
      const encrypted = CryptoJS.AES.encrypt(
        bytesToWordArray(toBytes(buffer)),
        bytesToWordArray(toBytes(key)),
        config,
      );
      return wordArrayToBytes(encrypted.ciphertext);
    },
    rsaEncrypt: () => {
      throw new Error('当前运行环境不支持 utils.crypto.rsaEncrypt');
    },
  },
};

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
      const homepageMatch = code.match(/@homepage\s+([^\r\n]+)/);

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
        // 洛雪运行时版本与环境标识：部分音源据此判断是否启用 utils、以及拼接 User-Agent
        version: '2.0.0',
        env: 'desktop',
        // 脚本元信息：flower/grass 等音源初始化时会直接读取 currentScriptInfo.version
        currentScriptInfo: {
          name: this.meta.name,
          description: this.meta.description,
          version: this.meta.version,
          author: this.meta.author,
          homepage: homepageMatch ? homepageMatch[1].trim() : undefined,
          rawScript: code
        },
        utils: lxUtils,
        request: (url: string, options: any, callback: (err: any, resp: any, body: any) => void) => {
          const headers = { ...(options?.headers || {}) };
          const init: RequestInit = {
            method: (options?.method || 'GET').toUpperCase(),
            headers,
            signal: options?.timeout > 0 ? AbortSignal.timeout(options.timeout) : undefined
          };

          if (options?.form) {
            init.body = new URLSearchParams(options.form).toString();
            if (!headers['content-type']) {
              headers['content-type'] = 'application/x-www-form-urlencoded';
            }
          } else if (options?.formData) {
            const fd = new FormData();
            for (const [k, v] of Object.entries(options.formData)) fd.append(k, v as any);
            init.body = fd;
          } else if (options?.body != null) {
            init.body =
              typeof options.body === 'string' ? options.body : JSON.stringify(options.body);
            if (typeof options.body === 'object' && !(options.body instanceof ArrayBuffer) && !headers['content-type']) {
              headers['content-type'] = 'application/json';
            }
          }

          fetch(url, init)
            .then(async (res) => {
              const buf = new Uint8Array(await res.arrayBuffer());
              let body: any = options?.binary ? buf : new TextDecoder().decode(buf);
              if (!options?.binary) {
                try {
                  body = JSON.parse(body);
                } catch {
                  /* 非 JSON 响应保持字符串 */
                }
              }
              callback(null, { statusCode: res.status, headers: Object.fromEntries(res.headers), body }, body);
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
        globalThis.lx = arguments[1];
        ${code}
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
