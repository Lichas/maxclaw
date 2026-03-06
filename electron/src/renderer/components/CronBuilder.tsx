import React, { useState, useEffect, useMemo } from 'react';
import './CronBuilder.css';
import { useTranslation } from '../i18n';

type CronPreset = 'custom' | 'minutely' | 'hourly' | 'daily' | 'weekly' | 'monthly';

interface CronBuilderProps {
  value: string;
  onChange: (value: string) => void;
}

interface CronParts {
  minute: string;
  hour: string;
  dayOfMonth: string;
  month: string;
  dayOfWeek: string;
}

const PRESETS: Record<Exclude<CronPreset, 'custom'>, string> = {
  minutely: '* * * * *',
  hourly: '0 * * * *',
  daily: '0 9 * * *',
  weekly: '0 9 * * 1',
  monthly: '0 9 1 * *',
};

export function CronBuilder({ value, onChange }: CronBuilderProps) {
  const { t } = useTranslation();
  const [preset, setPreset] = useState<CronPreset>('custom');
  const [parts, setParts] = useState<CronParts>({
    minute: '0',
    hour: '9',
    dayOfMonth: '*',
    month: '*',
    dayOfWeek: '*',
  });
  const [rawCron, setRawCron] = useState(value);
  const weekdays = useMemo(() => ([
    { value: '1', label: t('scheduled.cron.weekday.mon') },
    { value: '2', label: t('scheduled.cron.weekday.tue') },
    { value: '3', label: t('scheduled.cron.weekday.wed') },
    { value: '4', label: t('scheduled.cron.weekday.thu') },
    { value: '5', label: t('scheduled.cron.weekday.fri') },
    { value: '6', label: t('scheduled.cron.weekday.sat') },
    { value: '0', label: t('scheduled.cron.weekday.sun') },
  ]), [t]);

  useEffect(() => {
    const normalized = (value || '').trim();
    setRawCron(normalized);
    const parsed = parseCron(normalized);
    if (parsed) {
      setParts(parsed);
      detectPreset(parsed);
    } else {
      setPreset('custom');
    }
  }, [value]);

  const parseCron = (cron: string): CronParts | null => {
    const cronParts = cron.trim().split(/\s+/).filter(Boolean);
    if (cronParts.length !== 5) return null;
    return {
      minute: cronParts[0],
      hour: cronParts[1],
      dayOfMonth: cronParts[2],
      month: cronParts[3],
      dayOfWeek: cronParts[4],
    };
  };

  const detectPreset = (parts: CronParts) => {
    const cron = `${parts.minute} ${parts.hour} ${parts.dayOfMonth} ${parts.month} ${parts.dayOfWeek}`;
    for (const [presetName, presetCron] of Object.entries(PRESETS)) {
      if (presetCron === cron) {
        setPreset(presetName as CronPreset);
        return;
      }
    }
    setPreset('custom');
  };

  const updatePart = (key: keyof CronParts, value: string) => {
    const newParts = { ...parts, [key]: value };
    setParts(newParts);
    const cron = `${newParts.minute} ${newParts.hour} ${newParts.dayOfMonth} ${newParts.month} ${newParts.dayOfWeek}`;
    setRawCron(cron);
    onChange(cron);
    detectPreset(newParts);
  };

  const handlePresetChange = (newPreset: CronPreset) => {
    setPreset(newPreset);
    if (newPreset === 'custom') {
      return;
    }
    const cron = PRESETS[newPreset];
    setRawCron(cron);
    onChange(cron);
    const parsed = parseCron(cron);
    if (parsed) setParts(parsed);
  };

  const expressionValue = useMemo(() => {
    const trimmed = rawCron.trim();
    if (trimmed.length > 0) return trimmed;
    return `${parts.minute} ${parts.hour} ${parts.dayOfMonth} ${parts.month} ${parts.dayOfWeek}`;
  }, [rawCron, parts]);

  const handleExpressionChange = (newExpression: string) => {
    setRawCron(newExpression);
    onChange(newExpression);
    const parsed = parseCron(newExpression);
    if (!parsed) {
      setPreset('custom');
      return;
    }
    setParts(parsed);
    detectPreset(parsed);
  };

  const getDescription = () => {
    if (parts.minute === '*' && parts.hour === '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
      return t('scheduled.cron.desc.minutely');
    }
    if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
      return t('scheduled.cron.desc.daily').replace('{hour}', parts.hour);
    }
    if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek !== '*') {
      const weekday = weekdays.find((w) => w.value === parts.dayOfWeek)?.label || parts.dayOfWeek;
      return t('scheduled.cron.desc.weekly')
        .replace('{weekday}', weekday)
        .replace('{hour}', parts.hour);
    }
    if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '1' && parts.month === '*' && parts.dayOfWeek === '*') {
      return t('scheduled.cron.desc.monthly').replace('{hour}', parts.hour);
    }
    return t('scheduled.cron.desc.custom')
      .replace('{minute}', parts.minute)
      .replace('{hour}', parts.hour);
  };

  return (
    <div className="cron-builder">
      <div className="cron-presets">
        <label className="cron-label">{t('scheduled.cron.frequency')}</label>
        <div className="preset-buttons">
          {[
            { key: 'minutely', label: t('scheduled.cron.preset.minutely') },
            { key: 'hourly', label: t('scheduled.cron.preset.hourly') },
            { key: 'daily', label: t('scheduled.cron.preset.daily') },
            { key: 'weekly', label: t('scheduled.cron.preset.weekly') },
            { key: 'monthly', label: t('scheduled.cron.preset.monthly') },
            { key: 'custom', label: t('scheduled.cron.preset.custom') },
          ].map(({ key, label }) => (
            <button
              key={key}
              type="button"
              className={`preset-btn ${preset === key ? 'active' : ''}`}
              onClick={() => handlePresetChange(key as CronPreset)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {preset === 'custom' && (
        <div className="cron-custom">
          <div className="cron-row">
            <div className="cron-field">
              <label>{t('scheduled.cron.field.minute')}</label>
              <input
                type="text"
                value={parts.minute}
                onChange={(e) => updatePart('minute', e.target.value)}
                placeholder="0"
              />
            </div>
            <div className="cron-field">
              <label>{t('scheduled.cron.field.hour')}</label>
              <input
                type="text"
                value={parts.hour}
                onChange={(e) => updatePart('hour', e.target.value)}
                placeholder="9"
              />
            </div>
          </div>

          <div className="cron-row">
            <div className="cron-field">
              <label>{t('scheduled.cron.field.dayOfMonth')}</label>
              <input
                type="text"
                value={parts.dayOfMonth}
                onChange={(e) => updatePart('dayOfMonth', e.target.value)}
                placeholder="*"
              />
            </div>
            <div className="cron-field">
              <label>{t('scheduled.cron.field.month')}</label>
              <input
                type="text"
                value={parts.month}
                onChange={(e) => updatePart('month', e.target.value)}
                placeholder="*"
              />
            </div>
          </div>

          <div className="cron-weekday">
            <label>{t('scheduled.cron.field.dayOfWeek')}</label>
            <div className="weekday-buttons">
              {weekdays.map(({ value: dayValue, label }) => (
                <button
                  key={dayValue}
                  type="button"
                  className={`weekday-btn ${parts.dayOfWeek === dayValue ? 'active' : ''}`}
                  onClick={() => updatePart('dayOfWeek', parts.dayOfWeek === dayValue ? '*' : dayValue)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      <div className="cron-preview">
        <label>{t('scheduled.cron.expression')}</label>
        <input
          type="text"
          value={expressionValue}
          onChange={(e) => handleExpressionChange(e.target.value)}
          placeholder={t('scheduled.cron.expression.placeholder')}
          className="cron-expression-input"
        />
      </div>

      <div className="cron-description">
        {getDescription()}
      </div>
    </div>
  );
}
