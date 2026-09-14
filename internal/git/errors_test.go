package git

import "errors"

// errorAs wraps errors.As for the tests, keeping their import lists small.
func errorAs(err error, target any) bool { return errors.As(err, target) }
