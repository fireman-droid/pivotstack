import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import './assets/styles/fonts.css'
import './style.css'
import './design/stellar.css'
import './design/admin.css'

// Stellar Console: register ECharts custom theme once
import { ensureStellarTheme } from './design/echarts-stellar'
ensureStellarTheme()

// v6: naive-ui CSS reset 给新 view 用，theme 由 App.vue 的 <n-config-provider> 注入。
import { create, NConfigProvider } from 'naive-ui'

const naive = create({ components: [NConfigProvider] })

// 统一暗色主题
document.documentElement.classList.add('dark')
document.documentElement.setAttribute('data-theme', 'dark')

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(naive)
app.mount('#app')
