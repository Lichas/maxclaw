import React from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setActiveTab } from '../store';

const menuItems = [
  { id: 'chat', label: 'Chat', icon: ChatIcon },
  { id: 'sessions', label: 'History', icon: HistoryIcon },
  { id: 'scheduled', label: 'Scheduled', icon: ClockIcon },
  { id: 'skills', label: 'Skills', icon: PuzzleIcon },
  { id: 'settings', label: 'Settings', icon: SettingsIcon },
] as const;

export function Sidebar() {
  const dispatch = useDispatch();
  const { activeTab, sidebarCollapsed } = useSelector((state: RootState) => state.ui);
  const { status } = useSelector((state: RootState) => state.gateway);

  return (
    <aside
      className={`bg-secondary border-r border-border flex flex-col transition-all duration-200 ${
        sidebarCollapsed ? 'w-16' : 'w-64'
      }`}
    >
      {/* New Chat Button */}
      <div className="p-3">
        <button
          onClick={() => dispatch(setActiveTab('chat'))}
          className="w-full flex items-center justify-center gap-2 bg-primary text-primary-foreground rounded-lg py-2.5 px-4 hover:bg-primary/90 transition-colors"
        >
          <PlusIcon className="w-5 h-5" />
          {!sidebarCollapsed && <span className="font-medium">New Chat</span>}
        </button>
      </div>

      {/* Menu Items */}
      <nav className="flex-1 px-2">
        {menuItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeTab === item.id;

          return (
            <button
              key={item.id}
              onClick={() => dispatch(setActiveTab(item.id))}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg mb-1 transition-colors ${
                isActive
                  ? 'bg-background text-foreground'
                  : 'text-foreground/60 hover:bg-background/50 hover:text-foreground'
              }`}
            >
              <Icon className="w-5 h-5 flex-shrink-0" />
              {!sidebarCollapsed && <span className="font-medium">{item.label}</span>}
            </button>
          );
        })}
      </nav>

      {/* Gateway Status */}
      <div className="p-3 border-t border-border">
        <div className={`flex items-center gap-2 px-3 py-2 rounded-lg ${sidebarCollapsed ? 'justify-center' : ''}`}>
          <div
            className={`w-2 h-2 rounded-full flex-shrink-0 ${
              status === 'running'
                ? 'bg-green-500'
                : status === 'error'
                ? 'bg-red-500'
                : 'bg-yellow-500'
            }`}
          />
          {!sidebarCollapsed && (
            <span className="text-sm text-foreground/60 capitalize">{status}</span>
          )}
        </div>
      </div>
    </aside>
  );
}

// Icon components
function ChatIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  );
}

function HistoryIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function ClockIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function PuzzleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 4a2 2 0 114 0v1a1 1 0 001 1h3a1 1 0 011 1v3a1 1 0 01-1 1h-1a2 2 0 100 4h1a1 1 0 011 1v3a1 1 0 01-1 1h-3a1 1 0 01-1-1v-1a2 2 0 10-4 0v1a1 1 0 01-1 1H7a1 1 0 01-1-1v-3a1 1 0 00-1-1H4a2 2 0 110-4h1a1 1 0 001-1V7a1 1 0 011-1h3a1 1 0 001-1V4z" />
    </svg>
  );
}

function SettingsIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

function PlusIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
    </svg>
  );
}
