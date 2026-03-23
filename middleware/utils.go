package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func isClaudeMessagesRequest(c *gin.Context) bool {
	return strings.Contains(c.Request.URL.Path, "/v1/messages")
}

func abortWithClaudeStandardError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "Resource invocation service exception",
	})
	c.Abort()
}

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	userId := c.GetInt("id")
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))

	if isClaudeMessagesRequest(c) {
		abortWithClaudeStandardError(c)
		return
	}

	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			"type":    "new_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": description,
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}
