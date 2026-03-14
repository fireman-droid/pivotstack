import { defineStore } from 'pinia';

export const useWorldTheme = defineStore('worldTheme', {
  state: () => ({
    currentWorld: localStorage.getItem('kiro-world') || 'reality',
    isTransitioning: false,
    // 动画配置
    animationStyle: localStorage.getItem('kiro-animation-style') || 'default', // 'default' | 'simple'
    animationDuration: 2000 // 可配置总时长 (ms)
  }),
  actions: {
    async toggleWorld() {
      if (this.isTransitioning) return;

      this.isTransitioning = true;
      const nextWorld = this.currentWorld === 'reality' ? 'daogui' : 'reality';

      // 延迟切换 DOM 属性，等待过渡动画到达中心点
      setTimeout(() => {
        this.currentWorld = nextWorld;
        document.documentElement.setAttribute('data-world', nextWorld);
        localStorage.setItem('kiro-world', nextWorld);
      }, this.animationDuration / 2); // 动态计算 DOM 切换点

      // 结束过渡状态
      setTimeout(() => {
        this.isTransitioning = false;
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
