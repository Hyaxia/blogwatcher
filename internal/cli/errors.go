package cli

import "errors"

type printedError struct {
	err error
}

func (e printedError) Error() string {
	return e.err.Error()
}

func markError(err error) error {
	if err == nil {
		return nil
	}
	return printedError{err: err}
}

func isPrinted(err error) bool {
	var printed printedError
	return errors.As(err, &printed)
}
