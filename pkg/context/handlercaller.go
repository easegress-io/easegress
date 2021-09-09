package context

type (
	// HandlerCaller is a helper function to call the handler
	HandlerCaller func(lastResult string) string
)
