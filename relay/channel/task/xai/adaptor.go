package xai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type submitResponse struct {
	RequestID string `json:"request_id"`
}

type pollResponse struct {
	Status string     `json:"status"` // pending, done, expired
	Video  *videoData `json:"video,omitempty"`
	Model  string     `json:"model,omitempty"`
}

type videoData struct {
	URL               string  `json:"url"`
	Duration          float64 `json:"duration"`
	RespectModeration bool    `json:"respect_moderation"`
}

type TaskAdaptor struct {
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if taskErr := relaycommon.ValidateMultipartDirect(c, info); taskErr != nil {
		return taskErr
	}

	var req struct {
		Video json.RawMessage `json:"video"`
	}
	_ = common.UnmarshalBodyReusable(c, &req)

	if len(req.Video) > 0 {
		info.Action = constant.TaskActionEdit
	}

	return nil
}

// EstimateBilling returns OtherRatios for pre-charge calculation.
// For video generation: seconds × resolution multiplier.
// For video edits: always bill at max 8.7s (refund on completion via AdjustBillingOnComplete).
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	var req struct {
		Duration    int    `json:"duration"`
		Seconds     string `json:"seconds"`
		Resolution  string `json:"resolution"`
		AspectRatio string `json:"aspect_ratio"`
	}
	_ = common.UnmarshalBodyReusable(c, &req)

	isVideoEdit := info.Action == constant.TaskActionEdit

	seconds := req.Duration
	if seconds == 0 {
		s, _ := strconv.Atoi(req.Seconds)
		seconds = s
	}
	if seconds <= 0 {
		seconds = 5
	}

	billingSeconds := float64(seconds)
	if isVideoEdit {
		billingSeconds = 8.7
	}

	ratios := map[string]float64{
		"seconds": billingSeconds,
	}
	if req.Resolution == "720p" {
		ratios["resolution(720p)"] = 1.4
	}

	if isVideoEdit {
		ratios["xai_video_edit"] = 1.0
		ratios["xai_input_video_price"] = 1.0 // indicator; actual price $0.01/sec is fixed
	} else {
		ratios["xai_video_generation"] = 1.0
	}

	return ratios
}

// AdjustBillingOnSubmit returns nil — no submit-time adjustment needed.
func (a *TaskAdaptor) AdjustBillingOnSubmit(_ *relaycommon.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

// AdjustBillingOnComplete adjusts billing when the task reaches a terminal state.
// For video edits (success): calculates actual quota based on real duration vs pre-charged 8.7s.
// For content moderation failures: retains the pre-charged amount (no refund).
// Otherwise returns 0 (keep pre-charge on success, allow refund on failure).
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task.Quota <= 0 {
		return 0
	}

	isModeration := strings.Contains(strings.ToLower(taskResult.Reason), "content moderation")

	switch model.TaskStatus(taskResult.Status) {
	case model.TaskStatusSuccess:
		if task.Action == constant.TaskActionEdit && taskResult.Duration > 0 {
			const maxDuration = 8.7
			actualDuration := taskResult.Duration
			if actualDuration > maxDuration {
				actualDuration = maxDuration
			}
			actualQuota := task.Quota
			if actualDuration < maxDuration {
				actualQuota = int(float64(task.Quota) * actualDuration / maxDuration)
				if actualQuota <= 0 {
					actualQuota = 1
				}
			}
			return actualQuota
		}
	case model.TaskStatusFailure:
		if isModeration {
			if task.Action == constant.TaskActionEdit {
				const moderationDuration = 8.0
				moderationQuota := int(float64(task.Quota) * moderationDuration / 8.7)
				if moderationQuota <= 0 {
					moderationQuota = 1
				}
				return moderationQuota
			}
			return task.Quota
		}
	}
	return 0
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimSuffix(strings.TrimSuffix(a.baseURL, "/"), "/v1")
	if info.Action == constant.TaskActionEdit {
		return fmt.Sprintf("%s/v1/videos/edits", base), nil
	}
	return fmt.Sprintf("%s/v1/videos/generations", base), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, fmt.Errorf("get request body failed: %w", err)
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, fmt.Errorf("read request body failed: %w", err)
	}
	return bytes.NewReader(cachedBody), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("body: %s, err: %w", responseBody, err), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	if sResp.RequestID == "" {
		return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("request_id is empty, body: %s", responseBody), "invalid_response", http.StatusInternalServerError)
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = sResp.RequestID
	ov.Status = dto.VideoStatusQueued
	if info.UpstreamModelName != "" {
		ov.Model = info.UpstreamModelName
	} else {
		ov.Model = info.OriginModelName
	}
	c.JSON(http.StatusOK, ov)

	return sResp.RequestID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	key = strings.TrimSpace(strings.SplitN(key, "\n", 2)[0])
	baseURL = strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1")

	uri := fmt.Sprintf("%s/v1/videos/%s", baseURL, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		expired := `{"status":"expired"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expired)),
		}, nil
	}
	if resp.StatusCode == http.StatusBadRequest {
		return resp, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode/100 == 5 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		common.SysLog(fmt.Sprintf("xAI video poll transient HTTP %d for task %s: %s", resp.StatusCode, taskID, string(respBody)))
		pending := `{"status":"pending"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(pending)),
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("xAI video poll returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return resp, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var errResp struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if common.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
		return &relaycommon.TaskInfo{
			Status: model.TaskStatusFailure,
			Reason: errResp.Error,
		}, nil
	}

	var pResp pollResponse
	if err := common.Unmarshal(respBody, &pResp); err != nil {
		return nil, fmt.Errorf("unmarshal task result failed: %w", err)
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	switch pResp.Status {
	case "pending":
		taskResult.Status = model.TaskStatusQueued
	case "done":
		taskResult.Status = model.TaskStatusSuccess
		if pResp.Video != nil {
			taskResult.Url = pResp.Video.URL
			taskResult.Duration = pResp.Video.Duration
		}
	case "expired":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Reason = "request expired"
	default:
		if pResp.Video != nil && pResp.Video.URL != "" {
			taskResult.Status = model.TaskStatusSuccess
			taskResult.Url = pResp.Video.URL
			taskResult.Duration = pResp.Video.Duration
		} else {
			taskResult.Status = model.TaskStatusQueued
		}
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	ov := dto.NewOpenAIVideo()
	ov.ID = task.TaskID
	ov.Status = task.Status.ToVideoStatus()
	ov.SetProgressStr(task.Progress)
	ov.CreatedAt = task.CreatedAt
	if task.FinishTime > 0 {
		ov.CompletedAt = task.FinishTime
	} else if task.UpdatedAt > 0 {
		ov.CompletedAt = task.UpdatedAt
	}
	if task.Properties.OriginModelName != "" {
		ov.Model = task.Properties.OriginModelName
	}
	if task.Status == model.TaskStatusSuccess {
		resultURL := task.GetResultURL()
		if resultURL != "" {
			ov.SetMetadata("url", resultURL)
		}
		var taskData struct {
			Video *videoData `json:"video"`
		}
		if common.Unmarshal(task.Data, &taskData) == nil && taskData.Video != nil {
			if taskData.Video.Duration > 0 {
				ov.SetMetadata("duration", taskData.Video.Duration)
			}
		}
	} else if task.Status == model.TaskStatusFailure && task.FailReason != "" {
		ov.Error = &dto.OpenAIVideoError{
			Message: task.FailReason,
			Code:    "generation_failed",
		}
	}

	return common.Marshal(ov)
}
