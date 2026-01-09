package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	// 强制日志颜色
	gin.ForceConsoleColor()

	r := gin.Default()

	// 1. CORS 中间件 - 验证跨域
	r.Use(CorsMiddleware())

	// 2. 全局请求日志中间件 - 方便在 Pod 日志中查看网关转发过来的真实信息
	r.Use(func(c *gin.Context) {
		// 打印 Host, Method, Path
		fmt.Printf("[Request] Host=%s Method=%s Path=%s RemoteAddr=%s\n",
			c.Request.Host, c.Request.Method, c.Request.URL.Path, c.Request.RemoteAddr)

		// 打印特定的网关头，如 X-Forwarded-For 等
		for k, v := range c.Request.Header {
			if strings.HasPrefix(k, "X-Forwarded") || strings.HasPrefix(k, "X-Envoy") {
				fmt.Printf("[Gateway Header] %s: %s\n", k, strings.Join(v, ","))
			}
		}
		c.Next()
	})

	// 3. 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "hostname": os.Getenv("HOSTNAME")})
	})

	// 4. 验证路径重写 (Path Rewrite)
	// 假设网关配置： /gateway/prefix/api -> /api
	// 我们可以请求 /gateway/prefix/api/rewrite-test，如果后端收到 /api/rewrite-test 则重写成功
	api := r.Group("/api")
	{
		api.GET("/rewrite-test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message":       "Path rewrite verification successful",
				"received_path": c.Request.URL.Path,
				"raw_query":     c.Request.URL.RawQuery,
			})
		})

		api.GET("/users", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"users": []string{"alice", "bob", "charlie"},
			})
		})
	}

	// 5. 验证自定义头信息 (Custom Headers)
	// 这个接口回显所有接收到的 Header，用于检查 Envoy Gateway 是否正确传递了 Header
	r.Any("/debug/headers", func(c *gin.Context) {
		headers := make(map[string]string)
		for k, v := range c.Request.Header {
			headers[k] = strings.Join(v, ",")
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Headers received",
			"headers": headers,
			"host":    c.Request.Host,
		})
	})

	// 6. 验证任意路径 (Catch-all)
	// 无论网关重写成什么路径，这里都能接住并显示
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "No specific route matched, but request received",
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"headers": c.Request.Header,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s...\n", port)
	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

// CorsMiddleware 处理跨域请求
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin) // 允许请求来源
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Custom-Header")
			c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// 处理 OPTIONS 请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
