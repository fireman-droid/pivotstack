<script setup>
import { RouterView } from 'vue-router'
import { NConfigProvider, NMessageProvider, NDialogProvider, NNotificationProvider } from 'naive-ui'
import zhCN from 'naive-ui/es/locales/common/zhCN.mjs'
import dateZhCN from 'naive-ui/es/locales/date/zhCN.mjs'
import { baseTheme, themeOverrides } from './design/theme'
</script>

<template>
  <n-config-provider :theme="baseTheme" :theme-overrides="themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <a href="#main-content" class="skip-to-content">跳转到主内容</a>
          <router-view />
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
:root {
  color-scheme: dark;
  --color-bg-base: #000000;
  --color-bg-surface: #0a0a0a;
  --color-bg-elevated: #141414;
  --color-bg-overlay: #1a1a1a;
  --color-bg-hover: rgba(255,255,255,0.05);
  --color-border-default: rgba(255,255,255,0.08);
  --color-border-strong: rgba(255,255,255,0.16);
  --color-divider: rgba(255,255,255,0.10);
  --color-text-primary: #ededed;
  --color-text-secondary: #a1a1a1;
  --color-text-tertiary: #707070;
  --color-text-disabled: #4d4d4d;
  --color-success: #0bd470;
  --color-warning: #f5a623;
  --color-error: #ff4d4d;
  --color-info: #52a8ff;
}
html, body, #app {
  background: var(--color-bg-base);
  color: var(--color-text-primary);
  font-family: "Geist Sans", Inter, "PingFang SC", "Microsoft YaHei", sans-serif;
}
.skip-to-content {
  position: absolute;
  top: -100px; left: 0;
  background: var(--color-bg-surface);
  color: var(--color-text-primary);
  padding: 8px 16px; border-radius: 4px;
  z-index: 9999;
}
.skip-to-content:focus { top: 8px; left: 8px; }
</style>
