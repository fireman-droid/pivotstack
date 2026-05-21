<script setup lang="ts">
// 通用占位 view —— v6 URL 已经在 sidebar 上但视图本体还没迁完时使用。
// 视觉走标准 PageContainer + PageHeader 模板，跟真实 v6 view 完全一致。
// 主体不堆卡片，用 1px divider 内的状态块 + 一个紧急通道按钮指向旧版本。

import { Construction, ArrowUpRight } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import PageContainer from '../components/common/PageContainer.vue'
import PageHeader from '../components/common/PageHeader.vue'

defineProps<{
  /** 顶部小标签，对齐当前域 */
  kicker: string
  /** 主标题，跟 sidebar item label 一致 */
  title: string
  /** 副标 — 描述这个页面应该做什么 */
  desc?: string
  /** 旧版本 URL；如有则展示一个紧急通道按钮 */
  legacyTo?: string
  /** 已废弃 prop，仅为兼容旧调用保留；新代码不要传 */
  stage?: string
}>()
</script>

<template>
  <PageContainer>
    <PageHeader :kicker="kicker" :title="title" :desc="desc" />

    <section class="ph-state">
      <span class="ph-state__ico" aria-hidden="true">
        <Construction :size="22" />
      </span>
      <div class="ph-state__body">
        <h2 class="ph-state__title">页面建设中</h2>
        <p class="ph-state__desc">
          此页面的视觉与交互尚未完成。后端能力可能已就绪，前端 UI 还在排期。
        </p>
        <RouterLink v-if="legacyTo" :to="legacyTo" class="ph-state__cta">
          <span>跳转到相关页面</span>
          <ArrowUpRight :size="14" />
        </RouterLink>
      </div>
    </section>
  </PageContainer>
</template>

<style scoped>
.ph-state {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 20px 22px;
  background: #0a0a0a;
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 8px;
  max-width: 720px;
}
.ph-state__ico {
  width: 40px; height: 40px;
  border-radius: 8px;
  background: rgba(245, 166, 35, 0.08);
  color: #f5a623;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.ph-state__body { flex: 1; min-width: 0; }
.ph-state__title {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
  color: #ededed;
}
.ph-state__desc {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.5;
  color: #a1a1a1;
}
.ph-state__cta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 5px;
  font-size: 12px;
  color: #ededed;
  text-decoration: none;
  transition: border-color 150ms, background 150ms;
}
.ph-state__cta:hover {
  border-color: rgba(255, 255, 255, 0.32);
  background: rgba(255, 255, 255, 0.04);
}
</style>
