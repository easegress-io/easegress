package gateway

import (
	"fmt"
	"net/http"
)

type ClusterErrorType uint8

const (
	NoneClusterError ClusterErrorType = iota

	WrongMessageFormatError
	InternalServerError
	TimeoutError
	IssueMemberGoneError

	OperationLogHugeGapError
	OperationSeqConflictError
	OperationInvalidSeqError
	OperationInvalidContentError
	OperationGeneralFailureError
	OperationTargetNotFoundFailureError
	OperationNotAcceptableFailureError
	OperationConflictFailureError
	OperationUnknownFailureError
	OperationPartiallyCompleteError

	RetrieveInconsistencyError
	RetrievePipelinesError
	RetrievePluginsError
	RetrievePipelineNotFoundError
	RetrievePluginNotFoundError

	PipelineStatNotFoundError
	RetrievePipelineStatIndicatorNotFoundError
	RetrievePipelineStatValueError
	RetrievePipelineStatDescError
	RetrievePluginStatIndicatorNotFoundError
	RetrievePluginStatValueError
	RetrievePluginStatDescError
	RetrieveTaskStatIndicatorNotFoundError
	RetrieveTaskStatValueError
	RetrieveTaskStatDescError
)

func (t ClusterErrorType) HTTPStatusCode() int {
	ret := http.StatusInternalServerError

	switch t {
	case NoneClusterError:
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
	case OperationInvalidSeqError:
		ret = http.StatusBadRequest
	case OperationInvalidContentError:
		ret = http.StatusBadRequest
	case OperationGeneralFailureError:
		ret = http.StatusBadRequest
	case OperationTargetNotFoundFailureError:
		ret = http.StatusNotFound
	case OperationNotAcceptableFailureError:
		ret = http.StatusNotAcceptable
	case OperationConflictFailureError:
		ret = http.StatusConflict
	case OperationUnknownFailureError:
		ret = http.StatusInternalServerError
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
	case RetrievePipelineStatIndicatorNotFoundError:
		ret = http.StatusNotFound
	case RetrievePipelineStatValueError:
		ret = http.StatusForbidden
	case RetrievePipelineStatDescError:
		ret = http.StatusForbidden
	case RetrievePluginStatIndicatorNotFoundError:
		ret = http.StatusNotFound
	case RetrievePluginStatValueError:
		ret = http.StatusForbidden
	case RetrievePluginStatDescError:
		ret = http.StatusForbidden
	case RetrieveTaskStatIndicatorNotFoundError:
		ret = http.StatusNotFound
	case RetrieveTaskStatValueError:
		ret = http.StatusForbidden
	case RetrieveTaskStatDescError:
		ret = http.StatusForbidden
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

////

type OperationFailureType uint8

const (
	NoneOperationFailure = iota

	OperationGeneralFailure
	OperationTargetNotFoundFailure
	OperationNotAcceptableFailure
	OperationConflictFailure
	OperationUnknownFailure
)
