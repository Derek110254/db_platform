package routes

// web.go 负责前端页面静态资源路由。
// 它与 API 路由分离，核心目标是支持 Vue 单页应用在刷新、直达子路径时也能正确返回 index.html。

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
// 3. 若是 Vue 前端路由（例如 /query），则统一回退到 index.html，由前端接管页面渲染。
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
