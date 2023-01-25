package cloudtrace

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/trace/apiv1/tracepb"
)

const (
	serviceKey = "g.co/gae/app/module"
	methodKey  = "/http/method"
)

// TimeRange holds both a from and to time
type TimeRange struct {
	From time.Time
	To   time.Time
}

// GetTraceName gets the name, service label value, and method label value
// for the span and combines them to create a descriptive name
func GetTraceName(span *tracepb.TraceSpan) string {
	namePart := span.GetName()
	labels := span.GetLabels()

	servicePart := labels[serviceKey]
	if servicePart != "" {
		servicePart = fmt.Sprintf("%s: ", servicePart)
	}

	methodPart := labels[methodKey]
	if methodPart != "" {
		methodPart = fmt.Sprintf("HTTP %s ", methodPart)
	}

	return fmt.Sprintf("%s%s%s", servicePart, methodPart, namePart)
}

// GetListTracesFilter takes the raw query text from a user and converts it
// to a filter string as expected by the Cloud Trace API
func GetListTracesFilter(queryText string) (string, error) {
	re := regexp.MustCompile(`(?:[^\s"]+|"(?:\\"|[^"])*")+`)
	// Collect all filter parts from the query text
	qTFilters := re.FindAllString(queryText, -1)

	filters := make([]string, 0, len(qTFilters))
	for _, qTFilter := range qTFilters {
		key, value, err := getFilterKeyValue(qTFilter)
		if err != nil {
			return "", err
		}

		filters = append(filters, fmt.Sprintf("%s:%s", key, value))
	}

	return strings.Join(filters, " "), nil
}

func getFilterKeyValue(qTFilter string) (key string, value string, err error) {
	// Filter part must be in form [key]:[value] from user
	qTFilterParts := strings.SplitN(qTFilter, ":", 2)
	if len(qTFilterParts) != 2 {
		return "", "", fmt.Errorf("bad filter [%s]. Must be in form [key]:[value]", qTFilter)
	}

	key = qTFilterParts[0]
	value = qTFilterParts[1]

	// OR for generic labels filter must be in form LABEL:[key]:[value] from user
	if strings.ToLower(key) == "label" {
		qTFilterParts := strings.SplitN(value, ":", 2)

		if len(qTFilterParts) != 2 {
			return "", "", fmt.Errorf("bad filter [%s]. Must be in form LABEL:[key]:[value]", qTFilter)
		}

		// Cloud Trace API should not have "LABEL:" in filter
		key = qTFilterParts[0]
		value = qTFilterParts[1]
	}

	// Convert key to Cloud Trace API expected form if needed
	switch key {
	case "RootSpan":
		key = "root"
	case "SpanName":
		key = "span"
	case "HasLabel":
		key = "label"
	case "MinLatency":
		key = "latency"
	case "URL":
		key = "url"
	case "Method":
		key = "method"
	case "Version":
		key = "g.co/gae/app/version" // This is somewhat of a guess as our test service doesn't seem to be reporting this
	case "Service":
		key = "g.co/gae/app/module"
	case "Status":
		key = "/http/status_code"
	}

	// If the value has less than 2 chars, no need to check for special filter chars
	if len(value) < 2 {
		return key, value, nil
	}

	firstChar := string(value[0])
	secondChar := string(value[1])

	// Move specials chars from the front of value to key for Google Cloud Trace compatibility
	if (secondChar == "^" && firstChar == "+") || (secondChar == "+" && firstChar == "^") {
		key = fmt.Sprintf("+^%s", key)
		value = value[2:]
	} else if firstChar == "+" || firstChar == "^" {
		key = fmt.Sprintf("%s%s", firstChar, key)
		value = value[1:]
	}

	return key, value, nil
}
