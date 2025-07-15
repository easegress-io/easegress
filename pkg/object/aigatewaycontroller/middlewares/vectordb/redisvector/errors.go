package redisvector

type ErrParsingRedisURL struct {
	Message string
	Err     error
}

// NewErrParsingRedisURL creates a new ErrParsingRedisURL with the given message and error.
func NewErrParsingRedisURL(message string, err error) *ErrParsingRedisURL {
	return &ErrParsingRedisURL{
		Message: message,
		Err:     err,
	}
}

func (e *ErrParsingRedisURL) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreateRedisClient struct {
	Message string
	Err     error
}

// NewErrCreateRedisClient creates a new ErrCreateRedisClient with the given message and error.
func NewErrCreateRedisClient(message string, err error) *ErrCreateRedisClient {
	return &ErrCreateRedisClient{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreateRedisClient) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrUnexpectedIndexSchema struct {
	Message string
	Err     error
}

// NewErrUnexpectedIndexSchema creates a new ErrUnexpectedIndexSchema with the given message and error.
func NewErrUnexpectedIndexSchema(message string, err error) *ErrUnexpectedIndexSchema {
	return &ErrUnexpectedIndexSchema{
		Message: message,
		Err:     err,
	}
}

func (e *ErrUnexpectedIndexSchema) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreateRedisIndex struct {
	Message string
	Err     error
}

// NewErrCreateRedisIndex creates a new ErrCreateRedisIndex with the given message and error.
func NewErrCreateRedisIndex(message string, err error) *ErrCreateRedisIndex {
	return &ErrCreateRedisIndex{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreateRedisIndex) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrInsertDocument struct {
	Message string
	Err     error
}

// NewErrInsertDocument creates a new ErrInsertDocument with the given message and error.
func NewErrInsertDocument(message string, err error) *ErrInsertDocument {
	return &ErrInsertDocument{
		Message: message,
		Err:     err,
	}
}

func (e *ErrInsertDocument) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type InvalidScoreThreshold struct{}

// NewInvalidScoreThreshold creates a new InvalidScoreThreshold error.
func NewInvalidScoreThreshold() *InvalidScoreThreshold {
	return &InvalidScoreThreshold{}
}

func (e *InvalidScoreThreshold) Error() string {
	return "invalid score threshold: must be between 0 and 1"
}
