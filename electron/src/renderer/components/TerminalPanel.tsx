import React, { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

interface TerminalPanelProps {
  sessionKey: string;
}

const LIGHT_TERMINAL_THEME = {
  background: '#ffffff',
  foreground: '#1f2937',
  cursor: '#4b5563',
  selectionBackground: 'rgba(107, 144, 128, 0.22)',
  black: '#1f2937',
  brightBlack: '#4b5563',
  red: '#dc2626',
  brightRed: '#ef4444',
  green: '#15803d',
  brightGreen: '#22c55e',
  yellow: '#a16207',
  brightYellow: '#ca8a04',
  blue: '#1d4ed8',
  brightBlue: '#3b82f6',
  magenta: '#9333ea',
  brightMagenta: '#a855f7',
  cyan: '#0f766e',
  brightCyan: '#14b8a6',
  white: '#f3f4f6',
  brightWhite: '#ffffff'
};

const DARK_TERMINAL_THEME = {
  background: '#0f1419',
  foreground: '#d8e1e8',
  cursor: '#84a98c',
  selectionBackground: 'rgba(132, 169, 140, 0.32)',
  black: '#0f1419',
  brightBlack: '#5d6874',
  red: '#f38ba8',
  brightRed: '#f38ba8',
  green: '#a6e3a1',
  brightGreen: '#a6e3a1',
  yellow: '#f9e2af',
  brightYellow: '#f9e2af',
  blue: '#89b4fa',
  brightBlue: '#89b4fa',
  magenta: '#cba6f7',
  brightMagenta: '#cba6f7',
  cyan: '#94e2d5',
  brightCyan: '#94e2d5',
  white: '#cdd6f4',
  brightWhite: '#f5f7fa'
};

function resolveTerminalTheme() {
  return document.documentElement.classList.contains('dark') ? DARK_TERMINAL_THEME : LIGHT_TERMINAL_THEME;
}

export function TerminalPanel({ sessionKey }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const [running, setRunning] = useState(false);
  const [starting, setStarting] = useState(true);
  const [shellLabel, setShellLabel] = useState('');

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const terminal = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontFamily:
        'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 12,
      lineHeight: 1.35,
      theme: resolveTerminalTheme()
    });

    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();

    terminalRef.current = terminal;

    const applyTheme = () => {
      terminal.options.theme = resolveTerminalTheme();
    };
    applyTheme();

    const themeObserver = new MutationObserver(() => applyTheme());
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });

    const resizeToContainer = () => {
      fitAddon.fit();
      if (terminal.cols > 0 && terminal.rows > 0) {
        void window.electronAPI.terminal.resize(sessionKey, terminal.cols, terminal.rows);
      }
    };

    const onResize = () => resizeToContainer();
    window.addEventListener('resize', onResize);
    const resizeObserver = new ResizeObserver(() => resizeToContainer());
    resizeObserver.observe(containerRef.current);

    const unsubscribeData = window.electronAPI.terminal.onData((payload) => {
      if (payload.sessionKey !== sessionKey) {
        return;
      }
      terminal.write(payload.chunk);
    });

    const unsubscribeExit = window.electronAPI.terminal.onExit((payload) => {
      if (payload.sessionKey !== sessionKey) {
        return;
      }
      setRunning(false);
      const suffix = payload.signal ? ` signal=${payload.signal}` : '';
      terminal.write(`\r\n[terminal exited code=${payload.code ?? 'null'}${suffix}]\r\n`);
    });

    const inputDisposable = terminal.onData((data) => {
      void window.electronAPI.terminal.input(sessionKey, data);
    });

    setStarting(true);
    setRunning(false);
    setShellLabel('');
    void window.electronAPI.terminal
      .start(sessionKey, { cols: terminal.cols || 120, rows: terminal.rows || 28 })
      .then((result) => {
        if (!result.success) {
          terminal.write(`\r\n[terminal start failed: ${result.error || 'unknown error'}]\r\n`);
          setRunning(false);
          return;
        }

        setRunning(true);
        setShellLabel(result.shell || '');
        resizeToContainer();
      })
      .finally(() => {
        setStarting(false);
      });

    return () => {
      unsubscribeData();
      unsubscribeExit();
      inputDisposable.dispose();
      themeObserver.disconnect();
      resizeObserver.disconnect();
      window.removeEventListener('resize', onResize);
      terminal.dispose();
      terminalRef.current = null;
    };
  }, [sessionKey]);

  const handleClear = () => {
    terminalRef.current?.clear();
  };

  const handleStop = async () => {
    await window.electronAPI.terminal.stop(sessionKey);
    setRunning(false);
  };

  return (
    <div className="mt-3 overflow-hidden rounded-xl border border-border/70 bg-card shadow-[0_8px_24px_rgba(15,23,42,0.06)]">
      <div className="flex items-center justify-between border-b border-border/50 px-3 py-2">
        <div className="flex items-center gap-2 text-xs text-foreground/80">
          <TerminalPanelIcon className="h-3.5 w-3.5" />
          <span className="font-medium">Terminal</span>
          <span className={`inline-block h-1.5 w-1.5 rounded-full ${running ? 'bg-emerald-500' : 'bg-foreground/40'}`} />
          {starting && <span className="text-foreground/45">starting...</span>}
          {shellLabel && <span className="text-foreground/45">{shellLabel}</span>}
        </div>
        <div className="flex items-center gap-2 text-xs">
          <button
            type="button"
            onClick={handleClear}
            className="rounded px-2 py-1 text-foreground/60 hover:bg-secondary hover:text-foreground"
          >
            Clear
          </button>
          <button
            type="button"
            onClick={handleStop}
            className="rounded px-2 py-1 text-foreground/60 hover:bg-secondary hover:text-foreground"
          >
            Stop
          </button>
        </div>
      </div>
      <div ref={containerRef} className="h-52 px-2 py-1" />
    </div>
  );
}

function TerminalPanelIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={3} y={5} width={18} height={14} rx={2.5} strokeWidth={1.8} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M7 9l3 3-3 3m5 0h5" />
    </svg>
  );
}
