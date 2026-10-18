import React, { useState } from 'react';
import {
  Card,
  Input,
  Button,
  Select,
  Switch,
  Slider,
  Space,
  Tag,
  Divider,
  Typography,
  message,
  Upload
} from 'antd';
import {
  FolderOutlined,
  ApiOutlined,
  UploadOutlined
} from '@ant-design/icons';
import { AppSettings, SoundSourceMeta } from '../types/music';
import { SoundSourceEngine } from '../services/soundSourceEngine';

const { Paragraph } = Typography;

interface SettingsViewProps {
  settings: AppSettings;
  onUpdateSettings: (newSettings: Partial<AppSettings>) => void;
  soundSourceMeta: SoundSourceMeta;
  onSelectDir: () => void;
  onReloadSource: () => Promise<boolean>;
}

export const SettingsView: React.FC<SettingsViewProps> = ({
  settings,
  onUpdateSettings,
  soundSourceMeta,
  onSelectDir,
  onReloadSource
}) => {
  const [testing, setTesting] = useState(false);

  const handleTestConnectivity = async () => {
    setTesting(true);
    try {
      const engine = SoundSourceEngine.getInstance();
      const res = await engine.resolveAudioUrl(
        {
          id: 'test_1',
          source: 'kw',
          name: '我们的纪念',
          singer: '李雅微',
          album: '放羊的星星',
          duration: 280,
          cover: '',
          songmid: '231616',
          types: ['128k', '320k', 'flac']
        },
        '320k'
      );
      message.success(`解析测试成功！通道: [${res.resolver}]，响应正常！`);
    } catch (e: any) {
      message.error(`解析测试失败: ${e.message}`);
    } finally {
      setTesting(false);
    }
  };

  const handleCustomScriptUpload = async (file: File) => {
    try {
      const content = await file.text();
      if (!content.trim()) {
        message.error('音源脚本内容为空');
        return false;
      }

      // 1. 先在内存里验证脚本能否注册解析处理器，失败则不入库（避免重启后音源彻底失效）
      const engine = SoundSourceEngine.getInstance();
      if (!engine.initFromScript(content)) {
        message.error(`脚本无效：${engine.meta.error || '未注册 request 处理器'}`);
        await onReloadSource();
        return false;
      }

      // 2. 持久化到后端，否则重新加载时会被内置脚本覆盖（之前“提示成功实际失败”的根因）
      const resp = await fetch('/api/source-script', {
        method: 'POST',
        headers: { 'Content-Type': 'application/javascript; charset=utf-8' },
        body: content
      });
      if (!resp.ok) {
        const detail = await resp.text();
        await onReloadSource();
        message.error(`保存音源脚本失败：${detail || resp.status}`);
        return false;
      }

      const ok = await onReloadSource();
      if (ok) {
        message.success(`音源「${engine.meta.name} v${engine.meta.version}」已保存并生效！`);
      } else {
        message.error('脚本已保存，但重新加载失败，请重新上传或恢复内置音源');
      }
    } catch (e: any) {
      message.error(`更换音源失败：${e?.message || e}`);
    }
    return false;
  };

  const handleRestoreBuiltin = async () => {
    try {
      const resp = await fetch('/api/source-script', { method: 'DELETE' });
      if (!resp.ok) {
        message.error(`恢复内置音源失败：${(await resp.text()) || resp.status}`);
        return;
      }
      const ok = await onReloadSource();
      if (ok) message.success('已恢复内置长青音源');
      else message.warning('已删除自定义音源，但内置音源初始化失败');
    } catch (e: any) {
      message.error(`恢复内置音源失败：${e?.message || e}`);
    }
  };

  const cardStyle = {
    background: '#ffffff',
    border: '1px solid #e2e8f0',
    borderRadius: 14,
    boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)'
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 860 }}>
      {/* 存储与文件命名 */}
      <Card
        title={<span style={{ color: '#0f172a', fontWeight: 600 }}>📁 下载存储与命名</span>}
        style={cardStyle}
      >
        <Space direction="vertical" style={{ width: '100%' }} size={16}>
          <div>
            <div style={{ color: '#64748b', fontSize: 13, marginBottom: 6 }}>默认保存路径:</div>
            <div style={{ display: 'flex', gap: 12 }}>
              <Input
                value={settings.downloadDir}
                readOnly
                style={{
                  background: '#f8fafc',
                  borderColor: '#cbd5e1',
                  color: '#0f172a'
                }}
              />
              <Button type="primary" icon={<FolderOutlined />} onClick={onSelectDir} style={{ background: '#0284c7' }}>
                更改路径
              </Button>
            </div>
          </div>

          <div>
            <div style={{ color: '#64748b', fontSize: 13, marginBottom: 6 }}>文件命名规则:</div>
            <Select
              value={settings.fileNamePattern}
              onChange={(val) => onUpdateSettings({ fileNamePattern: val })}
              style={{ width: 320 }}
              options={[
                { value: '{singer} - {name}', label: '歌手 - 歌曲名 (例: 李雅微 - 我们的纪念.flac)' },
                { value: '{name} - {singer}', label: '歌曲名 - 歌手 (例: 我们的纪念 - 李雅微.flac)' },
                { value: '{name}', label: '仅歌曲名 (例: 我们的纪念.flac)' }
              ]}
            />
          </div>
        </Space>
      </Card>

      {/* 音质偏好与标签 */}
      <Card
        title={<span style={{ color: '#0f172a', fontWeight: 600 }}>🎧 音质偏好与标签</span>}
        style={cardStyle}
      >
        <Space direction="vertical" style={{ width: '100%' }} size={16}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ color: '#0f172a', fontWeight: 500 }}>首选下载音质</div>
              <div style={{ color: '#64748b', fontSize: 12 }}>单曲或批量下载时的默认选定音质</div>
            </div>
            <Select
              value={settings.preferredQuality}
              onChange={(val) => onUpdateSettings({ preferredQuality: val })}
              style={{ width: 170 }}
              options={[
                { value: 'flac', label: 'FLAC 无损 (优先)' },
                { value: '320k', label: '320Kbps 高品' },
                { value: '128k', label: '128Kbps 标准' }
              ]}
            />
          </div>

          <Divider style={{ margin: '8px 0', borderColor: '#f1f5f9' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ color: '#0f172a', fontWeight: 500 }}>智能音质自动降级</div>
              <div style={{ color: '#64748b', fontSize: 12 }}>若歌曲未提供无损音质时，自动降级为 320k 或 128k 下载</div>
            </div>
            <Switch
              checked={settings.autoFallback}
              onChange={(checked) => onUpdateSettings({ autoFallback: checked })}
            />
          </div>

          <Divider style={{ margin: '8px 0', borderColor: '#f1f5f9' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ color: '#0f172a', fontWeight: 500 }}>嵌入专辑高清封面</div>
              <div style={{ color: '#64748b', fontSize: 12 }}>将专辑封面图片写入音频文件 ID3 / FLAC Picture 标签中</div>
            </div>
            <Switch
              checked={settings.embedCover}
              onChange={(checked) => onUpdateSettings({ embedCover: checked })}
            />
          </div>

          <Divider style={{ margin: '8px 0', borderColor: '#f1f5f9' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ color: '#0f172a', fontWeight: 500 }}>嵌入歌词元数据（推荐）</div>
              <div style={{ color: '#64748b', fontSize: 12 }}>
                向 MP3 写入 ID3 USLT + SYLT 同步歌词，FLAC 写入 Vorbis Comment，单文件即可显示滚动歌词
              </div>
            </div>
            <Switch
              checked={settings.embedLyrics}
              onChange={(checked) => onUpdateSettings({ embedLyrics: checked })}
            />
          </div>

          <Divider style={{ margin: '8px 0', borderColor: '#f1f5f9' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ color: '#0f172a', fontWeight: 500 }}>额外另存 .lrc 歌词文件</div>
              <div style={{ color: '#64748b', fontSize: 12 }}>
                默认关闭。开启后会在歌曲同目录额外生成同名 .lrc（歌词已内嵌，无需外挂文件）
              </div>
            </div>
            <Switch
              checked={settings.saveLrcFile}
              onChange={(checked) => onUpdateSettings({ saveLrcFile: checked })}
            />
          </div>

          <Divider style={{ margin: '8px 0', borderColor: '#f1f5f9' }} />

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ color: '#0f172a', fontWeight: 500 }}>最大并发下载数</div>
              <div style={{ color: '#64748b', fontSize: 12 }}>同时进行下载的任务线程数 (1 ~ 5)</div>
            </div>
            <div style={{ width: 160 }}>
              <Slider
                min={1}
                max={5}
                value={settings.maxConcurrent}
                onChange={(val) => onUpdateSettings({ maxConcurrent: val })}
              />
            </div>
          </div>
        </Space>
      </Card>

      {/* 洛雪音源状态与管理 */}
      <Card
        title={<span style={{ color: '#0f172a', fontWeight: 600 }}>📡 洛雪音源解析系统</span>}
        style={cardStyle}
      >
        <Space direction="vertical" style={{ width: '100%' }} size={16}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <span style={{ fontSize: 16, fontWeight: 700, color: '#0284c7' }}>{soundSourceMeta.name}</span>
                <Tag color="blue">v{soundSourceMeta.version}</Tag>
                <Tag color="purple">作者: {soundSourceMeta.author}</Tag>
                <Tag color={soundSourceMeta.status === 'active' ? 'success' : 'error'}>
                  {soundSourceMeta.status === 'active' ? '🟢 运行中' : '🔴 异常'}
                </Tag>
              </div>
              <div style={{ color: '#64748b', fontSize: 12, marginTop: 4 }}>
                支持平台: {soundSourceMeta.supportedSources?.join(' / ').toUpperCase()}
              </div>
            </div>

            <Space>
              <Button
                icon={<ApiOutlined />}
                loading={testing}
                onClick={handleTestConnectivity}
              >
                连通性测试
              </Button>
              <Upload beforeUpload={handleCustomScriptUpload} showUploadList={false} accept=".js">
                <Button icon={<UploadOutlined />}>更换音源脚本</Button>
              </Upload>
              <Button onClick={handleRestoreBuiltin}>恢复内置音源</Button>
            </Space>
          </div>

          <Paragraph style={{ color: '#64748b', fontSize: 12, margin: 0 }}>
            提示：上传的 .js 音源会先验证能否注册 request 处理器，验证通过后保存为程序目录下的
            <code style={{ margin: '0 4px' }}>custom-source.js</code> 并立即生效，下次启动也继续使用；
            若脚本损坏可点「恢复内置音源」回退到长青SVIP音源。若三方服务器超时或 404，会自动触发后端备用解析引擎。
          </Paragraph>
        </Space>
      </Card>
    </div>
  );
};
