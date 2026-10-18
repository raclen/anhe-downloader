import React, { useState } from 'react';
import { Input, Button, Table, Space, message, Tag, Typography } from 'antd';
import { CloudDownloadOutlined, LinkOutlined, ThunderboltOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { SongEntity, MusicQuality } from '../types/music';
import { MusicApi, formatDuration } from '../services/musicApi';

const { Title, Paragraph } = Typography;

interface BatchImportViewProps {
  onBatchDownload: (songs: SongEntity[], quality: MusicQuality) => void;
  preferredQuality: MusicQuality;
}

const PRESET_PLAYLISTS = [
  { name: '网易云热歌榜', id: '3778678' },
  { name: '网易云新歌榜', id: '3779629' },
  { name: '网易云飙升榜', id: '19723756' },
  { name: '说唱榜', id: '991319590' }
];

export const BatchImportView: React.FC<BatchImportViewProps> = ({
  onBatchDownload,
  preferredQuality
}) => {
  const [inputUrl, setInputUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [songs, setSongs] = useState<SongEntity[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  const handleParse = async (targetId = inputUrl) => {
    if (!targetId.trim()) {
      message.warning('请输入歌单链接或歌单 ID');
      return;
    }

    setLoading(true);
    try {
      const list = await MusicApi.parseNetEasePlaylist(targetId);
      setSongs(list);
      setSelectedRowKeys(list.map((s) => s.id));
      message.success(`成功解析歌单！共获取到 ${list.length} 首歌曲`);
    } catch (e: any) {
      message.error(`歌单解析失败: ${e.message || '网络或格式错误'}`);
    } finally {
      setLoading(false);
    }
  };

  const selectedSongs = songs.filter((s) => selectedRowKeys.includes(s.id));

  const columns: ColumnsType<SongEntity> = [
    {
      title: '序号',
      key: 'idx',
      width: 60,
      render: (_, __, i) => <span style={{ color: '#94a3b8' }}>{i + 1}</span>
    },
    {
      title: '歌曲标题',
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <span style={{ fontWeight: 600, color: '#0f172a' }}>{text}</span>
    },
    {
      title: '歌手',
      dataIndex: 'singer',
      key: 'singer',
      render: (text: string) => <span style={{ color: '#334155' }}>{text}</span>
    },
    {
      title: '专辑',
      dataIndex: 'album',
      key: 'album',
      render: (text: string) => <span style={{ color: '#64748b' }}>{text || '-'}</span>
    },
    {
      title: '时长',
      dataIndex: 'duration',
      key: 'duration',
      width: 80,
      render: (sec: number) => <span style={{ color: '#94a3b8' }}>{formatDuration(sec)}</span>
    }
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 链接输入区域 */}
      <div
        style={{
          background: '#ffffff',
          padding: '24px',
          borderRadius: 14,
          border: '1px solid #e2e8f0',
          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)'
        }}
      >
        <Title level={4} style={{ color: '#0f172a', marginTop: 0, marginBottom: 8 }}>
          歌单批量导入解析
        </Title>
        <Paragraph style={{ color: '#64748b', fontSize: 13, marginBottom: 16 }}>
          支持直接粘贴网易云音乐歌单分享链接（如 <code>https://music.163.com/playlist?id=3778678</code>）或纯歌单数字 ID。
        </Paragraph>

        <Space direction="vertical" style={{ width: '100%' }} size={14}>
          <div style={{ display: 'flex', gap: 12 }}>
            <Input
              size="large"
              placeholder="粘贴歌单链接或歌单 ID..."
              value={inputUrl}
              prefix={<LinkOutlined style={{ color: '#94a3b8' }} />}
              onChange={(e) => setInputUrl(e.target.value)}
              onPressEnter={() => handleParse()}
              style={{
                borderRadius: 10,
                background: '#f8fafc',
                borderColor: '#cbd5e1'
              }}
            />
            <Button
              type="primary"
              size="large"
              loading={loading}
              icon={<ThunderboltOutlined />}
              onClick={() => handleParse()}
              style={{
                borderRadius: 10,
                padding: '0 28px',
                background: 'linear-gradient(135deg, #0284c7, #6366f1)',
                border: 'none',
                fontWeight: 600
              }}
            >
              一键解析歌单
            </Button>
          </div>

          <Space size={8}>
            <span style={{ fontSize: 12, color: '#64748b' }}>快捷预设榜单:</span>
            {PRESET_PLAYLISTS.map((p) => (
              <Tag
                key={p.id}
                style={{
                  cursor: 'pointer',
                  background: '#f1f5f9',
                  borderColor: '#e2e8f0',
                  color: '#0284c7',
                  borderRadius: 6
                }}
                onClick={() => {
                  setInputUrl(p.id);
                  handleParse(p.id);
                }}
              >
                {p.name}
              </Tag>
            ))}
          </Space>
        </Space>
      </div>

      {/* 结果控制与表格 */}
      {songs.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              background: '#ffffff',
              padding: '12px 18px',
              borderRadius: 10,
              border: '1px solid #e2e8f0',
              boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)'
            }}
          >
            <span style={{ color: '#475569', fontSize: 13 }}>
              已解析 <b>{songs.length}</b> 首歌曲，已勾选 <b style={{ color: '#16a34a' }}>{selectedRowKeys.length}</b> 首
            </span>

            <Button
              type="primary"
              icon={<CloudDownloadOutlined />}
              disabled={selectedSongs.length === 0}
              style={{ background: '#16a34a', border: 'none', borderRadius: 6 }}
              onClick={() => onBatchDownload(selectedSongs, preferredQuality)}
            >
              一键加入下载队列 ({selectedSongs.length} 首)
            </Button>
          </div>

          <div
            style={{
              background: '#ffffff',
              borderRadius: 14,
              border: '1px solid #e2e8f0',
              boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)',
              overflow: 'hidden'
            }}
          >
            <Table
              rowKey="id"
              columns={columns}
              dataSource={songs}
              pagination={{ pageSize: 20 }}
              rowSelection={{
                selectedRowKeys,
                onChange: (keys) => setSelectedRowKeys(keys)
              }}
            />
          </div>
        </div>
      )}
    </div>
  );
};
