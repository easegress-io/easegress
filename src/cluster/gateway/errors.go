package gateway

import "fmt"

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
