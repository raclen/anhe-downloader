import React, { useState, useRef, useEffect } from 'react';
import { Button, Slider, Tooltip, Tag } from 'antd';
import {
  PlayCircleFilled,
  PauseCircleFilled,
  SoundOutlined,
  CloseOutlined,
  CustomerServiceOutlined
} from '@ant-design/icons';
import { SongEntity } from '../types/music';
import { formatDuration } from '../services/musicApi';
import { CoverAvatar } from './CoverAvatar';

interface MiniPlayerProps {
  currentSong: SongEntity | null;
  audioUrl: string | null;
  onClose: () => void;
}

export const MiniPlayer: React.FC<MiniPlayerProps> = ({ currentSong, audioUrl, onClose }) => {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.8);

  useEffect(() => {
    if (audioUrl && audioRef.current) {
      audioRef.current.src = audioUrl;
      audioRef.current
        .play()
        .then(() => setIsPlaying(true))
        .catch((e) => console.log('Autoplay blocked:', e));
    } else {
      setIsPlaying(false);
      setCurrentTime(0);
      setDuration(0);
    }
  }, [audioUrl]);

  const togglePlay = () => {
    if (!audioRef.current || !audioUrl) return;
    if (isPlaying) {
      audioRef.current.pause();
      setIsPlaying(false);
    } else {
      audioRef.current.play();
      setIsPlaying(true);
    }
  };

  const handleTimeUpdate = () => {
    if (audioRef.current) {
      setCurrentTime(audioRef.current.currentTime);
      if (audioRef.current.duration) {
        setDuration(audioRef.current.duration);
      }
    }
  };

  const handleSeek = (val: number) => {
    if (audioRef.current && audioUrl) {
      audioRef.current.currentTime = val;
      setCurrentTime(val);
    }
  };

  const handleVolumeChange = (val: number) => {
    setVolume(val);
    if (audioRef.current) {
      audioRef.current.volume = val;
    }
  };

  // 未播放时的底栏占位态：确保底栏图标与状态始终完整展现
  if (!currentSong || !audioUrl) {
    return (
      <div className="mini-player-bar">
        {/* 左侧：就绪状态图标 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 260 }}>
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 8,
              background: 'linear-gradient(135deg, #e0f2fe, #e0e7ff)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#0284c7',
              fontSize: 22,
              boxShadow: '0 1px 3px rgba(0, 0, 0, 0.05)'
            }}
          >
            <CustomerServiceOutlined />
          </div>
          <div>
            <div style={{ fontWeight: 600, color: '#0f172a', fontSize: 13 }}>风宝音乐播放器就绪</div>
            <div style={{ color: '#94a3b8', fontSize: 12 }}>点击歌曲列表中的「试听」即可在此播放</div>
          </div>
        </div>

        {/* 中间：空闲进度条 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 14, flex: 1, maxWidth: 540, justifyContent: 'center' }}>
          <Button
            type="text"
            shape="circle"
            disabled
            icon={<PlayCircleFilled style={{ fontSize: 32, color: '#cbd5e1' }} />}
          />
          <span style={{ fontSize: 12, color: '#94a3b8', width: 40, textAlign: 'right' }}>00:00</span>
          <div style={{ flex: 1 }}>
            <Slider disabled min={0} max={100} value={0} style={{ margin: 0 }} />
          </div>
          <span style={{ fontSize: 12, color: '#94a3b8', width: 40 }}>00:00</span>
        </div>

        {/* 右侧：状态指示 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 200, justifyContent: 'flex-end' }}>
          <Tag color="cyan" style={{ borderRadius: 6, margin: 0 }}>洛雪SVIP双通道</Tag>
          <SoundOutlined style={{ color: '#94a3b8', fontSize: 16 }} />
          <div style={{ width: 70 }}>
            <Slider min={0} max={1} step={0.01} value={volume} onChange={handleVolumeChange} style={{ margin: 0 }} />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mini-player-bar">
      <audio
        ref={audioRef}
        onTimeUpdate={handleTimeUpdate}
        onEnded={() => setIsPlaying(false)}
      />

      {/* 左侧：当前歌曲信息 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 260 }}>
        <CoverAvatar src={currentSong.cover} size={44} />
        <div style={{ maxWidth: 200, overflow: 'hidden' }}>
          <div
            style={{
              fontWeight: 600,
              color: '#0f172a',
              fontSize: 13,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap'
            }}
          >
            {currentSong.name}
          </div>
          <div
            style={{
              color: '#64748b',
              fontSize: 12,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap'
            }}
          >
            {currentSong.singer} · {currentSong.album || '单曲'}
          </div>
        </div>
      </div>

      {/* 中间：播放控制与进度条 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, flex: 1, maxWidth: 540, justifyContent: 'center' }}>
        <Button
          type="text"
          shape="circle"
          icon={
            isPlaying ? (
              <PauseCircleFilled style={{ fontSize: 34, color: '#0284c7' }} />
            ) : (
              <PlayCircleFilled style={{ fontSize: 34, color: '#0284c7' }} />
            )
          }
          onClick={togglePlay}
        />

        <span style={{ fontSize: 12, color: '#94a3b8', width: 40, textAlign: 'right' }}>
          {formatDuration(Math.floor(currentTime))}
        </span>

        <div style={{ flex: 1 }}>
          <Slider
            min={0}
            max={duration || currentSong.duration || 100}
            value={currentTime}
            onChange={handleSeek}
            tooltip={{ formatter: (v) => formatDuration(v || 0) }}
            style={{ margin: 0 }}
          />
        </div>

        <span style={{ fontSize: 12, color: '#94a3b8', width: 40 }}>
          {formatDuration(Math.floor(duration || currentSong.duration))}
        </span>
      </div>

      {/* 右侧：音量与关闭 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 200, justifyContent: 'flex-end' }}>
        <SoundOutlined style={{ color: '#64748b', fontSize: 16 }} />
        <div style={{ width: 70 }}>
          <Slider
            min={0}
            max={1}
            step={0.01}
            value={volume}
            onChange={handleVolumeChange}
            tooltip={{ formatter: (v) => `${Math.round((v || 0) * 100)}%` }}
            style={{ margin: 0 }}
          />
        </div>
        <Tooltip title="关闭当前试听">
          <Button
            type="text"
            shape="circle"
            size="small"
            icon={<CloseOutlined style={{ color: '#94a3b8' }} />}
            onClick={() => {
              if (audioRef.current) audioRef.current.pause();
              onClose();
            }}
          />
        </Tooltip>
      </div>
    </div>
  );
};
