import React, { useEffect, useMemo, useRef, useState } from 'react';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface CustomSelectProps {
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  triggerClassName?: string;
  menuClassName?: string;
  size?: 'sm' | 'md';
}

function findNextEnabled(options: SelectOption[], start: number, step: 1 | -1): number {
  if (options.length === 0) {
    return -1;
  }

  let cursor = start;
  for (let i = 0; i < options.length; i += 1) {
    cursor = (cursor + step + options.length) % options.length;
    if (!options[cursor]?.disabled) {
      return cursor;
    }
  }
  return -1;
}

export function CustomSelect({
  value,
  options,
  onChange,
  placeholder,
  disabled = false,
  className = '',
  triggerClassName = '',
  menuClassName = '',
  size = 'md'
}: CustomSelectProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const rootRef = useRef<HTMLDivElement>(null);

  const selectedOption = useMemo(() => options.find((option) => option.value === value), [options, value]);

  const triggerSizeClass = size === 'sm' ? 'px-3 py-1.5 text-xs' : 'px-3 py-2.5 text-sm';

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  useEffect(() => {
    if (!open) {
      return;
    }

    const selectedIndex = options.findIndex((option) => option.value === value && !option.disabled);
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : findNextEnabled(options, -1, 1));
  }, [open, options, value]);

  const chooseOption = (option: SelectOption) => {
    if (option.disabled) {
      return;
    }
    onChange(option.value);
    setOpen(false);
  };

  const handleTriggerKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) {
      return;
    }

    if (!open && (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown')) {
      event.preventDefault();
      setOpen(true);
      return;
    }

    if (!open) {
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      setOpen(false);
      return;
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((prev) => findNextEnabled(options, prev, 1));
      return;
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((prev) => findNextEnabled(options, prev, -1));
      return;
    }

    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      if (activeIndex >= 0 && options[activeIndex]) {
        chooseOption(options[activeIndex]);
      }
    }
  };

  return (
    <div ref={rootRef} className={`relative ${className}`}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen((prev) => !prev)}
        onKeyDown={handleTriggerKeyDown}
        className={`w-full rounded-lg border border-border bg-background text-left font-medium text-foreground transition-all duration-150 ${
          triggerSizeClass
        } ${
          open ? 'border-primary/40 ring-2 ring-primary/20' : 'hover:border-primary/35'
        } ${
          disabled ? 'cursor-not-allowed opacity-50' : ''
        } ${triggerClassName}`}
      >
        <span className="flex items-center justify-between gap-3">
          <span className="truncate text-inherit">{selectedOption?.label || placeholder || ''}</span>
          <ChevronDownIcon className={`h-4 w-4 text-foreground/45 transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
        </span>
      </button>

      {open && (
        <div
          className={`absolute left-0 right-0 top-[calc(100%+0.35rem)] z-50 overflow-hidden rounded-xl border border-border bg-card shadow-xl ${menuClassName}`}
        >
          <div className="max-h-60 overflow-y-auto p-1.5">
            {options.map((option, index) => {
              const selected = option.value === value;
              const active = index === activeIndex;
              return (
                <button
                  key={option.value}
                  type="button"
                  disabled={option.disabled}
                  onMouseEnter={() => setActiveIndex(index)}
                  onClick={() => chooseOption(option)}
                  className={`mb-0.5 w-full rounded-lg px-3 py-2 text-left text-sm transition-colors ${
                    option.disabled
                      ? 'cursor-not-allowed text-foreground/35'
                      : selected
                      ? 'bg-primary/15 text-primary'
                      : active
                      ? 'bg-secondary text-foreground'
                      : 'text-foreground/75 hover:bg-secondary hover:text-foreground'
                  }`}
                >
                  {option.label}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  );
}
