import React, { useState, useEffect } from 'react';
import { ConfigProvider, theme, message, Badge } from 'antd';
import {
  SearchOutlined,
  CloudDownloadOutlined,
  ThunderboltOutlined,
  SettingOutlined,
  CheckCircleFilled
} from '@ant-design/icons';
import { AppSettings, DownloadTask, MusicQuality, SongEntity, SoundSourceMeta } from './types/music';
import { SoundSourceEngine } from './services/soundSourceEngine';
import { MusicApi } from './services/musicApi';
import { SearchView } from './views/SearchView';
import { BatchImportView } from './views/BatchImportView';
import { DownloadsView } from './views/DownloadsView';
import { SettingsView } from './views/SettingsView';
import { MiniPlayer } from './components/MiniPlayer';
import './App.css';

export const App: React.FC = () => {
  const [activeNav, setActiveNav] = useState<'search' | 'batch' | 'downloads' | 'settings'>('search');
  const [settings, setSettings] = useState<AppSettings>({
    downloadDir: 'C:\\Users\\Administrator\\Music\\AnHeMusic',
    preferredQuality: '320k',
    autoFallback: true,
    fileNamePattern: '{singer} - {name}',
    embedCover: true,
    embedLyrics: true,
    saveLrcFile: false,
    maxConcurrent: 3,
    activeSourceScript: '长青SVIP音源v1.3.0.protected.js'
  });

  const [soundSourceMeta, setSoundSourceMeta] = useState<SoundSourceMeta>(
    SoundSourceEngine.getInstance().meta
  );

  const [tasks, setTasks] = useState<DownloadTask[]>([]);
  const [currentPlayingSong, setCurrentPlayingSong] = useState<SongEntity | null>(null);
  const [currentAudioUrl, setCurrentAudioUrl] = useState<string | null>(null);

  // 初始化加载配置和音源脚本
  useEffect(() => {
    // 1. 读取后端配置
    fetch('/api/config')
      .then((r) => r.json())
      .then((cfg) => {
        if (cfg.defaultDownloadDir) {
          setSettings((prev) => ({ ...prev, downloadDir: cfg.defaultDownloadDir }));
        }
      })
      .catch((err) => console.log('无法连接后端 API:', err));

    // 2. 加载长青音源脚本
    loadSourceScript();

    // 3. 任务轮询更新
    const interval = setInterval(fetchTasks, 1000);
    return () => clearInterval(interval);
  }, []);

  const loadSourceScript = async (): Promise<boolean> => {
    try {
      const r = await fetch('/api/source-script');
      if (!r.ok) throw new Error('音源脚本文件未找到');
      const code = await r.text();
      const engine = SoundSourceEngine.getInstance();
      const ok = engine.initFromScript(code);
      setSoundSourceMeta({ ...engine.meta });
      return ok;
    } catch (e) {
      console.warn('加载服务端音源脚本失败:', e);
      return false;
    }
  };

  const fetchTasks = () => {
    fetch('/api/tasks')
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) {
          setTasks(
            data.map((item: any) => ({
              id: item.id,
              song: {
                id: item.id,
                source: 'kw',
                name: item.title,
                singer: item.singer,
                album: item.album,
                duration: 0,
                cover: item.coverUrl,
                songmid: '',
                types: ['128k', '320k', 'flac']
              },
              quality: item.quality,
              status: item.status,
              progress: item.progress,
              downloadedBytes: item.downloadedBytes,
              totalBytes: item.totalBytes,
              speed: item.speed,
              savePath: item.savePath,
              errorMsg: item.errorMsg,
              createdAt: item.createdAt
            }))
          );
        }
      })
      .catch(() => {});
  };

  // 单曲下载触发
  const handleDownloadSong = async (song: SongEntity, quality: MusicQuality) => {
    const hide = message.loading(`正在为《${song.name}》解析 ${quality.toUpperCase()} 直链...`, 0);
    try {
      const engine = SoundSourceEngine.getInstance();
      const resolved = await engine.resolveAudioUrl(song, quality);
      hide();

      // 获取歌词
      let lyric = '';
      if (settings.embedLyrics || settings.saveLrcFile) {
        lyric = await MusicApi.fetchLyric(song);
      }

      // 生成文件名
      let fileName = settings.fileNamePattern
        .replace('{singer}', song.singer)
        .replace('{name}', song.name)
        .replace('{album}', song.album || '');

      const format = resolved.format || (resolved.actualQuality === 'flac' ? 'flac' : 'mp3');

      const downloadPayload = {
        taskId: `task_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
        url: resolved.url,
        title: song.name,
        singer: song.singer,
        album: song.album,
        quality: resolved.actualQuality,
        format,
        coverUrl: song.cover,
        lyric,
        saveDir: settings.downloadDir,
        fileName,
        embedCover: settings.embedCover,
        embedLyrics: settings.embedLyrics,
        saveLrcFile: settings.saveLrcFile,
        source: song.source,
        songmid: song.songmid
      };

      const res = await fetch('/api/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(downloadPayload)
      });

      if (!res.ok) throw new Error('提交下载任务失败');

      message.success(`已添加任务: ${song.name} [${resolved.resolver}]`);
      fetchTasks();
    } catch (err: any) {
      hide();
      message.error(`解析下载失败: ${err.message}`);
    }
  };

  // 批量下载触发
  const handleBatchDownload = async (songs: SongEntity[], quality: MusicQuality) => {
    message.info(`已开始逐一解析添加 ${songs.length} 首歌曲到下载队列...`);
    for (const s of songs) {
      try {
        await handleDownloadSong(s, quality);
      } catch {
        // 继续下一个
      }
    }
  };

  // 试听触发
  const handlePlaySong = async (song: SongEntity) => {
    const hide = message.loading(`正在获取《${song.name}》试听音轨...`, 0);
    try {
      const engine = SoundSourceEngine.getInstance();
      const resolved = await engine.resolveAudioUrl(song, '128k');
      hide();
      setCurrentPlayingSong(song);
      setCurrentAudioUrl(resolved.url);
    } catch (err: any) {
      hide();
      message.error(`试听加载失败: ${err.message}`);
    }
  };

  // 选择目录
  const handleSelectDir = async () => {
    try {
      const res = await fetch('/api/select-dir', { method: 'POST' });
      const data = await res.json();
      if (data.selectedDir) {
        setSettings((prev) => ({ ...prev, downloadDir: data.selectedDir }));
        message.success(`已更改下载路径为: ${data.selectedDir}`);
      }
    } catch (e: any) {
      message.error(`选择路径失败: ${e.message}`);
    }
  };

  // 打开所在目录
  const handleOpenFolder = (path?: string) => {
    fetch('/api/open-folder', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: path || settings.downloadDir })
    }).catch(() => {});
  };

  // 取消任务
  const handleCancelTask = (id: string) => {
    fetch('/api/cancel-task', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    })
      .then(() => {
        message.info('任务已取消');
        fetchTasks();
      })
      .catch(() => {});
  };

  const activeTasksCount = tasks.filter((t) => t.status === 'downloading' || t.status === 'pending').length;

  return (
    <ConfigProvider
      theme={{
        algorithm: theme.defaultAlgorithm, // 纯净日间模式算法
        token: {
          colorPrimary: '#0284c7',
          colorBgBase: '#f8fafc',
          colorBgContainer: '#ffffff',
          colorText: '#0f172a',
          colorTextSecondary: '#475569',
          colorBorder: '#e2e8f0',
          borderRadius: 8
        }
      }}
    >
      <div className="app-container">
        {/* 左侧侧边栏 */}
        <aside className="sidebar">
          <div className="logo-area">
            <div className="logo-icon">♪</div>
            <div>
              <div className="logo-text">风宝音乐下载器</div>
              <div style={{ fontSize: 11, color: '#64748b' }}>AnHe Downloader</div>
            </div>
          </div>

          <nav className="nav-menu">
            <div
              className={`nav-item ${activeNav === 'search' ? 'active' : ''}`}
              onClick={() => setActiveNav('search')}
            >
              <div className="nav-item-left">
                <SearchOutlined />
                <span>音乐搜索</span>
              </div>
            </div>

            <div
              className={`nav-item ${activeNav === 'batch' ? 'active' : ''}`}
              onClick={() => setActiveNav('batch')}
            >
              <div className="nav-item-left">
                <ThunderboltOutlined />
                <span>歌单批量导入</span>
              </div>
            </div>

            <div
              className={`nav-item ${activeNav === 'downloads' ? 'active' : ''}`}
              onClick={() => setActiveNav('downloads')}
            >
              <div className="nav-item-left">
                <CloudDownloadOutlined />
                <span>下载管理</span>
              </div>
              {activeTasksCount > 0 && <Badge count={activeTasksCount} size="small" />}
            </div>

            <div
              className={`nav-item ${activeNav === 'settings' ? 'active' : ''}`}
              onClick={() => setActiveNav('settings')}
            >
              <div className="nav-item-left">
                <SettingOutlined />
                <span>偏好设置</span>
              </div>
            </div>
          </nav>

          {/* 底部音源状态指示灯 */}
          <div className="source-status-badge">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <CheckCircleFilled style={{ color: '#16a34a', fontSize: 13 }} />
                <span style={{ fontSize: 12, fontWeight: 600, color: '#0f172a' }}>{soundSourceMeta.name}</span>
              </div>
              <span style={{ fontSize: 11, color: '#16a34a', fontWeight: 600 }}>在线就绪</span>
            </div>
            <div style={{ fontSize: 11, color: '#64748b', marginTop: 2 }}>
              洛雪音源协议 v{soundSourceMeta.version}
            </div>
          </div>
        </aside>

        {/* 主工作区 */}
        <main className="main-content">
          <div className="view-container">
            {activeNav === 'search' && (
              <SearchView
                onPlaySong={handlePlaySong}
                onDownloadSong={handleDownloadSong}
                onBatchDownload={handleBatchDownload}
                preferredQuality={settings.preferredQuality}
              />
            )}

            {activeNav === 'batch' && (
              <BatchImportView
                onBatchDownload={handleBatchDownload}
                preferredQuality={settings.preferredQuality}
              />
            )}

            {activeNav === 'downloads' && (
              <DownloadsView
                tasks={tasks}
                onCancelTask={handleCancelTask}
                onOpenFolder={handleOpenFolder}
                onClearCompleted={() => setTasks((prev) => prev.filter((t) => t.status !== 'completed'))}
              />
            )}

            {activeNav === 'settings' && (
              <SettingsView
                settings={settings}
                onUpdateSettings={(newVals) => setSettings((prev) => ({ ...prev, ...newVals }))}
                soundSourceMeta={soundSourceMeta}
                onSelectDir={handleSelectDir}
                onReloadSource={loadSourceScript}
              />
            )}
          </div>

          {/* 底部迷你试听条 */}
          <MiniPlayer
            currentSong={currentPlayingSong}
            audioUrl={currentAudioUrl}
            onClose={() => {
              setCurrentPlayingSong(null);
              setCurrentAudioUrl(null);
            }}
          />
        </main>
      </div>
    </ConfigProvider>
  );
};

export default App;
