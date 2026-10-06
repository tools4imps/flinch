package ops

import "errors"

// Check's error is this file's only use of errors, so making it nil outright would strand the import.
func Check(n int) error {
	if n > 9 {
		return errors.New("too big")
	}
	return nil
}
