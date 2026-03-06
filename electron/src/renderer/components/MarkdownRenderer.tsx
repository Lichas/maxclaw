import React, { useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark, oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { useSelector } from 'react-redux';
import { RootState } from '../store';
import { MermaidRenderer } from './MermaidRenderer';

interface MarkdownRendererProps {
  content: string;
  className?: string;
  onFileLinkClick?: (href: string) => boolean;
}

interface CodeBlockCardProps {
  code: string;
  language: string;
  isDark: boolean;
}

function isPreviewableLocalLink(href: string): boolean {
  if (!href) {
    return false;
  }
  if (/^https?:\/\//i.test(href) || /^mailto:/i.test(href)) {
    return false;
  }
  return true;
}

function formatCodeLanguage(language: string): string {
  if (!language) {
    return 'text';
  }
  if (language === 'plaintext') {
    return 'plain text';
  }
  if (language === 'shell' || language === 'sh' || language === 'bash' || language === 'zsh') {
    return 'shell';
  }
  if (language === 'js') {
    return 'javascript';
  }
  if (language === 'ts') {
    return 'typescript';
  }
  return language;
}

function CodeBlockCard({ code, language, isDark }: CodeBlockCardProps) {
  const [copied, setCopied] = useState(false);
  const normalizedLanguage = formatCodeLanguage(language);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div
      className="my-4 overflow-hidden rounded-2xl border shadow-sm"
      style={{
        background: isDark
          ? 'linear-gradient(180deg, #0f172a 0%, #111827 100%)'
          : 'linear-gradient(180deg, #f8fbfa 0%, #edf3f1 100%)',
        borderColor: isDark ? '#243041' : '#d5e0db'
      }}
    >
      <div
        className="flex items-center justify-between border-b px-4 py-2.5"
        style={{
          backgroundColor: isDark ? 'rgba(15, 23, 42, 0.78)' : 'rgba(244, 248, 246, 0.92)',
          borderColor: isDark ? '#243041' : '#d5e0db'
        }}
      >
        <div className="flex items-center gap-2">
          <span className="h-2.5 w-2.5 rounded-full bg-[#ff6b6b]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#f4b942]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#53c26b]" />
          <span
            className="ml-2 rounded-full px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-[0.14em]"
            style={{
              backgroundColor: isDark ? 'rgba(148, 163, 184, 0.16)' : 'rgba(96, 125, 116, 0.12)',
              color: isDark ? '#cbd5e1' : '#486157'
            }}
          >
            {normalizedLanguage}
          </span>
        </div>
        <button
          type="button"
          onClick={() => void handleCopy()}
          className="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-colors"
          style={{
            borderColor: isDark ? '#334155' : '#c8d6d0',
            color: isDark ? '#e2e8f0' : '#365247',
            backgroundColor: copied
              ? (isDark ? 'rgba(34, 197, 94, 0.16)' : 'rgba(34, 197, 94, 0.1)')
              : (isDark ? 'rgba(15, 23, 42, 0.55)' : 'rgba(255, 255, 255, 0.74)')
          }}
        >
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <div className="overflow-x-auto">
        {language ? (
          <SyntaxHighlighter
            style={isDark ? oneDark : oneLight}
            language={language}
            PreTag="div"
            customStyle={{
              margin: 0,
              padding: '1rem 1.25rem',
              background: 'transparent',
              borderRadius: 0
            }}
          >
            {code}
          </SyntaxHighlighter>
        ) : (
          <pre
            className="m-0 whitespace-pre overflow-x-auto px-5 py-4 text-[13px] leading-6"
            style={{ color: isDark ? '#e5e7eb' : '#1f2937' }}
          >
            <code>{code}</code>
          </pre>
        )}
      </div>
    </div>
  );
}

export function MarkdownRenderer({ content, className = '', onFileLinkClick }: MarkdownRendererProps) {
  const { theme } = useSelector((state: RootState) => state.ui);
  const isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);

  const processedContent = useMemo(() => {
    return content;
  }, [content]);

  return (
    <div className={`prose prose-sm dark:prose-invert max-w-none ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code({ inline, className, children, ...props }: {
            inline?: boolean;
            className?: string;
            children?: React.ReactNode;
          }) {
            const match = /language-(\w+)/.exec(className || '');
            const language = (match ? match[1] : '').toLowerCase();
            const isPlainTextBlock = ['text', 'plain', 'plaintext', 'txt'].includes(language);
            const code = String(children).replace(/\n$/, '');
            const isInlineCode = Boolean(inline) || (!className && !code.includes('\n'));

            if (!isInlineCode && language === 'mermaid') {
              return (
                <MermaidRenderer
                  chart={code}
                  theme={isDark ? 'dark' : 'light'}
                />
              );
            }

            if (!isInlineCode) {
              return (
                <CodeBlockCard
                  code={code}
                  language={isPlainTextBlock ? '' : language}
                  isDark={isDark}
                />
              );
            }

            return (
              <code
                className="rounded-md border border-border/70 bg-secondary/65 px-1.5 py-0.5 font-mono text-sm"
                {...props}
              >
                {children}
              </code>
            );
          },
          pre({ children }: { children?: React.ReactNode }) {
            return <>{children}</>;
          },
          p({ children }: { children?: React.ReactNode }) {
            return <p className="my-2 leading-7">{children}</p>;
          },
          ul({ children }: { children?: React.ReactNode }) {
            return <ul className="my-2 list-disc space-y-1 pl-5">{children}</ul>;
          },
          ol({ children }: { children?: React.ReactNode }) {
            return <ol className="my-2 list-decimal space-y-1 pl-5">{children}</ol>;
          },
          li({ children }: { children?: React.ReactNode }) {
            return <li className="leading-6">{children}</li>;
          },
          h1({ children }: { children?: React.ReactNode }) {
            return <h1 className="my-3 text-xl font-bold">{children}</h1>;
          },
          h2({ children }: { children?: React.ReactNode }) {
            return <h2 className="my-3 text-lg font-semibold">{children}</h2>;
          },
          h3({ children }: { children?: React.ReactNode }) {
            return <h3 className="my-2 text-base font-semibold">{children}</h3>;
          },
          blockquote({ children }: { children?: React.ReactNode }) {
            return (
              <blockquote className="my-3 border-l-4 border-primary/30 pl-4 italic text-foreground/70">
                {children}
              </blockquote>
            );
          },
          table({ children }: { children?: React.ReactNode }) {
            return (
              <div className="my-3 overflow-x-auto">
                <table className="min-w-full rounded-lg border border-border">
                  {children}
                </table>
              </div>
            );
          },
          thead({ children }: { children?: React.ReactNode }) {
            return <thead className="bg-secondary">{children}</thead>;
          },
          th({ children }: { children?: React.ReactNode }) {
            return (
              <th className="border-b border-border px-4 py-2 text-left text-sm font-semibold">
                {children}
              </th>
            );
          },
          td({ children }: { children?: React.ReactNode }) {
            return (
              <td className="border-b border-border px-4 py-2 text-sm">
                {children}
              </td>
            );
          },
          a({ href, children }: { href?: string; children?: React.ReactNode }) {
            const isLocal = Boolean(href && isPreviewableLocalLink(href));
            return (
              <a
                href={href}
                target={isLocal ? undefined : '_blank'}
                rel={isLocal ? undefined : 'noopener noreferrer'}
                className="text-primary hover:underline"
                onClick={(event) => {
                  if (!href || !isLocal || !onFileLinkClick) {
                    return;
                  }
                  const handled = onFileLinkClick(href);
                  if (handled) {
                    event.preventDefault();
                  }
                }}
              >
                {children}
              </a>
            );
          },
          hr() {
            return <hr className="my-4 border-border" />;
          }
        }}
      >
        {processedContent}
      </ReactMarkdown>
    </div>
  );
}
