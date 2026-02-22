import React, { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export function TerminalPanel() {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
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
      theme: {
        background: '#0f1419',
        foreground: '#d8e1e8',
        cursor: '#84a98c',
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
      }
    });

    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();

    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;

    const resizeToContainer = () => {
      fitAddon.fit();
      if (terminal.cols > 0 && terminal.rows > 0) {
        void window.electronAPI.terminal.resize(terminal.cols, terminal.rows);
      }
    };

    const onResize = () => resizeToContainer();
    window.addEventListener('resize', onResize);
    const resizeObserver = new ResizeObserver(() => resizeToContainer());
    resizeObserver.observe(containerRef.current);

    const unsubscribeData = window.electronAPI.terminal.onData((chunk) => {
      terminal.write(chunk);
    });

    const unsubscribeExit = window.electronAPI.terminal.onExit((code, signal) => {
      setRunning(false);
      const suffix = signal ? ` signal=${signal}` : '';
      terminal.write(`\r\n[terminal exited code=${code ?? 'null'}${suffix}]\r\n`);
    });

    const inputDisposable = terminal.onData((data) => {
      void window.electronAPI.terminal.input(data);
    });

    setStarting(true);
    void window.electronAPI.terminal
      .start({ cols: terminal.cols || 120, rows: terminal.rows || 28 })
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
      resizeObserver.disconnect();
      window.removeEventListener('resize', onResize);
      void window.electronAPI.terminal.stop();
      terminal.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
    };
  }, []);

  const handleClear = () => {
    terminalRef.current?.clear();
  };

  const handleStop = async () => {
    await window.electronAPI.terminal.stop();
    setRunning(false);
  };

  return (
    <div className="mt-3 rounded-xl border border-border/70 bg-[#0f1419]">
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
