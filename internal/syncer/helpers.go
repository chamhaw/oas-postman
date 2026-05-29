package syncer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var operationMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

var methodOrder = map[string]int{
	"GET": 10, "POST": 20, "PUT": 30, "PATCH": 40, "DELETE": 50,
	"HEAD": 60, "OPTIONS": 70, "TRACE": 80,
}

func asMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

func asStringSlice(v any) []string {
	items := asSlice(v)
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := asString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled"
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "",
		"\"", "'",
		"<", "(",
		">", ")",
		"|", "-",
		"\n", " ",
		"\r", " ",
		"\t", " ",
	)
	name = replacer.Replace(name)
	name = strings.Join(strings.Fields(name), " ")
	if len([]rune(name)) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled"
	}
	return name
}

func uniqueName(base string, used map[string]bool) string {
	base = cleanName(base)
	name := base
	for i := 2; used[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	used[strings.ToLower(name)] = true
	return name
}

func titleFromOperation(op Operation) string {
	return firstString(op.Summary, op.ID, strings.ToUpper(op.Method)+" "+op.Path)
}

func postmanURL(baseURL, specPath string, query []KeyValue) string {
	converted := regexp.MustCompile(`\{([^}/]+)\}`).ReplaceAllString(specPath, ":$1")
	if converted == "" || converted[0] != '/' {
		converted = "/" + converted
	}
	out := strings.TrimRight(baseURL, "/") + converted
	if strings.TrimRight(baseURL, "/") == "" {
		out = converted
	}
	if len(query) == 0 {
		return out
	}
	params := make([]string, 0, len(query))
	for _, q := range query {
		if q.Key == "" {
			continue
		}
		params = append(params, url.QueryEscape(q.Key)+"="+url.QueryEscape(q.Value))
	}
	if len(params) == 0 {
		return out
	}
	return out + "?" + strings.Join(params, "&")
}

func normalizePathFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "/"
	}

	if strings.HasPrefix(rawURL, "{{") {
		if idx := strings.Index(rawURL, "}}"); idx >= 0 {
			rawURL = rawURL[idx+2:]
		}
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		if parsed, err := url.Parse(rawURL); err == nil {
			rawURL = parsed.Path
			if rawURL == "" {
				rawURL = "/"
			}
		}
	}
	if idx := strings.Index(rawURL, "?"); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	rawURL = regexp.MustCompile(`/\:([^/]+)`).ReplaceAllString(rawURL, `/{$1}`)
	rawURL = regexp.MustCompile(`\{([^}/]+)\}`).ReplaceAllString(rawURL, `{$1}`)
	rawURL = path.Clean("/" + strings.TrimLeft(rawURL, "/"))
	if rawURL == "." {
		return "/"
	}
	return rawURL
}

func operationKey(method, specPath string) string {
	return strings.ToUpper(method) + " " + normalizePathFromURL(specPath)
}

func prettyJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func jsonSchemaString(schema map[string]any) string {
	if len(schema) == 0 {
		return ""
	}
	return prettyJSON(schema)
}

func statusText(code string) string {
	switch code {
	case "100":
		return "Continue"
	case "101":
		return "Switching Protocols"
	case "200":
		return "OK"
	case "201":
		return "Created"
	case "202":
		return "Accepted"
	case "204":
		return "No Content"
	case "301":
		return "Moved Permanently"
	case "302":
		return "Found"
	case "304":
		return "Not Modified"
	case "400":
		return "Bad Request"
	case "401":
		return "Unauthorized"
	case "403":
		return "Forbidden"
	case "404":
		return "Not Found"
	case "409":
		return "Conflict"
	case "422":
		return "Unprocessable Entity"
	case "429":
		return "Too Many Requests"
	case "500":
		return "Internal Server Error"
	case "502":
		return "Bad Gateway"
	case "503":
		return "Service Unavailable"
	default:
		if code == "default" {
			return "Default"
		}
		return ""
	}
}

func statusOrder(code string, index int) int {
	n, err := strconv.Atoi(code)
	if err != nil {
		return 9000 + index
	}
	return n * 10
}

func valueString(v any, fallback string) string {
	if v == nil {
		return fallback
	}
	if s := asString(v); s != "" {
		return s
	}
	return fallback
}

func exportedFormulaName(name string) string {
	var b strings.Builder
	upperNext := true
	for _, r := range name {
		if r == '-' || r == '_' || unicode.IsSpace(r) {
			upperNext = true
			continue
		}
		if upperNext {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
