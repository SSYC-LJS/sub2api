package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// systemHasBillingAttributionBlock recognizes genuine Claude Code payloads
// whose User-Agent was replaced by an intermediate API gateway.
func systemHasBillingAttributionBlock(body []byte) bool {
	system := gjson.GetBytes(body, "system")
	if !system.IsArray() {
		return false
	}
	found := false
	system.ForEach(func(_, item gjson.Result) bool {
		text := item.Get("text").String()
		if strings.HasPrefix(text, claudeCodeBillingHeaderPrefix) &&
			strings.Contains(text, claudeCodeEntrypointMarker) {
			found = true
			return false
		}
		return true
	})
	return found
}
