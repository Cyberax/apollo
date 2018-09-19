package data


type StoreError interface {
	error
	// Returns the error details message.
	Message() string
	// Returns the original error if one was set.  Nil is returned if not set.
	OrigErr() error
}

type storeError struct {
	message string
	origErr error
}

func (e storeError) Error() string {
	return e.Message()
}

func (e storeError) Message() string {
	msg := e.message
	if e.origErr == nil {
		return msg
	}
	orig := e.origErr.Error()
	if orig != "" {
		return msg + ", caused by " + orig
	}
	return msg
}

func (e storeError) OrigErr() error {
	return e.origErr
}

func NewStoreError(message string, orig error) StoreError {
	return storeError{message: message, origErr: orig}
}
