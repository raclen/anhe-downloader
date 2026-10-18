import React, { useState } from 'react';
import { Tabs, Progress, Button, Space, Tag, Empty, Tooltip } from 'antd';
import {
  FolderOpenOutlined,
  CloseCircleOutlined,
  CheckCircleOutlined,
  SyncOutlined
} from '@ant-design/icons';
import { DownloadTask } from '../types/music';
import { CoverAvatar } from '../components/CoverAvatar';

interface DownloadsViewProps {
  tasks: DownloadTask[];
  onCancelTask: (id: string) => void;
  onOpenFolder: (path?: string) => void;
  onClearCompleted: () => void;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

export const DownloadsView: React.FC<DownloadsViewProps> = ({
  tasks,
  onCancelTask,
  onOpenFolder,
  onClearCompleted
}) => {
  const [activeTab, setActiveTab] = useState<'active' | 'completed' | 'all'>('active');

  const activeTasks = tasks.filter((t) => t.status === 'downloading' || t.status === 'pending' || t.status === 'tagging');
  const completedTasks = tasks.filter((t) => t.status === 'completed');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 顶部控制栏 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          background: '#ffffff',
          padding: '16px 20px',
          borderRadius: 14,
          border: '1px solid #e2e8f0',
          boxShadow: '0 1px 3px rgba(0, 0, 0, 0.04)'
        }}
      >
        <Tabs
          activeKey={activeTab}
          onChange={(k) => setActiveTab(k as any)}
          style={{ marginBottom: -16 }}
          items={[
            {
              key: 'active',
              label: `正在下载 (${activeTasks.length})`
            },
            {
              key: 'completed',
              label: `已完成 (${completedTasks.length})`
            },
            {
              key: 'all',
              label: `全部任务 (${tasks.length})`
            }
          ]}
        />

        <Space>
          {completedTasks.length > 0 && activeTab === 'completed' && (
            <Button size="middle" onClick={onClearCompleted}>
              清空记录
            </Button>
          )}
          <Button
            type="primary"
            icon={<FolderOpenOutlined />}
            style={{ background: '#0284c7', border: 'none', borderRadius: 6 }}
            onClick={() => onOpenFolder()}
          >
            打开下载目录
          </Button>
        </Space>
      </div>

      {/* 任务列表展示 */}
      {activeTab === 'active' && (
        <div>
          {activeTasks.length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={<span style={{ color: '#94a3b8' }}>当前暂无正在下载的任务</span>}
              style={{ padding: '60px 0' }}
            />
          ) : (
            activeTasks.map((t) => (
              <div key={t.id} className="task-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                    <div style={{ position: 'relative', width: 44, height: 44 }}>
                      <CoverAvatar src={t.song?.cover} size={44} />
                    </div>

                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ fontWeight: 600, color: '#0f172a', fontSize: 14 }}>{t.song?.name || '未知曲目'}</span>
                        <span className={`quality-badge quality-${t.quality === 'flac' ? 'flac' : t.quality === '320k' ? '320' : '128'}`}>
                          {t.quality.toUpperCase()}
                        </span>
                        {t.status === 'tagging' && <Tag color="processing" icon={<SyncOutlined spin />}>嵌入封面标签</Tag>}
                      </div>
                      <div style={{ color: '#64748b', fontSize: 12, marginTop: 2 }}>
                        {t.song?.singer} · {t.song?.album || '未知专辑'}
                      </div>
                    </div>
                  </div>

                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontSize: 13, fontWeight: 700, color: '#0284c7' }}>{t.speed || '0 KB/s'}</div>
                    <div style={{ fontSize: 12, color: '#94a3b8' }}>
                      {formatBytes(t.downloadedBytes)} / {t.totalBytes > 0 ? formatBytes(t.totalBytes) : '未知'}
                    </div>
                  </div>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                  <div style={{ flex: 1 }}>
                    <Progress
                      percent={Math.floor(t.progress)}
                      size="small"
                      status={t.status === 'error' ? 'exception' : 'active'}
                      strokeColor={{ '0%': '#38bdf8', '100%': '#0284c7' }}
                    />
                  </div>
                  <Tooltip title="取消任务">
                    <Button
                      type="text"
                      danger
                      size="small"
                      icon={<CloseCircleOutlined />}
                      onClick={() => onCancelTask(t.id)}
                    />
                  </Tooltip>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {activeTab === 'completed' && (
        <div>
          {completedTasks.length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={<span style={{ color: '#94a3b8' }}>暂无已完成的下载</span>}
              style={{ padding: '60px 0' }}
            />
          ) : (
            completedTasks.map((t) => (
              <div key={t.id} className="task-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                    <CoverAvatar src={t.song?.cover} size={44} />

                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ fontWeight: 600, color: '#0f172a', fontSize: 14 }}>{t.song?.name}</span>
                        <span className={`quality-badge quality-${t.quality === 'flac' ? 'flac' : t.quality === '320k' ? '320' : '128'}`}>
                          {t.quality.toUpperCase()}
                        </span>
                        <Tag color="success" icon={<CheckCircleOutlined />}>已完成</Tag>
                      </div>
                      <div style={{ color: '#64748b', fontSize: 12, marginTop: 2 }}>
                        {t.song?.singer} · 文件大小: {formatBytes(t.totalBytes || t.downloadedBytes)}
                      </div>
                    </div>
                  </div>

                  <Space>
                    <Button
                      size="small"
                      icon={<FolderOpenOutlined />}
                      onClick={() => onOpenFolder(t.savePath)}
                    >
                      定位文件
                    </Button>
                  </Space>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {activeTab === 'all' && (
        <div>
          {tasks.map((t) => (
            <div key={t.id} className="task-card">
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <span style={{ fontWeight: 600, color: '#0f172a' }}>{t.song?.name}</span>
                  <Tag color={t.status === 'completed' ? 'success' : t.status === 'error' ? 'error' : 'processing'}>
                    {t.status}
                  </Tag>
                </div>
                {t.savePath && (
                  <Button size="small" onClick={() => onOpenFolder(t.savePath)}>
                    查看文件
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
