// main.ts 是前端应用入口。
// 这里只做最基础的两件事：加载全局样式，并把根组件 App 挂载到 index.html 中的 #app 节点。
import './assets/main.css'

import { createApp } from 'vue'
import App from './App.vue'

createApp(App).mount('#app')
