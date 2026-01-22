<script lang="ts" setup>
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { EventsOn } from '../wailsjs/runtime/runtime';
import {
  ClearChangeLog,
  ClearMonitoringData,
  FetchNow,
  GetChangeLog,
  GetConfig,
  GetLogs,
  GetMonitoringStatus,
  GetStats,
  SaveConfig,
  StartMonitoring,
  StopMonitoring,
  TestNotification,
} from '../wailsjs/go/main/App';
import alertSoundUrl from './assets/alert.wav';

type SeverityCounts = {
  critical: number;
  severe: number;
  major: number;
  minor: number;
};

type Stats = {
  total: number;
  severity: SeverityCounts;
  lastUpdated: string;
};

type Config = {
  url: string;
  cookie: string;
  intervalMinutes: number;
  enableNotifications: boolean;
  enableSound: boolean;
};

type ChangeLogEntry = {
  timestamp: string;
  total: number;
  delta: number;
  severity: SeverityCounts;
};

type LogEntry = {
  timestamp: string;
  level: string;
  status: number;
  message: string;
};

const config = reactive<Config>({
  url: '',
  cookie: '',
  intervalMinutes: 15,
  enableNotifications: true,
  enableSound: true,
});

const stats = reactive<Stats>({
  total: 0,
  severity: {
    critical: 0,
    severe: 0,
    major: 0,
    minor: 0,
  },
  lastUpdated: '',
});

const previousStats = ref<Stats | null>(null);
const changeLog = ref<ChangeLogEntry[]>([]);
const logs = ref<LogEntry[]>([]);
const debugOpen = ref(false);
const audioRef = ref<HTMLAudioElement | null>(null);
const monitoringEnabled = ref(true);
const configReady = ref(false);

const lastUpdatedText = computed(() => formatTime(stats.lastUpdated));
const totalDelta = computed(() => stats.total - (previousStats.value?.total ?? stats.total));
const totalDeltaPercent = computed(() =>
  formatDeltaPercent(totalDelta.value, previousStats.value?.total ?? stats.total),
);
const monitoringLabel = computed(() => (monitoringEnabled.value ? '实时监控中' : '监控已暂停'));
const monitoringBadgeClass = computed(() =>
  monitoringEnabled.value
    ? 'bg-green-50 text-green-600 border-green-100'
    : 'bg-amber-50 text-amber-600 border-amber-100',
);
const monitoringDotClass = computed(() =>
  monitoringEnabled.value ? 'bg-green-500 animate-pulse' : 'bg-amber-500',
);

function snapshotStats(source: Stats): Stats {
  return {
    total: source.total,
    lastUpdated: source.lastUpdated,
    severity: { ...source.severity },
  };
}

function formatTime(value?: string): string {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 2000) return '--';
  return date.toLocaleTimeString('zh-CN', { hour12: false });
}

function formatRelative(value?: string): string {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 2000) return '--';
  const diffMs = Date.now() - date.getTime();
  if (diffMs < 60_000) return '刚刚';
  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  return `${days} 天前`;
}

function formatNumber(value: number): string {
  return value.toLocaleString('zh-CN');
}

function formatDeltaPercent(delta: number, base: number): string {
  if (!base) return '0%';
  const percent = (delta / base) * 100;
  const sign = percent > 0 ? '+' : percent < 0 ? '' : '';
  return `${sign}${percent.toFixed(1)}%`;
}

function deltaIcon(delta: number): string {
  if (delta > 0) return 'trending_up';
  if (delta < 0) return 'trending_down';
  return 'trending_flat';
}

function deltaClass(delta: number): string {
  if (delta > 0) return 'text-red-500';
  if (delta < 0) return 'text-emerald-600';
  return 'text-slate-400';
}

function severityDelta(key: keyof SeverityCounts): number {
  const prev = previousStats.value?.severity?.[key] ?? stats.severity[key];
  return stats.severity[key] - prev;
}

function severityPercent(key: keyof SeverityCounts): string {
  const prev = previousStats.value?.severity?.[key] ?? stats.severity[key];
  return formatDeltaPercent(severityDelta(key), prev);
}

function changeTitle(entry: ChangeLogEntry): string {
  if (entry.delta > 0) {
    return `发现 ${entry.delta} 个新增缺陷`;
  }
  if (entry.delta < 0) {
    return `缺陷减少 ${Math.abs(entry.delta)} 个`;
  }
  return '缺陷数量无变化';
}

function changeDetail(entry: ChangeLogEntry): string {
  return `当前总数 ${entry.total} · 一级 ${entry.severity.critical} · 二级 ${entry.severity.severe} · 三级 ${entry.severity.major} · 四级 ${entry.severity.minor}`;
}

function changeBadge(entry: ChangeLogEntry): { label: string; className: string } {
  if (entry.delta > 0) {
    return { label: 'Bug 新增通知', className: 'text-red-600 bg-red-50 border-red-100' };
  }
  if (entry.delta < 0) {
    return { label: 'Bug 减少通知', className: 'text-emerald-600 bg-emerald-50 border-emerald-100' };
  }
  return { label: 'Bug 状态通知', className: 'text-primary bg-blue-50 border-blue-100' };
}

function playSound(): void {
  const audio = audioRef.value;
  if (!audio) return;
  audio.currentTime = 0;
  audio.play().catch(() => {});
}

async function saveConfig(): Promise<void> {
  await SaveConfig({ ...config });
}

async function applyAdvancedSettings(): Promise<void> {
  await SaveConfig({ ...config });
}

async function toggleMonitoring(): Promise<void> {
  if (monitoringEnabled.value) {
    await StopMonitoring();
    monitoringEnabled.value = false;
    return;
  }
  await StartMonitoring();
  monitoringEnabled.value = true;
}

async function syncNow(): Promise<void> {
  await FetchNow();
}

async function clearData(): Promise<void> {
  await ClearMonitoringData();
}

async function clearChangeLog(): Promise<void> {
  await ClearChangeLog();
}

async function testNotification(): Promise<void> {
  await TestNotification();
}

onMounted(async () => {
  Object.assign(config, await GetConfig());
  Object.assign(stats, await GetStats());
  changeLog.value = await GetChangeLog();
  logs.value = await GetLogs();
  monitoringEnabled.value = await GetMonitoringStatus();
  previousStats.value = snapshotStats(stats);
  configReady.value = true;

  EventsOn('config', (payload: Config) => {
    Object.assign(config, payload);
  });

  EventsOn('stats', (payload: Stats) => {
    previousStats.value = snapshotStats(stats);
    Object.assign(stats, payload);
  });

  EventsOn('changelog', (entries: ChangeLogEntry[]) => {
    changeLog.value = entries || [];
  });

  EventsOn('logs', (entries: LogEntry[]) => {
    logs.value = entries || [];
  });

  EventsOn('monitoring', (enabled: boolean) => {
    monitoringEnabled.value = enabled;
  });

  EventsOn('play-sound', (force?: boolean) => {
    if (force || config.enableSound) {
      playSound();
    }
  });
});

watch(
  () => config.enableNotifications,
  async () => {
    if (!configReady.value) return;
    await SaveConfig({ ...config });
  },
);

watch(
  () => config.enableSound,
  async () => {
    if (!configReady.value) return;
    await SaveConfig({ ...config });
  },
);
</script>

<template>
  <audio ref="audioRef" :src="alertSoundUrl" preload="auto"></audio>
  <div class="flex flex-col h-screen overflow-hidden">
    <header
      class="h-16 border-b border-border-color bg-white/90 backdrop-blur-md sticky top-0 z-10 px-8 flex items-center justify-between drag-region shrink-0"
    >
      <div class="flex items-center gap-4">
        <div class="bg-primary size-8 rounded-lg flex items-center justify-center text-white shadow-sm">
          <span class="material-symbols-outlined text-xl leading-none">bug_report</span>
        </div>
        <h2 class="text-lg font-bold tracking-tight text-text-main">极简禅道监控仪表盘</h2>
        <div
          class="flex items-center gap-2 px-2.5 py-0.5 rounded-full text-[11px] font-semibold uppercase tracking-wider border"
          :class="monitoringBadgeClass"
        >
          <span class="size-1.5 rounded-full" :class="monitoringDotClass"></span>
          {{ monitoringLabel }}
        </div>
      </div>
      <div class="flex items-center gap-4 no-drag">
        <div class="flex items-center gap-3 pr-6 border-r border-border-color">
          <div class="text-right">
            <p class="text-[10px] text-text-secondary uppercase font-bold tracking-tighter leading-none mb-1">
              上次更新
            </p>
            <p class="text-xs font-semibold tabular-nums text-text-main">{{ lastUpdatedText }}</p>
          </div>
          <button class="p-2 rounded-lg hover:bg-slate-100 transition-colors group" @click="syncNow">
            <span
              class="material-symbols-outlined text-text-secondary group-hover:rotate-180 transition-transform duration-500"
            >
              refresh
            </span>
          </button>
        </div>
        <button
          class="flex items-center gap-2 rounded-lg bg-primary px-5 py-2 text-white text-sm font-bold shadow-sm hover:opacity-90 transition-all active:scale-95"
          @click="syncNow"
        >
          <span class="material-symbols-outlined text-sm">sync</span>
          <span>立即同步</span>
        </button>
        <button
          class="flex items-center gap-2 rounded-lg bg-white border border-border-color px-4 py-2 text-text-secondary text-sm font-bold hover:bg-slate-50 transition-colors"
          @click="toggleMonitoring"
        >
          <span class="material-symbols-outlined text-sm">
            {{ monitoringEnabled ? 'pause' : 'play_arrow' }}
          </span>
          <span>{{ monitoringEnabled ? '结束监控' : '恢复监控' }}</span>
        </button>
        <button
          class="flex items-center gap-2 rounded-lg bg-white border border-border-color px-4 py-2 text-text-secondary text-sm font-bold hover:bg-slate-50 transition-colors"
          @click="debugOpen = true"
        >
          <span class="material-symbols-outlined text-sm">terminal</span>
          <span>调试</span>
        </button>
      </div>
    </header>
    <main class="flex-1 overflow-y-auto">
      <div class="p-8 space-y-8 max-w-[1400px] mx-auto w-full">
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          <div class="bg-card-bg p-5 rounded-xl border border-border-color shadow-sm">
            <div class="flex justify-between items-start">
              <p class="text-text-secondary text-xs font-semibold uppercase tracking-wider">总缺陷数</p>
              <span class="material-symbols-outlined text-primary text-xl">bug_report</span>
            </div>
            <p class="text-3xl font-bold mt-2 text-text-main">{{ formatNumber(stats.total) }}</p>
            <div class="flex items-center gap-1 mt-3 text-[11px] font-bold" :class="deltaClass(totalDelta)">
              <span class="material-symbols-outlined text-xs">{{ deltaIcon(totalDelta) }}</span>
              <span>{{ totalDeltaPercent }}</span>
            </div>
          </div>
          <div class="bg-card-bg p-5 rounded-xl border border-border-color shadow-sm border-t-4 border-t-red-500">
            <div class="flex justify-between items-start">
              <p class="text-text-secondary text-xs font-semibold uppercase tracking-wider">一级</p>
              <span class="material-symbols-outlined text-red-500 text-xl">dangerous</span>
            </div>
            <p class="text-3xl font-bold mt-2 text-red-500">{{ formatNumber(stats.severity.critical) }}</p>
            <div
              class="flex items-center gap-1 mt-3 text-[11px] font-bold"
              :class="deltaClass(severityDelta('critical'))"
            >
              <span class="material-symbols-outlined text-xs">{{ deltaIcon(severityDelta('critical')) }}</span>
              <span>{{ severityPercent('critical') }}</span>
            </div>
          </div>
          <div class="bg-card-bg p-5 rounded-xl border border-border-color shadow-sm border-t-4 border-t-orange-500">
            <div class="flex justify-between items-start">
              <p class="text-text-secondary text-xs font-semibold uppercase tracking-wider">二级</p>
              <span class="material-symbols-outlined text-orange-500 text-xl">priority_high</span>
            </div>
            <p class="text-3xl font-bold mt-2 text-orange-500">{{ formatNumber(stats.severity.severe) }}</p>
            <div
              class="flex items-center gap-1 mt-3 text-[11px] font-bold"
              :class="deltaClass(severityDelta('severe'))"
            >
              <span class="material-symbols-outlined text-xs">{{ deltaIcon(severityDelta('severe')) }}</span>
              <span>{{ severityPercent('severe') }}</span>
            </div>
          </div>
          <div class="bg-card-bg p-5 rounded-xl border border-border-color shadow-sm border-t-4 border-t-amber-500">
            <div class="flex justify-between items-start">
              <p class="text-text-secondary text-xs font-semibold uppercase tracking-wider">三级</p>
              <span class="material-symbols-outlined text-amber-500 text-xl">warning</span>
            </div>
            <p class="text-3xl font-bold mt-2 text-amber-500">{{ formatNumber(stats.severity.major) }}</p>
            <div
              class="flex items-center gap-1 mt-3 text-[11px] font-bold"
              :class="deltaClass(severityDelta('major'))"
            >
              <span class="material-symbols-outlined text-xs">{{ deltaIcon(severityDelta('major')) }}</span>
              <span>{{ severityPercent('major') }}</span>
            </div>
          </div>
          <div class="bg-card-bg p-5 rounded-xl border border-border-color shadow-sm border-t-4 border-t-blue-400">
            <div class="flex justify-between items-start">
              <p class="text-text-secondary text-xs font-semibold uppercase tracking-wider">四级</p>
              <span class="material-symbols-outlined text-blue-400 text-xl">info</span>
            </div>
            <p class="text-3xl font-bold mt-2 text-blue-400">{{ formatNumber(stats.severity.minor) }}</p>
            <div
              class="flex items-center gap-1 mt-3 text-[11px] font-bold"
              :class="deltaClass(severityDelta('minor'))"
            >
              <span class="material-symbols-outlined text-xs">{{ deltaIcon(severityDelta('minor')) }}</span>
              <span>{{ severityPercent('minor') }}</span>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div class="lg:col-span-8 space-y-6">
            <div class="bg-card-bg rounded-xl border border-border-color shadow-sm overflow-hidden">
              <div class="px-6 py-4 border-b border-border-color flex items-center justify-between bg-slate-50/50">
                <h3 class="font-bold flex items-center gap-2 text-text-main">
                  <span class="material-symbols-outlined text-primary text-xl">database</span>
                  数据源配置
                </h3>
                <span
                  class="text-[10px] bg-slate-200 px-2 py-0.5 rounded font-bold uppercase tracking-widest text-text-secondary"
                >
                  禅道 API v15.x
                </span>
              </div>
              <div class="p-6 space-y-5">
                <div class="space-y-2">
                  <label class="text-sm font-semibold text-text-secondary">禅道 URL</label>
                  <div class="relative">
                    <span
                      class="absolute left-3 top-1/2 -translate-y-1/2 material-symbols-outlined text-slate-400 text-lg"
                    >
                      link
                    </span>
                    <input
                      v-model.trim="config.url"
                      class="w-full bg-slate-50 border border-border-color rounded-lg py-2.5 pl-10 pr-4 text-sm focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all"
                      type="text"
                    />
                  </div>
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-semibold text-text-secondary">登录 Cookie</label>
                  <div class="relative">
                    <textarea
                      v-model.trim="config.cookie"
                      class="w-full bg-slate-50 border border-border-color rounded-lg py-2.5 px-4 text-sm font-mono focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all"
                      placeholder="请输入 zentaosid=..."
                      rows="3"
                    ></textarea>
                    <div class="absolute right-2 bottom-2">
                      <span
                        class="material-symbols-outlined text-slate-400 cursor-help hover:text-primary transition-colors"
                        title="登录禅道后，在浏览器开发者工具中查找 zentaosid。"
                      >
                        help_outline
                      </span>
                    </div>
                  </div>
                  <p class="text-[11px] text-slate-400 italic">
                    凭证信息将仅保存在本地设备中，不会上传至任何云端服务器。
                  </p>
                </div>
                <div class="flex items-center gap-3 pt-2">
                  <button
                    class="bg-primary text-white text-sm font-bold px-8 py-2.5 rounded-lg shadow-sm hover:opacity-90 transition-all"
                    @click="saveConfig"
                  >
                    保存配置
                  </button>
                  <button
                    class="bg-white text-text-secondary border border-border-color text-sm font-bold px-6 py-2.5 rounded-lg hover:bg-slate-50 transition-colors"
                    @click="clearData"
                  >
                    清除数据
                  </button>
                </div>
              </div>
            </div>

            <div class="bg-card-bg rounded-xl border border-border-color shadow-sm">
              <div class="px-6 py-4 border-b border-border-color bg-slate-50/50 flex items-center justify-between">
                <h3 class="font-bold flex items-center gap-2 text-text-main">
                  <span class="material-symbols-outlined text-primary text-xl">settings_input_component</span>
                  高级设置
                </h3>
                <button
                  class="text-[11px] font-bold text-primary bg-primary/10 px-3 py-1 rounded-full hover:bg-primary/20 transition"
                  @click="applyAdvancedSettings"
                >
                  应用设置
                </button>
              </div>
              <div class="p-6 grid grid-cols-1 md:grid-cols-2 gap-10">
                <div class="space-y-4">
                  <div class="flex justify-between items-center">
                    <label class="text-sm font-semibold text-text-secondary">监控间隔 (分钟)</label>
                    <span class="text-primary font-bold text-sm bg-primary/10 px-2 py-1 rounded">
                      每 {{ config.intervalMinutes }} 分钟
                    </span>
                  </div>
                  <input
                    v-model.number="config.intervalMinutes"
                    class="w-full h-2 bg-slate-100 rounded-lg appearance-none cursor-pointer accent-primary"
                    max="60"
                    min="1"
                    type="range"
                  />
                  <div class="flex justify-between text-[10px] text-slate-400 font-bold uppercase tracking-tighter">
                    <span>1 分钟</span>
                    <span>30 分钟</span>
                    <span>60 分钟</span>
                  </div>
                </div>
                <div class="space-y-4">
                  <label class="text-sm font-semibold text-text-secondary">通知偏好</label>
                  <div class="flex flex-col gap-4">
                    <label class="relative inline-flex items-center cursor-pointer group">
                      <input v-model="config.enableNotifications" class="sr-only peer" type="checkbox" />
                      <div
                        class="w-10 h-5 bg-slate-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-primary"
                      ></div>
                      <span class="ml-3 text-sm font-medium text-text-secondary group-hover:text-primary transition-colors">
                        启用系统通知
                      </span>
                    </label>
                    <label class="relative inline-flex items-center cursor-pointer group">
                      <input v-model="config.enableSound" class="sr-only peer" type="checkbox" />
                      <div
                        class="w-10 h-5 bg-slate-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-primary"
                      ></div>
                      <span class="ml-3 text-sm font-medium text-text-secondary group-hover:text-primary transition-colors">
                        声音提醒
                      </span>
                    </label>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="lg:col-span-4 space-y-6">
            <div class="bg-card-bg h-full rounded-xl border border-border-color shadow-sm flex flex-col">
              <div class="px-6 py-4 border-b border-border-color flex items-center justify-between">
                <h3 class="font-bold flex items-center gap-2 text-text-main">
                  <span class="material-symbols-outlined text-primary text-xl">notifications_active</span>
                  通知记录
                </h3>
                <button class="text-slate-400 text-[11px] font-bold hover:text-primary transition-colors" @click="clearChangeLog">
                  清空记录
                </button>
              </div>
              <div class="flex-1 overflow-y-auto p-5 space-y-6 max-h-[650px]">
                <div
                  v-if="changeLog.length === 0"
                  class="text-center text-xs text-slate-400 bg-slate-50 border border-dashed border-slate-200 rounded-lg py-6"
                >
                  暂无变更记录
                </div>
                <div v-for="entry in changeLog" :key="entry.timestamp" class="flex gap-4 items-start group">
                  <div
                    class="mt-1 size-2 rounded-full shadow-sm flex-shrink-0"
                    :class="entry.delta > 0 ? 'bg-red-500' : entry.delta < 0 ? 'bg-emerald-500' : 'bg-primary'"
                  ></div>
                  <div class="space-y-1">
                    <div class="flex items-center gap-2">
                      <span
                        class="text-[10px] font-bold px-1.5 py-0.5 rounded uppercase border"
                        :class="changeBadge(entry).className"
                      >
                        {{ changeBadge(entry).label }}
                      </span>
                      <p class="text-[10px] text-slate-400 font-medium">{{ formatRelative(entry.timestamp) }}</p>
                    </div>
                    <p class="text-sm font-bold text-text-main">{{ changeTitle(entry) }}</p>
                    <p class="text-xs text-text-secondary leading-relaxed">{{ changeDetail(entry) }}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div
          class="bg-white p-8 rounded-xl border border-primary/20 flex flex-col md:flex-row items-center justify-between gap-6 shadow-sm"
        >
          <div class="flex items-center gap-6">
            <div
              class="w-32 h-20 bg-center bg-no-repeat bg-cover rounded-lg shadow-sm border border-slate-100"
              style="
                background-image: url('https://lh3.googleusercontent.com/aida-public/AB6AXuDU09AQGZ4DE2vmFV-D38-45Zgfty2Gu6HCco7_P4izx8IeobSTQoYVbETrmozcyfd-owcwtAQYU7H45JuOuVvoYoGAJMnsoh24ECAavWi_2hfUSOz5hemk3Uk2gIfyOLokAHNPEOajtLh1Wt_UK3vCCaIfUpniSg6KHnp94-AZIjrRobaxKa6Z4EARQdxENaWbHXs-4eMj3YCIpEmqNhV_7OZYgA5KkOorq0bfDtEAVM1-zlu8YRu52xdgWGL4cHgBRsuQJ1Z_jf9c');
              "
            ></div>
            <div class="space-y-1">
              <h4 class="text-lg font-bold text-text-main">极简禅道深度分析 专业版</h4>
              <p class="text-text-secondary text-sm max-w-md">
                解锁基于机器学习的 Bug 修复周期预测、全量历史数据报表及团队协作功能。
              </p>
            </div>
          </div>
          <button
            class="bg-primary text-white font-bold py-3 px-10 rounded-lg text-sm hover:shadow-md transition-all hover:-translate-y-0.5 active:translate-y-0"
          >
            立即升级
          </button>
        </div>
      </div>
    </main>
    <footer class="mt-auto px-8 py-3 bg-white border-t border-border-color flex justify-between items-center shrink-0">
      <div class="flex items-center gap-6 text-[10px] font-bold text-slate-400 tracking-widest uppercase">
        <span class="flex items-center gap-1.5">
          <span class="material-symbols-outlined text-[14px]">bolt</span>
          引擎: 已开启
        </span>
        <span class="flex items-center gap-1.5">
          <span class="material-symbols-outlined text-[14px]">lan</span>
          API: 已连接
        </span>
        <span class="flex items-center gap-1.5">
          <span class="material-symbols-outlined text-[14px]">terminal</span>
          V1.2.0 STABLE
        </span>
      </div>
      <div class="text-[10px] font-bold text-slate-400 tracking-tighter">INST-ID: ZEN-APP-88A-921-X1</div>
    </footer>
  </div>

  <div v-if="debugOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-6">
    <div class="w-full max-w-3xl bg-white rounded-2xl shadow-xl border border-slate-200 overflow-hidden">
      <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 bg-slate-50">
        <div class="flex items-center gap-2 text-text-main font-bold">
          <span class="material-symbols-outlined text-primary">terminal</span>
          调试面板
        </div>
        <button class="text-slate-400 hover:text-slate-600" @click="debugOpen = false">
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>
      <div class="p-6 space-y-6">
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="font-bold text-sm text-text-main">实时请求日志</h4>
            <button
              class="text-[11px] font-bold text-primary bg-primary/10 px-3 py-1 rounded-full hover:bg-primary/20 transition"
              @click="testNotification"
            >
              测试通知
            </button>
          </div>
          <div class="border border-slate-200 rounded-lg max-h-64 overflow-y-auto">
            <div v-if="logs.length === 0" class="text-center text-xs text-slate-400 py-6">暂无日志</div>
            <div
              v-for="entry in logs"
              :key="entry.timestamp + entry.message"
              class="px-4 py-3 border-b border-slate-100 last:border-b-0 flex items-start gap-3"
            >
              <span
                class="mt-1 size-2 rounded-full"
                :class="entry.level === 'error' ? 'bg-red-500' : 'bg-emerald-500'"
              ></span>
              <div class="flex-1">
                <div class="flex items-center justify-between text-[11px] text-slate-400">
                  <span>{{ formatTime(entry.timestamp) }}</span>
                  <span v-if="entry.status">HTTP {{ entry.status }}</span>
                </div>
                <p class="text-sm text-text-main font-medium">{{ entry.message }}</p>
              </div>
            </div>
          </div>
        </div>
        <div class="flex justify-end">
          <button
            class="px-6 py-2 rounded-lg text-sm font-bold text-white bg-primary hover:opacity-90 transition"
            @click="debugOpen = false"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
