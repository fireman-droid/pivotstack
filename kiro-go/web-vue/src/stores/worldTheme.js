import { defineStore } from 'pinia';

export const useWorldTheme = defineStore('worldTheme', {
  state: () => ({
    currentWorld: localStorage.getItem('kiro-world') || 'reality',
    isTransitioning: false,
    // 动画配置
    animationStyle: localStorage.getItem('kiro-animation-style') || 'default', // 'default' | 'simple'
    animationDuration: 1100 // ms，道诡异仙双世界过场（v6 — GPU-only 3 母题×3 层视差）
  }),
  actions: {
    /**
     * @param {DOMRect} [originRect] 触发按钮的 bounding rect，用作 portal 动画的视觉原点。
     *                               不传则原点回落到屏幕几何中心 (50%, 50%)。
     */
    async toggleWorld(originRect) {
      if (this.isTransitioning) return;

      // 设置 portal 原点 CSS variables（让笔锋/印章/血脉等从按钮位置发散）
      // 同时计算笔锋指向屏幕几何中心 (50%, 55% 印章落点) 的角度，
      // 让笔触自动从被点击的按钮指向屏幕中心，无需在 CSS 里硬编码角度。
      const root = document.documentElement;
      const TARGET_X = 50;
      const TARGET_Y = 55;
      let cx = TARGET_X;
      let cy = TARGET_Y;
      if (originRect && originRect.width > 0) {
        cx = ((originRect.left + originRect.width / 2) / window.innerWidth) * 100;
        cy = ((originRect.top + originRect.height / 2) / window.innerHeight) * 100;
      }
      const dx = TARGET_X - cx;
      const dy = TARGET_Y - cy;
      const angleDeg = (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5)
        ? -34  // 原点 ≈ 印章位置时回落到 v4 默认角度
        : Math.atan2(dy, dx) * 180 / Math.PI;
      root.style.setProperty('--portal-origin-x', cx.toFixed(2) + '%');
      root.style.setProperty('--portal-origin-y', cy.toFixed(2) + '%');
      root.style.setProperty('--portal-angle', angleDeg.toFixed(2) + 'deg');

      this.isTransitioning = true;
      const nextWorld = this.currentWorld === 'reality' ? 'daogui' : 'reality';

      // 切换期间在 <html> 上加 data-switching，全局禁用主题色 transition
      // 防止「转场结束后页面还在慢慢变色」的拖尾效应
      root.setAttribute('data-switching', '');

      // DOM 切换在 50% 帧（落印峰值，血脉/碎裂之前）
      setTimeout(() => {
        this.currentWorld = nextWorld;
        root.setAttribute('data-world', nextWorld);
        localStorage.setItem('kiro-world', nextWorld);
      }, this.animationDuration / 2);

      // 结束过渡状态
      setTimeout(() => {
        this.isTransitioning = false;
        root.removeAttribute('data-switching');
      }, this.animationDuration);
    },

    setAnimationStyle(style) {
      this.animationStyle = style;
      localStorage.setItem('kiro-animation-style', style);
    },

    setAnimationDuration(ms) {
      this.animationDuration = Math.max(500, Math.min(5000, ms));
    }
  }
});
