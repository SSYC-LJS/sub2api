package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/SSYC-LJS/sub2api/internal/config"
	"github.com/SSYC-LJS/sub2api/internal/pkg/claude"
	"github.com/SSYC-LJS/sub2api/internal/pkg/logger"
	"github.com/SSYC-LJS/sub2api/internal/util/responseheaders"
	"github.com/cespare/xxhash/v2"
	gocache "github.com/patrickmn/go-cache"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/sync/singleflight"

	"github.com/gin-gonic/gin"
)

const (
	claudeAPIURL            = "https://api.anthropic.com/v1/messages?beta=true"
	claudeAPICountTokensURL = "https://api.anthropic.com/v1/messages/count_tokens?beta=true"
	stickySessionTTL        = time.Hour // 粘性会话TTL
	defaultMaxLineSize      = 500 * 1024 * 1024
	// Canonical Claude Code banner. Keep it EXACT (no trailing whitespace/newlines)
	// to match real Claude CLI traffic as closely as possible. When we need a visual
	// separator between system blocks, we add "\n\n" at concatenation time.
	claudeCodeSystemPrompt = "You are Claude Code, Anthropic's official CLI for Claude."
	// claudeCodeSystemPromptExpansion 是真实 Claude Code 主系统提示词中"与具体工具无关"
	// 的通用段落（身份/用途总述 + 安全声明 + URL 告警 + Tone and style），逐字取自真实
	// CLI（2.1.x 一致）。伪装路径用它把 system 块数从 2 提升到 3、体量贴近真实 CC，同时
	// 刻意排除 # Doing tasks / # Using your tools / # Executing actions 等会污染被代理
	// 用户行为的工具专属指令。
	claudeCodeSystemPromptExpansion = `You are an interactive agent that helps users with software engineering tasks. Use the instructions below and the tools available to you to assist the user.

IMPORTANT: Assist with authorized security testing, defensive security, CTF challenges, and educational contexts. Refuse requests for destructive techniques, DoS attacks, mass targeting, supply chain compromise, or detection evasion for malicious purposes. Dual-use security tools (C2 frameworks, credential testing, exploit development) require clear authorization context: pentesting engagements, CTF competitions, security research, or defensive use cases.
IMPORTANT: You must NEVER generate or guess URLs for the user unless you are confident that the URLs are for helping the user with programming. You may use URLs provided by the user in their messages or local files.

# Tone and style
 - Only use emojis if the user explicitly requests it. Avoid using emojis in all communication unless asked.
 - Your responses should be short and concise.
 - When referencing specific functions or pieces of code include the pattern file_path:line_number to allow the user to easily navigate to the source code location.
 - When referencing GitHub issues or pull requests, use the owner/repo#123 format (e.g. anthropics/claude-code#100) so they render as clickable links.
 - Do not use a colon before tool calls. Your tool calls may not be shown directly in the output, so text like "Let me read the file:" followed by a read tool call should just be "Let me read the file." with a period.`
	maxCacheControlBlocks = 4 // Anthropic API 允许的最大 cache_control 块数量

	defaultUserGroupRateCacheTTL = 30 * time.Second
	defaultModelsListCacheTTL    = 15 * time.Second
	postUsageBillingTimeout      = 15 * time.Second
	debugGatewayBodyEnv          = "SUB2API_DEBUG_GATEWAY_BODY"
	// 上游错误体只需要提取错误 JSON/日志摘要，默认 512KiB 避免错误风暴叠加大请求体。
	gatewayUpstreamErrorBodyReadLimit int64 = 512 << 10
)

const (
	claudeMimicDebugInfoKey = "claude_mimic_debug_info"
)

const (
	cacheTTLTarget5m = "5m"
	cacheTTLTarget1h = "1h"
)

// ForceCacheBillingContextKey 强制缓存计费上下文键
// 用于粘性会话切换时，将 input_tokens 转为 cache_read_input_tokens 计费
type forceCacheBillingKeyType struct{}

// accountWithLoad 账号与负载信息的组合，用于负载感知调度
type accountWithLoad struct {
	account  *Account
	loadInfo *AccountLoadInfo
}

var ForceCacheBillingContextKey = forceCacheBillingKeyType{}

var (
	windowCostPrefetchCacheHitTotal  atomic.Int64
	windowCostPrefetchCacheMissTotal atomic.Int64
	windowCostPrefetchBatchSQLTotal  atomic.Int64
	windowCostPrefetchFallbackTotal  atomic.Int64
	windowCostPrefetchErrorTotal     atomic.Int64

	userGroupRateCacheHitTotal      atomic.Int64
	userGroupRateCacheMissTotal     atomic.Int64
	userGroupRateCacheLoadTotal     atomic.Int64
	userGroupRateCacheSFSharedTotal atomic.Int64
	userGroupRateCacheFallbackTotal atomic.Int64

	modelsListCacheHitTotal   atomic.Int64
	modelsListCacheMissTotal  atomic.Int64
	modelsListCacheStoreTotal atomic.Int64

	// Deprecated: flusher_enabled=true 后不再增长(仅 flag=false 降级直写路径使用);新主路径见 FlusherMetrics。remove after 2026-09。
	// userPlatformQuotaDBIncrErrorTotal 统计 finalizePostUsageBilling 异步 goroutine
	// 中 IncrementUsageWithReset 失败次数。Redis 已成功累加 + DB 写失败意味着
	// Redis cache TTL 过期或被清后该笔 cost 会丢失（与实际消费偏差）。
	// oncall 通过 GatewayUserPlatformQuotaIncrStats() 暴露给 ops 面板做阈值告警。
	userPlatformQuotaDBIncrErrorTotal atomic.Int64
	// Deprecated: flusher_enabled=true 后不再增长(仅 flag=false 降级直写路径使用);新主路径见 FlusherMetrics。remove after 2026-09。
	// userPlatformQuotaDBIncrLegacyErrorTotal 统计 legacy postUsageBilling
	// （applyUsageBilling 在 repo==nil 时 fallback）路径下的失败次数；
	// 与 DB Incr 失败分开计数，便于区分"主路径暂时故障"vs"基础设施长期未配齐"。
	userPlatformQuotaDBIncrLegacyErrorTotal atomic.Int64
	// userPlatformQuotaSentinelSetCacheErrorTotal 统计 checkUserPlatformQuotaEligibility
	// 在 DB 无行时回填 sentinel cache entry 写 Redis 失败的次数（phase A）。
	userPlatformQuotaSentinelSetCacheErrorTotal atomic.Int64
)

func GatewayWindowCostPrefetchStats() (cacheHit, cacheMiss, batchSQL, fallback, errCount int64) {
	return windowCostPrefetchCacheHitTotal.Load(),
		windowCostPrefetchCacheMissTotal.Load(),
		windowCostPrefetchBatchSQLTotal.Load(),
		windowCostPrefetchFallbackTotal.Load(),
		windowCostPrefetchErrorTotal.Load()
}

func GatewayUserGroupRateCacheStats() (cacheHit, cacheMiss, load, singleflightShared, fallback int64) {
	return userGroupRateCacheHitTotal.Load(),
		userGroupRateCacheMissTotal.Load(),
		userGroupRateCacheLoadTotal.Load(),
		userGroupRateCacheSFSharedTotal.Load(),
		userGroupRateCacheFallbackTotal.Load()
}

func GatewayModelsListCacheStats() (cacheHit, cacheMiss, store int64) {
	return modelsListCacheHitTotal.Load(), modelsListCacheMissTotal.Load(), modelsListCacheStoreTotal.Load()
}

// GatewayUserPlatformQuotaIncrStats 返回 (mainPathErr, legacyPathErr, sentinelSetErr)。
// mainPathErr：finalizePostUsageBilling 异步 goroutine 写 DB 失败累计次数；
// legacyPathErr：postUsageBilling fallback 路径写 DB 失败累计次数；
// sentinelSetErr：DB 无行时回填 sentinel cache entry 写 Redis 失败累计次数。
// ops 监控面板可以按"持续上升斜率"做告警阈值。
func GatewayUserPlatformQuotaIncrStats() (mainPathErr, legacyPathErr, sentinelSetErr int64) {
	return userPlatformQuotaDBIncrErrorTotal.Load(),
		userPlatformQuotaDBIncrLegacyErrorTotal.Load(),
		userPlatformQuotaSentinelSetCacheErrorTotal.Load()
}

// GatewayUserPlatformQuotaFlusherStats 暴露 flusher 运行指标供 ops/health 面板查询。
func GatewayUserPlatformQuotaFlusherStats(f *UserPlatformQuotaUsageFlusher) map[string]int64 {
	if f == nil || f.metrics == nil {
		return nil
	}
	m := f.metrics
	return map[string]int64{
		"flush_success":        m.FlushSuccessTotal.Load(),
		"flush_error":          m.FlushErrorTotal.Load(),
		"flush_batch_size":     m.FlushBatchSizeTotal.Load(),
		"flush_latency_ms_max": m.FlushLatencyMsMax.Load(),
		"dirty_readd":          m.DirtyReaddTotal.Load(),
		"dirty_lost":           m.DirtyLostTotal.Load(),
		"flush_fk_violation":   m.FlushFKViolationTotal.Load(),
	}
}

func openAIStreamEventIsTerminal(data string) bool {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return false
	}
	if trimmed == "[DONE]" {
		return true
	}
	return openAIStreamEventTypeIsTerminal(gjson.Get(trimmed, "type").String())
}

// openAIStreamEventIsTerminalWithType 复用已提取的 type，避免 SSE 热路径重复扫描 JSON。
func openAIStreamEventIsTerminalWithType(data, eventType string) bool {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return false
	}
	if trimmed == "[DONE]" {
		return true
	}
	return openAIStreamEventTypeIsTerminal(eventType)
}

func openAIStreamEventTypeIsTerminal(eventType string) bool {
	switch eventType {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func anthropicStreamEventIsTerminal(eventName, data string) bool {
	if strings.EqualFold(strings.TrimSpace(eventName), "message_stop") {
		return true
	}
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return false
	}
	if trimmed == "[DONE]" {
		return true
	}
	return gjson.Get(trimmed, "type").String() == "message_stop"
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// IsForceCacheBilling 检查是否启用强制缓存计费
func IsForceCacheBilling(ctx context.Context) bool {
	v, _ := ctx.Value(ForceCacheBillingContextKey).(bool)
	return v
}

// WithForceCacheBilling 返回带有强制缓存计费标记的上下文
func WithForceCacheBilling(ctx context.Context) context.Context {
	return context.WithValue(ctx, ForceCacheBillingContextKey, true)
}

func (s *GatewayService) debugModelRoutingEnabled() bool {
	if s == nil {
		return false
	}
	return s.debugModelRouting.Load()
}

func (s *GatewayService) debugClaudeMimicEnabled() bool {
	if s == nil {
		return false
	}
	return s.debugClaudeMimic.Load()
}

func parseDebugEnvBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func shortSessionHash(sessionHash string) string {
	if sessionHash == "" {
		return ""
	}
	if len(sessionHash) <= 8 {
		return sessionHash
	}
	return sessionHash[:8]
}

func redactAuthHeaderValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Keep scheme for debugging, redact secret.
	if strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return "Bearer [redacted]"
	}
	return "[redacted]"
}

func safeHeaderValueForLog(key string, v string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "authorization", "x-api-key":
		return redactAuthHeaderValue(v)
	default:
		return strings.TrimSpace(v)
	}
}

func extractSystemPreviewFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	sys := gjson.GetBytes(body, "system")
	if !sys.Exists() {
		return ""
	}

	switch {
	case sys.IsArray():
		for _, item := range sys.Array() {
			if !item.IsObject() {
				continue
			}
			if strings.EqualFold(item.Get("type").String(), "text") {
				if t := item.Get("text").String(); strings.TrimSpace(t) != "" {
					return t
				}
			}
		}
		return ""
	case sys.Type == gjson.String:
		return sys.String()
	default:
		return ""
	}
}

func buildClaudeMimicDebugLine(req *http.Request, body []byte, account *Account, tokenType string, mimicClaudeCode bool) string {
	if req == nil {
		return ""
	}

	// Only log a minimal fingerprint to avoid leaking user content.
	interesting := []string{
		"user-agent",
		"x-app",
		"anthropic-dangerous-direct-browser-access",
		"anthropic-version",
		"anthropic-beta",
		"x-stainless-lang",
		"x-stainless-package-version",
		"x-stainless-os",
		"x-stainless-arch",
		"x-stainless-runtime",
		"x-stainless-runtime-version",
		"x-stainless-retry-count",
		"x-stainless-timeout",
		"authorization",
		"x-api-key",
		"content-type",
		"accept",
		"x-stainless-helper-method",
	}

	h := make([]string, 0, len(interesting))
	for _, k := range interesting {
		if v := req.Header.Get(k); v != "" {
			h = append(h, fmt.Sprintf("%s=%q", k, safeHeaderValueForLog(k, v)))
		}
	}

	metaUserID := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())
	sysPreview := strings.TrimSpace(extractSystemPreviewFromBody(body))

	// Truncate preview to keep logs sane.
	if len(sysPreview) > 300 {
		sysPreview = sysPreview[:300] + "..."
	}
	sysPreview = strings.ReplaceAll(sysPreview, "\n", "\\n")
	sysPreview = strings.ReplaceAll(sysPreview, "\r", "\\r")

	aid := int64(0)
	aname := ""
	if account != nil {
		aid = account.ID
		aname = account.Name
	}

	return fmt.Sprintf(
		"url=%s account=%d(%s) tokenType=%s mimic=%t meta.user_id=%q system.preview=%q headers={%s}",
		req.URL.String(),
		aid,
		aname,
		tokenType,
		mimicClaudeCode,
		metaUserID,
		sysPreview,
		strings.Join(h, " "),
	)
}

func logClaudeMimicDebug(req *http.Request, body []byte, account *Account, tokenType string, mimicClaudeCode bool) {
	line := buildClaudeMimicDebugLine(req, body, account, tokenType, mimicClaudeCode)
	if line == "" {
		return
	}
	logger.LegacyPrintf("service.gateway", "[ClaudeMimicDebug] %s", line)
}

func isClaudeCodeCredentialScopeError(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	if m == "" {
		return false
	}
	return strings.Contains(m, "only authorized for use with claude code") &&
		strings.Contains(m, "cannot be used for other api requests")
}

// sseDataRe matches SSE data lines with optional whitespace after colon.
// Some upstream APIs return non-standard "data:" without space (should be "data: ").
var (
	sseDataRe            = regexp.MustCompile(`^data:\s*`)
	claudeCliUserAgentRe = regexp.MustCompile(`(?i)^claude-cli/\d+\.\d+\.\d+`)

	// claudeCodePromptPrefixes 用于检测 Claude Code 系统提示词的前缀列表
	// 支持多种变体：标准版、Agent SDK 版、Explore Agent 版、Compact 版等
	// 注意：前缀之间不应存在包含关系，否则会导致冗余匹配
	claudeCodePromptPrefixes = []string{
		"You are Claude Code, Anthropic's official CLI for Claude",             // 标准版 & Agent SDK 版（含 running within...）
		"You are a Claude agent, built on Anthropic's Claude Agent SDK",        // Agent SDK 变体
		"You are a file search specialist for Claude Code",                     // Explore Agent 版
		"You are a helpful AI assistant tasked with summarizing conversations", // Compact 版
	}
)

func claudeCodeNeedsNoopDeltaKeepalive(userAgent string) bool {
	userAgent = strings.TrimSpace(userAgent)
	if !claudeCliUserAgentRe.MatchString(userAgent) {
		return false
	}
	fields := strings.Fields(userAgent)
	if len(fields) == 0 {
		return false
	}
	version := strings.TrimPrefix(fields[0], "claude-cli/")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	if major != 2 || minor != 1 {
		return major > 2 || (major == 2 && minor > 1)
	}
	return patch >= 198
}

func anthropicNoopDeltaKeepaliveEvent(index int, blockType string) string {
	if blockType == "tool_use" {
		return fmt.Sprintf("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\"}}\n\n", index)
	}
	return fmt.Sprintf("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"text_delta\",\"text\":\"\"}}\n\n", index)
}

// ErrNoAvailableAccounts 表示没有可用的账号
var ErrNoAvailableAccounts = errors.New("no available accounts")

// ErrClaudeCodeOnly 表示分组仅允许 Claude Code 客户端访问
var ErrClaudeCodeOnly = errors.New("this group only allows Claude Code clients")

// allowedHeaders 白名单headers（参考CRS项目）
var allowedHeaders = map[string]bool{
	"accept":                                    true,
	"x-stainless-retry-count":                   true,
	"x-stainless-timeout":                       true,
	"x-stainless-lang":                          true,
	"x-stainless-package-version":               true,
	"x-stainless-os":                            true,
	"x-stainless-arch":                          true,
	"x-stainless-runtime":                       true,
	"x-stainless-runtime-version":               true,
	"x-stainless-helper-method":                 true,
	"anthropic-dangerous-direct-browser-access": true,
	"anthropic-version":                         true,
	"x-app":                                     true,
	"anthropic-beta":                            true,
	"accept-language":                           true,
	"sec-fetch-mode":                            true,
	"user-agent":                                true,
	"content-type":                              true,
	"accept-encoding":                           true,
	"x-claude-code-session-id":                  true,
	"x-client-request-id":                       true,
}

// GatewayCache 定义网关服务的缓存操作接口。
// 提供粘性会话（Sticky Session）的存储、查询、刷新和删除功能。
//
// GatewayCache defines cache operations for gateway service.
// Provides sticky session storage, retrieval, refresh and deletion capabilities.
type GatewayCache interface {
	// GetSessionAccountID 获取粘性会话绑定的账号 ID
	// Get the account ID bound to a sticky session
	GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error)
	// SetSessionAccountID 设置粘性会话与账号的绑定关系
	// Set the binding between sticky session and account
	SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error
	// RefreshSessionTTL 刷新粘性会话的过期时间
	// Refresh the expiration time of a sticky session
	RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error
	// DeleteSessionAccountID 删除粘性会话绑定，用于账号不可用时主动清理
	// Delete sticky session binding, used to proactively clean up when account becomes unavailable
	DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error
}

// derefGroupID safely dereferences *int64 to int64, returning 0 if nil
func derefGroupID(groupID *int64) int64 {
	if groupID == nil {
		return 0
	}
	return *groupID
}

func resolveUserGroupRateCacheTTL(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.Gateway.UserGroupRateCacheTTLSeconds <= 0 {
		return defaultUserGroupRateCacheTTL
	}
	return time.Duration(cfg.Gateway.UserGroupRateCacheTTLSeconds) * time.Second
}

func resolveModelsListCacheTTL(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.Gateway.ModelsListCacheTTLSeconds <= 0 {
		return defaultModelsListCacheTTL
	}
	return time.Duration(cfg.Gateway.ModelsListCacheTTLSeconds) * time.Second
}

func modelsListCacheKey(groupID *int64, platform string) string {
	return fmt.Sprintf("%d|%s", derefGroupID(groupID), strings.TrimSpace(platform))
}

func prefetchedStickyGroupIDFromContext(ctx context.Context) (int64, bool) {
	return PrefetchedStickyGroupIDFromContext(ctx)
}

func prefetchedStickyAccountIDFromContext(ctx context.Context, groupID *int64) int64 {
	prefetchedGroupID, ok := prefetchedStickyGroupIDFromContext(ctx)
	if !ok || prefetchedGroupID != derefGroupID(groupID) {
		return 0
	}
	if accountID, ok := PrefetchedStickyAccountIDFromContext(ctx); ok && accountID > 0 {
		return accountID
	}
	return 0
}

// shouldClearStickySession 检查账号是否处于不可调度状态，需要清理粘性会话绑定。
// 委托 IsSchedulable() 判断账号级可调度性（状态、配额、过载、限流等），
// 额外检查模型级限流。
//
// shouldClearStickySession checks if an account is in an unschedulable state
// and the sticky session binding should be cleared.
// Delegates to IsSchedulable() for account-level checks, plus model-level rate limiting.
func shouldClearStickySession(account *Account, requestedModel string) bool {
	if account == nil {
		return false
	}
	if !account.IsSchedulable() {
		return true
	}
	if remaining := account.GetRateLimitRemainingTimeWithContext(context.Background(), requestedModel); remaining > 0 {
		return true
	}
	return false
}

type AccountWaitPlan struct {
	AccountID      int64
	MaxConcurrency int
	Timeout        time.Duration
	MaxWaiting     int
}

type AccountSelectionResult struct {
	Account     *Account
	Acquired    bool
	ReleaseFunc func()
	WaitPlan    *AccountWaitPlan // nil means no wait allowed
}

// ClaudeUsage 表示Claude API返回的usage信息
type ClaudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreation5mTokens    int // 5分钟缓存创建token（来自嵌套 cache_creation 对象）
	CacheCreation1hTokens    int // 1小时缓存创建token（来自嵌套 cache_creation 对象）
	ImageOutputTokens        int `json:"image_output_tokens,omitempty"`
}

// ForwardResult 转发结果
type ForwardResult struct {
	RequestID string
	Usage     ClaudeUsage
	Model     string
	// UpstreamModel is the actual upstream model after mapping.
	// Prefer empty when it is identical to Model; persistence normalizes equal values away as no-op mappings.
	UpstreamModel    string
	Stream           bool
	Duration         time.Duration
	FirstTokenMs     *int // 首字时间（流式请求）
	ClientDisconnect bool // 客户端是否在流式传输过程中断开
	ReasoningEffort  *string

	// 图片生成计费字段（图片生成模型使用）
	ImageCount         int    // 生成的图片数量
	ImageSize          string // 最终计费尺寸 "1K", "2K", "4K"
	ImageInputSize     string // 请求中的原始图片尺寸
	ImageOutputSize    string // 上游响应中的图片尺寸
	ImageOutputSizes   []string
	ImageSizeSource    string
	ImageSizeBreakdown map[string]int
}

// GatewayFailureStage identifies which request stage failed. The zero value is
// intentionally treated as inference so existing UpstreamFailoverError callers
// retain their current behavior.
type GatewayFailureStage string

const (
	GatewayFailureStageInference   GatewayFailureStage = "inference"
	GatewayFailureStageAccountAuth GatewayFailureStage = "account_auth"
)

// GatewayFailureScope identifies whether selecting another account can help.
type GatewayFailureScope string

const (
	GatewayFailureScopeAccount  GatewayFailureScope = "account"
	GatewayFailureScopeProvider GatewayFailureScope = "provider"
	GatewayFailureScopeRequest  GatewayFailureScope = "request"
)

// NextAccountAction is tri-state for backwards compatibility. The zero value
// means legacy retry behavior; only NextAccountStop explicitly short-circuits.
type NextAccountAction uint8

const (
	NextAccountLegacyRetry NextAccountAction = iota
	NextAccountRetry
	NextAccountStop
)

type GatewayFailureReason string

// UpstreamFailoverError indicates an upstream or credential error that may
// trigger account failover. Additive metadata keeps existing composite literals
// source-compatible and preserves their legacy retry-next-account behavior.
type UpstreamFailoverError struct {
	StatusCode               int
	ResponseBody             []byte      // 上游响应体，用于错误透传规则匹配
	ResponseHeaders          http.Header // 上游响应头，用于透传 cf-ray/cf-mitigated/content-type 等诊断信息
	ForceCacheBilling        bool        // Antigravity 粘性会话切换时设为 true
	RetryableOnSameAccount   bool        // 临时性错误（如 Google 间歇性 400、空响应），应在同一账号上重试 N 次再切换
	SafeToFailoverAfterWrite bool        // 仅写出 SSE 注释等非语义字节时，仍可在同一客户端流中切换账号
	Stage                    GatewayFailureStage
	Scope                    GatewayFailureScope
	Reason                   GatewayFailureReason
	NextAccountAction        NextAccountAction
	ClientStatusCode         int
	ClientMessage            string
}

func (e *UpstreamFailoverError) Error() string {
	if e != nil && e.Stage == GatewayFailureStageAccountAuth {
		return fmt.Sprintf("credential failure: %s (failover)", e.Reason)
	}
	return fmt.Sprintf("upstream error: %d (failover)", e.StatusCode)
}

func (e *UpstreamFailoverError) ShouldRetryNextAccount() bool {
	return e != nil && e.NextAccountAction != NextAccountStop
}

func (e *UpstreamFailoverError) IsCredentialFailure() bool {
	return e != nil && e.Stage == GatewayFailureStageAccountAuth
}

// ShouldReportAccountScheduleFailure prevents provider- and request-scoped
// credential failures from being misattributed to the selected account. Legacy
// and inference failures retain their existing scheduler-health behavior.
func (e *UpstreamFailoverError) ShouldReportAccountScheduleFailure() bool {
	if e == nil {
		return false
	}
	return !e.IsCredentialFailure() || e.Scope == GatewayFailureScopeAccount
}

// sseStreamErrorEventError 表示上游 SSE 流体内出现 event:error 帧。
// RawData 是该事件 data: 行的原始 JSON 字符串
// （Anthropic 标准结构 {"type":"error","error":{"type":"...","message":"..."}}）。
// Error() 保持原字符串以兼容现有日志/检索；调用方应通过 errors.As
// 提取 RawData 并构造 UpstreamFailoverError.ResponseBody。
type sseStreamErrorEventError struct {
	RawData string
}

func (e *sseStreamErrorEventError) Error() string { return "have error in stream" }

// TempUnscheduleRetryableError 对 RetryableOnSameAccount 类型的 failover 错误触发临时封禁。
// 由 handler 层在同账号重试全部用尽、切换账号时调用。
func (s *GatewayService) TempUnscheduleRetryableError(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) {
	if failoverErr == nil || !failoverErr.RetryableOnSameAccount {
		return
	}
	// 根据状态码选择封禁策略
	switch failoverErr.StatusCode {
	case http.StatusBadRequest:
		tempUnscheduleGoogleConfigError(ctx, s.accountRepo, accountID, "[handler]")
	case http.StatusBadGateway:
		tempUnscheduleEmptyResponse(ctx, s.accountRepo, accountID, "[handler]")
	}
}

// GatewayService handles API gateway operations
type GatewayService struct {
	accountRepo           AccountRepository
	groupRepo             GroupRepository
	usageLogRepo          UsageLogRepository
	usageBillingRepo      UsageBillingRepository
	userRepo              UserRepository
	userSubRepo           UserSubscriptionRepository
	userGroupRateRepo     UserGroupRateRepository
	cache                 GatewayCache
	digestStore           *DigestSessionStore
	cfg                   *config.Config
	schedulerSnapshot     *SchedulerSnapshotService
	billingService        *BillingService
	rateLimitService      *RateLimitService
	billingCacheService   *BillingCacheService
	identityService       *IdentityService
	httpUpstream          HTTPUpstream
	deferredService       *DeferredService
	concurrencyService    *ConcurrencyService
	claudeTokenProvider   *ClaudeTokenProvider
	sessionLimitCache     SessionLimitCache // 会话数量限制缓存（仅 Anthropic OAuth/SetupToken）
	rpmCache              RPMCache          // RPM 计数缓存（仅 Anthropic OAuth/SetupToken）
	userGroupRateResolver *userGroupRateResolver
	userGroupRateCache    *gocache.Cache
	userGroupRateSF       singleflight.Group
	modelsListCache       *gocache.Cache
	modelsListCacheTTL    time.Duration
	settingService        *SettingService
	responseHeaderFilter  *responseheaders.CompiledHeaderFilter
	debugModelRouting     atomic.Bool
	debugClaudeMimic      atomic.Bool
	channelService        *ChannelService
	resolver              *ModelPricingResolver
	compositeResolver     *CompositeRouteResolver
	debugGatewayBodyFile  atomic.Pointer[os.File] // non-nil when SUB2API_DEBUG_GATEWAY_BODY is set
	tlsFPProfileService   *TLSFingerprintProfileService
	balanceNotifyService  *BalanceNotifyService
	userPlatformQuotaRepo UserPlatformQuotaRepository
}

// NewGatewayService creates a new GatewayService
func NewGatewayService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	usageLogRepo UsageLogRepository,
	usageBillingRepo UsageBillingRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	cache GatewayCache,
	cfg *config.Config,
	schedulerSnapshot *SchedulerSnapshotService,
	concurrencyService *ConcurrencyService,
	billingService *BillingService,
	rateLimitService *RateLimitService,
	billingCacheService *BillingCacheService,
	identityService *IdentityService,
	httpUpstream HTTPUpstream,
	deferredService *DeferredService,
	claudeTokenProvider *ClaudeTokenProvider,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
	digestStore *DigestSessionStore,
	settingService *SettingService,
	tlsFPProfileService *TLSFingerprintProfileService,
	channelService *ChannelService,
	resolver *ModelPricingResolver,
	compositeResolver *CompositeRouteResolver,
	balanceNotifyService *BalanceNotifyService,
	userPlatformQuotaRepo UserPlatformQuotaRepository,
) *GatewayService {
	userGroupRateTTL := resolveUserGroupRateCacheTTL(cfg)
	modelsListTTL := resolveModelsListCacheTTL(cfg)

	svc := &GatewayService{
		accountRepo:           accountRepo,
		groupRepo:             groupRepo,
		usageLogRepo:          usageLogRepo,
		usageBillingRepo:      usageBillingRepo,
		userRepo:              userRepo,
		userSubRepo:           userSubRepo,
		userGroupRateRepo:     userGroupRateRepo,
		cache:                 cache,
		digestStore:           digestStore,
		cfg:                   cfg,
		schedulerSnapshot:     schedulerSnapshot,
		concurrencyService:    concurrencyService,
		billingService:        billingService,
		rateLimitService:      rateLimitService,
		billingCacheService:   billingCacheService,
		identityService:       identityService,
		httpUpstream:          httpUpstream,
		deferredService:       deferredService,
		claudeTokenProvider:   claudeTokenProvider,
		sessionLimitCache:     sessionLimitCache,
		rpmCache:              rpmCache,
		userGroupRateCache:    gocache.New(userGroupRateTTL, time.Minute),
		settingService:        settingService,
		modelsListCache:       gocache.New(modelsListTTL, time.Minute),
		modelsListCacheTTL:    modelsListTTL,
		responseHeaderFilter:  compileResponseHeaderFilter(cfg),
		tlsFPProfileService:   tlsFPProfileService,
		channelService:        channelService,
		resolver:              resolver,
		compositeResolver:     compositeResolver,
		balanceNotifyService:  balanceNotifyService,
		userPlatformQuotaRepo: userPlatformQuotaRepo,
	}
	svc.userGroupRateResolver = newUserGroupRateResolver(
		userGroupRateRepo,
		svc.userGroupRateCache,
		userGroupRateTTL,
		&svc.userGroupRateSF,
		"service.gateway",
	)
	svc.debugModelRouting.Store(parseDebugEnvBool(os.Getenv("SUB2API_DEBUG_MODEL_ROUTING")))
	svc.debugClaudeMimic.Store(parseDebugEnvBool(os.Getenv("SUB2API_DEBUG_CLAUDE_MIMIC")))
	if path := strings.TrimSpace(os.Getenv(debugGatewayBodyEnv)); path != "" {
		svc.initDebugGatewayBodyFile(path)
	}
	return svc
}

// GenerateSessionHash 从预解析请求计算粘性会话 hash
func (s *GatewayService) GenerateSessionHash(parsed *ParsedRequest) string {
	if parsed == nil {
		return ""
	}

	// 1. 最高优先级：从 metadata.user_id 提取 session_xxx
	if parsed.MetadataUserID != "" {
		uid := ParseMetadataUserID(parsed.MetadataUserID)
		if uid != nil && uid.SessionID != "" {
			slog.Info("sticky.hash_source",
				"source", "metadata_user_id",
				"session_id", uid.SessionID,
				"device_id", uid.DeviceID,
				"is_new_format", uid.IsNewFormat,
			)
			return uid.SessionID
		}
		slog.Info("sticky.hash_metadata_parse_failed",
			"metadata_user_id", parsed.MetadataUserID,
			"parsed_nil", uid == nil,
		)
	}

	// 2. 提取带 cache_control: {type: "ephemeral"} 的内容
	cacheableContent := s.extractCacheableContent(parsed)
	if cacheableContent != "" {
		hash := s.hashContent(cacheableContent)
		slog.Info("sticky.hash_source",
			"source", "cacheable_content",
			"hash", hash,
		)
		return hash
	}

	// 3. 最后 fallback: 使用 session上下文 + system + 所有消息的完整摘要串
	var combined strings.Builder
	// 混入请求上下文区分因子，避免不同用户相同消息产生相同 hash
	if parsed.SessionContext != nil {
		_, _ = combined.WriteString(parsed.SessionContext.ClientIP)
		_, _ = combined.WriteString(":")
		_, _ = combined.WriteString(NormalizeSessionUserAgent(parsed.SessionContext.UserAgent))
		_, _ = combined.WriteString(":")
		_, _ = combined.WriteString(strconv.FormatInt(parsed.SessionContext.APIKeyID, 10))
		_, _ = combined.WriteString("|")
	}
	if systemText := extractTextFromSystemRaw(parsed.SystemRaw()); systemText != "" {
		_, _ = combined.WriteString(systemText)
	}
	contentStart := combined.Len()
	appendMessageTextsFromRaw(&combined, parsed.MessagesRaw())
	if combined.Len() == contentStart {
		appendResponsesSessionAnchorFromRaw(&combined, parsed.InputRaw())
	}
	if combined.Len() > 0 {
		hash := s.hashContent(combined.String())
		slog.Info("sticky.hash_source",
			"source", "message_content_fallback",
			"hash", hash,
			"content_len", combined.Len(),
		)
		return hash
	}

	return ""
}

// BindStickySession sets session -> account binding with standard TTL.
func (s *GatewayService) BindStickySession(ctx context.Context, groupID *int64, sessionHash string, accountID int64) error {
	if sessionHash == "" || accountID <= 0 || s.cache == nil {
		return nil
	}
	return s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), sessionHash, accountID, stickySessionTTL)
}

// GetCachedSessionAccountID retrieves the account ID bound to a sticky session.
// Returns 0 if no binding exists or on error.
func (s *GatewayService) GetCachedSessionAccountID(ctx context.Context, groupID *int64, sessionHash string) (int64, error) {
	if sessionHash == "" || s.cache == nil {
		return 0, nil
	}
	accountID, err := s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
	if err != nil {
		return 0, err
	}
	return accountID, nil
}

// FindGeminiSession 查找 Gemini 会话（基于内容摘要链的 Fallback 匹配）
// 返回最长匹配的会话信息（uuid, accountID）
func (s *GatewayService) FindGeminiSession(_ context.Context, groupID int64, prefixHash, digestChain string) (uuid string, accountID int64, matchedChain string, found bool) {
	if digestChain == "" || s.digestStore == nil {
		return "", 0, "", false
	}
	return s.digestStore.Find(groupID, prefixHash, digestChain)
}

// SaveGeminiSession 保存 Gemini 会话。oldDigestChain 为 Find 返回的 matchedChain，用于删旧 key。
func (s *GatewayService) SaveGeminiSession(_ context.Context, groupID int64, prefixHash, digestChain, uuid string, accountID int64, oldDigestChain string) error {
	if digestChain == "" || s.digestStore == nil {
		return nil
	}
	s.digestStore.Save(groupID, prefixHash, digestChain, uuid, accountID, oldDigestChain)
	return nil
}

// FindAnthropicSession 查找 Anthropic 会话（基于内容摘要链的 Fallback 匹配）
func (s *GatewayService) FindAnthropicSession(_ context.Context, groupID int64, prefixHash, digestChain string) (uuid string, accountID int64, matchedChain string, found bool) {
	if digestChain == "" || s.digestStore == nil {
		return "", 0, "", false
	}
	return s.digestStore.Find(groupID, prefixHash, digestChain)
}

// SaveAnthropicSession 保存 Anthropic 会话
func (s *GatewayService) SaveAnthropicSession(_ context.Context, groupID int64, prefixHash, digestChain, uuid string, accountID int64, oldDigestChain string) error {
	if digestChain == "" || s.digestStore == nil {
		return nil
	}
	s.digestStore.Save(groupID, prefixHash, digestChain, uuid, accountID, oldDigestChain)
	return nil
}

func (s *GatewayService) extractCacheableContent(parsed *ParsedRequest) string {
	if parsed == nil {
		return ""
	}

	systemText := extractCacheableTextFromSystemRaw(parsed.SystemRaw())
	if messageText := extractCacheableTextFromMessagesRaw(parsed.MessagesRaw()); messageText != "" {
		return messageText
	}
	return systemText
}

func parseRawJSONView(raw []byte) gjson.Result {
	if len(raw) == 0 {
		return gjson.Result{}
	}
	// 这里只做同步只读解析，避免 gjson.ParseBytes 为大 messages/contents 复制整段 raw。
	return gjson.Parse(*(*string)(unsafe.Pointer(&raw)))
}

func extractTextFromSystemRaw(raw []byte) string {
	system := parseRawJSONView(raw)
	switch system.Type {
	case gjson.String:
		return system.String()
	case gjson.JSON:
		if !system.IsArray() {
			return ""
		}
		var builder strings.Builder
		system.ForEach(func(_, part gjson.Result) bool {
			if text := part.Get("text").String(); text != "" {
				_, _ = builder.WriteString(text)
			}
			return true
		})
		return builder.String()
	}
	return ""
}

func extractTextFromContentRaw(content gjson.Result) string {
	switch content.Type {
	case gjson.String:
		return content.String()
	case gjson.JSON:
		if !content.IsArray() {
			return ""
		}
		var builder strings.Builder
		content.ForEach(func(_, part gjson.Result) bool {
			if part.Get("type").String() == "text" {
				if text := part.Get("text").String(); text != "" {
					_, _ = builder.WriteString(text)
				}
			}
			return true
		})
		return builder.String()
	}
	return ""
}

func appendMessageTextsFromRaw(builder *strings.Builder, raw []byte) {
	if builder == nil || len(raw) == 0 {
		return
	}
	messages := parseRawJSONView(raw)
	if !messages.IsArray() {
		return
	}
	messages.ForEach(func(_, msg gjson.Result) bool {
		if content := msg.Get("content"); content.Exists() {
			_, _ = builder.WriteString(extractTextFromContentRaw(content))
			return true
		}
		parts := msg.Get("parts")
		if parts.IsArray() {
			parts.ForEach(func(_, part gjson.Result) bool {
				if text := part.Get("text").String(); text != "" {
					_, _ = builder.WriteString(text)
				}
				return true
			})
		}
		return true
	})
}

func appendResponsesSessionAnchorFromRaw(builder *strings.Builder, raw []byte) {
	if builder == nil || len(raw) == 0 {
		return
	}
	input := parseRawJSONView(raw)
	if input.Type == gjson.String {
		_, _ = builder.WriteString(input.String())
		return
	}
	if !input.IsArray() {
		return
	}

	input.ForEach(func(_, item gjson.Result) bool {
		if item.Type == gjson.String {
			_, _ = builder.WriteString(item.String())
			return false
		}

		switch item.Get("role").String() {
		case "system", "developer":
			appendResponsesContentText(builder, item.Get("content"))
		case "user":
			appendResponsesContentText(builder, item.Get("content"))
			return false
		default:
			if item.Get("type").String() == "input_text" {
				if text := item.Get("text").String(); text != "" {
					_, _ = builder.WriteString(text)
				}
				return false
			}
		}
		return true
	})
}

func appendResponsesContentText(builder *strings.Builder, content gjson.Result) {
	if builder == nil || !content.Exists() {
		return
	}
	if content.Type == gjson.String {
		_, _ = builder.WriteString(content.String())
		return
	}
	if !content.IsArray() {
		return
	}
	content.ForEach(func(_, part gjson.Result) bool {
		switch part.Get("type").String() {
		case "input_text", "text":
			if text := part.Get("text").String(); text != "" {
				_, _ = builder.WriteString(text)
			}
		}
		return true
	})
}

func extractCacheableTextFromSystemRaw(raw []byte) string {
	system := parseRawJSONView(raw)
	if !system.IsArray() {
		return ""
	}
	var builder strings.Builder
	system.ForEach(func(_, part gjson.Result) bool {
		if part.Get("cache_control.type").String() == "ephemeral" {
			if text := part.Get("text").String(); text != "" {
				_, _ = builder.WriteString(text)
			}
		}
		return true
	})
	return builder.String()
}

func extractCacheableTextFromMessagesRaw(raw []byte) string {
	messages := parseRawJSONView(raw)
	if !messages.IsArray() {
		return ""
	}
	var text string
	messages.ForEach(func(_, msg gjson.Result) bool {
		content := msg.Get("content")
		if !content.IsArray() {
			return true
		}
		found := false
		content.ForEach(func(_, part gjson.Result) bool {
			if part.Get("cache_control.type").String() == "ephemeral" {
				found = true
				return false
			}
			return true
		})
		if found {
			text = extractTextFromContentRaw(content)
			return false
		}
		return true
	})
	return text
}

func (s *GatewayService) hashContent(content string) string {
	h := xxhash.Sum64String(content)
	return strconv.FormatUint(h, 36)
}

// replaceModelInBody 替换请求体中的model字段
// 优先使用定点修改，尽量保持客户端原始字段顺序。
// sanitizeSystemText rewrites only the fixed OpenCode identity sentence (if present).
// We intentionally avoid broad keyword replacement in system prompts to prevent
// accidentally changing user-provided instructions.
// applyClaudeCodeOAuthMimicryToBody 将"非 Claude Code 客户端 + Claude OAuth 账号"
// 路径上原本只在 /v1/messages 里做的完整伪装应用到任意 body 上。
//
// 这是 /v1/messages 主路径上 rewriteSystemForNonClaudeCode +
// normalizeClaudeOAuthRequestBody 流程的通用版，供 OpenAI 协议兼容层
// (ForwardAsChatCompletions / ForwardAsResponses) 复用。
//
// 未抽离之前，OpenAI 协议兼容层仅做 injectClaudeCodePrompt（前置追加），
// 而仓内 /v1/messages 路径自己的注释明确说过"仅前置追加无法通过 Anthropic
// 第三方检测"；那条注释就是本函数存在的根因。
//
// 参数：
//   - ctx / c：用于读取指纹和 gateway settings；c 可为 nil（如 count_tokens）。
//   - account：必须是 OAuth 账号，且调用方已判断不是 Claude Code 客户端。
//   - body：已经 marshal 成 Anthropic /v1/messages 格式的请求体。
//   - systemRaw：body 中原始 system 字段（用于判断是否需要 rewrite）。
//   - model：最终会发给上游的模型 ID（用于 haiku 旁路 + metadata 版本选择）。
//
// 返回：改写后的 body。即使中间任何一步失败，也会退化成原 body（不会 panic）。
// buildOAuthMetadataUserIDFromBody 是 buildOAuthMetadataUserID 的变体，
// 适用于调用方手上没有 ParsedRequest 的场景（如 OpenAI 协议兼容层）。
//
// 与 buildOAuthMetadataUserID 的唯一区别：
//   - session hash 从 body 本体按同样规则重算，而不是读取 ParsedRequest 缓存值。
//   - 如果 body 里已经存在 metadata.user_id，则返回空（由 ensureClaudeOAuthMetadataUserID
//     自行决定是否覆盖）。
//
// buildStableSessionSeed 为伪装路径合成的 metadata.user_id session_id 生成"会话级稳定"种子。
//
// 真实 Claude Code 的 session_id 是进程级随机 UUID，在一段会话内跨请求保持不变。无状态代理
// 无法恢复该值，这里用"会话内不变的锚点"近似：账号 ID + 客户端区分因子 + 首条 user 消息文本。
// 对话在尾部追加 messages 时这三者都不变，因此 generateSessionUUID(seed) 跨轮稳定。
//
// 注意：粘性路由键 GenerateSessionHash 按设计逐轮变化（见其测试），本函数与之独立、互不影响。
// accountID 恒存在，故 seed 永不为空 —— 输出始终是确定性 UUID，而非随机值。
// sessionContextDiscriminator 把请求上下文（客户端 IP / 归一化 UA / API Key ID）拼成
// 一个跨客户端的区分因子，避免不同用户的相同首条消息派生出相同 session_id。
// GenerateSessionUUID creates a deterministic UUID4 from a seed string.
// SelectAccount 选择账号（粘性会话+优先级）
// SelectAccountForModel 选择支持指定模型的账号（粘性会话+优先级+模型映射）
// SelectAccountForModelWithExclusions selects an account supporting the requested model while excluding specified accounts.
// SelectAccountWithLoadAwareness selects account with load-awareness and wait plan.
// metadataUserID: 用于客户端亲和调度，从中提取客户端 ID
// sub2apiUserID: 系统用户 ID，用于二维亲和调度
// checkClaudeCodeRestriction 检查分组的 Claude Code 客户端限制
// 如果分组启用了 claude_code_only 且请求不是来自 Claude Code 客户端：
//   - 有降级分组：返回降级分组的 ID
//   - 无降级分组：返回 ErrClaudeCodeOnly 错误
//
// IsSingleAntigravityAccountGroup 检查指定分组是否只有一个 antigravity 平台的可调度账号。
// 用于 Handler 层在首次请求时提前设置 SingleAccountRetry context，
// 避免单账号分组收到 503 时错误地设置模型限流标记导致后续请求连续快速失败。
// isAccountInGroup checks if the account belongs to the specified group.
// When groupID is nil, returns true only for ungrouped accounts (no group assignments).
// isAccountSchedulableForQuota 检查账号是否在配额限制内
// 适用于配置了 quota_limit 的 apikey 和 bedrock 类型账号
// isAccountSchedulableForWindowCost 检查账号是否可根据窗口费用进行调度
// 仅适用于 Anthropic OAuth/SetupToken 账号
// 返回 true 表示可调度，false 表示不可调度
// rpmPrefetchContextKey is the context key for prefetched RPM counts.
// withRPMPrefetch 批量预取所有候选账号的 RPM 计数
// isAccountSchedulableForRPM 检查账号是否可根据 RPM 进行调度
// 仅适用于 Anthropic OAuth/SetupToken 账号
// IncrementAccountRPM increments the RPM counter for the given account.
// 已知 TOCTOU 竞态：调度时读取 RPM 计数与此处递增之间存在时间窗口，
// 高并发下可能短暂超出 RPM 限制。这是与 WindowCost 一致的 soft-limit
// 设计权衡——可接受的少量超额优于加锁带来的延迟和复杂度。
// checkAndRegisterSession 检查并注册会话，用于会话数量限制
// 仅适用于 Anthropic OAuth/SetupToken 账号
// sessionID: 会话标识符（使用粘性会话的 hash）
// 返回 true 表示允许（在限制内或会话已存在），false 表示拒绝（超出限制且是新会话）
// filterByMinPriority 过滤出优先级最小的账号集合
// filterByMinLoadRate 过滤出负载率最低的账号集合
// filterBySoonestReset 过滤出「会话窗口最早重置」的账号集合（use-it-or-lose-it）。
// 仅保留拥有未来重置时间（SessionWindowEnd 在当前时间之后）且最早的账号；
// 窗口为空或已过期的账号视为无活跃窗口、优先级最低。
// 当所有账号都没有活跃窗口时，返回原集合（不改变后续 LRU 选择）。
// selectByLRU 从集合中选择最久未用的账号
// 如果有多个账号具有相同的最小 LastUsedAt，则随机选择一个
// shuffleWithinSortGroups 对排序后的 accountWithLoad 切片，按 (Priority, LoadRate, LastUsedAt) 分组后组内随机打乱。
// 防止并发请求读取同一快照时，确定性排序导致所有请求命中相同账号。
// sameAccountWithLoadGroup 判断两个 accountWithLoad 是否属于同一排序组
// shuffleWithinPriorityAndLastUsed 对排序后的 []*Account 切片，按 (Priority, LastUsedAt) 分组后组内随机打乱。
//
// 注意：当 preferOAuth=true 时，需要保证 OAuth 账号在同组内仍然优先，否则会把排序时的偏好打散掉。
// 因此这里采用"组内分区 + 分区内 shuffle"的方式：
// - 先把同组账号按 (OAuth / 非 OAuth) 拆成两段，保持 OAuth 段在前；
// - 再分别在各段内随机打散，避免热点。
// sameAccountGroup 判断两个 Account 是否属于同一排序组（Priority + LastUsedAt）
// sameLastUsedAt 判断两个 LastUsedAt 是否相同（精度到秒）
// sortCandidatesForFallback 根据配置选择排序策略
// mode: "last_used"(按最后使用时间) 或 "random"(随机)
// sortAccountsByPriorityOnly 仅按优先级排序
// shuffleWithinPriority 在同优先级内随机打乱顺序
// selectAccountForModelWithPlatform 选择单平台账户（完全隔离）
// selectAccountWithMixedScheduling 选择账户（支持混合调度）
// 查询原生平台账户 + 启用 mixed_scheduling 的 antigravity 账户
// isModelSupportedByAccountWithContext 根据账户平台检查模型支持（带 context）
// 对于 Antigravity 平台，会先获取映射后的最终模型名（包括 thinking 后缀）再检查支持
// isModelSupportedByAccount 根据账户平台检查模型支持（无 context，用于非 Antigravity 平台）
// GetAccessToken 获取账号凭证
func (s *GatewayService) GetAccessToken(ctx context.Context, account *Account) (string, string, error) {
	switch account.Type {
	case AccountTypeOAuth, AccountTypeSetupToken:
		// Both oauth and setup-token use OAuth token flow
		return s.getOAuthToken(ctx, account)
	case AccountTypeAPIKey:
		apiKey := account.GetCredential("api_key")
		if apiKey == "" {
			return "", "", errors.New("api_key not found in credentials")
		}
		return apiKey, "apikey", nil
	case AccountTypeBedrock:
		return "", "bedrock", nil // Bedrock 使用 SigV4 签名或 API Key，由 forwardBedrock 处理
	case AccountTypeServiceAccount:
		if account.Platform != PlatformAnthropic {
			return "", "", fmt.Errorf("unsupported service account platform: %s", account.Platform)
		}
		if s.claudeTokenProvider == nil {
			return "", "", errors.New("claude token provider not configured")
		}
		accessToken, err := s.claudeTokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return "", "", err
		}
		return accessToken, "service_account", nil
	default:
		return "", "", fmt.Errorf("unsupported account type: %s", account.Type)
	}
}

func (s *GatewayService) getOAuthToken(ctx context.Context, account *Account) (string, string, error) {
	// 对于 Anthropic OAuth 账号，使用 ClaudeTokenProvider 获取缓存的 token
	if account.Platform == PlatformAnthropic && account.Type == AccountTypeOAuth && s.claudeTokenProvider != nil {
		accessToken, err := s.claudeTokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return "", "", err
		}
		return accessToken, "oauth", nil
	}

	// 其他情况（Gemini 有自己的 TokenProvider，setup-token 类型等）直接从账号读取
	accessToken := account.GetCredential("access_token")
	if accessToken == "" {
		return "", "", errors.New("access_token not found in credentials")
	}
	// Token刷新由后台 TokenRefreshService 处理，此处只返回当前token
	return accessToken, "oauth", nil
}

// shouldFailoverUpstreamError determines whether an upstream error should trigger account failover.
// isClaudeCodeClient 判断请求是否来自真正的 Claude Code 客户端。
// 判定条件：
//  1. User-Agent 匹配 claude-cli/X.Y.Z（大小写不敏感）
//  2. metadata.user_id 符合 Claude Code 格式（legacy 或 JSON 格式）
//
// 只检查 metadata.user_id 非空不够严格：第三方工具（opencode 等）可能伪造 UA
// 并附带任意 metadata.user_id 字符串，从而绕过 mimicry。必须通过 ParseMetadataUserID
// 验证格式才能确认是真正的 Claude Code 客户端。
func isClaudeCodeClient(userAgent string, metadataUserID string) bool {
	if !claudeCliUserAgentRe.MatchString(userAgent) {
		return false
	}
	return ParseMetadataUserID(metadataUserID) != nil
}

// normalizeSystemParam 将 json.RawMessage 类型的 system 参数转为标准 Go 类型（string / []any / nil），
// 避免 type switch 中 json.RawMessage（底层 []byte）无法匹配 case string / case []any / case nil 的问题。
// 这是 Go 的 typed nil 陷阱：(json.RawMessage, nil) ≠ (nil, nil)。
// systemIncludesClaudeCodePrompt 检查 system 中是否已包含 Claude Code 提示词
// 使用前缀匹配支持多种变体（标准版、Agent SDK 版等）
// hasClaudeCodePrefix 检查文本是否以 Claude Code 提示词的特征前缀开头
// injectClaudeCodePrompt 在 system 开头注入 Claude Code 提示词
// 处理 null、字符串、数组三种格式
// rewriteSystemForNonClaudeCode 将非 Claude Code 客户端的 system prompt 迁移至 messages，
// system 字段仅保留 Claude Code 标识提示词。
// Anthropic 基于 system 参数内容检测第三方应用，仅前置追加 Claude Code 提示词
// 无法通过检测，因为后续内容仍为非 Claude Code 格式。
// 策略：将原始 system prompt 提取并注入为 user/assistant 消息对，system 仅保留 Claude Code 标识。
// enforceCacheControlLimit 强制执行 cache_control 块数量限制（最多 4 个）
// 超限时优先移除工具断点，再移除 messages 断点，最后才移除 system 断点。
// injectAnthropicCacheControlTTL1h 将已有 ephemeral cache_control 块的 ttl 强制写为 1h。
// 仅修改已经存在的 cache_control，不新增缓存断点。
// shouldNormalizeClientDateline reports whether the request body's client
// dateline should be normalized before forwarding to Anthropic. The switch is
// scoped to Anthropic OAuth/SetupToken accounts only; API-Key accounts and
// non-Anthropic platforms bypass this step entirely.
// normalizeClientDatelineIfEnabled applies dateline normalization to body when
// the switch is on and the account qualifies. Returns (nextBody, true) only
// when the body actually changed; otherwise returns (nil, false) so callers
// can skip the writeback.
// Forward 转发请求到Claude API
// ApplyBedrockCCCompat 应用 Bedrock CC 兼容转换（渠道级模型映射后调用）
// 清理 body 中 Anthropic API 专有字段、修复 thinking/tool_use ID、过滤 beta token，
// 同时过滤 HTTP header 中的 anthropic-beta（防止 Passthrough 路径透传不支持的 token）。
func (s *GatewayService) ApplyBedrockCCCompat(c *gin.Context, body []byte, model string, account *Account, groupID *int64) []byte {
	if !s.isBedrockCCCompatEnabled(c.Request.Context(), account, groupID) {
		return body
	}
	body = sanitizeBedrockCCFields(body)
	body = sanitizeBedrockThinking(body, model)
	body = sanitizeBedrockToolUseIDs(body)
	body = sanitizeBedrockCCBetaTokens(body, model)
	// 过滤 HTTP header 中的 anthropic-beta，只保留 Bedrock 支持的 token
	if betaHeader := c.GetHeader("anthropic-beta"); betaHeader != "" {
		if filtered := ResolveBedrockBetaTokens(betaHeader, body, model); len(filtered) > 0 {
			c.Request.Header.Set("anthropic-beta", strings.Join(filtered, ", "))
		} else {
			c.Request.Header.Del("anthropic-beta")
		}
	}
	return body
}

// isBedrockCCCompatEnabled 检查渠道是否启用了 Bedrock CC 兼容模式
func (s *GatewayService) isBedrockCCCompatEnabled(ctx context.Context, account *Account, groupID *int64) bool {
	if groupID == nil || s.channelService == nil {
		return false
	}
	ch, err := s.channelService.GetChannelForGroup(ctx, *groupID)
	if err != nil || ch == nil {
		return false
	}
	return ch.IsBedrockCCCompatEnabled(account.Platform)
}

// forwardBedrock 转发请求到 AWS Bedrock
func (s *GatewayService) forwardBedrock(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
	startTime time.Time,
) (*ForwardResult, error) {
	reqModel := parsed.Model
	reqStream := parsed.Stream
	body := parsed.Body.Bytes()

	region := bedrockRuntimeRegion(account)
	mappedModel, ok := ResolveBedrockModelID(account, reqModel)
	if !ok {
		return nil, fmt.Errorf("unsupported bedrock model: %s", reqModel)
	}
	if mappedModel != reqModel {
		logger.LegacyPrintf("service.gateway", "[Bedrock] Model mapping: %s -> %s (account: %s)", reqModel, mappedModel, account.Name)
	}

	betaHeader := ""
	if c != nil && c.Request != nil {
		betaHeader = c.GetHeader("anthropic-beta")
	}

	// 准备请求体（注入 anthropic_version/anthropic_beta，移除 Bedrock 不支持的字段，清理 cache_control）
	betaTokens, err := s.resolveBedrockBetaTokensForRequest(ctx, account, betaHeader, body, mappedModel)
	if err != nil {
		return nil, err
	}

	bedrockBody, err := PrepareBedrockRequestBodyWithTokens(body, mappedModel, betaTokens, false)
	if err != nil {
		return nil, fmt.Errorf("prepare bedrock request body: %w", err)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	logger.LegacyPrintf("service.gateway", "[Bedrock] 命中 Bedrock 分支: account=%d name=%s model=%s->%s stream=%v",
		account.ID, account.Name, reqModel, mappedModel, reqStream)

	// 根据账号类型选择认证方式
	var signer *BedrockSigner
	var bedrockAPIKey string
	if account.IsBedrockAPIKey() {
		bedrockAPIKey = account.GetCredential("api_key")
		if bedrockAPIKey == "" {
			return nil, fmt.Errorf("api_key not found in bedrock credentials")
		}
	} else {
		signer, err = NewBedrockSignerFromAccount(account)
		if err != nil {
			return nil, fmt.Errorf("create bedrock signer: %w", err)
		}
	}

	// 执行上游请求（含重试）
	resp, err := s.executeBedrockUpstream(ctx, c, account, bedrockBody, mappedModel, region, reqStream, signer, bedrockAPIKey, proxyURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 将 Bedrock 的 x-amzn-requestid 映射到 x-request-id，
	// 使通用错误处理函数（handleErrorResponse、handleRetryExhaustedError）能正确提取 AWS request ID。
	if awsReqID := resp.Header.Get("x-amzn-requestid"); awsReqID != "" && resp.Header.Get("x-request-id") == "" {
		resp.Header.Set("x-request-id", awsReqID)
	}

	// 错误/failover 处理
	if resp.StatusCode >= 400 {
		return s.handleBedrockUpstreamErrors(ctx, resp, c, account)
	}

	// Bedrock 分支绕过通用 Forward 成功路径，这里保持上游接受回调语义一致。
	if parsed.OnUpstreamAccepted != nil {
		parsed.OnUpstreamAccepted()
	}

	// 响应处理
	var usage *ClaudeUsage
	var firstTokenMs *int
	var clientDisconnect bool
	if reqStream {
		streamResult, err := s.handleBedrockStreamingResponse(ctx, resp, c, account, startTime, reqModel)
		if err != nil {
			return nil, err
		}
		usage = streamResult.usage
		firstTokenMs = streamResult.firstTokenMs
		clientDisconnect = streamResult.clientDisconnect
	} else {
		usage, err = s.handleBedrockNonStreamingResponse(ctx, resp, c, account)
		if err != nil {
			return nil, err
		}
	}
	if usage == nil {
		usage = &ClaudeUsage{}
	}

	return &ForwardResult{
		RequestID:        resp.Header.Get("x-amzn-requestid"),
		Usage:            *usage,
		Model:            reqModel,
		UpstreamModel:    mappedModel,
		Stream:           reqStream,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnect,
	}, nil
}

// executeBedrockUpstream 执行 Bedrock 上游请求（含重试逻辑）
func (s *GatewayService) executeBedrockUpstream(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	modelID string,
	region string,
	stream bool,
	signer *BedrockSigner,
	apiKey string,
	proxyURL string,
) (*http.Response, error) {
	var resp *http.Response
	var err error
	retryStart := time.Now()
	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		var upstreamReq *http.Request
		if account.IsBedrockAPIKey() {
			upstreamReq, err = s.buildUpstreamRequestBedrockAPIKey(ctx, body, modelID, region, stream, apiKey)
		} else {
			upstreamReq, err = s.buildUpstreamRequestBedrock(ctx, body, modelID, region, stream, signer)
		}
		if err != nil {
			return nil, err
		}

		resp, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, nil)
		if err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			safeErr := sanitizeUpstreamErrorMessage(err.Error())
			setOpsUpstreamError(c, 0, safeErr, "")
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: 0,
				UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
				Kind:               "request_error",
				Message:            safeErr,
			})
			c.JSON(http.StatusBadGateway, gin.H{
				"type": "error",
				"error": gin.H{
					"type":    "upstream_error",
					"message": "Upstream request failed",
				},
			})
			return nil, fmt.Errorf("upstream request failed: %s", safeErr)
		}

		if resp.StatusCode >= 400 && resp.StatusCode != 400 && s.shouldRetryUpstreamError(account, resp.StatusCode) {
			if attempt < maxRetryAttempts {
				elapsed := time.Since(retryStart)
				if elapsed >= maxRetryElapsed {
					break
				}

				delay := retryBackoffDelay(attempt)
				remaining := maxRetryElapsed - elapsed
				if delay > remaining {
					delay = remaining
				}
				if delay <= 0 {
					break
				}

				respBody, _ := s.readUpstreamErrorBody(resp)
				_ = resp.Body.Close()
				appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
					Platform:           account.Platform,
					AccountID:          account.ID,
					AccountName:        account.Name,
					UpstreamStatusCode: resp.StatusCode,
					UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
					Kind:               "retry",
					Message:            extractUpstreamErrorMessage(respBody),
					Detail: func() string {
						if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
							return truncateString(string(respBody), s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes)
						}
						return ""
					}(),
				})
				logger.LegacyPrintf("service.gateway", "[Bedrock] account %d: upstream error %d, retry %d/%d after %v",
					account.ID, resp.StatusCode, attempt, maxRetryAttempts, delay)
				if err := sleepWithContext(ctx, delay); err != nil {
					return nil, err
				}
				continue
			}
			break
		}

		break
	}
	if resp == nil || resp.Body == nil {
		return nil, errors.New("upstream request failed: empty response")
	}
	return resp, nil
}

// handleBedrockUpstreamErrors 处理 Bedrock 上游 4xx/5xx 错误（failover + 错误响应）
func (s *GatewayService) handleBedrockUpstreamErrors(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
) (*ForwardResult, error) {
	// retry exhausted + failover
	if s.shouldRetryUpstreamError(account, resp.StatusCode) {
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			respBody, _ := s.readUpstreamErrorBody(resp)
			_ = resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(respBody))

			logger.LegacyPrintf("service.gateway", "[Bedrock] Upstream error (retry exhausted, failover): Account=%d(%s) Status=%d Body=%s",
				account.ID, account.Name, resp.StatusCode, truncateString(string(respBody), 1000))

			s.handleRetryExhaustedSideEffects(ctx, resp, account)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				Kind:               "retry_exhausted_failover",
				Message:            extractUpstreamErrorMessage(respBody),
			})
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return s.handleRetryExhaustedError(ctx, resp, c, account)
	}

	// non-retryable failover
	if s.shouldFailoverUpstreamError(resp.StatusCode) {
		respBody, _ := s.readUpstreamErrorBody(resp)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		s.handleFailoverSideEffects(ctx, resp, account)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			Kind:               "failover",
			Message:            extractUpstreamErrorMessage(respBody),
		})
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           respBody,
			RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}

	// other errors
	return s.handleErrorResponse(ctx, resp, c, account)
}

// buildUpstreamRequestBedrock 构建 Bedrock 上游请求
func (s *GatewayService) buildUpstreamRequestBedrock(
	ctx context.Context,
	body []byte,
	modelID string,
	region string,
	stream bool,
	signer *BedrockSigner,
) (*http.Request, error) {
	targetURL := BuildBedrockURL(region, modelID, stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// SigV4 签名
	if err := signer.SignRequest(ctx, req, body); err != nil {
		return nil, fmt.Errorf("sign bedrock request: %w", err)
	}

	return req, nil
}

// buildUpstreamRequestBedrockAPIKey 构建 Bedrock API Key (Bearer Token) 上游请求
func (s *GatewayService) buildUpstreamRequestBedrockAPIKey(
	ctx context.Context,
	body []byte,
	modelID string,
	region string,
	stream bool,
	apiKey string,
) (*http.Request, error) {
	targetURL := BuildBedrockURL(region, modelID, stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	return req, nil
}

// handleBedrockNonStreamingResponse 处理 Bedrock 非流式响应
// Bedrock InvokeModel 非流式响应的 body 格式与 Claude API 兼容
func (s *GatewayService) handleBedrockNonStreamingResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
) (*ClaudeUsage, error) {
	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, anthropicTooLargeError)
	if err != nil {
		return nil, err
	}

	// 转换 Bedrock 特有的 amazon-bedrock-invocationMetrics 为标准 Anthropic usage 格式
	// 并移除该字段避免透传给客户端
	body = transformBedrockInvocationMetrics(body)

	usage := parseClaudeUsageFromResponseBody(body)

	c.Header("Content-Type", "application/json")
	if v := resp.Header.Get("x-amzn-requestid"); v != "" {
		c.Header("x-request-id", v)
	}
	c.Data(resp.StatusCode, "application/json", body)
	return usage, nil
}

// vertexSupportedBetaTokens 是 Vertex AI 的 Anthropic 端点接受的 anthropic-beta
// 白名单。Vertex 对任何未知 token 直接 HTTP 400，故采用白名单（与 Bedrock 的
// bedrockSupportedBetaTokens 同思路）而非黑名单：未来 Claude Code 新增的、Vertex 尚未
// 支持的 token 天然被剥离。当 Vertex 新增支持某 beta 时在此补充。
//
// 明确排除（issue #3358 中 Vertex 报 400 的 token）：advisor-tool-2026-03-01、
// prompt-caching-scope-2026-01-05、redact-thinking-2026-02-12、
// thinking-token-count-2026-05-13；以及 claude-code-20250219 / oauth-2025-04-20 等
// 客户端身份 beta——Vertex service_account 走 Bearer 鉴权，不需要它们。
// filterVertexBetaTokens 解析 client 的 anthropic-beta header，先剔除 drop 集合中的
// token（BetaPolicy filter + 默认 drop），再只保留 Vertex 支持的 token，去重后逗号拼接。
// 返回最终 header（可能为空字符串）。
// getBetaHeader 处理anthropic-beta header
// 对于OAuth账号，需要确保包含oauth-2025-04-20
// computeFinalAnthropicBeta 计算发往上游的最终 anthropic-beta header 值。
//
// 设计动机：将原本在 buildUpstreamRequest 内联在一起、依赖 req.Header 的
// anthropic-beta 计算逻辑抽成纯函数。这样调用方可以在 NewRequest 之前
// 就提前拿到最终 beta header，进而能按它对 body 做能力维度 sanitize 后再做
// CCH 签名——一举修复了以下之前由顺序依赖导致的能力维度 sanitize
// 无法部署的问题（签名与最终 body 不一致可以被判 third-party）。
//
// 返回 (value, shouldSet)：
//   - shouldSet=false 意为“不主动设置 anthropic-beta header”，与原代码“
//     API-key 账号 + 客户端未传 anthropic-beta + InjectBetaForAPIKey 未开启或
//     requestNeedsBetaFeatures=false”的行为对齐。
//   - shouldSet=true 时 value 可能为空字符串（例如客户端透传的 beta 被 dropSet
//     全部过滤掉），这与原代码中 setHeaderRaw 的结果一致。
//
// clientHeaders 是客户端原始 HTTP header（通常为 c.Request.Header）；nil 时按“客户端
// 未传”处理。body 是已经 metadata 重写 / billing version sync 之后但未 sanitize 上游
// 不兼容字段之前的版本。
// computeFinalCountTokensAnthropicBeta 是 count_tokens 路径上 anthropic-beta header 的
// 计算纯函数。语义与 computeFinalAnthropicBeta 对齐，但备份了 count_tokens 独有的
// 两条特殊规则：
//
//   - OAuth mimic：requiredBetas 为 FullClaudeCodeMimicryBetas + BetaTokenCounting
//     （与 messages 不同的是：不按 haiku 排除；count_tokens 始终携带 token-counting beta）
//   - OAuth 透传 + 客户端未传 anthropic-beta：补齐 CountTokensBetaHeader
//   - OAuth 透传 + 客户端传了：补齐 BetaTokenCounting（如果未含）
//
// 返回语义同 computeFinalAnthropicBeta。
// stripBetaTokens removes the given beta tokens from a comma-separated header value.
// BetaBlockedError indicates a request was blocked by a beta policy rule.
// betaPolicyResult holds the evaluated result of beta policy rules for a single request.
// evaluateBetaPolicy loads settings once and evaluates all rules against the given request.
// mergeDropSets merges the static defaultDroppedBetasSet with dynamic policy filter tokens.
// Returns defaultDroppedBetasSet directly when policySet is empty (zero allocation).
// betaPolicyFilterSetKey is the gin.Context key for caching the policy filter set within a request.
// getBetaPolicyFilterSet returns the beta policy filter set, using the gin context cache if available.
// In the /v1/messages path, Forward() evaluates the policy first and caches the result;
// buildUpstreamRequest reuses it (zero extra DB calls). In the count_tokens path, this
// evaluates on demand (one DB call).
// betaPolicyScopeMatches checks whether a rule's scope matches the current account type.
// matchModelWhitelist checks if a model matches any pattern in the whitelist.
// Reuses matchModelPattern from group.go which supports exact and wildcard prefix matching.
// resolveRuleAction determines the effective action and error message for a rule given the request model.
// When ModelWhitelist is empty, the rule's primary Action/ErrorMessage applies unconditionally.
// When non-empty, Action applies to matching models; FallbackAction/FallbackErrorMessage applies to others.
// droppedBetaSet returns claude.DroppedBetas as a set, with optional extra tokens.
// containsBetaToken checks if a comma-separated header value contains the given token.
// checkBetaPolicyBlockForTokens 检查 token 列表中是否有被管理员 block 规则命中的 token。
// 用于补充 evaluateBetaPolicy 对 header 的检查，覆盖 body 自动注入的 token。
// applyClaudeCodeMimicHeaders forces "Claude Code-like" request headers.
// This mirrors opencode-anthropic-auth behavior: do not trust downstream
// headers when using Claude Code-scoped OAuth credentials.
// shouldRectifySignatureError 统一判断是否应触发签名整流（strip thinking blocks 并重试）。
// 根据账号类型检查对应的开关和匹配模式。
//
// mappedModel 用于按 thinking 协议族分流：passback-required (DeepSeek/Kimi/GLM 等) 上游
// 的 400 不是签名缺失问题，retry 任何 thinking 变形都会破坏「原样回传」契约——直接透传
// 错误给客户端。详见 thinking_protocol.go。
func (s *GatewayService) shouldRectifySignatureError(ctx context.Context, account *Account, respBody []byte, mappedModel string) bool {
	if !ShouldRectifyThinkingSignatureError(mappedModel) {
		return false
	}
	if account.Type == AccountTypeAPIKey {
		// API Key 账号：独立开关，一次读取配置
		settings, err := s.settingService.GetRectifierSettings(ctx)
		if err != nil || !settings.Enabled || !settings.APIKeySignatureEnabled {
			return false
		}
		// 先检查内置模式（同 OAuth），再检查自定义关键词
		if s.isThinkingBlockSignatureError(respBody) {
			return true
		}
		return matchSignaturePatterns(respBody, settings.APIKeySignaturePatterns)
	}
	// OAuth/SetupToken/Upstream/Bedrock 等：保持原有行为（内置模式 + 原开关）
	return s.isThinkingBlockSignatureError(respBody) && s.settingService.IsSignatureRectifierEnabled(ctx)
}

// isSignatureErrorPattern 仅做模式匹配，不检查开关。
// 用于已进入重试流程后的二阶段检测（此时开关已在首次调用时验证过）。
func (s *GatewayService) isSignatureErrorPattern(ctx context.Context, account *Account, respBody []byte) bool {
	if s.isThinkingBlockSignatureError(respBody) {
		return true
	}
	if account.Type == AccountTypeAPIKey {
		settings, err := s.settingService.GetRectifierSettings(ctx)
		if err != nil {
			return false
		}
		return matchSignaturePatterns(respBody, settings.APIKeySignaturePatterns)
	}
	return false
}

// matchSignaturePatterns 检查响应体是否匹配自定义关键词列表（不区分大小写）。
func matchSignaturePatterns(respBody []byte, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	bodyLower := strings.ToLower(string(respBody))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(bodyLower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// isThinkingBlockSignatureError 检测是否是thinking block相关错误
// 这类错误可以通过过滤thinking blocks并重试来解决
func (s *GatewayService) isThinkingBlockSignatureError(respBody []byte) bool {
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	if msg == "" {
		return false
	}

	// 检测signature相关的错误（更宽松的匹配）
	// 例如: "Invalid `signature` in `thinking` block", "***.signature" 等
	if strings.Contains(msg, "signature") {
		return true
	}

	// 检测 thinking block 顺序/类型错误
	// 例如: "Expected `thinking` or `redacted_thinking`, but found `text`"
	if strings.Contains(msg, "expected") && (strings.Contains(msg, "thinking") || strings.Contains(msg, "redacted_thinking")) {
		logger.LegacyPrintf("service.gateway", "[SignatureCheck] Detected thinking block type error")
		return true
	}

	// 检测 thinking block 被修改的错误
	// 例如: "thinking or redacted_thinking blocks in the latest assistant message cannot be modified"
	if strings.Contains(msg, "cannot be modified") && (strings.Contains(msg, "thinking") || strings.Contains(msg, "redacted_thinking")) {
		logger.LegacyPrintf("service.gateway", "[SignatureCheck] Detected thinking block modification error")
		return true
	}

	// 检测空消息内容错误（可能是过滤 thinking blocks 后导致的，或客户端发送了空 text block）
	// 例如: "all messages must have non-empty content"
	//       "messages: text content blocks must be non-empty"
	if strings.Contains(msg, "non-empty content") || strings.Contains(msg, "empty content") ||
		strings.Contains(msg, "content blocks must be non-empty") {
		logger.LegacyPrintf("service.gateway", "[SignatureCheck] Detected empty content error")
		return true
	}

	// 检测 thinking block 缺少 thinking 字段的错误（跨模型切换时常见：
	// 其他模型回过的 assistant 历史里有 type=thinking 但没有 thinking 文本，
	// 喂给开启 extended thinking 的 claude 时会被拒）
	// 例如: "messages.1.content.0.thinking: each thinking block must contain thinking"
	if strings.Contains(msg, "thinking block must contain") {
		logger.LegacyPrintf("service.gateway", "[SignatureCheck] Detected thinking block missing content error")
		return true
	}

	return false
}

func (s *GatewayService) shouldFailoverOn400(respBody []byte) bool {
	// 只对"可能是兼容性差异导致"的 400 允许切换，避免无意义重试。
	// 默认保守：无法识别则不切换。
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	if msg == "" {
		return false
	}

	// 缺少/错误的 beta header：换账号/链路可能成功（尤其是混合调度时）。
	// 更精确匹配 beta 相关的兼容性问题，避免误触发切换。
	if strings.Contains(msg, "anthropic-beta") ||
		strings.Contains(msg, "beta feature") ||
		strings.Contains(msg, "requires beta") {
		return true
	}

	// thinking/tool streaming 等兼容性约束（常见于中间转换链路）
	if strings.Contains(msg, "thinking") || strings.Contains(msg, "thought_signature") || strings.Contains(msg, "signature") {
		return true
	}
	if strings.Contains(msg, "tool_use") || strings.Contains(msg, "tool_result") || strings.Contains(msg, "tools") {
		return true
	}

	return false
}

// sanitizeStreamError 返回不含网络地址的客户端可见错误描述。
// 默认 (*net.OpError).Error() 会拼接 Source/Addr 字段，泄露内部 IP/端口与上游
// 服务器地址（例如 "read tcp 10.0.0.1:54321->52.1.2.3:443: read: connection
// reset by peer"）。该函数只保留可识别的错误类别，原始 err 仍在调用点写入日志。
func sanitizeStreamError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected EOF"
	case errors.Is(err, io.EOF):
		return "EOF"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline exceeded"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection reset by peer"
	case errors.Is(err, syscall.ECONNABORTED):
		return "connection aborted"
	case errors.Is(err, syscall.ETIMEDOUT):
		return "connection timed out"
	case errors.Is(err, syscall.EPIPE):
		return "broken pipe"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			if netErr.Op != "" {
				return netErr.Op + " timeout"
			}
			return "i/o timeout"
		}
		if netErr.Op != "" {
			return netErr.Op + " network error"
		}
	}
	return "upstream connection error"
}

// ExtractUpstreamErrorMessage 从上游响应体中提取错误消息
// 支持 Claude 风格的错误格式：{"type":"error","error":{"type":"...","message":"..."}}
func ExtractUpstreamErrorMessage(body []byte) string {
	return extractUpstreamErrorMessage(body)
}

func extractUpstreamErrorMessage(body []byte) string {
	// Claude 风格：{"type":"error","error":{"type":"...","message":"..."}}
	if m := gjson.GetBytes(body, "error.message").String(); strings.TrimSpace(m) != "" {
		inner := strings.TrimSpace(m)
		// 有些上游会把完整 JSON 作为字符串塞进 message
		if strings.HasPrefix(inner, "{") {
			if innerMsg := gjson.Get(inner, "error.message").String(); strings.TrimSpace(innerMsg) != "" {
				return innerMsg
			}
		}
		return m
	}

	// ChatGPT 内部 API 风格：{"detail":"..."}
	if d := gjson.GetBytes(body, "detail").String(); strings.TrimSpace(d) != "" {
		return d
	}

	// 兜底：尝试顶层 message
	return gjson.GetBytes(body, "message").String()
}

func extractUpstreamErrorCode(body []byte) string {
	if code := strings.TrimSpace(gjson.GetBytes(body, "error.code").String()); code != "" {
		return code
	}

	inner := strings.TrimSpace(gjson.GetBytes(body, "error.message").String())
	if !strings.HasPrefix(inner, "{") {
		return ""
	}

	if code := strings.TrimSpace(gjson.Get(inner, "error.code").String()); code != "" {
		return code
	}

	if lastBrace := strings.LastIndex(inner, "}"); lastBrace >= 0 {
		if code := strings.TrimSpace(gjson.Get(inner[:lastBrace+1], "error.code").String()); code != "" {
			return code
		}
	}

	return ""
}

func isCountTokensUnsupported404(statusCode int, body []byte) bool {
	if statusCode != http.StatusNotFound {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "/v1/messages/count_tokens") {
		return true
	}
	return strings.Contains(msg, "count_tokens") && strings.Contains(msg, "not found")
}

func (s *GatewayService) readUpstreamErrorBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	limit := gatewayUpstreamErrorBodyReadLimit
	if s != nil && s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody && s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes > int(limit) {
		limit = int64(s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func (s *GatewayService) handleErrorResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, requestedModel ...string) (*ForwardResult, error) {
	body, _ := s.readUpstreamErrorBody(resp)

	// 调试日志：打印上游错误响应
	logger.LegacyPrintf("service.gateway", "[Forward] Upstream error (non-retryable): Account=%d(%s) Status=%d RequestID=%s Body=%s",
		account.ID, account.Name, resp.StatusCode, resp.Header.Get("x-request-id"), truncateString(string(body), 1000))

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

	// Print a compact upstream request fingerprint when we hit the Claude Code OAuth
	// credential scope error. This avoids requiring env-var tweaks in a fixed deploy.
	if isClaudeCodeCredentialScopeError(upstreamMsg) && c != nil {
		if v, ok := c.Get(claudeMimicDebugInfoKey); ok {
			if line, ok := v.(string); ok && strings.TrimSpace(line) != "" {
				logger.LegacyPrintf("service.gateway", "[ClaudeMimicDebugOnError] status=%d request_id=%s %s",
					resp.StatusCode,
					resp.Header.Get("x-request-id"),
					line,
				)
			}
		}
	}

	// Enrich Ops error logs with upstream status + message, and optionally a truncated body snippet.
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               "http_error",
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})

	// 处理上游错误，标记账号状态
	shouldDisable := false
	if s.rateLimitService != nil {
		if len(requestedModel) > 0 {
			shouldDisable = s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body, requestedModel[0])
		} else {
			shouldDisable = s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
		}
	}
	if shouldDisable {
		return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: body}
	}

	MarkResponseCommitted(c)

	// 记录上游错误响应体摘要便于排障（可选：由配置控制；不回显到客户端）
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		logger.LegacyPrintf("service.gateway",
			"Upstream error %d (account=%d platform=%s type=%s): %s",
			resp.StatusCode,
			account.ID,
			account.Platform,
			account.Type,
			truncateForLog(body, s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes),
		)
	}

	// 非 failover 错误也支持错误透传规则匹配。
	if status, errType, errMsg, matched := applyErrorPassthroughRule(
		c,
		account.Platform,
		resp.StatusCode,
		body,
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	); matched {
		c.JSON(status, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": errMsg,
			},
		})

		summary := upstreamMsg
		if summary == "" {
			summary = errMsg
		}
		if summary == "" {
			return nil, fmt.Errorf("upstream error: %d (passthrough rule matched)", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d (passthrough rule matched) message=%s", resp.StatusCode, summary)
	}

	// 根据状态码返回适当的自定义错误响应（不透传上游详细信息）
	var errType, errMsg string
	var statusCode int

	switch resp.StatusCode {
	case 400:
		c.Data(http.StatusBadRequest, "application/json", body)
		summary := upstreamMsg
		if summary == "" {
			summary = truncateForLog(body, 512)
		}
		if summary == "" {
			return nil, fmt.Errorf("upstream error: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, summary)
	case 401:
		statusCode = http.StatusBadGateway
		errType = "upstream_error"
		errMsg = "Upstream authentication failed, please contact administrator"
	case 403:
		statusCode = http.StatusBadGateway
		errType = "upstream_error"
		errMsg = "Upstream access forbidden, please contact administrator"
	case 429:
		statusCode = http.StatusTooManyRequests
		errType = "rate_limit_error"
		errMsg = "Upstream rate limit exceeded, please retry later"
	case 529:
		statusCode = http.StatusServiceUnavailable
		errType = "overloaded_error"
		errMsg = "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		statusCode = http.StatusBadGateway
		errType = "upstream_error"
		errMsg = "Upstream service temporarily unavailable"
	default:
		statusCode = http.StatusBadGateway
		errType = "upstream_error"
		errMsg = "Upstream request failed"
	}

	// 返回自定义错误响应
	c.JSON(statusCode, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": errMsg,
		},
	})

	if upstreamMsg == "" {
		return nil, fmt.Errorf("upstream error: %d", resp.StatusCode)
	}
	return nil, fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
}

func (s *GatewayService) handleRetryExhaustedSideEffects(ctx context.Context, resp *http.Response, account *Account) {
	body, _ := s.readUpstreamErrorBody(resp)
	statusCode := resp.StatusCode

	// OAuth/Setup Token 账号的 403：标记账号异常
	if account.IsOAuth() && statusCode == 403 {
		s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, resp.Header, body)
		logger.LegacyPrintf("service.gateway", "Account %d: marked as error after %d retries for status %d", account.ID, maxRetryAttempts, statusCode)
	} else {
		// API Key 未配置错误码：不标记账号状态
		logger.LegacyPrintf("service.gateway", "Account %d: upstream error %d after %d retries (not marking account)", account.ID, statusCode, maxRetryAttempts)
	}
}

func (s *GatewayService) handleFailoverSideEffects(ctx context.Context, resp *http.Response, account *Account, requestedModel ...string) {
	body, _ := s.readUpstreamErrorBody(resp)
	if len(requestedModel) > 0 {
		s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body, requestedModel[0])
		return
	}
	s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
}

// handleRetryExhaustedError 处理重试耗尽后的错误
// OAuth 403：标记账号异常
// API Key 未配置错误码：仅返回错误，不标记账号
func (s *GatewayService) handleRetryExhaustedError(ctx context.Context, resp *http.Response, c *gin.Context, account *Account) (*ForwardResult, error) {
	MarkResponseCommitted(c)
	// Capture upstream error body before side-effects consume the stream.
	respBody, _ := s.readUpstreamErrorBody(resp)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	s.handleRetryExhaustedSideEffects(ctx, resp, account)

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

	if isClaudeCodeCredentialScopeError(upstreamMsg) && c != nil {
		if v, ok := c.Get(claudeMimicDebugInfoKey); ok {
			if line, ok := v.(string); ok && strings.TrimSpace(line) != "" {
				logger.LegacyPrintf("service.gateway", "[ClaudeMimicDebugOnError] status=%d request_id=%s %s",
					resp.StatusCode,
					resp.Header.Get("x-request-id"),
					line,
				)
			}
		}
	}

	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(respBody), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               "retry_exhausted",
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})

	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		logger.LegacyPrintf("service.gateway",
			"Upstream error %d retries_exhausted (account=%d platform=%s type=%s): %s",
			resp.StatusCode,
			account.ID,
			account.Platform,
			account.Type,
			truncateForLog(respBody, s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes),
		)
	}

	if status, errType, errMsg, matched := applyErrorPassthroughRule(
		c,
		account.Platform,
		resp.StatusCode,
		respBody,
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed after retries",
	); matched {
		c.JSON(status, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    errType,
				"message": errMsg,
			},
		})

		summary := upstreamMsg
		if summary == "" {
			summary = errMsg
		}
		if summary == "" {
			return nil, fmt.Errorf("upstream error: %d (retries exhausted, passthrough rule matched)", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d (retries exhausted, passthrough rule matched) message=%s", resp.StatusCode, summary)
	}

	// 返回统一的重试耗尽错误响应
	c.JSON(http.StatusBadGateway, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    "upstream_error",
			"message": "Upstream request failed after retries",
		},
	})

	if upstreamMsg == "" {
		return nil, fmt.Errorf("upstream error: %d (retries exhausted)", resp.StatusCode)
	}
	return nil, fmt.Errorf("upstream error: %d (retries exhausted) message=%s", resp.StatusCode, upstreamMsg)
}

// streamingResult 流式响应结果
type streamingResult struct {
	usage            *ClaudeUsage
	firstTokenMs     *int
	clientDisconnect bool // 客户端是否在流式传输过程中断开
}

func (s *GatewayService) handleStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, startTime time.Time, originalModel, mappedModel string, mimicClaudeCode bool) (*streamingResult, error) {
	// 更新5h窗口状态
	s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 透传其他响应头
	if v := resp.Header.Get("x-request-id"); v != "" {
		c.Header("x-request-id", v)
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	usage := &ClaudeUsage{}
	var firstTokenMs *int
	scanner := bufio.NewScanner(resp.Body)
	// 设置更大的buffer以处理长行
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanBuf := getSSEScannerBuf64K()
	scanner.Buffer(scanBuf[:0], maxLineSize)

	type scanEvent struct {
		line string
		err  error
	}
	// 独立 goroutine 读取上游，避免读取阻塞导致超时/keepalive无法处理
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	sendEvent := func(ev scanEvent) bool {
		select {
		case events <- ev:
			return true
		case <-done:
			return false
		}
	}
	var lastReadAt int64
	atomic.StoreInt64(&lastReadAt, time.Now().UnixNano())
	go func(scanBuf *sseScannerBuf64K) {
		defer putSSEScannerBuf64K(scanBuf)
		defer close(events)
		for scanner.Scan() {
			atomic.StoreInt64(&lastReadAt, time.Now().UnixNano())
			if !sendEvent(scanEvent{line: scanner.Text()}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = sendEvent(scanEvent{err: err})
		}
	}(scanBuf)
	defer close(done)

	streamInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamDataIntervalTimeout > 0 {
		streamInterval = time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
	}
	// 仅监控上游数据间隔超时，避免下游写入阻塞导致误判
	var intervalTicker *time.Ticker
	if streamInterval > 0 {
		intervalTicker = time.NewTicker(streamInterval)
		defer intervalTicker.Stop()
	}
	var intervalCh <-chan time.Time
	if intervalTicker != nil {
		intervalCh = intervalTicker.C
	}

	// 下游 keepalive：防止代理/Cloudflare Tunnel 因连接空闲而断开
	keepaliveInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamKeepaliveInterval > 0 {
		keepaliveInterval = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}
	var keepaliveTicker *time.Ticker
	if keepaliveInterval > 0 {
		keepaliveTicker = time.NewTicker(keepaliveInterval)
		defer keepaliveTicker.Stop()
	}
	var keepaliveCh <-chan time.Time
	if keepaliveTicker != nil {
		keepaliveCh = keepaliveTicker.C
	}
	lastDataAt := time.Now()

	// 仅发送一次错误事件，避免多次写入导致协议混乱（写失败时尽力通知客户端）。
	// 事件格式遵循 Anthropic SSE 标准：{"type":"error","error":{"type":<reason>,"message":<message>}}
	// 这样 Anthropic SDK / Claude Code 等客户端能按标准 error 类型解析，UI 能显示具体错误文案，
	// 服务端 ExtractUpstreamErrorMessage 也能从透传的 body 中提取 message。
	errorEventSent := false
	sendErrorEvent := func(reason, message string) {
		if errorEventSent {
			return
		}
		errorEventSent = true
		if message == "" {
			message = reason
		}
		body, err := json.Marshal(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    reason,
				"message": message,
			},
		})
		if err != nil {
			// json.Marshal 不可能在已知 string-only 输入上失败，保守 fallback
			body = []byte(fmt.Sprintf(`{"type":"error","error":{"type":%q,"message":%q}}`, reason, message))
		}
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", body)
		flusher.Flush()
	}

	needModelReplace := originalModel != mappedModel
	clientDisconnected := false // 客户端断开标志，断开后继续读取上游以获取完整usage
	sawTerminalEvent := false
	noopDeltaKeepalive := false
	if c != nil && c.Request != nil {
		noopDeltaKeepalive = claudeCodeNeedsNoopDeltaKeepalive(c.Request.UserAgent())
	}
	activeContentBlockIndex := -1
	activeContentBlockType := ""

	pendingEventLines := make([]string, 0, 4)

	processSSEEvent := func(lines []string) ([]string, string, *sseUsagePatch, error) {
		if len(lines) == 0 {
			return nil, "", nil, nil
		}

		eventName := ""
		dataLine := ""
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
				continue
			}
			if dataLine == "" && sseDataRe.MatchString(trimmed) {
				dataLine = sseDataRe.ReplaceAllString(trimmed, "")
			}
		}

		if eventName == "error" {
			return nil, dataLine, nil, &sseStreamErrorEventError{RawData: dataLine}
		}

		if dataLine == "" {
			return []string{strings.Join(lines, "\n") + "\n\n"}, "", nil, nil
		}

		if dataLine == "[DONE]" {
			sawTerminalEvent = true
			block := ""
			if eventName != "" {
				block = "event: " + eventName + "\n"
			}
			block += "data: " + dataLine + "\n\n"
			return []string{block}, dataLine, nil, nil
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(dataLine), &event); err != nil {
			// JSON 解析失败，直接透传原始数据
			block := ""
			if eventName != "" {
				block = "event: " + eventName + "\n"
			}
			block += "data: " + dataLine + "\n\n"
			return []string{block}, dataLine, nil, nil
		}

		eventType, _ := event["type"].(string)
		if eventName == "" {
			eventName = eventType
		}
		if eventType == "content_block_start" {
			if index, ok := event["index"].(float64); ok {
				activeContentBlockIndex = int(index)
			}
			activeContentBlockType = "text"
			if block, ok := event["content_block"].(map[string]any); ok {
				if blockType, ok := block["type"].(string); ok && blockType != "" {
					activeContentBlockType = blockType
				}
			}
		}
		if eventType == "content_block_stop" {
			activeContentBlockIndex = -1
			activeContentBlockType = ""
		}
		eventChanged := false

		// 兼容 Kimi cached_tokens → cache_read_input_tokens
		if eventType == "message_start" {
			if msg, ok := event["message"].(map[string]any); ok {
				if u, ok := msg["usage"].(map[string]any); ok {
					eventChanged = reconcileCachedTokens(u) || eventChanged
				}
			}
		}
		if eventType == "message_delta" {
			if u, ok := event["usage"].(map[string]any); ok {
				eventChanged = reconcileCachedTokens(u) || eventChanged
			}
		}

		// Cache TTL Override: 重写 SSE 事件中的 cache_creation 分类。
		// 账号级设置优先；全局 1h 请求注入开启时，默认把 usage 计费归回 5m。
		if overrideTarget, ok := s.resolveCacheTTLUsageOverrideTarget(ctx, account); ok {
			if eventType == "message_start" {
				if msg, ok := event["message"].(map[string]any); ok {
					if u, ok := msg["usage"].(map[string]any); ok {
						eventChanged = rewriteCacheCreationJSON(u, overrideTarget) || eventChanged
					}
				}
			}
			if eventType == "message_delta" {
				if u, ok := event["usage"].(map[string]any); ok {
					eventChanged = rewriteCacheCreationJSON(u, overrideTarget) || eventChanged
				}
			}
		}

		if needModelReplace {
			if msg, ok := event["message"].(map[string]any); ok {
				if model, ok := msg["model"].(string); ok && model == mappedModel {
					msg["model"] = originalModel
					eventChanged = true
				}
			}
		}

		usagePatch := s.extractSSEUsagePatch(event)
		if anthropicStreamEventIsTerminal(eventName, dataLine) {
			sawTerminalEvent = true
		}
		if !eventChanged {
			block := ""
			if eventName != "" {
				block = "event: " + eventName + "\n"
			}
			block += "data: " + dataLine + "\n\n"
			return []string{block}, dataLine, usagePatch, nil
		}

		newData, err := json.Marshal(event)
		if err != nil {
			// 序列化失败，直接透传原始数据
			block := ""
			if eventName != "" {
				block = "event: " + eventName + "\n"
			}
			block += "data: " + dataLine + "\n\n"
			return []string{block}, dataLine, usagePatch, nil
		}

		block := ""
		if eventName != "" {
			block = "event: " + eventName + "\n"
		}
		block += "data: " + string(newData) + "\n\n"
		return []string{block}, string(newData), usagePatch, nil
	}

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				// 上游完成，返回结果
				if !sawTerminalEvent {
					return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: clientDisconnected}, fmt.Errorf("stream usage incomplete: missing terminal event")
				}
				return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: clientDisconnected}, nil
			}
			if ev.err != nil {
				if sawTerminalEvent {
					return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: clientDisconnected}, nil
				}
				// 检测 context 取消（客户端断开会导致 context 取消，进而影响上游读取）
				if errors.Is(ev.err, context.Canceled) || errors.Is(ev.err, context.DeadlineExceeded) {
					return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: true}, fmt.Errorf("stream usage incomplete: %w", ev.err)
				}
				// 客户端已通过写入失败检测到断开，上游也出错了，返回已收集的 usage
				if clientDisconnected {
					return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: true}, fmt.Errorf("stream usage incomplete after disconnect: %w", ev.err)
				}
				// 客户端未断开，正常的错误处理
				if errors.Is(ev.err, bufio.ErrTooLong) {
					logger.LegacyPrintf("service.gateway", "SSE line too long: account=%d max_size=%d error=%v", account.ID, maxLineSize, ev.err)
					sendErrorEvent("response_too_large", fmt.Sprintf("upstream SSE line exceeded %d bytes", maxLineSize))
					return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, ev.err
				}
				// 上游中途读错误（unexpected EOF / connection reset 等，常见于 HTTP/2 GOAWAY）：
				// 若尚未向客户端写过任何字节，包成 UpstreamFailoverError 让 handler 层走 failover/重试。
				// 已经开始写流时 SSE 协议无 resume，只能透传错误事件给客户端。
				// 注意:面向客户端的 disconnectMsg 必须用 sanitizeStreamError 剥离地址,
				// 默认 *net.OpError 的 Error() 会泄露内部 IP/端口和上游地址。完整 ev.err
				// 仅在下方 LegacyPrintf 内部日志中保留供运维诊断。
				disconnectMsg := "upstream stream disconnected: " + sanitizeStreamError(ev.err)
				if !c.Writer.Written() {
					logger.LegacyPrintf("service.gateway", "Upstream stream read error before any client output (account=%d), failing over: %v", account.ID, ev.err)
					body, _ := json.Marshal(map[string]any{
						"type": "error",
						"error": map[string]string{
							"type":    "upstream_disconnected",
							"message": disconnectMsg,
						},
					})
					return nil, &UpstreamFailoverError{
						StatusCode:             http.StatusBadGateway,
						ResponseBody:           body,
						RetryableOnSameAccount: true,
					}
				}
				sendErrorEvent("stream_read_error", disconnectMsg)
				return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, fmt.Errorf("stream read error: %w", ev.err)
			}
			line := ev.line
			trimmed := strings.TrimSpace(line)

			if trimmed == "" {
				if len(pendingEventLines) == 0 {
					continue
				}

				outputBlocks, data, usagePatch, err := processSSEEvent(pendingEventLines)
				pendingEventLines = pendingEventLines[:0]
				if err != nil {
					if clientDisconnected {
						return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: true}, nil
					}
					return nil, err
				}

				for _, block := range outputBlocks {
					if !clientDisconnected {
						restored := reverseToolNamesIfPresent(c, []byte(block))
						if _, werr := fmt.Fprint(w, string(restored)); werr != nil {
							clientDisconnected = true
							logger.LegacyPrintf("service.gateway", "Client disconnected during streaming, continuing to drain upstream for billing")
							break
						}
						flusher.Flush()
						lastDataAt = time.Now()
					}
					if data != "" {
						if firstTokenMs == nil && data != "[DONE]" {
							ms := int(time.Since(startTime).Milliseconds())
							firstTokenMs = &ms
						}
						if usagePatch != nil {
							mergeSSEUsagePatch(usage, usagePatch)
						}
					}
				}
				continue
			}

			pendingEventLines = append(pendingEventLines, line)

		case <-intervalCh:
			lastRead := time.Unix(0, atomic.LoadInt64(&lastReadAt))
			if time.Since(lastRead) < streamInterval {
				continue
			}
			if clientDisconnected {
				return &streamingResult{usage: usage, firstTokenMs: firstTokenMs, clientDisconnect: true}, fmt.Errorf("stream usage incomplete after timeout")
			}
			logger.LegacyPrintf("service.gateway", "Stream data interval timeout: account=%d model=%s interval=%s", account.ID, originalModel, streamInterval)
			// 处理流超时，可能标记账户为临时不可调度或错误状态
			if s.rateLimitService != nil {
				s.rateLimitService.HandleStreamTimeout(ctx, account, originalModel)
			}
			sendErrorEvent("stream_timeout", fmt.Sprintf("upstream stream idle for %s", streamInterval))
			return &streamingResult{usage: usage, firstTokenMs: firstTokenMs}, fmt.Errorf("stream data interval timeout")

		case <-keepaliveCh:
			if clientDisconnected {
				continue
			}
			if time.Since(lastDataAt) < keepaliveInterval {
				continue
			}
			keepaliveEvent := "event: ping\ndata: {\"type\": \"ping\"}\n\n"
			if noopDeltaKeepalive && activeContentBlockIndex >= 0 {
				keepaliveEvent = anthropicNoopDeltaKeepaliveEvent(activeContentBlockIndex, activeContentBlockType)
			}
			if _, werr := fmt.Fprint(w, keepaliveEvent); werr != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.gateway", "Client disconnected during keepalive ping, continuing to drain upstream for billing")
				continue
			}
			flusher.Flush()
		}
	}

}

func (s *GatewayService) parseSSEUsage(data string, usage *ClaudeUsage) {
	if usage == nil {
		return
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return
	}

	if patch := s.extractSSEUsagePatch(event); patch != nil {
		mergeSSEUsagePatch(usage, patch)
	}
}

type sseUsagePatch struct {
	inputTokens              int
	hasInputTokens           bool
	outputTokens             int
	hasOutputTokens          bool
	cacheCreationInputTokens int
	hasCacheCreationInput    bool
	cacheReadInputTokens     int
	hasCacheReadInput        bool
	cacheCreation5mTokens    int
	hasCacheCreation5m       bool
	cacheCreation1hTokens    int
	hasCacheCreation1h       bool
}

func (s *GatewayService) extractSSEUsagePatch(event map[string]any) *sseUsagePatch {
	if len(event) == 0 {
		return nil
	}

	eventType, _ := event["type"].(string)
	switch eventType {
	case "message_start":
		msg, _ := event["message"].(map[string]any)
		usageObj, _ := msg["usage"].(map[string]any)
		if len(usageObj) == 0 {
			return nil
		}

		patch := &sseUsagePatch{}
		patch.hasInputTokens = true
		if v, ok := parseSSEUsageInt(usageObj["input_tokens"]); ok {
			patch.inputTokens = v
		}
		patch.hasCacheCreationInput = true
		if v, ok := parseSSEUsageInt(usageObj["cache_creation_input_tokens"]); ok {
			patch.cacheCreationInputTokens = v
		}
		patch.hasCacheReadInput = true
		if v, ok := parseSSEUsageInt(usageObj["cache_read_input_tokens"]); ok {
			patch.cacheReadInputTokens = v
		}
		if cc, ok := usageObj["cache_creation"].(map[string]any); ok {
			if v, exists := parseSSEUsageInt(cc["ephemeral_5m_input_tokens"]); exists {
				patch.cacheCreation5mTokens = v
				patch.hasCacheCreation5m = true
			}
			if v, exists := parseSSEUsageInt(cc["ephemeral_1h_input_tokens"]); exists {
				patch.cacheCreation1hTokens = v
				patch.hasCacheCreation1h = true
			}
		}
		return patch

	case "message_delta":
		usageObj, _ := event["usage"].(map[string]any)
		if len(usageObj) == 0 {
			return nil
		}

		patch := &sseUsagePatch{}
		if v, ok := parseSSEUsageInt(usageObj["input_tokens"]); ok && v > 0 {
			patch.inputTokens = v
			patch.hasInputTokens = true
		}
		if v, ok := parseSSEUsageInt(usageObj["output_tokens"]); ok && v > 0 {
			patch.outputTokens = v
			patch.hasOutputTokens = true
		}
		if v, ok := parseSSEUsageInt(usageObj["cache_creation_input_tokens"]); ok && v > 0 {
			patch.cacheCreationInputTokens = v
			patch.hasCacheCreationInput = true
		}
		if v, ok := parseSSEUsageInt(usageObj["cache_read_input_tokens"]); ok && v > 0 {
			patch.cacheReadInputTokens = v
			patch.hasCacheReadInput = true
		}
		if cc, ok := usageObj["cache_creation"].(map[string]any); ok {
			if v, exists := parseSSEUsageInt(cc["ephemeral_5m_input_tokens"]); exists && v > 0 {
				patch.cacheCreation5mTokens = v
				patch.hasCacheCreation5m = true
			}
			if v, exists := parseSSEUsageInt(cc["ephemeral_1h_input_tokens"]); exists && v > 0 {
				patch.cacheCreation1hTokens = v
				patch.hasCacheCreation1h = true
			}
		}
		return patch
	}

	return nil
}

func mergeSSEUsagePatch(usage *ClaudeUsage, patch *sseUsagePatch) {
	if usage == nil || patch == nil {
		return
	}

	if patch.hasInputTokens {
		usage.InputTokens = patch.inputTokens
	}
	if patch.hasCacheCreationInput {
		usage.CacheCreationInputTokens = patch.cacheCreationInputTokens
	}
	if patch.hasCacheReadInput {
		usage.CacheReadInputTokens = patch.cacheReadInputTokens
	}
	if patch.hasOutputTokens {
		usage.OutputTokens = patch.outputTokens
	}
	if patch.hasCacheCreation5m {
		usage.CacheCreation5mTokens = patch.cacheCreation5mTokens
	}
	if patch.hasCacheCreation1h {
		usage.CacheCreation1hTokens = patch.cacheCreation1hTokens
	}
}

func parseSSEUsageInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case int32:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if f, err := v.Float64(); err == nil {
			return int(f), true
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// applyCacheTTLOverride 将所有 cache creation tokens 归入指定的 TTL 类型。
// target 为 "5m" 或 "1h"。返回 true 表示发生了变更。
func applyCacheTTLOverride(usage *ClaudeUsage, target string) bool {
	// Fallback: 如果只有聚合字段但无 5m/1h 明细，将聚合字段归入 5m 默认类别
	if usage.CacheCreation5mTokens == 0 && usage.CacheCreation1hTokens == 0 && usage.CacheCreationInputTokens > 0 {
		usage.CacheCreation5mTokens = usage.CacheCreationInputTokens
	}

	total := usage.CacheCreation5mTokens + usage.CacheCreation1hTokens
	if total == 0 {
		return false
	}
	switch target {
	case "1h":
		if usage.CacheCreation1hTokens == total {
			return false // 已经全是 1h
		}
		usage.CacheCreation1hTokens = total
		usage.CacheCreation5mTokens = 0
	default: // "5m"
		if usage.CacheCreation5mTokens == total {
			return false // 已经全是 5m
		}
		usage.CacheCreation5mTokens = total
		usage.CacheCreation1hTokens = 0
	}
	return true
}

// rewriteCacheCreationJSON 在 JSON usage 对象中重写 cache_creation 嵌套对象的 TTL 分类。
// usageObj 是 usage JSON 对象（map[string]any）。
func rewriteCacheCreationJSON(usageObj map[string]any, target string) bool {
	ccObj, ok := usageObj["cache_creation"].(map[string]any)
	if !ok {
		return false
	}
	v5m, _ := parseSSEUsageInt(ccObj["ephemeral_5m_input_tokens"])
	v1h, _ := parseSSEUsageInt(ccObj["ephemeral_1h_input_tokens"])
	total := v5m + v1h
	if total == 0 {
		return false
	}
	switch target {
	case "1h":
		if v1h == total {
			return false
		}
		ccObj["ephemeral_1h_input_tokens"] = float64(total)
		ccObj["ephemeral_5m_input_tokens"] = float64(0)
	default: // "5m"
		if v5m == total {
			return false
		}
		ccObj["ephemeral_5m_input_tokens"] = float64(total)
		ccObj["ephemeral_1h_input_tokens"] = float64(0)
	}
	return true
}

func (s *GatewayService) resolveCacheTTLUsageOverrideTarget(ctx context.Context, account *Account) (string, bool) {
	if account == nil {
		return "", false
	}
	if account.IsCacheTTLOverrideEnabled() {
		return account.GetCacheTTLOverrideTarget(), true
	}
	if account.IsAnthropicOAuthOrSetupToken() && s != nil && s.settingService != nil && s.settingService.IsAnthropicCacheTTL1hInjectionEnabled(ctx) {
		return cacheTTLTarget5m, true
	}
	return "", false
}

func (s *GatewayService) handleNonStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, originalModel, mappedModel string) (*ClaudeUsage, error) {
	// 更新5h窗口状态
	s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)

	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, anthropicTooLargeError)
	if err != nil {
		return nil, err
	}

	// 解析usage
	var response struct {
		Usage ClaudeUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return nil, s.invalidNonStreamingJSONFailoverError(ctx, resp, account, body, err, mappedModel)
		}
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// 解析嵌套的 cache_creation 对象中的 5m/1h 明细
	cc5m := gjson.GetBytes(body, "usage.cache_creation.ephemeral_5m_input_tokens")
	cc1h := gjson.GetBytes(body, "usage.cache_creation.ephemeral_1h_input_tokens")
	if cc5m.Exists() || cc1h.Exists() {
		response.Usage.CacheCreation5mTokens = int(cc5m.Int())
		response.Usage.CacheCreation1hTokens = int(cc1h.Int())
	}

	// 兼容 Kimi cached_tokens → cache_read_input_tokens
	if response.Usage.CacheReadInputTokens == 0 {
		cachedTokens := gjson.GetBytes(body, "usage.cached_tokens").Int()
		if cachedTokens > 0 {
			response.Usage.CacheReadInputTokens = int(cachedTokens)
			if newBody, err := sjson.SetBytes(body, "usage.cache_read_input_tokens", cachedTokens); err == nil {
				body = newBody
			}
		}
	}

	// Cache TTL Override: 重写 non-streaming 响应中的 cache_creation 分类。
	// 账号级设置优先；全局 1h 请求注入开启时，默认把 usage 计费归回 5m。
	if overrideTarget, ok := s.resolveCacheTTLUsageOverrideTarget(ctx, account); ok {
		if applyCacheTTLOverride(&response.Usage, overrideTarget) {
			// 同步更新 body JSON 中的嵌套 cache_creation 对象
			if newBody, err := sjson.SetBytes(body, "usage.cache_creation.ephemeral_5m_input_tokens", response.Usage.CacheCreation5mTokens); err == nil {
				body = newBody
			}
			if newBody, err := sjson.SetBytes(body, "usage.cache_creation.ephemeral_1h_input_tokens", response.Usage.CacheCreation1hTokens); err == nil {
				body = newBody
			}
		}
	}

	// 如果有模型映射，替换响应中的model字段
	if originalModel != mappedModel {
		body = s.replaceModelInResponseBody(body, mappedModel, originalModel)
	}

	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)

	contentType := "application/json"
	if s.cfg != nil && !s.cfg.Security.ResponseHeaders.Enabled {
		if upstreamType := resp.Header.Get("Content-Type"); upstreamType != "" {
			contentType = upstreamType
		}
	}

	body = reverseToolNamesIfPresent(c, body)

	// 写入响应
	c.Data(resp.StatusCode, contentType, body)

	return &response.Usage, nil
}

// replaceModelInResponseBody 替换响应体中的model字段
// 使用 gjson/sjson 精确替换，避免全量 JSON 反序列化
func (s *GatewayService) replaceModelInResponseBody(body []byte, fromModel, toModel string) []byte {
	if m := gjson.GetBytes(body, "model"); m.Exists() && m.Str == fromModel {
		newBody, err := sjson.SetBytes(body, "model", toModel)
		if err != nil {
			return body
		}
		return newBody
	}
	return body
}

// RecordUsageInput 记录使用量的输入参数。
// 异步 worker 只接收计费所需快照，不能持有 ParsedRequest/RequestBodyRef 这类大请求体引用。
// APIKeyQuotaUpdater defines the interface for updating API Key quota and rate limit usage
// postUsageBillingParams 统一扣费所需的参数
// PlatformFromAPIKey 从 APIKey 关联的 Group 推导 platform 名称。
// apiKey 为 nil 或 Group 信息缺失时返回空串（调用方据此 short-circuit quota 累加）。
// 导出供 handler 层调用。
// QuotaPlatform 返回 user×platform 配额计量使用的平台标识。
// 强制平台路由（如 /antigravity）优先按 ctx 中的 ForcePlatform 计量，否则回退到
// APIKey 关联 Group 的平台。
//
// 注意：必须用带 ForcePlatform 的请求 context 调用（如 handler 的 c.Request.Context()）。
// 后扣运行在 worker 池的 background ctx 上没有 ForcePlatform，因此后扣平台由 handler
// 预先算定、经 RecordUsageInput.QuotaPlatform 传入，不要在后扣链路用 worker ctx 调用本函数。
// postUsageBilling is the legacy fallback billing path used when the unified
// billing repo is unavailable (nil). Production uses applyUsageBilling → repo.Apply
// for atomic billing. This path only runs in tests or degraded mode.
// notifyBalanceLow sends balance low notification after deduction.
// When result.NewBalance is available (from DB transaction RETURNING), it is used directly
// to reconstruct oldBalance, avoiding stale Redis reads and concurrent-deduction races.
// resolveOldBalance returns the pre-deduction balance.
// Prefers the DB transaction result (newBalance + cost) over snapshot.
// notifyAccountQuota sends account quota threshold notification after increment.
// When result.QuotaState is available (from DB transaction RETURNING), it is passed directly
// to avoid a separate DB read that may see stale or concurrently-modified data.
// billingDeps 扣费逻辑依赖的服务（由各 gateway service 提供）
// recordUsageOpts 内部选项，参数化普通计费与长上下文计费的差异点。
// RecordUsage 记录使用量并扣费（或更新订阅用量）
// RecordUsageLongContextInput 记录使用量的输入参数（支持长上下文双倍计费）
// RecordUsageWithLongContext 记录使用量并扣费，支持长上下文双倍计费（用于 Gemini）
// recordUsageCoreInput 是 recordUsageCore 的公共输入字段，从两种输入结构体中提取。
// recordUsageCore 是 RecordUsage 和 RecordUsageWithLongContext 的统一实现。
// LongContextThreshold > 0 时 Token 计费回退走 CalculateCostWithLongContext。
// calculateRecordUsageCost 根据请求类型和选项计算费用。
// resolveChannelPricing 检查指定模型是否存在渠道级别定价。
// 返回非 nil 的 ResolvedPricing 表示有渠道定价，nil 表示走默认定价路径。
// calculateImageCost 计算图片生成费用：渠道级别定价优先，否则走按次计费。
func groupImagePriceConfiguredForTier(cfg *ImagePriceConfig, sizeTier string) bool {
	if cfg == nil {
		return false
	}
	switch NormalizeImageBillingTierOrDefault(sizeTier) {
	case ImageBillingSize1K:
		return cfg.Price1K != nil
	case ImageBillingSize4K:
		return cfg.Price4K != nil
	default:
		return cfg.Price2K != nil
	}
}

// calculateTokenCost 计算 Token 计费：根据 opts 决定走普通/长上下文/渠道统一计费。
// buildRecordUsageLog 构建使用日志并设置计费模式。
// resolveBillingMode 根据计费结果和请求类型确定计费模式。
// ResolveChannelMapping 委托渠道服务解析模型映射
// ReplaceModelInBody 替换请求体中的模型名（导出供 handler 使用）
// IsModelRestricted 检查模型是否被渠道限制
// ResolveChannelMappingAndRestrict 解析渠道映射。
// 模型限制检查已移至调度阶段（checkChannelPricingRestriction），restricted 始终返回 false。
// checkChannelPricingRestriction 根据渠道计费基准检查模型是否受定价列表限制。
// 供调度阶段预检查（requested / channel_mapped）。
// upstream 需逐账号检查，此处返回 false。
// billingModelForRestriction 根据计费基准确定限制检查使用的模型。
// upstream 返回空（需逐账号检查）。
// isUpstreamModelRestrictedByChannel 检查账号映射后的上游模型是否受渠道定价限制。
// 仅在 BillingModelSource="upstream" 且 RestrictModels=true 时由调度循环调用。
// resolveAccountUpstreamModel 确定账号将请求模型映射为什么上游模型。
// needsUpstreamChannelRestrictionCheck 判断是否需要在调度循环中逐账号检查上游模型的渠道限制。
// isStickyAccountUpstreamRestricted 检查粘性会话命中的账号是否受 upstream 渠道限制。
// 合并 needsUpstreamChannelRestrictionCheck + isUpstreamModelRestrictedByChannel 两步调用，
// 供 sticky session 条件链使用，避免内联多个函数调用导致行过长。
// ForwardCountTokens 转发 count_tokens 请求到上游 API
// 特点：不记录使用量、仅支持非流式响应
func (s *GatewayService) ForwardCountTokens(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) error {
	if parsed == nil {
		s.countTokensError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return fmt.Errorf("parse request: empty request")
	}

	if account != nil && account.IsAnthropicAPIKeyPassthroughEnabled() {
		passthroughBody := parsed.Body.Bytes()
		if reqModel := parsed.Model; reqModel != "" {
			if mappedModel := account.GetMappedModel(reqModel); mappedModel != reqModel {
				passthroughBody = s.replaceModelInBody(passthroughBody, mappedModel)
				logger.LegacyPrintf("service.gateway", "CountTokens passthrough model mapping: %s -> %s (account: %s)", reqModel, mappedModel, account.Name)
			}
		}
		return s.forwardCountTokensAnthropicAPIKeyPassthrough(ctx, c, account, passthroughBody)
	}

	// Bedrock 不支持 count_tokens 端点
	if account != nil && account.IsBedrock() {
		s.countTokensError(c, http.StatusNotFound, "not_found_error", "count_tokens endpoint is not supported for Bedrock")
		return nil
	}

	body := parsed.Body.Bytes()
	replaceBody := func(next []byte) error {
		if err := parsed.ReplaceBody(next); err != nil {
			return fmt.Errorf("rewrite count_tokens body: %w", err)
		}
		body = parsed.Body.Bytes()
		return nil
	}
	reqModel := parsed.Model

	// Pre-filter: strip empty text blocks to prevent upstream 400.
	if err := replaceBody(StripEmptyTextBlocks(body)); err != nil {
		return err
	}

	isClaudeCodeCT := IsClaudeCodeClient(ctx) || isClaudeCodeClient(c.GetHeader("User-Agent"), parsed.MetadataUserID)
	shouldMimicClaudeCode := account.IsOAuth() && !isClaudeCodeCT

	if shouldMimicClaudeCode {
		normalizeOpts := claudeOAuthNormalizeOptions{stripSystemCacheControl: true}
		var normalizedBody []byte
		normalizedBody, reqModel = normalizeClaudeOAuthRequestBody(body, reqModel, normalizeOpts)
		if err := replaceBody(normalizedBody); err != nil {
			return err
		}

		if err := replaceBody(s.rewriteMessageCacheControlIfEnabled(ctx, body)); err != nil {
			return err
		}
		if rw := buildToolNameRewriteFromBody(body); rw != nil {
			if err := replaceBody(applyToolNameRewriteToBody(body, rw)); err != nil {
				return err
			}
		} else {
			if err := replaceBody(applyToolsLastCacheBreakpoint(body)); err != nil {
				return err
			}
		}
	}

	// Antigravity 账户不支持 count_tokens，返回 404 让客户端 fallback 到本地估算。
	// 返回 nil 避免 handler 层记录为错误，也不设置 ops 上游错误上下文。
	if account.Platform == PlatformAntigravity {
		s.countTokensError(c, http.StatusNotFound, "not_found_error", "count_tokens endpoint is not supported for this platform")
		return nil
	}

	// 应用模型映射：
	// - APIKey 账号：使用账号级别的显式映射（如果配置），否则透传原始模型名
	// - OAuth/SetupToken 账号：使用 Anthropic 标准映射（短ID → 长ID）
	if reqModel != "" {
		mappedModel := reqModel
		mappingSource := ""
		if account.Type == AccountTypeAPIKey {
			mappedModel = account.GetMappedModel(reqModel)
			if mappedModel != reqModel {
				mappingSource = "account"
			}
		}
		if mappingSource == "" && account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
			normalized := claude.NormalizeModelID(reqModel)
			if normalized != reqModel {
				mappedModel = normalized
				mappingSource = "prefix"
			}
		}
		if mappedModel != reqModel {
			originalReqModel := reqModel
			if err := replaceBody(s.replaceModelInBody(body, mappedModel)); err != nil {
				return err
			}
			reqModel = mappedModel
			parsed.Model = mappedModel
			logger.LegacyPrintf("service.gateway", "CountTokens model mapping applied: %s -> %s (account: %s, source=%s)", originalReqModel, mappedModel, account.Name, mappingSource)
		}
	}

	// 获取凭证
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to get access token")
		return err
	}

	// 构建上游请求
	upstreamReq, wireBody, err := s.buildCountTokensRequest(ctx, c, account, body, token, tokenType, reqModel, shouldMimicClaudeCode)
	if err != nil {
		s.countTokensError(c, http.StatusInternalServerError, "api_error", "Failed to build request")
		return err
	}
	// 先记录首发 wire body；如果后面进入 400 retry，retry 会基于未签名的逻辑 body 重新构建。
	acceptedWireBody := wireBody

	// 获取代理URL（自定义 base URL 模式下，proxy 通过 buildCustomRelayURL 作为查询参数传递）
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		if !account.IsCustomBaseURLEnabled() || account.GetCustomBaseURL() == "" {
			proxyURL = account.Proxy.URL()
		}
	}

	// 发送请求
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		setOpsUpstreamError(c, 0, sanitizeUpstreamErrorMessage(err.Error()), "")
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Request failed")
		return fmt.Errorf("upstream request failed: %w", err)
	}

	// 读取响应体
	countTokensTooLarge := func(c *gin.Context) {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Upstream response too large")
	}
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, countTokensTooLarge)
	_ = resp.Body.Close()
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to read response")
		}
		return err
	}

	// 检测 thinking block 签名错误（400）并重试一次（过滤 thinking blocks）
	if resp.StatusCode == 400 && s.shouldRectifySignatureError(ctx, account, respBody, reqModel) {
		logger.LegacyPrintf("service.gateway", "Account %d: detected thinking block signature error on count_tokens, retrying with filtered thinking blocks", account.ID)

		filteredBody := FilterThinkingBlocksForRetry(body, reqModel)
		retryReq, retryWireBody, buildErr := s.buildCountTokensRequest(ctx, c, account, filteredBody, token, tokenType, reqModel, shouldMimicClaudeCode)
		if buildErr == nil {
			retryResp, retryErr := s.httpUpstream.DoWithTLS(retryReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
			if retryErr == nil {
				if retryResp.StatusCode < 400 {
					// count_tokens 签名重试成功后记录最终 wire body，错误响应仍保留原 body 便于后续处理。
					acceptedWireBody = retryWireBody
				}
				resp = retryResp
				respBody, err = ReadUpstreamResponseBody(resp.Body, s.cfg, c, countTokensTooLarge)
				_ = resp.Body.Close()
				if err != nil {
					if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
						s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to read response")
					}
					return err
				}
			}
		}
	}

	if resp.StatusCode < 400 && !bytes.Equal(acceptedWireBody, body) {
		// count_tokens 成功后再同步最终 wire body，避免 retry 从已签名 body 派生。
		if err := replaceBody(acceptedWireBody); err != nil {
			return err
		}
	}

	// 处理错误响应
	if resp.StatusCode >= 400 {
		// 标记账号状态（429/529等）
		s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)

		// 记录上游错误摘要便于排障（不回显请求内容）
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			logger.LegacyPrintf("service.gateway",
				"count_tokens upstream error %d (account=%d platform=%s type=%s): %s",
				resp.StatusCode,
				account.ID,
				account.Platform,
				account.Type,
				truncateForLog(respBody, s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes),
			)
		}

		// 返回简化的错误响应
		errMsg := "Upstream request failed"
		switch resp.StatusCode {
		case 429:
			errMsg = "Rate limit exceeded"
		case 529:
			errMsg = "Service overloaded"
		}
		s.countTokensError(c, resp.StatusCode, "upstream_error", errMsg)
		if upstreamMsg == "" {
			return fmt.Errorf("upstream error: %d", resp.StatusCode)
		}
		return fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
	}

	// 透传成功响应
	c.Data(resp.StatusCode, "application/json", respBody)
	return nil
}

func (s *GatewayService) forwardCountTokensAnthropicAPIKeyPassthrough(ctx context.Context, c *gin.Context, account *Account, body []byte) error {
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to get access token")
		return err
	}
	if tokenType != "apikey" {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Invalid account token type")
		return fmt.Errorf("anthropic api key passthrough requires apikey token, got: %s", tokenType)
	}

	upstreamReq, err := s.buildCountTokensRequestAnthropicAPIKeyPassthrough(ctx, c, account, body, token)
	if err != nil {
		s.countTokensError(c, http.StatusInternalServerError, "api_error", "Failed to build request")
		return err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		setOpsUpstreamError(c, 0, sanitizeUpstreamErrorMessage(err.Error()), "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
			Passthrough:        true,
			Kind:               "request_error",
			Message:            sanitizeUpstreamErrorMessage(err.Error()),
		})
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Request failed")
		return fmt.Errorf("upstream request failed: %w", err)
	}

	countTokensTooLarge := func(c *gin.Context) {
		s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Upstream response too large")
	}
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, countTokensTooLarge)
	_ = resp.Body.Close()
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			s.countTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to read response")
		}
		return err
	}

	if resp.StatusCode >= 400 {
		if s.rateLimitService != nil {
			s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		}

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

		// 中转站不支持 count_tokens 端点时（404），返回 404 让客户端 fallback 到本地估算。
		// 仅在错误消息明确指向 count_tokens endpoint 不存在时生效，避免误吞其他 404（如错误 base_url）。
		// 返回 nil 避免 handler 层记录为错误，也不设置 ops 上游错误上下文。
		if isCountTokensUnsupported404(resp.StatusCode, respBody) {
			logger.LegacyPrintf("service.gateway",
				"[count_tokens] Upstream does not support count_tokens (404), returning 404: account=%d name=%s msg=%s",
				account.ID, account.Name, truncateString(upstreamMsg, 512))
			s.countTokensError(c, http.StatusNotFound, "not_found_error", "count_tokens endpoint is not supported by upstream")
			return nil
		}

		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
			Passthrough:        true,
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})

		errMsg := "Upstream request failed"
		switch resp.StatusCode {
		case 429:
			errMsg = "Rate limit exceeded"
		case 529:
			errMsg = "Service overloaded"
		}
		s.countTokensError(c, resp.StatusCode, "upstream_error", errMsg)
		if upstreamMsg == "" {
			return fmt.Errorf("upstream error: %d", resp.StatusCode)
		}
		return fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
	}

	writeAnthropicPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)
	return nil
}

func (s *GatewayService) buildCountTokensRequestAnthropicAPIKeyPassthrough(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) (*http.Request, error) {
	targetURL := claudeAPICountTokensURL
	baseURL := account.GetBaseURL()
	if baseURL != "" {
		validatedURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return nil, err
		}
		targetURL = validatedURL + "/v1/messages/count_tokens?beta=true"
	}
	body = sanitizeCountTokensRequestBody(body)

	// 同 buildUpstreamRequestAnthropicAPIKeyPassthrough：能力维度 sanitize。
	clientBeta := ""
	if c != nil && c.Request != nil {
		clientBeta = getHeaderRaw(c.Request.Header, "anthropic-beta")
	}
	if sanitized, changed := sanitizeAnthropicBodyForBetaTokens(body, clientBeta); changed {
		body = sanitized
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if !allowedHeaders[lowerKey] {
				continue
			}
			wireKey := resolveWireCasing(key)
			for _, v := range values {
				addHeaderRaw(req.Header, wireKey, v)
			}
		}
	}

	req.Header.Del("authorization")
	req.Header.Del("x-api-key")
	req.Header.Del("x-goog-api-key")
	req.Header.Del("cookie")
	setAnthropicAPIKeyAuthHeader(req.Header, account, token)

	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}
	if req.Header.Get("anthropic-version") == "" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	return req, nil
}

// buildCountTokensRequest 构建 count_tokens 上游请求
func (s *GatewayService) buildCountTokensRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token, tokenType, modelID string, mimicClaudeCode bool) (*http.Request, []byte, error) {
	// 确定目标 URL
	targetURL := claudeAPICountTokensURL
	if account.Type == AccountTypeAPIKey {
		baseURL := account.GetBaseURL()
		if baseURL != "" {
			validatedURL, err := s.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return nil, nil, err
			}
			targetURL = validatedURL + "/v1/messages/count_tokens?beta=true"
		}
	} else if account.IsCustomBaseURLEnabled() {
		customURL := account.GetCustomBaseURL()
		if customURL == "" {
			return nil, nil, fmt.Errorf("custom_base_url is enabled but not configured for account %d", account.ID)
		}
		validatedURL, err := s.validateUpstreamBaseURL(customURL)
		if err != nil {
			return nil, nil, err
		}
		targetURL = s.buildCustomRelayURL(validatedURL, "/v1/messages/count_tokens", account)
	}

	clientHeaders := http.Header{}
	if c != nil && c.Request != nil {
		clientHeaders = c.Request.Header
	}

	// OAuth 账号：应用统一指纹和重写 userID（受设置开关控制）
	// 如果启用了会话ID伪装，会在重写后替换 session 部分为固定值
	ctEnableFP, ctEnableMPT := true, false
	if s.settingService != nil {
		ctEnableFP, ctEnableMPT, _ = s.settingService.GetGatewayForwardingSettings(ctx)
	}
	var ctFingerprint *Fingerprint
	if account.IsOAuth() && s.identityService != nil {
		fp, err := s.identityService.GetOrCreateFingerprint(ctx, account.ID, clientHeaders)
		if err == nil {
			ctFingerprint = fp
			if !ctEnableMPT {
				accountUUID := account.GetExtraString("account_uuid")
				if accountUUID != "" && fp.ClientID != "" {
					if newBody, err := s.identityService.RewriteUserIDWithMasking(ctx, body, account, accountUUID, fp.ClientID, fp.UserAgent); err == nil && len(newBody) > 0 {
						body = newBody
					}
				}
			}
		}
	}

	// 同步 billing header cc_version 与实际发送的 User-Agent 版本
	if ctFingerprint != nil && ctEnableFP {
		body = syncBillingHeaderVersion(body, ctFingerprint.UserAgent)
	}

	// === 计算最终 anthropic-beta header（先于 body sanitize 与 CCH 签名）===
	// 顺序约束同 buildUpstreamRequest。
	ctEffectiveDropSet := mergeDropSets(s.getBetaPolicyFilterSet(ctx, c, account, modelID))
	finalBetaHeader, finalBetaShouldSet := s.computeFinalCountTokensAnthropicBeta(
		tokenType, mimicClaudeCode, modelID, clientHeaders, body, ctEffectiveDropSet,
	)

	// 能力维度 body sanitize：与最终 anthropic-beta header 对称
	if sanitized, changed := sanitizeAnthropicBodyForBetaTokens(body, finalBetaHeader); changed {
		body = sanitized
	}

	body = sanitizeCountTokensRequestBody(body)

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}

	// 设置认证头（保持原始大小写）
	if tokenType == "oauth" {
		setHeaderRaw(req.Header, "authorization", "Bearer "+token)
	} else {
		setHeaderRaw(req.Header, "x-api-key", token)
	}

	// 白名单透传 headers（恢复真实 wire casing）
	for key, values := range clientHeaders {
		lowerKey := strings.ToLower(key)
		if allowedHeaders[lowerKey] {
			wireKey := resolveWireCasing(key)
			for _, v := range values {
				addHeaderRaw(req.Header, wireKey, v)
			}
		}
	}

	// OAuth 账号：应用指纹到请求头（受设置开关控制）
	if ctEnableFP && ctFingerprint != nil {
		s.identityService.ApplyFingerprint(req, ctFingerprint)
	}

	// 确保必要的 headers 存在（保持原始大小写）
	if getHeaderRaw(req.Header, "content-type") == "" {
		setHeaderRaw(req.Header, "content-type", "application/json")
	}
	if getHeaderRaw(req.Header, "anthropic-version") == "" {
		setHeaderRaw(req.Header, "anthropic-version", "2023-06-01")
	}
	if tokenType == "oauth" {
		applyClaudeOAuthHeaderDefaults(req)
	}

	// OAuth + mimic Claude Code：强制注入 CLI 指纹 header
	if tokenType == "oauth" && mimicClaudeCode {
		applyClaudeCodeMimicHeaders(req, false)
	}

	// 写入最终 anthropic-beta header（Del 一次避免白名单透传值残留）
	deleteHeaderAllForms(req.Header, "anthropic-beta")
	if finalBetaShouldSet {
		setHeaderRaw(req.Header, "anthropic-beta", finalBetaHeader)
	}

	// 同步 X-Claude-Code-Session-Id 头：取 body 中已处理的 metadata.user_id 的 session_id 覆盖
	if sessionHeader := getHeaderRaw(req.Header, "X-Claude-Code-Session-Id"); sessionHeader != "" {
		if uid := gjson.GetBytes(body, "metadata.user_id").String(); uid != "" {
			if parsed := ParseMetadataUserID(uid); parsed != nil {
				setHeaderRaw(req.Header, "X-Claude-Code-Session-Id", parsed.SessionID)
			}
		}
	}

	if c != nil && tokenType == "oauth" {
		c.Set(claudeMimicDebugInfoKey, buildClaudeMimicDebugLine(req, body, account, tokenType, mimicClaudeCode))
	}
	if s.debugClaudeMimicEnabled() {
		logClaudeMimicDebug(req, body, account, tokenType, mimicClaudeCode)
	}

	return req, body, nil
}

func sanitizeCountTokensRequestBody(body []byte) []byte {
	out := body
	for _, path := range []string{
		"temperature",
		"top_p",
		"top_k",
		"stream",
		"stop_sequences",
		"stop",
	} {
		if gjson.GetBytes(out, path).Exists() {
			if next, ok := deleteJSONPathBytes(out, path); ok {
				out = next
			}
		}
	}
	return out
}

// countTokensError 返回 count_tokens 错误响应
func (s *GatewayService) countTokensError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// buildCustomRelayURL 构建自定义中继转发 URL
// 在 path 后附加 beta=true 和可选的 proxy 查询参数
// GetAvailableModels returns the list of models available for a group
// It aggregates model_mapping keys from all schedulable accounts in the group
func (s *GatewayService) GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string {
	cacheKey := modelsListCacheKey(groupID, platform)
	if s.modelsListCache != nil {
		if cached, found := s.modelsListCache.Get(cacheKey); found {
			if models, ok := cached.([]string); ok {
				modelsListCacheHitTotal.Add(1)
				return cloneStringSlice(models)
			}
		}
	}
	modelsListCacheMissTotal.Add(1)

	var accounts []Account
	var err error

	if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupID(ctx, *groupID)
	} else {
		accounts, err = s.accountRepo.ListSchedulable(ctx)
	}

	if err != nil || len(accounts) == 0 {
		return nil
	}

	// Filter by platform if specified
	if platform != "" {
		filtered := make([]Account, 0)
		for _, acc := range accounts {
			if acc.Platform == platform {
				filtered = append(filtered, acc)
			}
		}
		accounts = filtered
	}

	// Collect unique models from all accounts
	modelSet := make(map[string]struct{})
	hasAnyMapping := false

	for _, acc := range accounts {
		mapping := acc.GetModelMapping()
		if len(mapping) > 0 {
			hasAnyMapping = true
			for model := range mapping {
				modelSet[model] = struct{}{}
			}
		}
	}

	// If no account has model_mapping, return nil (use default)
	if !hasAnyMapping {
		if s.modelsListCache != nil {
			s.modelsListCache.Set(cacheKey, []string(nil), s.modelsListCacheTTL)
			modelsListCacheStoreTotal.Add(1)
		}
		return nil
	}

	// Convert to slice
	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	sort.Strings(models)

	if s.modelsListCache != nil {
		s.modelsListCache.Set(cacheKey, cloneStringSlice(models), s.modelsListCacheTTL)
		modelsListCacheStoreTotal.Add(1)
	}
	return cloneStringSlice(models)
}

// GetSchedulablePlatforms returns the concrete platforms that currently have
// schedulable accounts in the target group.
func (s *GatewayService) GetSchedulablePlatforms(ctx context.Context, groupID *int64) map[string]struct{} {
	platforms := make(map[string]struct{})
	if s == nil || s.accountRepo == nil {
		return platforms
	}

	var accounts []Account
	var err error
	if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupID(ctx, *groupID)
	} else {
		accounts, err = s.accountRepo.ListSchedulable(ctx)
	}
	if err != nil {
		return platforms
	}

	for _, acc := range accounts {
		platform := strings.TrimSpace(acc.Platform)
		if platform != "" {
			platforms[platform] = struct{}{}
		}
	}
	return platforms
}

func (s *GatewayService) InvalidateAvailableModelsCache(groupID *int64, platform string) {
	if s == nil || s.modelsListCache == nil {
		return
	}

	normalizedPlatform := strings.TrimSpace(platform)
	// 完整匹配时精准失效；否则按维度批量失效。
	if groupID != nil && normalizedPlatform != "" {
		s.modelsListCache.Delete(modelsListCacheKey(groupID, normalizedPlatform))
		return
	}

	targetGroup := derefGroupID(groupID)
	for key := range s.modelsListCache.Items() {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		groupPart, parseErr := strconv.ParseInt(parts[0], 10, 64)
		if parseErr != nil {
			continue
		}
		if groupID != nil && groupPart != targetGroup {
			continue
		}
		if normalizedPlatform != "" && parts[1] != normalizedPlatform {
			continue
		}
		s.modelsListCache.Delete(key)
	}
}

// reconcileCachedTokens 兼容 Kimi 等上游：
// 将 OpenAI 风格的 cached_tokens 映射到 Claude 标准的 cache_read_input_tokens
func reconcileCachedTokens(usage map[string]any) bool {
	if usage == nil {
		return false
	}
	cacheRead, _ := usage["cache_read_input_tokens"].(float64)
	if cacheRead > 0 {
		return false // 已有标准字段，无需处理
	}
	cached, _ := usage["cached_tokens"].(float64)
	if cached <= 0 {
		return false
	}
	usage["cache_read_input_tokens"] = cached
	return true
}

const debugGatewayBodyDefaultFilename = "gateway_debug.log"

// initDebugGatewayBodyFile 初始化网关调试日志文件。
//
//   - "1"/"true" 等布尔值 → 当前目录下 gateway_debug.log
//   - 已有目录路径        → 该目录下 gateway_debug.log
//   - 其他               → 视为完整文件路径
func (s *GatewayService) initDebugGatewayBodyFile(path string) {
	if parseDebugEnvBool(path) {
		path = debugGatewayBodyDefaultFilename
	}

	// 如果 path 指向一个已存在的目录，自动追加默认文件名
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, debugGatewayBodyDefaultFilename)
	}

	// 确保父目录存在
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create gateway debug log directory", "dir", dir, "error", err)
			return
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("failed to open gateway debug log file", "path", path, "error", err)
		return
	}
	s.debugGatewayBodyFile.Store(f)
	slog.Info("gateway debug logging enabled", "path", path)
}

// debugLogGatewaySnapshot 将网关请求的完整快照（headers + body）写入独立的调试日志文件，
// 用于对比客户端原始请求和上游转发请求。
//
// 启用方式（环境变量）：
//
//	SUB2API_DEBUG_GATEWAY_BODY=1                          # 写入 gateway_debug.log
//	SUB2API_DEBUG_GATEWAY_BODY=/tmp/gateway_debug.log     # 写入指定路径
//
// tag: "CLIENT_ORIGINAL" 或 "UPSTREAM_FORWARD"
func (s *GatewayService) debugLogGatewaySnapshot(tag string, headers http.Header, body []byte, extra map[string]string) {
	f := s.debugGatewayBodyFile.Load()
	if f == nil {
		return
	}

	var buf strings.Builder
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(&buf, "\n========== [%s] %s ==========\n", ts, tag)

	// 1. context
	if len(extra) > 0 {
		fmt.Fprint(&buf, "--- context ---\n")
		extraKeys := make([]string, 0, len(extra))
		for k := range extra {
			extraKeys = append(extraKeys, k)
		}
		sort.Strings(extraKeys)
		for _, k := range extraKeys {
			fmt.Fprintf(&buf, "  %s: %s\n", k, extra[k])
		}
	}

	// 2. headers（按真实 Claude CLI wire 顺序排列，便于与抓包对比；auth 脱敏）
	fmt.Fprint(&buf, "--- headers ---\n")
	for _, k := range sortHeadersByWireOrder(headers) {
		for _, v := range headers[k] {
			fmt.Fprintf(&buf, "  %s: %s\n", k, safeHeaderValueForLog(k, v))
		}
	}

	// 3. body（完整输出，格式化 JSON 便于 diff）
	fmt.Fprint(&buf, "--- body ---\n")
	if len(body) == 0 {
		fmt.Fprint(&buf, "  (empty)\n")
	} else {
		var pretty bytes.Buffer
		if json.Indent(&pretty, body, "  ", "  ") == nil {
			fmt.Fprintf(&buf, "  %s\n", pretty.Bytes())
		} else {
			// JSON 格式化失败时原样输出
			fmt.Fprintf(&buf, "  %s\n", body)
		}
	}

	// 写入文件（调试用，并发写入可能交错但不影响可读性）
	_, _ = f.WriteString(buf.String())
}
