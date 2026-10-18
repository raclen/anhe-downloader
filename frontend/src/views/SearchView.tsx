import React, { useState } from 'react';
import {
  Input,
  Button,
  Radio,
  Table,
  Tag,
  Space,
  Tooltip,
  Dropdown,
  message
} from 'antd';
import {
  SearchOutlined,
  DownloadOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { SongEntity, MusicQuality, MusicSource } from '../types/music';
import { MusicApi, formatDuration } from '../services/musicApi';
import { CoverAvatar } from '../components/CoverAvatar';

interface SearchViewProps {
  onPlaySong: (song: SongEntity) => void;
  onDownloadSong: (song: SongEntity, quality: MusicQuality) => void;
  onBatchDownload: (songs: SongEntity[], quality: MusicQuality) => void;
  preferredQuality: MusicQuality;
}

const HOT_KEYWORDS = ['晴天', '我们的纪念', '七里香', '周杰伦', '起风了', '陈奕迅', '兰亭序', '孤勇者'];

export const SearchView: React.FC<SearchViewProps> = ({
  onPlaySong,
  onDownloadSong,
  onBatchDownload,
  preferredQuality
}) => {
  const [keyword, setKeyword] = useState('');
  const [source, setSource] = useState<'all' | MusicSource>('all');
  const [loading, setLoading] = useState(false);
  const [songs, setSongs] = useState<SongEntity[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  const handleSearch = async (kw = keyword) => {
    if (!kw.trim()) {
      message.warning('请输入搜索歌曲名或歌手名');
      return;
    }
    setLoading(true);
    try {
      const res = await MusicApi.search(kw, source, 1, 40);
      setSongs(res.list);
      setSelectedRowKeys([]);
      if (res.list.length === 0) {
        message.info('未找到相关歌曲，换个关键词或源试试看吧');
      }
    } catch (err: any) {
      message.error(`搜索失败: ${err.message || '网络异常'}`);
    } finally {
      setLoading(false);
    }
  };

  const selectedSongs = songs.filter((s) => selectedRowKeys.includes(s.id));

  const columns: ColumnsType<SongEntity> = [
    {
      title: '封面',
      dataIndex: 'cover',
      key: 'cover',
      width: 60,
      render: (cover: string, record) => (
        <div style={{ position: 'relative', width: 42, height: 42, cursor: 'pointer' }} onClick={() => onPlaySong(record)}>
          <CoverAvatar src={cover} size={42} />
        </div>
      )
    },
    {
      title: '歌曲标题',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record) => (
        <div>
          <span style={{ fontWeight: 600, color: '#0f172a', fontSize: 14 }}>{text}</span>
          <Tag color="blue" style={{ marginLeft: 8, fontSize: 10, padding: '0 4px', borderRadius: 4 }}>
            {record.source.toUpperCase()}
          </Tag>
        </div>
      )
    },
    {
      title: '歌手',
      dataIndex: 'singer',
      key: 'singer',
      render: (text: string) => <span style={{ color: '#334155', fontWeight: 500 }}>{text}</span>
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
      width: 75,
      render: (sec: number) => <span style={{ color: '#94a3b8', fontSize: 13 }}>{formatDuration(sec)}</span>
    },
    {
      title: '音质档位',
      dataIndex: 'types',
      key: 'types',
      width: 170,
      render: (types: MusicQuality[], record) => (
        <Space size={4}>
          {types.includes('flac') && (
            <Tooltip title="点击直接下载 FLAC 无损">
              <span
                className="quality-badge quality-flac"
                style={{ cursor: 'pointer' }}
                onClick={(e) => {
                  e.stopPropagation();
                  onDownloadSong(record, 'flac');
                }}
              >
                FLAC
              </span>
            </Tooltip>
          )}
          {types.includes('320k') && (
            <Tooltip title="点击直接下载 320K 高品质">
              <span
                className="quality-badge quality-320"
                style={{ cursor: 'pointer' }}
                onClick={(e) => {
                  e.stopPropagation();
                  onDownloadSong(record, '320k');
                }}
              >
                320K
              </span>
            </Tooltip>
          )}
          {types.includes('128k') && (
            <Tooltip title="点击直接下载 128K 标准">
              <span
                className="quality-badge quality-128"
                style={{ cursor: 'pointer' }}
                onClick={(e) => {
                  e.stopPropagation();
                  onDownloadSong(record, '128k');
                }}
              >
                128K
              </span>
            </Tooltip>
          )}
        </Space>
      )
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_, record) => {
        const downloadMenuItems = [
          {
            key: 'flac',
            label: '无损品质 (FLAC)',
            disabled: !record.types.includes('flac'),
            onClick: () => onDownloadSong(record, 'flac')
          },
          {
            key: '320k',
            label: '极高品质 (320Kbps)',
            disabled: !record.types.includes('320k'),
            onClick: () => onDownloadSong(record, '320k')
          },
          {
            key: '128k',
            label: '标准品质 (128Kbps)',
            onClick: () => onDownloadSong(record, '128k')
          }
        ];

        return (
          <Space>
            <Tooltip title="试听预览">
              <Button
                type="text"
                shape="circle"
                icon={<PlayCircleOutlined style={{ fontSize: 18, color: '#0284c7' }} />}
                onClick={() => onPlaySong(record)}
              />
            </Tooltip>
            <Dropdown menu={{ items: downloadMenuItems }} placement="bottomRight">
              <Button
                type="primary"
                size="small"
                icon={<DownloadOutlined />}
                style={{ background: '#0284c7', border: 'none', borderRadius: 6 }}
                onClick={() => onDownloadSong(record, preferredQuality)}
              >
                下载
              </Button>
            </Dropdown>
          </Space>
        );
      }
    }
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 搜索卡片 */}
      <div
        style={{
          background: '#ffffff',
          padding: '20px 24px',
          borderRadius: 14,
          border: '1px solid #e2e8f0',
          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)'
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }} size={14}>
          <div style={{ display: 'flex', gap: 12 }}>
            <Input
              size="large"
              placeholder="搜索歌曲名、歌手、专辑 (例如: 我们的纪念、晴天)..."
              value={keyword}
              prefix={<SearchOutlined style={{ color: '#94a3b8' }} />}
              onChange={(e) => setKeyword(e.target.value)}
              onPressEnter={() => handleSearch()}
              allowClear
              style={{
                borderRadius: 10,
                background: '#f8fafc',
                borderColor: '#cbd5e1'
              }}
            />
            <Button
              type="primary"
              size="large"
              icon={<SearchOutlined />}
              loading={loading}
              onClick={() => handleSearch()}
              style={{
                borderRadius: 10,
                padding: '0 28px',
                background: 'linear-gradient(135deg, #0284c7, #2563eb)',
                border: 'none',
                fontWeight: 600
              }}
            >
              全网搜索
            </Button>
          </div>

          {/* 平台选择与热搜词 */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 12 }}>
            <Radio.Group
              value={source}
              onChange={(e) => {
                setSource(e.target.value);
                if (keyword.trim()) handleSearch();
              }}
              buttonStyle="solid"
            >
              <Radio.Button value="all">全网聚合</Radio.Button>
              <Radio.Button value="tx">QQ音乐</Radio.Button>
              <Radio.Button value="kg">酷狗音乐</Radio.Button>
              <Radio.Button value="kw">酷我音乐</Radio.Button>
              <Radio.Button value="wy">网易云</Radio.Button>
            </Radio.Group>

            <Space size={6}>
              <span style={{ fontSize: 12, color: '#64748b' }}>大家都在搜:</span>
              {HOT_KEYWORDS.map((word) => (
                <Tag
                  key={word}
                  style={{
                    cursor: 'pointer',
                    background: '#f1f5f9',
                    borderColor: '#e2e8f0',
                    color: '#475569',
                    borderRadius: 6
                  }}
                  onClick={() => {
                    setKeyword(word);
                    handleSearch(word);
                  }}
                >
                  {word}
                </Tag>
              ))}
            </Space>
          </div>
        </Space>
      </div>

      {/* 批量操作栏 */}
      {songs.length > 0 && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            background: '#ffffff',
            padding: '10px 18px',
            borderRadius: 10,
            border: '1px solid #e2e8f0',
            boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)'
          }}
        >
          <span style={{ color: '#475569', fontSize: 13 }}>
            共检索到 <b style={{ color: '#0284c7' }}>{songs.length}</b> 首歌曲，已勾选 <b style={{ color: '#16a34a' }}>{selectedRowKeys.length}</b> 首
          </span>

          <Space>
            <Button
              type="primary"
              size="middle"
              disabled={selectedSongs.length === 0}
              icon={<CheckCircleOutlined />}
              style={{ background: '#16a34a', border: 'none', borderRadius: 6 }}
              onClick={() => onBatchDownload(selectedSongs, preferredQuality)}
            >
              批量下载已选 ({selectedSongs.length})
            </Button>
          </Space>
        </div>
      )}

      {/* 歌曲数据表 */}
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
          loading={loading}
          pagination={{ pageSize: 15, showTotal: (t) => `共 ${t} 首歌曲` }}
          rowSelection={{
            selectedRowKeys,
            onChange: (keys) => setSelectedRowKeys(keys)
          }}
        />
      </div>
    </div>
  );
};
