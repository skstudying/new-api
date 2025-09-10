package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"one-api/common"
	"one-api/constant"
	"one-api/dto"
	"one-api/logger"
	"one-api/model"
	"one-api/relay"
	"one-api/relay/channel"
	relaycommon "one-api/relay/common"
	"one-api/setting/ratio_setting"
	"time"

	"github.com/gin-gonic/gin"
)

func UpdateVideoTaskAll(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		if err := updateVideoTaskAll(ctx, platform, channelId, taskIds, taskM); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update video async tasks: %s", channelId, err.Error()))
		}
	}
	return nil
}

func updateVideoTaskAll(ctx context.Context, platform constant.TaskPlatform, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("Channel #%d pending video tasks: %d", channelId, len(taskIds)))
	if len(taskIds) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		errUpdate := model.TaskBulkUpdate(taskIds, map[string]any{
			"fail_reason": fmt.Sprintf("Failed to get channel info, channel ID: %d", channelId),
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if errUpdate != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTask error: %v", errUpdate))
		}
		return fmt.Errorf("CacheGetChannel failed: %w", err)
	}
	adaptor := relay.GetTaskAdaptor(platform)
	if adaptor == nil {
		return fmt.Errorf("video adaptor not found")
	}
	for _, taskId := range taskIds {
		if err := updateVideoSingleTask(ctx, adaptor, cacheGetChannel, taskId, taskM); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update video task %s: %s", taskId, err.Error()))
		}
	}
	return nil
}

func updateVideoSingleTask(ctx context.Context, adaptor channel.TaskAdaptor, channel *model.Channel, taskId string, taskM map[string]*model.Task) error {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	task := taskM[taskId]
	if task == nil {
		logger.LogError(ctx, fmt.Sprintf("Task %s not found in taskM", taskId))
		return fmt.Errorf("task %s not found", taskId)
	}
	resp, err := adaptor.FetchTask(baseURL, channel.Key, map[string]any{
		"task_id": taskId,
		"action":  task.Action,
	})
	if err != nil {
		return fmt.Errorf("fetchTask failed for task %s: %w", taskId, err)
	}
	//if resp.StatusCode != http.StatusOK {
	//return fmt.Errorf("get Video Task status code: %d", resp.StatusCode)
	//}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("readAll failed for task %s: %w", taskId, err)
	}

	taskResult := &relaycommon.TaskInfo{}
	// try parse as New API response format
	var responseItems dto.TaskResponse[model.Task]
	if err = json.Unmarshal(responseBody, &responseItems); err == nil && responseItems.IsSuccess() {
		t := responseItems.Data
		taskResult.TaskID = t.TaskID
		taskResult.Status = string(t.Status)
		taskResult.Url = t.FailReason
		taskResult.Progress = t.Progress
		taskResult.Reason = t.FailReason
	} else if taskResult, err = adaptor.ParseTaskResult(responseBody); err != nil {
		return fmt.Errorf("parseTaskResult failed for task %s: %w", taskId, err)
	} else {
		task.Data = responseBody
	}

	now := time.Now().Unix()
	if taskResult.Status == "" {
		return fmt.Errorf("task %s status is empty", taskId)
	}
	task.Status = model.TaskStatus(taskResult.Status)
	switch taskResult.Status {
	case model.TaskStatusSubmitted:
		task.Progress = "10%"
	case model.TaskStatusQueued:
		task.Progress = "20%"
	case model.TaskStatusInProgress:
		task.Progress = "30%"
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		task.Progress = "100%"
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Url
		// 如果有usage信息，基于token计费
		if taskResult.TotalTokens > 0 {
			// 获取模型名称 - 优先使用任务保存的模型名称
			modelName := task.ModelName

			// 如果任务中没有保存模型名称，从taskResult.Reason中解析（包含完整的响应数据）
			if modelName == "" && taskResult.Reason != "" {
				var successData map[string]interface{}
				if err := json.Unmarshal([]byte(taskResult.Reason), &successData); err == nil {
					if model, ok := successData["model"].(string); ok {
						modelName = model
					}
				}
			}

			// 如果还是没有获取到，尝试从任务数据中获取
			if modelName == "" {
				var taskData map[string]interface{}
				if err := json.Unmarshal(task.Data, &taskData); err == nil {
					if model, ok := taskData["model"].(string); ok {
						modelName = model
					}
				}
			}

			// 如果还是没有，使用默认的模型名称
			if modelName == "" {
				modelName = "doubao-seedance-1-0-lite-i2v" // 默认模型
				logger.LogInfo(ctx, fmt.Sprintf("任务 %s 无法获取模型名称，使用默认模型: %s", task.TaskID, modelName))
			}

			// 获取模型倍率和补全倍率
			modelRatio, _, _ := ratio_setting.GetModelRatio(modelName)
			if modelRatio == 0 {
				modelRatio = 1.0 // 默认倍率
			}

			// 获取补全倍率（对于视频模型，主要是 completion tokens）
			completionRatio := ratio_setting.GetCompletionRatio(modelName)
			if completionRatio == 0 {
				completionRatio = 1.0 // 默认补全倍率
			}

			// 获取分组倍率（使用任务保存的分组信息）
			groupName := task.Group
			if groupName == "" {
				groupName = "default"
			}
			groupRatio := ratio_setting.GetGroupRatio(groupName)

			// 计算实际消耗的quota（按照GPT-4o的计费方式）
			// 对于视频模型，主要是 completion_tokens，没有 prompt_tokens
			actualQuota := int(float64(taskResult.TotalTokens) * completionRatio * modelRatio * groupRatio)
			if actualQuota <= 0 && modelRatio > 0 {
				actualQuota = 1
			}

			// 计算差额
			quotaDelta := actualQuota - task.Quota

			if quotaDelta != 0 {
				if quotaDelta > 0 {
					// 需要补扣费
					if err := model.DecreaseUserQuota(task.UserId, quotaDelta); err != nil {
						logger.LogError(ctx, "Failed to decrease user quota: "+err.Error())
					} else {
						// 使用消费日志记录
						logContent := fmt.Sprintf("视频任务补充计费，模型倍率 %.2f，补全倍率 %.2f，分组倍率 %.2f", modelRatio, completionRatio, groupRatio)
						other := make(map[string]interface{})
						other["task_id"] = task.TaskID
						other["action"] = task.Action
						other["model_ratio"] = modelRatio
						other["completion_ratio"] = completionRatio
						other["group_ratio"] = groupRatio
						other["adjustment_type"] = "supplement" // 补充计费

						useTime := int(task.FinishTime - task.SubmitTime)
						if useTime <= 0 {
							useTime = int(time.Now().Unix() - task.SubmitTime)
						}

						// 获取用户信息用于日志显示
						user, err := model.GetUserById(task.UserId, false)
						var username string
						if err == nil && user != nil {
							username = user.Username
						} else {
							username = fmt.Sprintf("user_%d", task.UserId)
						}

						// 创建一个临时的 gin.Context 用于记录日志
						tempCtx := &gin.Context{}
						tempCtx.Set("id", task.UserId)
						tempCtx.Set("token_id", task.TokenId)
						tempCtx.Set("token_name", task.TokenName)
						tempCtx.Set("username", username)

						model.RecordConsumeLog(tempCtx, task.UserId, model.RecordConsumeLogParams{
							ChannelId:        task.ChannelId,
							PromptTokens:     0,
							CompletionTokens: taskResult.TotalTokens,
							ModelName:        modelName,
							TokenName:        task.TokenName,
							Quota:            quotaDelta,
							Content:          logContent,
							TokenId:          task.TokenId,
							UseTimeSeconds:   useTime,
							IsStream:         false,
							Group:            task.Group,
							Other:            other,
						})

						model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quotaDelta)
						model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
					}
				} else {
					// 需要退费
					if err := model.IncreaseUserQuota(task.UserId, -quotaDelta, false); err != nil {
						logger.LogError(ctx, "Failed to increase user quota: "+err.Error())
					} else {
						// 退费时记录系统日志
						logContent := fmt.Sprintf("视频任务 %s 基于token计费调整，退还 %s (tokens: %d, 模型: %s)",
							task.TaskID, logger.LogQuota(-quotaDelta), taskResult.TotalTokens, modelName)
						model.RecordLog(task.UserId, model.LogTypeSystem, logContent)
					}
				}
				// 更新任务的实际消耗quota
				task.Quota = actualQuota
			}

			logger.LogInfo(ctx, fmt.Sprintf("视频任务 %s 完成，使用token计费: %d tokens, quota: %d", task.TaskID, taskResult.TotalTokens, actualQuota))
		}
	case model.TaskStatusFailure:
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		logger.LogInfo(ctx, fmt.Sprintf("Task %s failed: %s", task.TaskID, task.FailReason))
		quota := task.Quota
		if quota != 0 {
			if err := model.IncreaseUserQuota(task.UserId, quota, false); err != nil {
				logger.LogError(ctx, "Failed to increase user quota: "+err.Error())
			}
			logContent := fmt.Sprintf("Video async task failed %s, refund %s", task.TaskID, logger.LogQuota(quota))
			model.RecordLog(task.UserId, model.LogTypeSystem, logContent)
		}
	default:
		return fmt.Errorf("unknown task status %s for task %s", taskResult.Status, taskId)
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}
	if err := task.Update(); err != nil {
		common.SysLog("UpdateVideoTask task error: " + err.Error())
	}

	return nil
}
