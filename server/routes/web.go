package routes

// web.go 负责嵌入式前端资源和 Vue 单页应用回退路由。
// 页面文件可以公开加载，但业务数据只能通过受认证保护的 API 获取；未登录界面由前端固定登录页接管。

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterWebRoutes 注册前端资源访问逻辑。
// 处理顺序如下：
// 1. /api/* 路径保持后端语义，未命中直接返回 404 JSON；
// 2. 若请求的是实际存在的静态文件，则直接返回该文件；
// 3. 其他页面路径统一回退到 index.html，由前端先校验会话再决定渲染登录页或业务页。
func RegisterWebRoutes(r *gin.Engine, distFS fs.FS) {
	fileServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "api not found"})
			return
		}

		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(distFS, path); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		indexData, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})
}
