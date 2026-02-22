import React, { useState, useEffect } from 'react';
import './CronBuilder.css';

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

const PRESETS: Record<CronPreset, string> = {
  custom: '* * * * *',
  minutely: '* * * * *',
  hourly: '0 * * * *',
  daily: '0 9 * * *',
  weekly: '0 9 * * 1',
  monthly: '0 9 1 * *',
};

const WEEKDAYS = [
  { value: '1', label: '周一' },
  { value: '2', label: '周二' },
  { value: '3', label: '周三' },
  { value: '4', label: '周四' },
  { value: '5', label: '周五' },
  { value: '6', label: '周六' },
  { value: '0', label: '周日' },
];

export function CronBuilder({ value, onChange }: CronBuilderProps) {
  const [preset, setPreset] = useState<CronPreset>('custom');
  const [parts, setParts] = useState<CronParts>({
    minute: '0',
    hour: '9',
    dayOfMonth: '*',
    month: '*',
    dayOfWeek: '*',
  });

  useEffect(() => {
    const parsed = parseCron(value);
    if (parsed) {
      setParts(parsed);
      detectPreset(parsed);
    }
  }, []);

  const parseCron = (cron: string): CronParts | null => {
    const parts = cron.split(' ');
    if (parts.length !== 5) return null;
    return {
      minute: parts[0],
      hour: parts[1],
      dayOfMonth: parts[2],
      month: parts[3],
      dayOfWeek: parts[4],
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
    onChange(cron);
    detectPreset(newParts);
  };

  const handlePresetChange = (newPreset: CronPreset) => {
    setPreset(newPreset);
    if (newPreset !== 'custom') {
      const cron = PRESETS[newPreset];
      onChange(cron);
      const parsed = parseCron(cron);
      if (parsed) setParts(parsed);
    }
  };

  return (
    <div className="cron-builder">
      <div className="cron-presets">
        <label className="cron-label">执行频率</label>
        <div className="preset-buttons">
          {[
            { key: 'minutely', label: '每分钟' },
            { key: 'hourly', label: '每小时' },
            { key: 'daily', label: '每天' },
            { key: 'weekly', label: '每周' },
            { key: 'monthly', label: '每月' },
            { key: 'custom', label: '自定义' },
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
              <label>分钟 (0-59)</label>
              <input
                type="text"
                value={parts.minute}
                onChange={(e) => updatePart('minute', e.target.value)}
                placeholder="0"
              />
            </div>
            <div className="cron-field">
              <label>小时 (0-23)</label>
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
              <label>日期 (1-31)</label>
              <input
                type="text"
                value={parts.dayOfMonth}
                onChange={(e) => updatePart('dayOfMonth', e.target.value)}
                placeholder="*"
              />
            </div>
            <div className="cron-field">
              <label>月份 (1-12)</label>
              <input
                type="text"
                value={parts.month}
                onChange={(e) => updatePart('month', e.target.value)}
                placeholder="*"
              />
            </div>
          </div>

          <div className="cron-weekday">
            <label>星期</label>
            <div className="weekday-buttons">
              {WEEKDAYS.map(({ value: dayValue, label }) => (
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
        <label>Cron 表达式</label>
        <code>{`${parts.minute} ${parts.hour} ${parts.dayOfMonth} ${parts.month} ${parts.dayOfWeek}`}</code>
      </div>

      <div className="cron-description">
        {getCronDescription(parts)}
      </div>
    </div>
  );
}

function getCronDescription(parts: CronParts): string {
  if (parts.minute === '*' && parts.hour === '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
    return '每分钟执行一次';
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
    return `每天 ${parts.hour}:00 执行`;
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek !== '*') {
    const weekday = WEEKDAYS.find(w => w.value === parts.dayOfWeek)?.label || parts.dayOfWeek;
    return `每周${weekday.replace('周', '')} ${parts.hour}:00 执行`;
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '1' && parts.month === '*' && parts.dayOfWeek === '*') {
    return `每月 1 日 ${parts.hour}:00 执行`;
  }
  return `在 ${parts.minute} 分 ${parts.hour} 时执行`;
}
