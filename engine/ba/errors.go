package ba

import "fmt"

type paramError struct{ msg string }

func (e paramError) Error() string { return "ba: invalid parameters: " + e.msg }

func errParam(format string, a ...any) error {
	return paramError{msg: fmt.Sprintf(format, a...)}
}
