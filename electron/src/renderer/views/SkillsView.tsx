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

export function SkillsView() {
  const { t } = useTranslation();
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [installModalOpen, setInstallModalOpen] = useState(false);
  const [installType, setInstallType] = useState<'zip' | 'folder' | 'github'>('github');
  const [installUrl, setInstallUrl] = useState('');

  const fetchSkills = useCallback(async () => {
    try {
      setLoading(true);
      const response = await fetch('http://localhost:18890/api/skills');
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

  useEffect(() => {
    void fetchSkills();
    // Only fetch on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggleSkill = async (name: string, enabled: boolean) => {
    try {
      const response = await fetch(`http://localhost:18890/api/skills/${name}/${enabled ? 'disable' : 'enable'}`, {
        method: 'POST'
      });
      if (!response.ok) throw new Error('Failed to toggle skill');
      void fetchSkills();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.error'));
    }
  };

  const handleInstall = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const response = await fetch('http://localhost:18890/api/skills/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: installType,
          source: installUrl
        })
      });
      if (!response.ok) throw new Error('Failed to install skill');
      setInstallModalOpen(false);
      setInstallUrl('');
      void fetchSkills();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.error'));
    }
  };

  const getSkillIcon = (skill: Skill) => {
    if (skill.icon) return skill.icon;
    if (skill.name.includes('docx') || skill.name.includes('document')) return '📄';
    if (skill.name.includes('xlsx') || skill.name.includes('excel') || skill.name.includes('sheet')) return '📊';
    if (skill.name.includes('pptx') || skill.name.includes('slide')) return '📽️';
    if (skill.name.includes('pdf')) return '📑';
    if (skill.name.includes('web') || skill.name.includes('search')) return '🌐';
    if (skill.name.includes('image') || skill.name.includes('img')) return '🖼️';
    if (skill.name.includes('cron') || skill.name.includes('schedule')) return '⏰';
    return '🦞';
  };

  const getInstallPlaceholder = () => {
    switch (installType) {
      case 'github': return t('skills.install.placeholder.github');
      case 'zip': return t('skills.install.placeholder.zip');
      case 'folder': return t('skills.install.placeholder.folder');
      default: return '';
    }
  };

  return (
    <div className="h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-5xl">
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-foreground">{t('skills.title')}</h1>
            <p className="mt-1 text-sm text-foreground/55">{t('skills.subtitle')}</p>
          </div>
          <button
            onClick={() => setInstallModalOpen(true)}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            + {t('skills.install')}
          </button>
        </div>

        {error && (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
            {error}
          </div>
        )}

        {installModalOpen && (
          <div className="mb-6 rounded-xl border border-border bg-background p-5 shadow-sm">
            <h3 className="mb-4 text-base font-semibold">{t('skills.install.title')}</h3>
            <form onSubmit={handleInstall} className="space-y-4">
              <div className="flex gap-4">
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="github"
                    checked={installType === 'github'}
                    onChange={() => { setInstallType('github'); setInstallUrl(''); }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.github')}</span>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="zip"
                    checked={installType === 'zip'}
                    onChange={() => { setInstallType('zip'); setInstallUrl(''); }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.zip')}</span>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    value="folder"
                    checked={installType === 'folder'}
                    onChange={() => { setInstallType('folder'); setInstallUrl(''); }}
                    className="h-4 w-4 text-primary"
                  />
                  <span className="text-sm">{t('skills.install.folder')}</span>
                </label>
              </div>

              <div>
                <input
                  type="text"
                  value={installUrl}
                  onChange={(e) => setInstallUrl(e.target.value)}
                  placeholder={getInstallPlaceholder()}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                  required
                />
              </div>

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

        {loading && skills.length === 0 ? (
          <div className="py-12 text-center text-foreground/50">{t('common.loading')}</div>
        ) : skills.length === 0 ? (
          <div className="py-12 text-center">
            <p className="text-foreground/50">{t('skills.empty')}</p>
            <p className="mt-1 text-sm text-foreground/40">{t('skills.empty.hint')}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {skills.map((skill) => (
              <div
                key={skill.name}
                className={`rounded-xl border bg-background p-5 shadow-sm transition-all ${
                  skill.enabled ? 'border-border' : 'border-border/50 opacity-60'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-xl">
                      {getSkillIcon(skill)}
                    </span>
                    <div>
                      <h3 className="font-semibold text-foreground">{skill.displayName}</h3>
                      <p className="text-xs text-foreground/50">{skill.name}</p>
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
                    <p className="cursor-help text-sm text-foreground/70 line-clamp-2">{skill.description}</p>
                    <div className="pointer-events-none absolute left-0 top-full z-30 mt-2 w-full min-w-[16rem] translate-y-1 rounded-xl border border-border/80 bg-card/95 p-3 text-xs leading-5 text-foreground shadow-[0_14px_34px_rgba(15,23,42,0.18)] opacity-0 backdrop-blur-sm transition-all duration-200 group-hover:translate-y-0 group-hover:opacity-100">
                      {skill.description}
                    </div>
                  </div>
                )}

                {skill.installedAt && (
                  <p className="mt-3 text-xs text-foreground/40">
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
