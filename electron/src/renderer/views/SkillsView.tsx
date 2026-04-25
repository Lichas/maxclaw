import React, { useEffect, useState, useCallback } from 'react';
import { useTranslation } from '../i18n';

interface Skill {
  name: string;
  displayName: string;
  description?: string;
  icon?: string;
  enabled: boolean;
  installedAt?: string;
}

type InstallType = 'zip' | 'folder' | 'github' | 'clawhub';

function getSkillInitial(skill: Skill) {
  const seed = (skill.displayName || skill.name).trim();
  const letter = seed.charAt(0).toUpperCase();
  return /[A-Z0-9]/.test(letter) ? letter : 'S';
}

interface RecommendedSkillSource {
  id: string;
  name: string;
  type: InstallType;
  description: string;
  source?: string;
  browseUrl?: string;
  example?: string;
}

const DEFAULT_RECOMMENDED_SOURCES: RecommendedSkillSource[] = [
  {
    id: 'anthropics-official',
    name: 'Anthropics (Official)',
    type: 'github',
    description: 'Anthropic 官方技能库',
    source: 'https://github.com/anthropics/skills/tree/main/skills',
    browseUrl: 'https://github.com/anthropics/skills/tree/main/skills',
  },
  {
    id: 'playwright-cli',
    name: 'Playwright CLI',
    type: 'github',
    description: 'Microsoft Playwright 自动化测试技能',
    source: 'https://github.com/microsoft/playwright-cli/tree/main/skills',
    browseUrl: 'https://github.com/microsoft/playwright-cli/tree/main/skills',
  },
  {
    id: 'vercel-labs',
    name: 'Vercel Labs',
    type: 'github',
    description: 'Vercel Labs 技能库',
    source: 'https://github.com/vercel-labs/agent-skills/tree/main/skills',
    browseUrl: 'https://github.com/vercel-labs/agent-skills/tree/main/skills',
  },
  {
    id: 'vercel-skills',
    name: 'Vercel Skills',
    type: 'github',
    description: 'Vercel 官方技能',
    source: 'https://github.com/vercel-labs/skills/tree/main/skills',
    browseUrl: 'https://github.com/vercel-labs/skills/tree/main/skills',
  },
  {
    id: 'remotion',
    name: 'Remotion',
    type: 'github',
    description: 'Remotion 视频编辑技能',
    source: 'https://github.com/remotion-dev/skills/tree/main/skills',
    browseUrl: 'https://github.com/remotion-dev/skills/tree/main/skills',
  },
  {
    id: 'superpowers',
    name: 'Superpowers',
    type: 'github',
    description: 'Superpowers 增强技能',
    source: 'https://github.com/obra/superpowers/tree/main/skills',
    browseUrl: 'https://github.com/obra/superpowers/tree/main/skills',
  },
  {
    id: 'clawhub',
    name: 'ClawHub Registry',
    type: 'clawhub',
    description: 'ClawHub 公共技能市场',
    browseUrl: 'https://clawhub.ai/skills',
    example: 'clawhub://gifgrep',
  },
];

export function SkillsView() {
  const { t } = useTranslation();
  const [skills, setSkills] = useState<Skill[]>([]);
  const [recommendedSources, setRecommendedSources] = useState<
    RecommendedSkillSource[]
  >(DEFAULT_RECOMMENDED_SOURCES);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [installModalOpen, setInstallModalOpen] = useState(false);
  const [installType, setInstallType] = useState<InstallType>('github');
  const [installUrl, setInstallUrl] = useState('');
  const [selectedRecommend, setSelectedRecommend] = useState<string>('');
  const [useCustomUrl, setUseCustomUrl] = useState(false);
  const [nameFilter, setNameFilter] = useState('');

  const fetchSkills = useCallback(async () => {
    try {
      setLoading(true);
      const response = await fetch('http://127.0.0.1:18890/api/skills');
      if (!response.ok) throw new Error('Failed to fetch skills');
      const data = await response.json();
      setSkills(data.skills || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load skills');
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchRecommendedSources = useCallback(async () => {
    try {
      const response = await fetch('http://127.0.0.1:18890/api/skills/sources');
      if (!response.ok) throw new Error('Failed to fetch skill sources');
      const data = await response.json();
      if (Array.isArray(data.sources) && data.sources.length > 0) {
        setRecommendedSources(data.sources as RecommendedSkillSource[]);
      }
    } catch {
      setRecommendedSources(DEFAULT_RECOMMENDED_SOURCES);
    }
  }, []);

  useEffect(() => {
    void fetchSkills();
    void fetchRecommendedSources();
    // Only fetch on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggleSkill = async (name: string, enabled: boolean) => {
    try {
      const response = await fetch(
        `http://127.0.0.1:18890/api/skills/${name}/${enabled ? 'disable' : 'enable'}`,
        {
          method: 'POST',
        },
      );
      if (!response.ok) throw new Error('Failed to toggle skill');
      void fetchSkills();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.error'));
    }
  };

  const handleInstall = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const response = await fetch(
        'http://127.0.0.1:18890/api/skills/install',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            type: installType,
            source: installUrl,
          }),
        },
      );
      if (!response.ok) throw new Error('Failed to install skill');
      setInstallModalOpen(false);
      setInstallUrl('');
      setSelectedRecommend('');
      setUseCustomUrl(false);
      void fetchSkills();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.error'));
    }
  };

  const visibleRecommendedSources = recommendedSources.filter(
    (source) => source.type === installType,
  );
  const selectedSource =
    visibleRecommendedSources.find(
      (source) => source.id === selectedRecommend,
    ) || null;
  const installInputReadOnly = Boolean(selectedSource?.source) && !useCustomUrl;

  const handleRecommendChange = (value: string) => {
    if (value === 'custom') {
      setUseCustomUrl(true);
      setSelectedRecommend('');
      setInstallUrl('');
    } else {
      const next = recommendedSources.find((source) => source.id === value);
      if (!next) return;
      const nextValue = next.source || next.example || '';
      setInstallType(next.type);
      setUseCustomUrl(false);
      setSelectedRecommend(next.id);
      setInstallUrl(nextValue);
    }
  };

  const getInstallPlaceholder = () => {
    switch (installType) {
      case 'github':
        return t('skills.install.placeholder.github');
      case 'clawhub':
        return t('skills.install.placeholder.clawhub');
      case 'zip':
        return t('skills.install.placeholder.zip');
      case 'folder':
        return t('skills.install.placeholder.folder');
      default:
        return '';
    }
  };

  const filteredSkills = skills.filter((skill) => {
    const query = nameFilter.trim().toLowerCase();
    if (!query) {
      return true;
    }
    return [skill.name, skill.displayName]
      .join(' ')
      .toLowerCase()
      .includes(query);
  });

  return (
    <div className="relative isolate h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-5xl">
        <div className="relative z-20 mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-foreground">
              {t('skills.title')}
            </h1>
            <p className="mt-1 text-sm text-muted">
              {t('skills.subtitle')}
            </p>
          </div>
          <div className="relative z-20 no-drag">
            <button
              type="button"
              onClick={() => setInstallModalOpen(true)}
              className="inline-flex h-11 items-center gap-2 rounded-lg bg-primary px-5 text-sm font-semibold text-primary-foreground transition-transform duration-150 hover:-translate-y-0.5 hover:bg-primary/90"
            >
              <span className="text-base leading-none">+</span>
              <span>{t('skills.install')}</span>
            </button>
          </div>
        </div>

        {error && (
          <div className="mb-4 rounded-lg border border-danger/25 bg-danger-bg px-4 py-3 text-sm text-danger">
            {error}
          </div>
        )}

        {installModalOpen && (
          <div className="mb-6 rounded-xl border border-border bg-background p-5 shadow-sm">
            <h3 className="mb-4 text-base font-semibold">
              {t('skills.install.title')}
            </h3>
            <form onSubmit={handleInstall} className="space-y-4">
              <div className="flex gap-4">
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="github"
                    checked={installType === 'github'}
                    onChange={() => {
                      setInstallType('github');
                      setInstallUrl('');
                    }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.github')}</span>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="zip"
                    checked={installType === 'zip'}
                    onChange={() => {
                      setInstallType('zip');
                      setInstallUrl('');
                      setSelectedRecommend('');
                      setUseCustomUrl(false);
                    }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.zip')}</span>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="clawhub"
                    checked={installType === 'clawhub'}
                    onChange={() => {
                      setInstallType('clawhub');
                      setInstallUrl('');
                      setSelectedRecommend('');
                      setUseCustomUrl(false);
                    }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.clawhub')}</span>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="folder"
                    checked={installType === 'folder'}
                    onChange={() => {
                      setInstallType('folder');
                      setInstallUrl('');
                      setSelectedRecommend('');
                      setUseCustomUrl(false);
                    }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.folder')}</span>
                </label>
              </div>

              {(installType === 'github' || installType === 'clawhub') && (
                <div className="space-y-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-muted">
                      {installType === 'clawhub' ? '推荐市场' : '推荐技能源'}
                    </label>
                    <select
                      value={selectedRecommend || (useCustomUrl ? 'custom' : '')}
                      onChange={(e) => handleRecommendChange(e.target.value)}
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary/40 focus:outline-none"
                    >
                      <option value="">
                        {installType === 'clawhub'
                          ? '选择推荐市场...'
                          : '选择推荐技能源...'}
                      </option>
                      {visibleRecommendedSources.map((skill) => (
                        <option key={skill.id} value={skill.id}>
                          {skill.name} - {skill.description}
                        </option>
                      ))}
                      <option value="custom">自定义 URL...</option>
                    </select>
                  </div>

                  {(useCustomUrl || selectedRecommend) && (
                    <div>
                      <label className="mb-1.5 block text-xs font-medium text-muted">
                        {installType === 'clawhub'
                          ? 'ClawHub slug / URL'
                          : 'GitHub URL'}
                      </label>
                      <input
                        type="text"
                        value={installUrl}
                        onChange={(e) => setInstallUrl(e.target.value)}
                        placeholder={getInstallPlaceholder()}
                        readOnly={installInputReadOnly}
                        className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted focus:border-primary/40 focus:outline-none disabled:bg-secondary/50"
                        required
                      />
                      {!useCustomUrl && selectedRecommend && (
                        <p className="mt-1 text-xs text-muted">
                          已选择推荐源，如需修改请切换到"自定义 URL"
                        </p>
                      )}
                      {selectedSource?.browseUrl && (
                        <p className="mt-1 text-xs text-muted">
                          Browse:{' '}
                          <a
                            href={selectedSource.browseUrl}
                            className="underline"
                            target="_blank"
                            rel="noreferrer"
                          >
                            {selectedSource.browseUrl}
                          </a>
                        </p>
                      )}
                    </div>
                  )}
                </div>
              )}

              {installType === 'zip' && (
                <div className="space-y-2">
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={installUrl}
                      onChange={(e) => setInstallUrl(e.target.value)}
                      placeholder={getInstallPlaceholder()}
                      className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted focus:border-primary/40 focus:outline-none"
                      required
                    />
                    <button
                      type="button"
                      onClick={async () => {
                        const result =
                          await window.electronAPI.system.selectFile([
                            { name: 'ZIP files', extensions: ['zip'] },
                          ]);
                        if (result) setInstallUrl(result);
                      }}
                      className="flex items-center gap-1.5 rounded-lg border border-border bg-background px-3 py-2 text-sm font-medium text-foreground hover:bg-secondary"
                    >
                      <FolderOpenIcon className="h-4 w-4" />
                      Open
                    </button>
                  </div>
                  <p className="text-xs text-muted">
                    {t('skills.install.zip.help') ||
                      '选择 .zip 技能包文件，或输入完整路径'}
                  </p>
                </div>
              )}

              {installType === 'folder' && (
                <div className="space-y-2">
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={installUrl}
                      onChange={(e) => setInstallUrl(e.target.value)}
                      placeholder={getInstallPlaceholder()}
                      className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted focus:border-primary/40 focus:outline-none"
                      required
                    />
                    <button
                      type="button"
                      onClick={async () => {
                        const result =
                          await window.electronAPI.system.selectFolder();
                        if (result) setInstallUrl(result);
                      }}
                      className="flex items-center gap-1.5 rounded-lg border border-border bg-background px-3 py-2 text-sm font-medium text-foreground hover:bg-secondary"
                    >
                      <FolderOpenIcon className="h-4 w-4" />
                      Open
                    </button>
                  </div>
                  <p className="text-xs text-muted">
                    {t('skills.install.folder.help') ||
                      '选择技能文件夹，或输入完整路径'}
                  </p>
                </div>
              )}

              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setInstallModalOpen(false)}
                  className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary"
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
                >
                  {t('common.confirm')}
                </button>
              </div>
            </form>
          </div>
        )}

        <div className="mb-4">
          <input
            type="text"
            value={nameFilter}
            onChange={(event) => setNameFilter(event.target.value)}
            placeholder="按名称过滤已安装技能"
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted focus:border-primary/40 focus:outline-none"
          />
        </div>

        {loading && skills.length === 0 ? (
          <div className="py-12 text-center text-muted">
            {t('common.loading')}
          </div>
        ) : skills.length === 0 ? (
          <div className="py-12 text-center">
            <p className="text-muted">{t('skills.empty')}</p>
            <p className="mt-1 text-sm text-muted">
              {t('skills.empty.hint')}
            </p>
          </div>
        ) : filteredSkills.length === 0 ? (
          <div className="py-12 text-center text-muted">
            没有匹配的已安装技能
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {filteredSkills.map((skill) => (
              <div
                key={skill.name}
                className={`rounded-xl border bg-background p-5 shadow-sm transition-all ${
                  skill.enabled
                    ? 'border-border'
                    : 'border-border/50 opacity-60'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <span className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-secondary text-sm font-semibold uppercase tracking-[0.08em] text-foreground">
                      {getSkillInitial(skill)}
                    </span>
                    <div>
                      <h3 className="font-semibold text-foreground">
                        {skill.displayName}
                      </h3>
                      <p className="text-xs text-muted">{skill.name}</p>
                    </div>
                  </div>
                  <button
                    onClick={() => void toggleSkill(skill.name, skill.enabled)}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                      skill.enabled ? 'bg-primary' : 'bg-secondary'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-background transition-transform ${
                        skill.enabled ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>

                {skill.description && (
                  <div className="group relative mt-3">
                    <p className="cursor-help text-sm text-muted line-clamp-2">
                      {skill.description}
                    </p>
                    <div className="pointer-events-none absolute left-0 top-full z-30 mt-2 w-full min-w-[16rem] translate-y-1 rounded-xl border border-border bg-secondary p-3 text-xs leading-5 text-foreground opacity-0 transition-all duration-200 group-hover:translate-y-0 group-hover:opacity-100">
                      {skill.description}
                    </div>
                  </div>
                )}

                {skill.installedAt && (
                  <p className="mt-3 text-xs text-muted">
                    {new Date(skill.installedAt).toLocaleDateString()}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// Icon component for file/folder picker
function FolderOpenIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z"
      />
    </svg>
  );
}
