import React, { useState, useEffect } from 'react';

export function TitleBar() {
  const [isMaximized, setIsMaximized] = useState(false);
  const isMac = window.electronAPI.platform.isMac;

  useEffect(() => {
    window.electronAPI.window.isMaximized().then(setIsMaximized);
  }, []);

  const handleMinimize = () => window.electronAPI.window.minimize();
  const handleMaximize = async () => {
    const maximized = await window.electronAPI.window.maximize();
    setIsMaximized(maximized);
  };
  const handleClose = () => window.electronAPI.window.close();

  if (isMac) {
    // macOS: Hidden title bar with traffic lights, show custom title area
    return (
      <div className="h-9 bg-background border-b border-border flex items-center justify-center draggable">
        <span className="text-sm font-medium text-foreground/80">maxclaw</span>
      </div>
    );
  }

  // Windows/Linux: Custom title bar
  return (
    <div className="h-9 bg-background border-b border-border flex items-center justify-between draggable">
      <div className="flex items-center gap-2 px-4">
        <span className="text-sm font-medium text-foreground/80">maxclaw</span>
      </div>
      <div className="flex items-center no-drag">
        <button
          onClick={handleMinimize}
          className="w-12 h-9 flex items-center justify-center hover:bg-secondary transition-colors"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 12H4" />
          </svg>
        </button>
        <button
          onClick={handleMaximize}
          className="w-12 h-9 flex items-center justify-center hover:bg-secondary transition-colors"
        >
          {isMaximized ? (
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          ) : (
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          )}
        </button>
        <button
          onClick={handleClose}
          className="w-12 h-9 flex items-center justify-center hover:bg-red-500 hover:text-white transition-colors"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  );
}
