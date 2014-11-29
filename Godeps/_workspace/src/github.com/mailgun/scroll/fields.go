package scroll

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
)

// Retrieve a POST request field as a string.
// Returns `MissingFieldError` if requested field is missing.
func GetStringField(r *http.Request, fieldName string) (string, error) {
	if _, ok := r.Form[fieldName]; !ok {
		return "", MissingFieldError{fieldName}
	}
	return r.FormValue(fieldName), nil
}

// Retrieve a POST request field as a string.
// If the requested field is missing, returns provided default value.
func GetStringFieldWithDefault(r *http.Request, fieldName, defaultValue string) string {
	if fieldValue, err := GetStringField(r, fieldName); err == nil {
		return fieldValue
	}
	return defaultValue
}

// Retrieve fields with the same name as an array of strings.
func GetMultipleFields(r *http.Request, fieldName string) ([]string, error) {
	value, ok := r.Form[fieldName]
	if !ok {
		return []string{}, MissingFieldError{fieldName}
	}
	return value, nil
}

// Retrieve a POST request field as an integer.
// Returns `MissingFieldError` if requested field is missing.
func GetIntField(r *http.Request, fieldName string) (int, error) {
	stringField, err := GetStringField(r, fieldName)
	if err != nil {
		return 0, err
	}
	intField, err := strconv.Atoi(stringField)
	if err != nil {
		return 0, InvalidFormatError{fieldName, stringField}
	}
	return intField, nil
}

// Helper method to retrieve an optional timestamp from POST request field.
// If no timestamp provided, returns current time.
// Returns `InvalidFormatError` if provided timestamp can't be parsed.
func GetTimestampField(r *http.Request, fieldName string) (time.Time, error) {
	if _, ok := r.Form[fieldName]; !ok {
		return time.Now(), MissingFieldError{fieldName}
	}
	parsedTime, err := time.Parse(time.RFC1123, r.FormValue(fieldName))
	if err != nil {
		log.Infof("Failed to convert timestamp %v: %v", r.FormValue(fieldName), err)
		return time.Now(), InvalidFormatError{fieldName, r.FormValue(fieldName)}
	}
	return parsedTime, nil
}

// GetDurationField retrieves a request field as a time.Duration, which is not allowed to be negative.
// Returns `MissingFieldError` if requested field is missing.
func GetDurationField(r *http.Request, fieldName string) (time.Duration, error) {
	s, err := GetStringField(r, fieldName)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, InvalidFormatError{fieldName, s}
	}
	return d, nil
}
