// 前端应用入口。
// 负责加载全局样式，并将根组件挂载到 index.html 的 #app 节点。
import './assets/main.css'

import { createApp } from 'vue'
import App from './App.vue'

createApp(App).mount('#app')
