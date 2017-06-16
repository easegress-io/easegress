package gateway

import "fmt"

type ClusterErrorType uint8

const (
	NoneError ClusterErrorType = iota

	WrongMessageFormatError
	InternalServerError
	TimeoutError

	OperationLogHugeGapError
	OperationSeqConflictError
	OperationInvalidSeqError
	OperationInvalidContentError
	OperationPartiallyCompleteError

	RetrieveInconsistencyError
	RetrievePluginsError
	RetrievePipelinesError

	PipelineStatNotFoundError
	RetrievePipelineStatValueError
	RetrievePipelineStatDescError
	RetrievePluginStatValueError
	RetrievePluginStatDescError
	RetrieveTaskStatValueError
	RetrieveTaskStatDescError
)

////

type HTTPError struct {
	Msg        string
	StatusCode int
}

func NewHTTPError(msg string, code int) *HTTPError {
	return &HTTPError{
		Msg:        msg,
		StatusCode: code,
	}
}

func (err HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", err.StatusCode, err.Msg)
}
