package api

import "fmt"

type GenericAPIError struct {
	Reason string
}

func (e GenericAPIError) Error() string {
	return e.Reason
}

type MissingFieldError struct {
	Field string
}

func (e MissingFieldError) Error() string {
	return fmt.Sprintf("Missing mandatory parameter: %v", e.Field)
}

type InvalidFormatError struct {
	Field string
	Value string
}

func (e InvalidFormatError) Error() string {
	return fmt.Sprintf("Invalid %v format: %v", e.Field, e.Value)
}

type InvalidParameterError struct {
	Field string
	Value string
}

func (e InvalidParameterError) Error() string {
	return fmt.Sprintf("Invalid parameter: %v %v", e.Field, e.Value)
}

type NotFoundError struct {
	Description string
}

func (e NotFoundError) Error() string {
	return e.Description
}

type ConflictError struct {
	Description string
}

func (e ConflictError) Error() string {
	return e.Description
}
