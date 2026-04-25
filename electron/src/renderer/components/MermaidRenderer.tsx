import React, { useEffect, useRef, useState } from 'react';
import mermaid from 'mermaid';

// Initialize mermaid once globally
let mermaidInitialized = false;
function initMermaid(theme: 'light' | 'dark') {
  if (!mermaidInitialized) {
    mermaid.initialize({
      theme: theme === 'dark' ? 'dark' : 'default',
      securityLevel: 'strict',
      startOnLoad: false,
    });
    mermaidInitialized = true;
  }
}

interface MermaidRendererProps {
  chart: string;
  theme?: 'light' | 'dark';
}

export const MermaidRenderer = React.memo(function MermaidRenderer({ chart, theme = 'light' }: MermaidRendererProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [svg, setSvg] = useState<string>('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    initMermaid(theme);
  }, []); // Only initialize once

  useEffect(() => {
    const renderChart = async () => {
      if (!chart.trim()) return;

      try {
        // Generate unique ID for this chart
        const id = `mermaid-${Math.random().toString(36).substr(2, 9)}`;
        const { svg } = await mermaid.render(id, chart.trim());
        setSvg(svg);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to render diagram');
        setSvg('');
      }
    };

    void renderChart();
  }, [chart]);

  if (error) {
    return (
      <div className="rounded-lg border border-danger/25 bg-danger-bg p-4">
        <p className="text-sm text-danger font-medium">Diagram Error</p>
        <p className="text-xs text-danger mt-1">{error}</p>
        <pre className="mt-2 text-xs text-danger bg-secondary p-2 rounded overflow-x-auto">
          {chart}
        </pre>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="mermaid-chart my-4 overflow-x-auto rounded-lg border border-border bg-background p-4"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
});
