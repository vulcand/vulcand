package threshold

import (
	"fmt"
	"regexp"

	"github.com/mailgun/predicate"
)

// Parses expression in the go language into Failover predicates
func ParseExpression(in string) (Predicate, error) {
	p, err := predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: AND,
			OR:  OR,
			EQ:  EQ,
			NEQ: NEQ,
			LT:  LT,
			LE:  LE,
			GT:  GT,
			GE:  GE,
		},
		Functions: map[string]interface{}{
			"RequestMethod":  RequestMethod,
			"IsNetworkError": IsNetworkError,
			"Attempts":       Attempts,
			"ResponseCode":   ResponseCode,
		},
	})
	if err != nil {
		return nil, err
	}
	out, err := p.Parse(convertLegacy(in))
	if err != nil {
		return nil, err
	}
	pr, ok := out.(Predicate)
	if !ok {
		return nil, fmt.Errorf("expected predicate, got %T", out)
	}
	return pr, nil
}

func convertLegacy(in string) string {
	patterns := []struct {
		Pattern     string
		Replacement string
	}{
		{
			Pattern:     `IsNetworkError([^\(]|$)`,
			Replacement: "IsNetworkError()",
		},
		{
			Pattern:     `ResponseCodeEq\((\d+)\)`,
			Replacement: "ResponseCode() == $1",
		},
		{
			Pattern:     `RequestMethodEq\(("[^"]+")\)`,
			Replacement: `RequestMethod() == $1`,
		},
		{
			Pattern:     `AttemptsLe\((\d+)\)`,
			Replacement: "Attempts() <= $1",
		},
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p.Pattern)
		in = re.ReplaceAllString(in, p.Replacement)
	}
	return in
}
