import { MusicSource, SongEntity } from '../types/music';

// 格式化秒数为 mm:ss
export function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
}

export class MusicApi {
  /**
   * 聚合搜索歌曲（支持单源或全网并发搜索）
   */
  public static async search(
    keyword: string,
    source: 'all' | MusicSource = 'all',
    page = 1,
    pageSize = 30
  ): Promise<{ list: SongEntity[]; total: number }> {
    if (!keyword.trim()) return { list: [], total: 0 };

    try {
      // 优先通过本地后端代理搜索，无 CORS 限制、并发且包含完整平台结果
      const res = await fetch(
        `/api/search?keyword=${encodeURIComponent(keyword)}&source=${source}&page=${page}&pagesize=${pageSize}`
      );
      if (res.ok) {
        const data = await res.json();
        if (data && Array.isArray(data.list) && data.list.length > 0) {
          return data;
        }
      }
    } catch (e) {
      console.warn('后端搜索接口调用失败，尝试备用链路:', e);
    }

    if (source === 'all') {
      const [kwRes, kgRes, wyRes] = await Promise.allSettled([
        this.searchKuWo(keyword, page, Math.ceil(pageSize / 2)),
        this.searchKuGou(keyword, page, Math.ceil(pageSize / 4)),
        this.searchNetEase(keyword, page, Math.ceil(pageSize / 4))
      ]);

      const list: SongEntity[] = [];
      const seen = new Set<string>();

      const addUnique = (items: SongEntity[]) => {
        for (const item of items) {
          const key = `${item.name.toLowerCase()}_${item.singer.toLowerCase()}`;
          if (!seen.has(key)) {
            seen.add(key);
            list.push(item);
          }
        }
      };

      if (kwRes.status === 'fulfilled') addUnique(kwRes.value.list);
      if (kgRes.status === 'fulfilled') addUnique(kgRes.value.list);
      if (wyRes.status === 'fulfilled') addUnique(wyRes.value.list);

      return {
        list,
        total: list.length * 3
      };
    }

    switch (source) {
      case 'kw':
        return this.searchKuWo(keyword, page, pageSize);
      case 'kg':
        return this.searchKuGou(keyword, page, pageSize);
      case 'wy':
        return this.searchNetEase(keyword, page, pageSize);
      case 'tx':
        return this.searchTencent(keyword, page, pageSize);
      default:
        return { list: [], total: 0 };
    }
  }

  /**
   * QQ 音乐搜索（后端不可用时的浏览器直连兜底）
   */
  private static async searchTencent(
    keyword: string,
    page: number,
    pageSize: number
  ): Promise<{ list: SongEntity[]; total: number }> {
    const url = `https://c.y.qq.com/soso/fcgi-bin/client_search_cp?p=${page}&n=${pageSize}&w=${encodeURIComponent(
      keyword
    )}&format=json&cr=1&t=0&aggr=1&lossless=0&catZhida=1&new_json=1`;

    const res = await fetch(url, { headers: { Referer: 'https://y.qq.com/' } as any });
    const data = await res.json();
    const items = data.data?.song?.list || [];

    const list: SongEntity[] = items.map((item: any) => {
      const types: ('128k' | '320k' | 'flac')[] = ['128k'];
      if (item.file?.size_320mp3 && item.file.size_320mp3 !== '0') types.push('320k');
      if (item.file?.size_flac && item.file.size_flac !== '0') types.push('flac');

      return {
        id: `tx_${item.mid}`,
        source: 'tx',
        name: item.name || item.title || '未知曲目',
        singer: item.singer?.map((s: any) => s.name).join(' / ') || '群星',
        album: item.album?.name || '',
        duration: item.interval || 0,
        cover: item.album?.mid
          ? `https://y.qq.com/music/photo_new/T002R500x500M000${item.album.mid}.jpg`
          : '',
        songmid: item.mid,
        types
      };
    });

    return {
      list,
      total: data.data?.song?.totalnum || list.length
    };
  }

  /**
   * 酷我音乐搜索
   */
  private static async searchKuWo(
    keyword: string,
    page: number,
    pageSize: number
  ): Promise<{ list: SongEntity[]; total: number }> {
    const pn = page - 1;
    const url = `http://search.kuwo.cn/r.s?client=kt&all=${encodeURIComponent(
      keyword
    )}&pn=${pn}&rn=${pageSize}&vipver=1&ft=music&encoding=utf8&rformat=json&mobi=1`;

    const res = await fetch(url);
    const data = await res.json();
    const abslist = data.abslist || [];

    const list: SongEntity[] = abslist.map((item: any) => {
      const rid = (item.DC_TARGETID || '').replace(/^MUSIC_/, '');
      const hasFlac = item.MINFO && item.MINFO.includes('format:flac');
      const has320 = item.MINFO && item.MINFO.includes('bitrate:320');
      const coverUrl = item.web_albumpic_short
        ? `https://img4.kuwo.cn/star/albumcover/${item.web_albumpic_short}`
        : item.hts_MVPIC || '';

      const types: ('128k' | '320k' | 'flac')[] = ['128k'];
      if (has320) types.push('320k');
      if (hasFlac) types.push('flac');

      return {
        id: `kw_${rid}`,
        source: 'kw',
        name: item.SONGNAME || '未知曲目',
        singer: item.ARTIST || '群星',
        album: item.ALBUM || '未知专辑',
        duration: parseInt(item.DURATION, 10) || 0,
        cover: coverUrl,
        songmid: rid,
        types
      };
    });

    return {
      list,
      total: parseInt(data.TOTAL, 10) || list.length
    };
  }

  /**
   * 酷狗音乐搜索
   */
  private static async searchKuGou(
    keyword: string,
    page: number,
    pageSize: number
  ): Promise<{ list: SongEntity[]; total: number }> {
    const url = `http://mobilecdn.kugou.com/api/v3/search/song?format=json&keyword=${encodeURIComponent(
      keyword
    )}&page=${page}&pagesize=${pageSize}&showtype=1`;

    const res = await fetch(url);
    const data = await res.json();
    const items = data.data?.info || [];

    const list: SongEntity[] = items.map((item: any) => {
      const duration = parseInt(item.duration, 10) || 0;
      const types: ('128k' | '320k' | 'flac')[] = ['128k', '320k'];
      if (item.sqhash || item['320hash']) types.push('flac');

      return {
        id: `kg_${item.hash}`,
        source: 'kg',
        name: item.songname || '未知曲目',
        singer: item.singername || '群星',
        album: item.album_name || '',
        duration,
        cover: '',
        songmid: item.hash,
        hash: item.hash,
        types
      };
    });

    return {
      list,
      total: data.data?.total || list.length
    };
  }

  /**
   * 网易云音乐搜索
   */
  private static async searchNetEase(
    keyword: string,
    page: number,
    pageSize: number
  ): Promise<{ list: SongEntity[]; total: number }> {
    const offset = (page - 1) * pageSize;
    const url = 'https://music.163.com/api/search/get/web?csrf_token=';

    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: `s=${encodeURIComponent(keyword)}&type=1&offset=${offset}&total=true&limit=${pageSize}`
    });

    const data = await res.json();
    const songs = data.result?.songs || [];

    const list: SongEntity[] = songs.map((item: any) => {
      const singer = item.artists?.map((a: any) => a.name).join(', ') || '群星';
      const duration = Math.floor((item.duration || 0) / 1000);

      return {
        id: `wy_${item.id}`,
        source: 'wy',
        name: item.name || '未知曲目',
        singer,
        album: item.album?.name || '',
        duration,
        cover: item.album?.picUrl || '',
        songmid: String(item.id),
        types: ['128k', '320k', 'flac']
      };
    });

    return {
      list,
      total: data.result?.songCount || list.length
    };
  }

  /**
   * 解析网易云歌单
   */
  public static async parseNetEasePlaylist(playlistId: string): Promise<SongEntity[]> {
    const cleanId = playlistId.match(/(\d+)/)?.[1] || playlistId;
    const url = `https://music.163.com/api/playlist/detail?id=${cleanId}`;

    const res = await fetch(url);
    const data = await res.json();
    const tracks = data.result?.tracks || [];

    return tracks.map((item: any) => {
      const singer = item.artists?.map((a: any) => a.name).join(', ') || '群星';
      const duration = Math.floor((item.duration || 0) / 1000);
      return {
        id: `wy_${item.id}`,
        source: 'wy',
        name: item.name,
        singer,
        album: item.album?.name || '',
        duration,
        cover: item.album?.picUrl || '',
        songmid: String(item.id),
        types: ['128k', '320k', 'flac']
      };
    });
  }

  /**
   * 获取歌曲的同步歌词 (LRC 格式)
   */
  public static async fetchLyric(song: SongEntity): Promise<string> {
    try {
      // 1. 优先调用后端代理，彻底规避浏览器 CORS 跨域问题
      const params = new URLSearchParams({
        source: song.source || '',
        songmid: song.songmid || '',
        title: song.name || '',
        singer: song.singer || ''
      });
      const resp = await fetch(`/api/lyric?${params.toString()}`);
      if (resp.ok) {
        const data = await resp.json();
        if (data.lyric && data.lyric.trim()) {
          return data.lyric;
        }
      }
    } catch {
      // 降级尝试
    }

    try {
      if (song.source === 'wy' || /^\d+$/.test(song.songmid)) {
        const url = `https://music.163.com/api/song/lyric?id=${song.songmid}&lv=1&kv=1&tv=-1`;
        const res = await fetch(url);
        const data = await res.json();
        if (data.lrc?.lyric) {
          return data.lrc.lyric;
        }
      }

      // 酷我歌词
      if (song.source === 'kw' || song.songmid) {
        const kwRid = song.songmid.replace(/^MUSIC_/, '');
        const url = `http://m.kuwo.cn/newh5/singles/songinfoandlrc?musicId=${kwRid}`;
        const res = await fetch(url);
        const data = await res.json();
        const lrclist = data.data?.lrclist;
        if (Array.isArray(lrclist) && lrclist.length > 0) {
          return lrclist
            .map((line: any) => {
              const timeSec = parseFloat(line.time) || 0;
              const m = Math.floor(timeSec / 60);
              const s = Math.floor(timeSec % 60);
              const ms = Math.floor((timeSec % 1) * 100);
              const timeTag = `[${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${ms.toString().padStart(2, '0')}]`;
              return `${timeTag}${line.lineLyric || ''}`;
            })
            .join('\n');
        }
      }
    } catch (e) {
      console.warn('获取歌词失败:', e);
    }
    return '';
  }
}
