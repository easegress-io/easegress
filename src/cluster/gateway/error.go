package gateway

import (
	"fmt"
	"net/http"
)

type ClusterErrorType uint8

const (
	NoneError ClusterErrorType = iota

	WrongMessageFormatError
	InternalServerError
	TimeoutError
	IssueMemberGoneError

	OperationLogHugeGapError
	OperationSeqConflictError
	OperationInvalidSeqError
	OperationInvalidContentError
	OperationFailureError
	OperationPartiallyCompleteError

	RetrieveInconsistencyError
	RetrievePipelinesError
	RetrievePluginsError
	RetrievePipelineNotFoundError
	RetrievePluginNotFoundError

	PipelineStatNotFoundError
	RetrievePipelineStatValueError
	RetrievePipelineStatDescError
	RetrievePluginStatValueError
	RetrievePluginStatDescError
	RetrieveTaskStatValueError
	RetrieveTaskStatDescError
)

func (t ClusterErrorType) HTTPStatusCode() int {
	ret := http.StatusInternalServerError

	switch t {
	case NoneError:
		ret = http.StatusOK

	case WrongMessageFormatError:
		ret = http.StatusBadRequest
	case InternalServerError:
		ret = http.StatusInternalServerError
	case TimeoutError:
		ret = http.StatusRequestTimeout
	case IssueMemberGoneError:
		ret = http.StatusGone

	case OperationSeqConflictError:
		ret = http.StatusConflict
	case OperationInvalidContentError:
		ret = http.StatusBadRequest
	case OperationFailureError:
		ret = http.StatusBadRequest
	case OperationPartiallyCompleteError:
		ret = http.StatusAccepted

	case RetrieveInconsistencyError:
		ret = http.StatusConflict
	case RetrievePipelinesError:
		ret = http.StatusBadRequest
	case RetrievePluginsError:
		ret = http.StatusBadRequest
	case RetrievePipelineNotFoundError:
		ret = http.StatusNotFound
	case RetrievePluginNotFoundError:
		ret = http.StatusNotFound

	case PipelineStatNotFoundError:
		ret = http.StatusNotFound
	case RetrievePipelineStatValueError:
		ret = http.StatusInternalServerError
	case RetrievePipelineStatDescError:
		ret = http.StatusNotFound
	case RetrievePluginStatValueError:
		ret = http.StatusInternalServerError
	case RetrievePluginStatDescError:
		ret = http.StatusNotFound
	case RetrieveTaskStatValueError:
		ret = http.StatusInternalServerError
	case RetrieveTaskStatDescError:
		ret = http.StatusNotFound
	}

	return ret
}

////

type ClusterError struct {
	Type    ClusterErrorType
	Message string
}

func (e *ClusterError) Error() string {
	if e == nil {
		return ""
	}

	return fmt.Sprintf("%s (cluster error type=%d)", e.Message, e.Type)
}

func newClusterError(msg string, errorType ClusterErrorType) *ClusterError {
	return &ClusterError{
		Message: msg,
		Type:    errorType,
	}
}
