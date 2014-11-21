// Package template consolidates various templating utilities used throughout different
// parts of vulcan.
package template

import (
	"bytes"
	"net/http"
	"text/template"
)

// Data represents template data that is available to use in templates.
type Data struct {
	Request *http.Request
}

// Apply takes a template string in the http://golang.org/pkg/text/template/ format and
// returns a new string with data applied to the original string.
//
// In case of any error the original string is returned.
func Apply(original string, data Data) (string, error) {
	t, err := template.New("t").Parse(original)
	if err != nil {
		return original, err
	}

	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return original, err
	}

	return b.String(), nil
}
