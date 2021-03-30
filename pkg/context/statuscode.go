package context

import "net/http"

const (
	EGStatusContinue              = http.StatusContinue
	EGStatusSwitchingProtocols    = http.StatusSwitchingProtocols
	EGStatusProcessing            = http.StatusProcessing
	EGStatusOK                    = http.StatusOK
	EGStatusCreated               = http.StatusCreated
	EGStatusAccepted              = http.StatusAccepted
	EGStatusNoContent             = http.StatusNoContent
	EGStatusPartialContent        = http.StatusPartialContent
	EGStatusSpecialResponse       = 300 // http.StatusMultipleChoices
	EGStatusMovedPermanently      = http.StatusMovedPermanently
	EGStatusMovedTemporarily      = 302 // http.StatusFound
	EGStatusSeeOther              = http.StatusSeeOther
	EGStatusNotModified           = http.StatusNotModified
	EGStatusTemporaryRedirect     = http.StatusTemporaryRedirect
	EGStatusBadRequest            = http.StatusBadRequest
	EGStatusUnauthorized          = http.StatusUnauthorized
	EGStatusForbidden             = http.StatusForbidden
	EGStatusNotFound              = http.StatusNotFound
	EGStatusNotAllowed            = http.StatusMethodNotAllowed
	EGStatusRequestTimeOut        = http.StatusRequestTimeout
	EGStatusConflict              = http.StatusConflict
	EGStatusLengthRequired        = http.StatusLengthRequired
	EGStatusPreconditionFailed    = http.StatusPreconditionFailed
	EGStatusRequestEntityTooLarge = http.StatusRequestEntityTooLarge
	EGStatusRequestURITooLong     = http.StatusRequestURITooLong
	EGStatusUnsupportedMediaType  = http.StatusUnsupportedMediaType
	EGStatusRangeNotSatisfiable   = http.StatusRequestedRangeNotSatisfiable
	EGStatusTooManyRequests       = http.StatusTooManyRequests
	EGStatusClose                 = 444
	EGStatusEgCodes               = 494
	EGStatusRequestHeaderTooLarge = 494
	EGStatusHTTPSCertError        = 495
	EGStatusHTTPSNoCert           = 496
	EGStatusToHTTPS               = 497
	EGStatusClientClosedRequest   = 499
	EGStatusInternalServerError   = http.StatusInternalServerError
	EGStatusNotImplemented        = http.StatusNotImplemented
	EGStatusBadGateway            = http.StatusBadGateway
	EGStatusServiceUnavailable    = http.StatusServiceUnavailable
	EGStatusGatewayTimeOut        = http.StatusGatewayTimeout
	EGStatusInsufficientStorage   = http.StatusInsufficientStorage
	EGStatusBadResponse           = 599
)

// categories of status code
const (
	egNetworkError = 1 << iota
)

// map status code to categories, note one status code could belong to
// multiple categories
var (
	statusCodeCategory = map[int]uint8{
		EGStatusClientClosedRequest: egNetworkError,
		EGStatusServiceUnavailable:  egNetworkError,
		EGStatusBadGateway:          egNetworkError,
		EGStatusGatewayTimeOut:      egNetworkError,
		EGStatusRequestTimeOut:      egNetworkError,
	}
)

func IsNetworkError(code int) bool {
	c, ok := statusCodeCategory[code]
	return ok && (c&egNetworkError) != 0
}
