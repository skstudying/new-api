package middleware

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// SystemPerformanceCheck 检查系统性能中间件
func SystemPerformanceCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := checkSystemPerformance(); err != nil {
			if isClaudeMessagesRequest(c) {
				abortWithClaudeStandardError(c)
			} else {
				c.JSON(err.StatusCode, gin.H{
					"error": err.ToOpenAIError(),
				})
				c.Abort()
			}
			return
		}
		c.Next()
	}
}

// checkSystemPerformance 检查系统性能是否超过阈值
func checkSystemPerformance() *types.NewAPIError {
	config := common.GetPerformanceMonitorConfig()
	if !config.Enabled {
		return nil
	}

	status := common.GetSystemStatus()

	// 检查 CPU
	if config.CPUThreshold > 0 && int(status.CPUUsage) > config.CPUThreshold {
		return types.NewErrorWithStatusCode(errors.New("system cpu overloaded"), "system_cpu_overloaded", http.StatusServiceUnavailable)
	}

	// 检查内存
	if config.MemoryThreshold > 0 && int(status.MemoryUsage) > config.MemoryThreshold {
		return types.NewErrorWithStatusCode(errors.New("system memory overloaded"), "system_memory_overloaded", http.StatusServiceUnavailable)
	}

	// 检查磁盘
	if config.DiskThreshold > 0 && int(status.DiskUsage) > config.DiskThreshold {
		return types.NewErrorWithStatusCode(errors.New("system disk overloaded"), "system_disk_overloaded", http.StatusServiceUnavailable)
	}

	return nil
}
