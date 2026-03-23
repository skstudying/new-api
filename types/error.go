package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type OpenAIError struct {
	Message  string          `json:"message"`
	Type     string          `json:"type"`
	Param    string          `json:"param"`
	Code     any             `json:"code"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type ClaudeError struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

type ErrorType string

const (
	ErrorTypeNewAPIError     ErrorType = "new_api_error"
	ErrorTypeOpenAIError     ErrorType = "openai_error"
	ErrorTypeClaudeError     ErrorType = "claude_error"
	ErrorTypeMidjourneyError ErrorType = "midjourney_error"
	ErrorTypeGeminiError     ErrorType = "gemini_error"
	ErrorTypeRerankError     ErrorType = "rerank_error"
	ErrorTypeUpstreamError   ErrorType = "upstream_error"
)

type ErrorCode string

const (
	ErrorCodeInvalidRequest         ErrorCode = "invalid_request"
	ErrorCodeSensitiveWordsDetected ErrorCode = "sensitive_words_detected"
	ErrorCodeViolationFeeGrokCSAM   ErrorCode = "violation_fee.grok.csam"

	// new api error
	ErrorCodeCountTokenFailed   ErrorCode = "count_token_failed"
	ErrorCodeModelPriceError    ErrorCode = "model_price_error"
	ErrorCodeInvalidApiType     ErrorCode = "invalid_api_type"
	ErrorCodeJsonMarshalFailed  ErrorCode = "json_marshal_failed"
	ErrorCodeDoRequestFailed    ErrorCode = "do_request_failed"
	ErrorCodeGetChannelFailed   ErrorCode = "get_channel_failed"
	ErrorCodeGenRelayInfoFailed ErrorCode = "gen_relay_info_failed"

	// channel error
	ErrorCodeChannelNoAvailableKey        ErrorCode = "channel:no_available_key"
	ErrorCodeChannelParamOverrideInvalid  ErrorCode = "channel:param_override_invalid"
	ErrorCodeChannelHeaderOverrideInvalid ErrorCode = "channel:header_override_invalid"
	ErrorCodeChannelModelMappedError      ErrorCode = "channel:model_mapped_error"
	ErrorCodeChannelAwsClientError        ErrorCode = "channel:aws_client_error"
	ErrorCodeChannelInvalidKey            ErrorCode = "channel:invalid_key"
	ErrorCodeChannelResponseTimeExceeded  ErrorCode = "channel:response_time_exceeded"

	// client request error
	ErrorCodeReadRequestBodyFailed ErrorCode = "read_request_body_failed"
	ErrorCodeConvertRequestFailed  ErrorCode = "convert_request_failed"
	ErrorCodeAccessDenied          ErrorCode = "access_denied"

	// request error
	ErrorCodeBadRequestBody ErrorCode = "bad_request_body"

	// response error
	ErrorCodeReadResponseBodyFailed ErrorCode = "read_response_body_failed"
	ErrorCodeBadResponseStatusCode  ErrorCode = "bad_response_status_code"
	ErrorCodeBadResponse            ErrorCode = "bad_response"
	ErrorCodeBadResponseBody        ErrorCode = "bad_response_body"
	ErrorCodeEmptyResponse          ErrorCode = "empty_response"
	ErrorCodeAwsInvokeError         ErrorCode = "aws_invoke_error"
	ErrorCodeModelNotFound          ErrorCode = "model_not_found"
	ErrorCodePromptBlocked          ErrorCode = "prompt_blocked"

	// sql error
	ErrorCodeQueryDataError  ErrorCode = "query_data_error"
	ErrorCodeUpdateDataError ErrorCode = "update_data_error"

	// quota error
	ErrorCodeInsufficientUserQuota      ErrorCode = "insufficient_user_quota"
	ErrorCodePreConsumeTokenQuotaFailed ErrorCode = "pre_consume_token_quota_failed"
)

type NewAPIError struct {
	Err            error
	RelayError     any
	skipRetry      bool
	recordErrorLog *bool
	errorType      ErrorType
	errorCode      ErrorCode
	StatusCode     int
	Metadata       json.RawMessage
}

// Unwrap enables errors.Is / errors.As to work with NewAPIError by exposing the underlying error.
func (e *NewAPIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *NewAPIError) GetErrorCode() ErrorCode {
	if e == nil {
		return ""
	}
	return e.errorCode
}

func (e *NewAPIError) GetErrorType() ErrorType {
	if e == nil {
		return ""
	}
	return e.errorType
}

func (e *NewAPIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		// fallback message when underlying error is missing
		return string(e.errorCode)
	}
	return e.Err.Error()
}

func (e *NewAPIError) ErrorWithStatusCode() string {
	if e == nil {
		return ""
	}
	msg := e.Error()
	if e.StatusCode == 0 {
		return msg
	}
	if msg == "" {
		return fmt.Sprintf("status_code=%d", e.StatusCode)
	}
	return fmt.Sprintf("status_code=%d, %s", e.StatusCode, msg)
}

func (e *NewAPIError) MaskSensitiveError() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.errorCode)
	}
	errStr := e.Err.Error()
	if e.errorCode == ErrorCodeCountTokenFailed {
		return errStr
	}
	return common.MaskSensitiveInfo(errStr)
}

func (e *NewAPIError) MaskSensitiveErrorWithStatusCode() string {
	if e == nil {
		return ""
	}
	msg := e.MaskSensitiveError()
	if e.StatusCode == 0 {
		return msg
	}
	if msg == "" {
		return fmt.Sprintf("status_code=%d", e.StatusCode)
	}
	return fmt.Sprintf("status_code=%d, %s", e.StatusCode, msg)
}

func (e *NewAPIError) SetMessage(message string) {
	e.Err = errors.New(message)
}

func (e *NewAPIError) ToOpenAIError() OpenAIError {
	var result OpenAIError
	switch e.errorType {
	case ErrorTypeOpenAIError:
		if openAIError, ok := e.RelayError.(OpenAIError); ok {
			result = openAIError
		}
	case ErrorTypeClaudeError:
		if claudeError, ok := e.RelayError.(ClaudeError); ok {
			result = OpenAIError{
				Message: e.Error(),
				Type:    claudeError.Type,
				Param:   "",
				Code:    e.errorCode,
			}
		}
	default:
		result = OpenAIError{
			Message: e.Error(),
			Type:    string(e.errorType),
			Param:   "",
			Code:    e.errorCode,
		}
	}
	if e.errorCode != ErrorCodeCountTokenFailed {
		result.Message = common.MaskSensitiveInfo(result.Message)
	}
	if result.Message == "" {
		result.Message = string(e.errorType)
	}
	return result
}

func (e *NewAPIError) ToClaudeError() ClaudeError {
	var result ClaudeError
	switch e.errorType {
	case ErrorTypeOpenAIError:
		if openAIError, ok := e.RelayError.(OpenAIError); ok {
			result = ClaudeError{
				Message: e.Error(),
				Type:    fmt.Sprintf("%v", openAIError.Code),
			}
		}
	case ErrorTypeClaudeError:
		if claudeError, ok := e.RelayError.(ClaudeError); ok {
			result = claudeError
		}
	default:
		result = ClaudeError{
			Message: e.Error(),
			Type:    string(e.errorType),
		}
	}
	if e.errorCode != ErrorCodeCountTokenFailed {
		result.Message = common.MaskSensitiveInfo(result.Message)
	}
	if result.Message == "" {
		result.Message = string(e.errorType)
	}
	return result
}

type NewAPIErrorOptions func(*NewAPIError)

func NewError(err error, errorCode ErrorCode, ops ...NewAPIErrorOptions) *NewAPIError {
	var newErr *NewAPIError
	// 保留深层传递的 new err
	if errors.As(err, &newErr) {
		for _, op := range ops {
			op(newErr)
		}
		return newErr
	}
	e := &NewAPIError{
		Err:        err,
		RelayError: nil,
		errorType:  ErrorTypeNewAPIError,
		StatusCode: http.StatusInternalServerError,
		errorCode:  errorCode,
	}
	for _, op := range ops {
		op(e)
	}
	return e
}

func NewOpenAIError(err error, errorCode ErrorCode, statusCode int, ops ...NewAPIErrorOptions) *NewAPIError {
	var newErr *NewAPIError
	// 保留深层传递的 new err
	if errors.As(err, &newErr) {
		if newErr.RelayError == nil {
			openaiError := OpenAIError{
				Message: newErr.Error(),
				Type:    string(errorCode),
				Code:    errorCode,
			}
			newErr.RelayError = openaiError
		}
		for _, op := range ops {
			op(newErr)
		}
		return newErr
	}
	openaiError := OpenAIError{
		Message: err.Error(),
		Type:    string(errorCode),
		Code:    errorCode,
	}
	return WithOpenAIError(openaiError, statusCode, ops...)
}

func InitOpenAIError(errorCode ErrorCode, statusCode int, ops ...NewAPIErrorOptions) *NewAPIError {
	openaiError := OpenAIError{
		Type: string(errorCode),
		Code: errorCode,
	}
	return WithOpenAIError(openaiError, statusCode, ops...)
}

func NewErrorWithStatusCode(err error, errorCode ErrorCode, statusCode int, ops ...NewAPIErrorOptions) *NewAPIError {
	e := &NewAPIError{
		Err: err,
		RelayError: OpenAIError{
			Message: err.Error(),
			Type:    string(errorCode),
		},
		errorType:  ErrorTypeNewAPIError,
		StatusCode: statusCode,
		errorCode:  errorCode,
	}
	for _, op := range ops {
		op(e)
	}

	return e
}

func WithOpenAIError(openAIError OpenAIError, statusCode int, ops ...NewAPIErrorOptions) *NewAPIError {
	code, ok := openAIError.Code.(string)
	if !ok {
		if openAIError.Code != nil {
			code = fmt.Sprintf("%v", openAIError.Code)
		} else {
			code = "unknown_error"
		}
	}
	if openAIError.Type == "" {
		openAIError.Type = "upstream_error"
	}
	e := &NewAPIError{
		RelayError: openAIError,
		errorType:  ErrorTypeOpenAIError,
		StatusCode: statusCode,
		Err:        errors.New(openAIError.Message),
		errorCode:  ErrorCode(code),
	}
	// OpenRouter
	if len(openAIError.Metadata) > 0 {
		openAIError.Message = fmt.Sprintf("%s (%s)", openAIError.Message, openAIError.Metadata)
		e.Metadata = openAIError.Metadata
		e.RelayError = openAIError
		e.Err = errors.New(openAIError.Message)
	}
	for _, op := range ops {
		op(e)
	}
	return e
}

func WithClaudeError(claudeError ClaudeError, statusCode int, ops ...NewAPIErrorOptions) *NewAPIError {
	if claudeError.Type == "" {
		claudeError.Type = "upstream_error"
	}
	e := &NewAPIError{
		RelayError: claudeError,
		errorType:  ErrorTypeClaudeError,
		StatusCode: statusCode,
		Err:        errors.New(claudeError.Message),
		errorCode:  ErrorCode(claudeError.Type),
	}
	for _, op := range ops {
		op(e)
	}
	return e
}

func IsChannelError(err *NewAPIError) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(string(err.errorCode), "channel:")
}

func IsSkipRetryError(err *NewAPIError) bool {
	if err == nil {
		return false
	}

	return err.skipRetry
}

func ErrOptionWithSkipRetry() NewAPIErrorOptions {
	return func(e *NewAPIError) {
		e.skipRetry = true
	}
}

func ErrOptionWithNoRecordErrorLog() NewAPIErrorOptions {
	return func(e *NewAPIError) {
		e.recordErrorLog = common.GetPointer(false)
	}
}

func ErrOptionWithHideErrMsg(replaceStr string) NewAPIErrorOptions {
	return func(e *NewAPIError) {
		if common.DebugEnabled {
			fmt.Printf("ErrOptionWithHideErrMsg: %s, origin error: %s", replaceStr, e.Err)
		}
		e.Err = errors.New(replaceStr)
	}
}

func IsRecordErrorLog(e *NewAPIError) bool {
	if e == nil {
		return false
	}
	if e.recordErrorLog == nil {
		// default to true if not set
		return true
	}
	return *e.recordErrorLog
}

var anthropicOfficialErrorTypes = map[string]bool{
	"invalid_request_error": true,
	"authentication_error":  true,
	"permission_error":      true,
	"not_found_error":       true,
	"request_too_large":     true,
	"rate_limit_error":      true,
	"api_error":             true,
	"overloaded_error":      true,
}

var requestIdPattern = regexp.MustCompile(`\s*\(request id: [^)]*\)`)

func StripUpstreamRequestId(msg string) string {
	return strings.TrimSpace(requestIdPattern.ReplaceAllString(msg, ""))
}

// looksLikeUpstreamProviderError 识别经上游 new-api 转发的 AWS Bedrock / SDK 等错误正文。
// 上游把 Claude 错误里的 type 序列化成 "<nil>"（OpenAI Code 为 nil）时，不能仅靠 type 白名单判断。
func looksLikeUpstreamProviderError(message string) bool {
	if message == "" {
		return false
	}
	lower := strings.ToLower(message)
	// AWS SDK v2 / Bedrock
	if strings.Contains(lower, "bedrock") ||
		strings.Contains(lower, "invokemodel") ||
		strings.Contains(lower, "invoke model") ||
		strings.Contains(lower, "validationexception") ||
		strings.Contains(lower, "validation exception") ||
		strings.Contains(lower, "throttlingexception") ||
		strings.Contains(lower, "serviceunavailableexception") ||
		strings.Contains(lower, "accessdeniedexception") ||
		strings.Contains(lower, "resource not found") ||
		(strings.Contains(lower, "operation error") && strings.Contains(lower, "bedrock")) {
		return true
	}
	// AWS Go SDK v2 常见格式（大小写不敏感）
	if strings.Contains(lower, "https response error") ||
		(strings.Contains(lower, "requestid") && strings.Contains(lower, "statuscode")) {
		return true
	}
	return false
}

func isNewAPIShapedGatewayError(oaiErr OpenAIError) bool {
	if strings.EqualFold(strings.TrimSpace(oaiErr.Type), "new_api_error") {
		return true
	}
	// 上游 OpenAI 形 JSON 里 code 常为字符串
	if codeStr, ok := oaiErr.Code.(string); ok {
		switch ErrorCode(codeStr) {
		case ErrorCodeModelNotFound, ErrorCodeGetChannelFailed, ErrorCodeModelPriceError,
			ErrorCodeInsufficientUserQuota, ErrorCodeCountTokenFailed:
			return true
		}
	}
	return false
}

// NormalizePassthroughClaudeErrorType 将 "<nil>"、空、upstream_error 等规范为 Anthropic 常见 type，便于客户端展示。
func NormalizePassthroughClaudeErrorType(typ string, statusCode int) string {
	if anthropicOfficialErrorTypes[typ] {
		return typ
	}
	switch typ {
	case "", "<nil>", "upstream_error", "unknown_error":
		if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError {
			return "invalid_request_error"
		}
		return "api_error"
	}
	if typ == string(ErrorCodeAwsInvokeError) || strings.EqualFold(typ, "aws_invoke_error") {
		return "api_error"
	}
	if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError {
		return "invalid_request_error"
	}
	return "api_error"
}

func IsOfficialAnthropicError(err *NewAPIError) bool {
	if err == nil {
		return false
	}
	if err.errorCode == ErrorCodeAwsInvokeError {
		return true
	}
	switch err.errorType {
	case ErrorTypeClaudeError:
		if claudeErr, ok := err.RelayError.(ClaudeError); ok {
			if isNewAPIShapedGatewayError(ClaudeErrorToOpenAIError(claudeErr)) {
				return false
			}
			if anthropicOfficialErrorTypes[claudeErr.Type] {
				return true
			}
			if looksLikeUpstreamProviderError(claudeErr.Message) {
				return true
			}
			if looksLikeUpstreamProviderError(err.Error()) {
				return true
			}
		}
	case ErrorTypeOpenAIError:
		if oaiErr, ok := err.RelayError.(OpenAIError); ok {
			if isNewAPIShapedGatewayError(oaiErr) {
				return false
			}
			// 上游 new-api Claude 包装里常无 code 字段 → WithOpenAIError 固定为 unknown_error，正文仍是 Bedrock/厂商错误
			if err.errorCode == "unknown_error" {
				return true
			}
			if anthropicOfficialErrorTypes[oaiErr.Type] {
				return true
			}
			if looksLikeUpstreamProviderError(oaiErr.Message) {
				return true
			}
			// Message 未写入 OpenAIError 时，Err 或 ToMessage 的文本可能在 Error() 里
			if looksLikeUpstreamProviderError(err.Error()) {
				return true
			}
			// HTTP 4xx 且来自 bad_response_status_code：多为上游 API 业务错误体
			if err.errorCode == ErrorCodeBadResponseStatusCode &&
				err.StatusCode >= http.StatusBadRequest && err.StatusCode < http.StatusInternalServerError {
				return true
			}
		}
	}
	return false
}

// ClaudeErrorToOpenAIError 仅用于网关形态判定（与 OpenAI 形错误共用 isNewAPIShapedGatewayError）
func ClaudeErrorToOpenAIError(c ClaudeError) OpenAIError {
	return OpenAIError{Type: c.Type, Message: c.Message}
}

func (e *NewAPIError) GetOfficialClaudeErrorData() (errType string, message string) {
	switch e.errorType {
	case ErrorTypeClaudeError:
		if claudeErr, ok := e.RelayError.(ClaudeError); ok {
			msg := claudeErr.Message
			if strings.TrimSpace(msg) == "" {
				msg = e.Error()
			}
			msg = StripUpstreamRequestId(msg)
			return NormalizePassthroughClaudeErrorType(claudeErr.Type, e.StatusCode), msg
		}
	case ErrorTypeOpenAIError:
		if oaiErr, ok := e.RelayError.(OpenAIError); ok {
			msg := oaiErr.Message
			if strings.TrimSpace(msg) == "" {
				msg = e.Error()
			}
			msg = StripUpstreamRequestId(msg)
			return NormalizePassthroughClaudeErrorType(oaiErr.Type, e.StatusCode), msg
		}
	}
	return NormalizePassthroughClaudeErrorType("", e.StatusCode), StripUpstreamRequestId(e.Error())
}
