export type PreviewKind = 'markdown' | 'text' | 'image' | 'pdf' | 'audio' | 'video' | 'office' | 'binary';

export interface FileReference {
  id: string;
  pathHint: string;
  displayName: string;
  extension: string;
  kind: PreviewKind;
}

const markdownLinkPattern = /\[[^\]]+]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;
const codePathPattern = /`([^`\n]+)`/g;
const barePathPattern = /(?:^|[\s(])([~./\\\w-][^\s`"'<>|]+?\.[A-Za-z0-9]{1,10})(?=$|[\s),.;])/g;

const markdownExtensions = new Set(['.md', '.markdown', '.mdown']);
const textExtensions = new Set([
  '.txt',
  '.log',
  '.csv',
  '.ts',
  '.tsx',
  '.js',
  '.jsx',
  '.mjs',
  '.cjs',
  '.json',
  '.jsonl',
  '.yaml',
  '.yml',
  '.toml',
  '.xml',
  '.html',
  '.css',
  '.scss',
  '.go',
  '.py',
  '.java',
  '.rb',
  '.rs',
  '.c',
  '.cc',
  '.cpp',
  '.h',
  '.hpp',
  '.sh',
  '.zsh',
  '.bash',
  '.sql'
]);
const imageExtensions = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.svg']);
const videoExtensions = new Set(['.mp4', '.webm', '.mov', '.m4v']);
const audioExtensions = new Set(['.mp3', '.wav', '.m4a', '.ogg', '.flac']);
const officeExtensions = new Set(['.docx', '.pptx', '.xlsx']);

function trimToken(raw: string): string {
  let value = (raw || '').trim();
  value = value.replace(/^`+|`+$/g, '');
  value = value.replace(/^['"]+|['"]+$/g, '');
  value = value.replace(/[),.;]+$/g, '');
  return value.trim();
}

function maybeDecodeURI(input: string): string {
  try {
    return decodeURIComponent(input);
  } catch {
    return input;
  }
}

function isLikelyWebURL(input: string): boolean {
  return /^https?:\/\//i.test(input) || /^mailto:/i.test(input);
}

function extensionFromPath(pathHint: string): string {
  const normalized = pathHint.split('?')[0].split('#')[0];
  const slashIndex = Math.max(normalized.lastIndexOf('/'), normalized.lastIndexOf('\\'));
  const base = slashIndex >= 0 ? normalized.slice(slashIndex + 1) : normalized;
  const dot = base.lastIndexOf('.');
  if (dot <= 0 || dot === base.length - 1) {
    return '';
  }
  return base.slice(dot).toLowerCase();
}

function displayNameFromPath(pathHint: string): string {
  const normalized = pathHint.split('?')[0].split('#')[0];
  const slashIndex = Math.max(normalized.lastIndexOf('/'), normalized.lastIndexOf('\\'));
  if (slashIndex >= 0 && slashIndex < normalized.length - 1) {
    return normalized.slice(slashIndex + 1);
  }
  return normalized;
}

function detectPreviewKind(extension: string): PreviewKind {
  if (markdownExtensions.has(extension)) {
    return 'markdown';
  }
  if (textExtensions.has(extension)) {
    return 'text';
  }
  if (imageExtensions.has(extension)) {
    return 'image';
  }
  if (videoExtensions.has(extension)) {
    return 'video';
  }
  if (audioExtensions.has(extension)) {
    return 'audio';
  }
  if (extension === '.pdf') {
    return 'pdf';
  }
  if (officeExtensions.has(extension)) {
    return 'office';
  }
  return 'binary';
}

function isLocalCandidate(input: string): boolean {
  if (!input) {
    return false;
  }
  if (isLikelyWebURL(input)) {
    return false;
  }
  if (input.startsWith('file://')) {
    return true;
  }
  if (input.includes('/') || input.includes('\\') || input.startsWith('~')) {
    return true;
  }
  return /^[\w.-]+\.[A-Za-z0-9]{1,10}$/.test(input);
}

function collect(pattern: RegExp, content: string, out: Set<string>): void {
  pattern.lastIndex = 0;
  let matched: RegExpExecArray | null;
  while ((matched = pattern.exec(content)) !== null) {
    const raw = trimToken(matched[1] || '');
    if (!raw) {
      continue;
    }
    out.add(maybeDecodeURI(raw));
  }
}

export function extractFileReferences(content: string): FileReference[] {
  if (!content) {
    return [];
  }

  const candidates = new Set<string>();
  collect(markdownLinkPattern, content, candidates);
  collect(codePathPattern, content, candidates);
  collect(barePathPattern, content, candidates);

  const refs: FileReference[] = [];
  const dedupe = new Set<string>();
  for (const candidate of candidates) {
    if (!isLocalCandidate(candidate)) {
      continue;
    }
    const extension = extensionFromPath(candidate);
    if (!extension) {
      continue;
    }

    const normalized = candidate.toLowerCase();
    if (dedupe.has(normalized)) {
      continue;
    }
    dedupe.add(normalized);

    refs.push({
      id: normalized,
      pathHint: candidate,
      displayName: displayNameFromPath(candidate),
      extension,
      kind: detectPreviewKind(extension)
    });
  }

  return refs;
}

