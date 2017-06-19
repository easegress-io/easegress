package task

import "net/http"

type TaskResultCode uint

var (
	availableResultCodes = map[string]TaskResultCode{
		"ResultOK":                  http.StatusOK,
		"ResultUnknownError":        http.StatusNotImplemented,
		"ResultServiceUnavailable":  http.StatusServiceUnavailable,
		"ResultInternalServerError": http.StatusInternalServerError,
		"ResultTaskCancelled":       http.StatusBadGateway,
		"ResultMissingInput":        498,
		"ResultBadInput":            http.StatusBadRequest,
		"ResultRequesterGone":       499,
		"ResultFlowControl":         http.StatusTooManyRequests,
	}

	ResultOK                  TaskResultCode = availableResultCodes["ResultOK"]
	ResultUnknownError        TaskResultCode = availableResultCodes["ResultUnknownError"]
	ResultServiceUnavailable  TaskResultCode = availableResultCodes["ResultServiceUnavailable"]
	ResultInternalServerError TaskResultCode = availableResultCodes["ResultInternalServerError"]
	ResultTaskCancelled       TaskResultCode = availableResultCodes["ResultTaskCancelled"]
	ResultMissingInput        TaskResultCode = availableResultCodes["ResultMissingInput"]
	ResultBadInput            TaskResultCode = availableResultCodes["ResultBadInput"]
	ResultRequesterGone       TaskResultCode = availableResultCodes["ResultRequesterGone"]
	ResultFlowControl         TaskResultCode = availableResultCodes["ResultFlowControl"]
)

func SuccessfulResult(code TaskResultCode) bool {
	return code < 400
}

func ResultCodeToHTTPCode(code TaskResultCode) int {
	httpCode := -1

	switch code {
	case ResultMissingInput:
		httpCode = http.StatusServiceUnavailable
	default:
		httpCode = int(code)
	}

	return httpCode
}

func ValidResultCodeName(name string) bool {
	_, ok := availableResultCodes[name]
	return ok
}

func ValidResultCode(code TaskResultCode) bool {
	for _, c := range availableResultCodes {
		if c == code {
			return true
		}
	}
	return false
}

func ResultCodeValue(name string) TaskResultCode {
	if !ValidResultCodeName(name) {
		return ResultUnknownError
	}
	return availableResultCodes[name]
}
