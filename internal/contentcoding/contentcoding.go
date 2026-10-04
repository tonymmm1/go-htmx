// Package contentcoding chooses a response Content-Encoding from a request's
// Accept-Encoding header (RFC 9110, section 12.5.3). It is shared by the gzip
// middleware and the static asset handler so both negotiate the same way.
package contentcoding

import (
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Negotiate returns the coding from offered that the client accepts with the
// highest q-value, or "" to send the response unencoded. Ties go to the coding
// listed first in offered, so pass codings in the server's preference order,
// e.g. Negotiate(r, "br", "gzip").
//
// A coding is acceptable when it, or failing that the "*" wildcard, is listed
// with q > 0. Codings that aren't listed are not acceptable, so a request with
// no Accept-Encoding gets "". Malformed or out-of-range q-values count as 0.
// "x-gzip" is treated as "gzip". Repeated Accept-Encoding headers are combined.
func Negotiate(r *http.Request, offered ...string) string {
	return negotiate(strings.Join(r.Header.Values("Accept-Encoding"), ","), offered)
}

func negotiate(header string, offered []string) string {
	listed := make(map[string]float64)
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(part, ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if name == "x-gzip" {
			name = "gzip"
		}
		listed[name] = quality(params)
	}

	best, bestQ := "", 0.0
	for _, coding := range offered {
		q, ok := listed[coding]
		if !ok {
			q = listed["*"] // zero when "*" isn't listed either
		}
		if q > bestQ {
			best, bestQ = coding, q
		}
	}
	return best
}

// quality returns the q parameter from ";"-separated parameters, defaulting to
// 1. Values that don't parse or fall outside [0, 1] count as 0 (refused).
func quality(params string) float64 {
	q := 1.0
	for param := range strings.SplitSeq(params, ";") {
		name, value, ok := strings.Cut(param, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(parsed) || parsed < 0 || parsed > 1 {
			return 0
		}
		q = parsed
	}
	return q
}
