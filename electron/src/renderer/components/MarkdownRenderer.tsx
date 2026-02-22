import React, { useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { useSelector } from 'react-redux';
import { RootState } from '../store';
import { MermaidRenderer } from './MermaidRenderer';

interface MarkdownRendererProps {
  content: string;
  className?: string;
  onFileLinkClick?: (href: string) => boolean;
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

export function MarkdownRenderer({ content, className = '', onFileLinkClick }: MarkdownRendererProps) {
  const { theme } = useSelector((state: RootState) => state.ui);
  const isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);

  const processedContent = useMemo(() => {
    // Handle mermaid code blocks by preserving them
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
            const language = match ? match[1] : '';

            // Handle mermaid code blocks
            if (!inline && language === 'mermaid') {
              return (
                <MermaidRenderer
                  chart={String(children).replace(/\n$/, '')}
                  theme={isDark ? 'dark' : 'light'}
                />
              );
            }

            if (!inline && language) {
              return (
                <SyntaxHighlighter
                  style={oneDark}
                  language={language}
                  PreTag="div"
                  {...props}
                >
                  {String(children).replace(/\n$/, '')}
                </SyntaxHighlighter>
              );
            }

            return (
              <code className="bg-secondary px-1.5 py-0.5 rounded text-sm font-mono" {...props}>
                {children}
              </code>
            );
          },
          pre({ children }: { children?: React.ReactNode }) {
            return (
              <pre className="bg-secondary rounded-lg p-4 overflow-x-auto my-3">
                {children}
              </pre>
            );
          },
          p({ children }: { children?: React.ReactNode }) {
            return <p className="leading-7 my-2">{children}</p>;
          },
          ul({ children }: { children?: React.ReactNode }) {
            return <ul className="list-disc pl-5 my-2 space-y-1">{children}</ul>;
          },
          ol({ children }: { children?: React.ReactNode }) {
            return <ol className="list-decimal pl-5 my-2 space-y-1">{children}</ol>;
          },
          li({ children }: { children?: React.ReactNode }) {
            return <li className="leading-6">{children}</li>;
          },
          h1({ children }: { children?: React.ReactNode }) {
            return <h1 className="text-xl font-bold my-3">{children}</h1>;
          },
          h2({ children }: { children?: React.ReactNode }) {
            return <h2 className="text-lg font-semibold my-3">{children}</h2>;
          },
          h3({ children }: { children?: React.ReactNode }) {
            return <h3 className="text-base font-semibold my-2">{children}</h3>;
          },
          blockquote({ children }: { children?: React.ReactNode }) {
            return (
              <blockquote className="border-l-4 border-primary/30 pl-4 my-3 italic text-foreground/70">
                {children}
              </blockquote>
            );
          },
          table({ children }: { children?: React.ReactNode }) {
            return (
              <div className="overflow-x-auto my-3">
                <table className="min-w-full border border-border rounded-lg">
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
              <th className="px-4 py-2 text-left text-sm font-semibold border-b border-border">
                {children}
              </th>
            );
          },
          td({ children }: { children?: React.ReactNode }) {
            return (
              <td className="px-4 py-2 text-sm border-b border-border">
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
        {content}
      </ReactMarkdown>
    </div>
  );
}
