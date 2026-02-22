import { useSelector } from 'react-redux';
import { RootState } from '../store';

export type Language = 'zh' | 'en';

interface Translations {
  [key: string]: string | { [subKey: string]: string };
}

const zh: Translations = {
  // Common
  'common.save': '保存',
  'common.cancel': '取消',
  'common.confirm': '确认',
  'common.delete': '删除',
  'common.edit': '编辑',
  'common.loading': '加载中...',
  'common.error': '出错了',

  // Navigation
  'nav.chat': '聊天',
  'nav.sessions': '搜索任务',
  'nav.scheduled': '定时任务',
  'nav.skills': '技能市场',
  'nav.settings': '设置',

  // Settings
  'settings.title': '设置',
  'settings.appearance': '外观',
  'settings.theme': '主题',
  'settings.theme.light': '浅色',
  'settings.theme.dark': '深色',
  'settings.theme.system': '跟随系统',
  'settings.language': '语言',
  'settings.language.zh': '中文',
  'settings.language.en': 'English',
  'settings.system': '系统',
  'settings.autoLaunch': '开机自启动',
  'settings.minimizeToTray': '最小化到托盘',
  'settings.gateway': 'Gateway',
  'settings.gateway.status': 'Gateway 状态',
  'settings.gateway.restart': '重启 Gateway',
  'settings.gateway.currentModel': '当前模型',
  'settings.gateway.workspace': '工作空间',
  'settings.gateway.notConfigured': '未配置',
  'settings.notifications': '通知',
  'settings.notifications.enable': '启用系统通知',
  'settings.providers': '模型提供商',
  'settings.providers.add': '添加提供商',
  'settings.providers.empty': '暂无配置的提供商',
  'settings.category.general': '通用',
  'settings.category.general.desc': '界面偏好、语言与系统行为',
  'settings.category.providers': '模型配置',
  'settings.category.providers.desc': '管理模型提供商与连接参数',
  'settings.category.channels': '渠道配置',
  'settings.category.channels.desc': '邮箱与 IM Bot 接入设置',
  'settings.category.gateway': 'Gateway',
  'settings.category.gateway.desc': '查看状态并执行重启',
  'settings.email': '邮箱配置',
  'settings.imbot': 'IM Bot 配置',

  // Skills
  'skills.title': '技能市场',
  'skills.subtitle': '管理和安装 AI 技能插件',
  'skills.install': '安装技能',
  'skills.install.title': '安装新技能',
  'skills.install.github': 'GitHub',
  'skills.install.zip': 'ZIP 文件',
  'skills.install.folder': '本地文件夹',
  'skills.install.placeholder.github': 'https://github.com/username/skill-repo',
  'skills.install.placeholder.zip': '/path/to/skill.zip',
  'skills.install.placeholder.folder': '/path/to/skill-folder',
  'skills.empty': '暂无已安装技能',
  'skills.empty.hint': '点击右上角安装新技能',
  'skills.enabled': '已启用',
  'skills.disabled': '已禁用',

  // Sidebar
  'sidebar.newTask': '新建任务',
  'sidebar.history': '历史任务',
  'sidebar.filter.all': '全部',
  'sidebar.filter.desktop': '桌面',
  'sidebar.search': '搜索任务...',
  'sidebar.delete': '删除',
  'sidebar.rename': '重命名',
  'sidebar.confirmDelete': '确定要删除这个任务吗？',

  // Scheduled Tasks
  'scheduled.title': '定时任务',
  'scheduled.add': '添加任务',
  'scheduled.type.cron': 'Cron 表达式',
  'scheduled.type.every': '每隔',
  'scheduled.type.once': '一次性',
  'scheduled.prompt': '提示词',
  'scheduled.workdir': '工作目录',

  // Gateway Status
  'gateway.running': '运行中',
  'gateway.stopped': '已停止',
  'gateway.error': '错误',
  'gateway.starting': '启动中',
};

const en: Translations = {
  // Common
  'common.save': 'Save',
  'common.cancel': 'Cancel',
  'common.confirm': 'Confirm',
  'common.delete': 'Delete',
  'common.edit': 'Edit',
  'common.loading': 'Loading...',
  'common.error': 'Error',

  // Navigation
  'nav.chat': 'Chat',
  'nav.sessions': 'Search',
  'nav.scheduled': 'Scheduled',
  'nav.skills': 'Skills',
  'nav.settings': 'Settings',

  // Settings
  'settings.title': 'Settings',
  'settings.appearance': 'Appearance',
  'settings.theme': 'Theme',
  'settings.theme.light': 'Light',
  'settings.theme.dark': 'Dark',
  'settings.theme.system': 'System',
  'settings.language': 'Language',
  'settings.language.zh': '中文',
  'settings.language.en': 'English',
  'settings.system': 'System',
  'settings.autoLaunch': 'Auto Launch',
  'settings.minimizeToTray': 'Minimize to Tray',
  'settings.gateway': 'Gateway',
  'settings.gateway.status': 'Gateway Status',
  'settings.gateway.restart': 'Restart Gateway',
  'settings.gateway.currentModel': 'Current Model',
  'settings.gateway.workspace': 'Workspace',
  'settings.gateway.notConfigured': 'Not configured',
  'settings.notifications': 'Notifications',
  'settings.notifications.enable': 'Enable System Notifications',
  'settings.providers': 'Model Providers',
  'settings.providers.add': 'Add Provider',
  'settings.providers.empty': 'No providers configured',
  'settings.category.general': 'General',
  'settings.category.general.desc': 'Appearance, language, and system behavior',
  'settings.category.providers': 'Model Config',
  'settings.category.providers.desc': 'Manage providers and connection settings',
  'settings.category.channels': 'Channels',
  'settings.category.channels.desc': 'Email and IM bot integration settings',
  'settings.category.gateway': 'Gateway',
  'settings.category.gateway.desc': 'Status overview and restart controls',
  'settings.email': 'Email Configuration',
  'settings.imbot': 'IM Bot Configuration',

  // Skills
  'skills.title': 'Skills Marketplace',
  'skills.subtitle': 'Manage and install AI skill plugins',
  'skills.install': 'Install Skill',
  'skills.install.title': 'Install New Skill',
  'skills.install.github': 'GitHub',
  'skills.install.zip': 'ZIP File',
  'skills.install.folder': 'Local Folder',
  'skills.install.placeholder.github': 'https://github.com/username/skill-repo',
  'skills.install.placeholder.zip': '/path/to/skill.zip',
  'skills.install.placeholder.folder': '/path/to/skill-folder',
  'skills.empty': 'No skills installed',
  'skills.empty.hint': 'Click the button above to install new skills',
  'skills.enabled': 'Enabled',
  'skills.disabled': 'Disabled',

  // Sidebar
  'sidebar.newTask': 'New Task',
  'sidebar.history': 'History',
  'sidebar.filter.all': 'All',
  'sidebar.filter.desktop': 'Desktop',
  'sidebar.search': 'Search tasks...',
  'sidebar.delete': 'Delete',
  'sidebar.rename': 'Rename',
  'sidebar.confirmDelete': 'Are you sure you want to delete this task?',

  // Scheduled Tasks
  'scheduled.title': 'Scheduled Tasks',
  'scheduled.add': 'Add Task',
  'scheduled.type.cron': 'Cron Expression',
  'scheduled.type.every': 'Every',
  'scheduled.type.once': 'Once',
  'scheduled.prompt': 'Prompt',
  'scheduled.workdir': 'Working Directory',

  // Gateway Status
  'gateway.running': 'Running',
  'gateway.stopped': 'Stopped',
  'gateway.error': 'Error',
  'gateway.starting': 'Starting',
};

const translations: Record<Language, Translations> = { zh, en };

export function useTranslation() {
  const { language } = useSelector((state: RootState) => state.ui);

  const t = (key: string): string => {
    const currentTranslations = translations[language as Language] || translations.zh;
    const value = currentTranslations[key];
    return typeof value === 'string' ? value : key;
  };

  return { t, language };
}

export function getTranslation(lang: Language, key: string): string {
  const value = translations[lang]?.[key];
  return typeof value === 'string' ? value : key;
}
