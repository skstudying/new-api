package volcvideo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"one-api/common"
	"one-api/constant"
	"one-api/dto"
	"one-api/model"
	"one-api/relay/channel"
	relaycommon "one-api/relay/common"
	"one-api/service"
)

// ============================
// Request / Response structures (Volc Ark Video)
// ============================

type contentImageURL struct {
	URL string `json:"url"`
}

type contentItem struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	ImageURL *contentImageURL `json:"image_url,omitempty"`
	Role     string           `json:"role,omitempty"`
}

type submitRequest struct {
	Model           string        `json:"model"`
	Content         []contentItem `json:"content"`
	CallbackURL     string        `json:"callback_url,omitempty"`
	ReturnLastFrame bool          `json:"return_last_frame,omitempty"`
}

type submitResponse struct {
	ID string `json:"id"`
}

type fetchResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	Error           struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param"`
	} `json:"error,omitempty"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	ChannelType int
	baseURL     string
	apiKey      string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	// Only submit via POST /v1/video/generations
	info.Action = constant.TaskActionGenerate
	// Accept generic TaskSubmitReq
	req := relaycommon.TaskSubmitReq{}
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" && req.Image == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("prompt or image is required"), "invalid_request", http.StatusBadRequest)
	}
	// Store for body build
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, ok := c.Get("task_request")
	if !ok {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)

	// 使用映射后的模型名称
	modelName := req.Model
	if info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	}
	body := submitRequest{Model: modelName}
	// text
	if strings.TrimSpace(req.Prompt) != "" {
		body.Content = append(body.Content, contentItem{Type: "text", Text: req.Prompt})
	}
	// images handling: simple image or images in metadata
	if req.Image != "" {
		role := "first_frame"
		if r, ok := req.Metadata["role"].(string); ok && r != "" {
			role = r
		}
		body.Content = append(body.Content, contentItem{
			Type:     "image_url",
			ImageURL: &contentImageURL{URL: req.Image},
			Role:     role,
		})
	}
	if imgs, ok := req.Metadata["images"].([]any); ok {
		for _, it := range imgs {
			m, _ := it.(map[string]any)
			u := fmt.Sprint(m["url"])
			if u == "" {
				continue
			}
			role := fmt.Sprint(m["role"])
			body.Content = append(body.Content, contentItem{Type: "image_url", ImageURL: &contentImageURL{URL: u}, Role: role})
		}
	}
	if cb, ok := req.Metadata["callback_url"].(string); ok {
		body.CallbackURL = cb
	}
	if rlf, ok := req.Metadata["return_last_frame"].(bool); ok {
		body.ReturnLastFrame = rlf
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, _ *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()
	var sr submitResponse
	if err := json.Unmarshal(responseBody, &sr); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if sr.ID == "" {
		return "", nil, service.TaskErrorWrapperLocal(fmt.Errorf("empty task id"), "invalid_response", http.StatusInternalServerError)
	}
	c.JSON(http.StatusOK, gin.H{"task_id": sr.ID})
	return sr.ID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	url := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	return service.GetHttpClient().Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"doubao-seedance-pro-250528",
		"doubao-seedance-1-0-lite-t2v-250428",
		"doubao-seedance-1-0-lite-i2v-250428",
		"wan2-1-14b-t2v-250428",
		"wan2-1-14b-i2v-250428",
		"wan2-1-14b-flf2v-250428",
	}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "volcvideo"
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var fr fetchResponse
	if err := json.Unmarshal(respBody, &fr); err != nil {
		return nil, err
	}
	res := &relaycommon.TaskInfo{}
	res.TaskID = fr.ID

	// 检查是否有错误信息（API错误响应）
	if fr.Error.Code != "" {
		res.Status = model.TaskStatusFailure
		res.Progress = "100%"
		res.Reason = fmt.Sprintf("%s: %s", fr.Error.Code, fr.Error.Message)
		return res, nil
	}

	// 检查是否有状态信息（正常任务响应）
	if fr.Status == "" {
		res.Status = model.TaskStatusUnknown
		res.Progress = "0%"
		res.Reason = "未知响应格式"
		return res, nil
	}

	switch strings.ToLower(fr.Status) {
	case "queued":
		res.Status = model.TaskStatusQueued
		res.Progress = "20%"
	case "running":
		res.Status = model.TaskStatusInProgress
		res.Progress = "60%"
	case "succeeded":
		res.Status = model.TaskStatusSuccess
		res.Progress = "100%"
		res.Url = fr.Content.VideoURL
		// 设置成功时的详细信息到 Reason 字段（JSON格式）
		successData := map[string]interface{}{
			"id":              fr.ID,
			"model":           fr.Model,
			"status":          fr.Status,
			"content":         fr.Content,
			"usage":           fr.Usage,
			"created_at":      fr.CreatedAt,
			"updated_at":      fr.UpdatedAt,
			"seed":            fr.Seed,
			"resolution":      fr.Resolution,
			"duration":        fr.Duration,
			"ratio":           fr.Ratio,
			"framespersecond": fr.FramesPerSecond,
		}
		if dataBytes, err := json.Marshal(successData); err == nil {
			res.Reason = string(dataBytes)
		}
	case "failed":
		res.Status = model.TaskStatusFailure
		res.Progress = "100%"
		// 设置失败原因
		if fr.Error.Message != "" {
			res.Reason = fr.Error.Message
		} else {
			res.Reason = "任务执行失败"
		}
	default:
		res.Status = model.TaskStatusUnknown
		res.Progress = "0%"
		res.Reason = fmt.Sprintf("未知状态: %s", fr.Status)
	}

	return res, nil
}
