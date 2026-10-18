import React, { useState } from 'react';

interface CoverAvatarProps {
  src?: string;
  size?: number;
  className?: string;
  style?: React.CSSProperties;
}

export const CoverAvatar: React.FC<CoverAvatarProps> = ({
  src,
  size = 44,
  className = 'song-cover-thumb',
  style
}) => {
  const [hasError, setHasError] = useState(false);

  if (!src || hasError) {
    return (
      <div
        className={className}
        style={{
          width: size,
          height: size,
          minWidth: size,
          minHeight: size,
          borderRadius: 8,
          background: 'linear-gradient(135deg, #e0f2fe, #e0e7ff)',
          color: '#0284c7',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: Math.max(14, Math.floor(size * 0.42)),
          fontWeight: 700,
          userSelect: 'none',
          boxShadow: '0 1px 2px rgba(0,0,0,0.05)',
          ...style
        }}
      >
        ♪
      </div>
    );
  }

  return (
    <img
      src={src}
      alt=""
      className={className}
      style={{
        width: size,
        height: size,
        minWidth: size,
        minHeight: size,
        borderRadius: 8,
        objectFit: 'cover',
        ...style
      }}
      onError={() => setHasError(true)}
    />
  );
};
