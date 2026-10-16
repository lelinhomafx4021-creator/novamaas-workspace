package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

// Cache usage audit explains, per request, how /v1/messages responses that
// were converted from an OpenAI-compatible upstream reported cache reads and
// writes inside input_tokens as well as in the dedicated cache fields. The
// platform settled these requests correctly; the audit gives customers the
// token counts their own Anthropic-standard billing double-counted.

const (
	CacheAuditAffected    = "affected"     // cache tokens were duplicated into input_tokens
	CacheAuditReview      = "review"       // converted path, but logs cannot pin the old value
	CacheAuditNoEvidence  = "no_evidence"  // log predates request path / conversion metadata
	CacheAuditNotAffected = "not_affected" // native Claude, other protocol, no cache, or after the fix
	CacheAuditRefund      = "refund"       // refund ledger rows carry amounts only

	cacheUsageAuditMaxPeriod = 93 * 24 * 60 * 60
	cacheUsageAuditMaxRows   = 2000

	// Stamped on consume logs whose /v1/messages response was produced by the
	// converter that excludes cache tokens from input_tokens.
	CacheUsageWireVersionKey = "claude_wire_usage_version"
	CacheUsageWireVersion    = 2
)

var ErrCacheUsageAuditPeriod = errors.New("cache usage audit period must be between 1 second and 93 days")

type CacheUsageAuditRequest struct {
	UserID  int    `json:"user_id"`
	TokenID int    `json:"token_id"`
	StartAt int64  `json:"start_at"`
	EndAt   int64  `json:"end_at"`
	Model   string `json:"model"`
	// FixedAt is when the corrected converter went live; later logs are
	// treated as unaffected even without the version stamp. Zero disables it.
	FixedAt int64  `json:"fixed_at"`
	Status  string `json:"status"` // optional row filter for the returned detail
	// DownstreamInputPrices maps model -> the customer's own uncached input
	// price per 1M tokens, in the report currency. Used only to estimate how
	// much the customer over-billed their users.
	DownstreamInputPrices map[string]string `json:"downstream_input_prices"`
}

// CacheUsageCompensation prices the duplicated tokens. The cache portion was
// already billed separately, so the over-billing is exactly
// duplicate tokens x the uncached input price.
type CacheUsageCompensation struct {
	List          string `json:"list"`                 // at the platform's historical base input price
	Group         string `json:"group"`                // at list price x historical group ratio
	Downstream    string `json:"downstream,omitempty"` // at the customer's own input price
	ListMax       string `json:"list_max"`             // includes the upper bound of review rows
	GroupMax      string `json:"group_max"`
	DownstreamMax string `json:"downstream_max,omitempty"`
}

type CacheUsageTokens struct {
	TotalInput int64 `json:"total_input"` // platform view: uncached + cache read + cache write
	Input      int64 `json:"input"`       // uncached input, Anthropic input_tokens
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	Output     int64 `json:"output"`
}

type CacheUsageCost struct {
	Input      string `json:"input"`
	CacheRead  string `json:"cache_read"`
	CacheWrite string `json:"cache_write"`
	Output     string `json:"output"`
	Total      string `json:"total"`
}

type CacheUsageAuditRow struct {
	LogID         int    `json:"log_id"`
	RequestID     string `json:"request_id"`
	CreatedAt     int64  `json:"created_at"`
	Type          int    `json:"type"`
	UserID        int    `json:"user_id"`
	Username      string `json:"username"`
	TokenID       int    `json:"token_id"`
	TokenName     string `json:"token_name"`
	Model         string `json:"model"`
	Group         string `json:"group"`
	IsStream      bool   `json:"is_stream"`
	RequestPath   string `json:"request_path"`
	UsageSemantic string `json:"usage_semantic"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	PriceIssue    string `json:"price_issue,omitempty"`
	PriceMismatch bool   `json:"price_mismatch,omitempty"`
	GroupRatio    string `json:"group_ratio"`
	ActualQuota   int64  `json:"actual_quota"`
	ActualAmount  string `json:"actual_amount"`
	// What /v1/messages returned versus the Anthropic-standard value.
	ReportedInputTokens int64 `json:"reported_input_tokens"`
	CorrectInputTokens  int64 `json:"correct_input_tokens"`
	DuplicateTokens     int64 `json:"duplicate_tokens"`
	DuplicateTokensMax  int64 `json:"duplicate_tokens_max"`

	Tokens         CacheUsageTokens `json:"tokens"`
	ListCost       *CacheUsageCost  `json:"list_cost,omitempty"`       // base model price, no group ratio
	DiscountedCost *CacheUsageCost  `json:"discounted_cost,omitempty"` // list price x historical group ratio
	// InputPricePerMillion is the historical uncached input list price.
	InputPricePerMillion     string                  `json:"input_price_per_million,omitempty"`
	CacheReadPricePerMillion string                  `json:"cache_read_price_per_million,omitempty"`
	Compensation             *CacheUsageCompensation `json:"compensation,omitempty"`
}

type CacheUsageAuditTotals struct {
	Key                string           `json:"key"`
	Requests           int64            `json:"requests"`
	AffectedRequests   int64            `json:"affected_requests"`
	ReviewRequests     int64            `json:"review_requests"`
	NoEvidenceRequests int64            `json:"no_evidence_requests"`
	PricedRequests     int64            `json:"priced_requests"`
	Tokens             CacheUsageTokens `json:"tokens"`
	DuplicateTokens    int64            `json:"duplicate_tokens"`
	DuplicateTokensMax int64            `json:"duplicate_tokens_max"`
	// input_tokens across affected requests only: what /v1/messages returned
	// versus the Anthropic-standard value. Their difference is DuplicateTokens.
	AffectedReportedInput int64 `json:"affected_reported_input"`
	AffectedCorrectInput  int64 `json:"affected_correct_input"`
	// Historical list prices per 1M tokens of the duplicated requests. When a
	// group mixes several prices (price changes, groups), PriceMixed is set and
	// the first price is shown; compensation is still summed per request.
	InputPricePerMillion     string `json:"input_price_per_million,omitempty"`
	CacheReadPricePerMillion string `json:"cache_read_price_per_million,omitempty"`
	PriceMixed               bool   `json:"price_mixed,omitempty"`
	ActualAmount             string `json:"actual_amount"`
	ListAmount               string `json:"list_amount"`       // priced requests only
	DiscountedAmount         string `json:"discounted_amount"` // priced requests only

	Compensation CacheUsageCompensation `json:"compensation"`
	// Duplicate tokens on rows without a historical input price; they need a
	// manually supplied price before they can be compensated.
	UnpricedDuplicateTokens int64 `json:"unpriced_duplicate_tokens"`

	actual, list, discounted                       decimal.Decimal
	compList, compGroup, compListMax, compGroupMax decimal.Decimal // quota
	compDown, compDownMax                          decimal.Decimal // currency
	hasDownstream                                  bool
}

type CacheUsageAuditReport struct {
	Request   CacheUsageAuditRequest  `json:"request"`
	Currency  BillingCurrency         `json:"currency"`
	Total     CacheUsageAuditTotals   `json:"total"`
	ByDay     []CacheUsageAuditTotals `json:"by_day"`
	ByModel   []CacheUsageAuditTotals `json:"by_model"`
	ByToken   []CacheUsageAuditTotals `json:"by_token"`
	ByUser    []CacheUsageAuditTotals `json:"by_user"`
	Rows      []CacheUsageAuditRow    `json:"rows"`
	Truncated bool                    `json:"truncated"`
}
type cacheAuditMeta map[string]any

func (m cacheAuditMeta) number(key string) (float64, bool) {
	value, found := m[key].(float64)
	return value, found
}

func (m cacheAuditMeta) tokens(key string) int64 {
	value, _ := m.number(key)
	if value < 0 {
		return 0
	}
	return int64(value)
}

func (m cacheAuditMeta) text(key string) string {
	value, _ := m[key].(string)
	return value
}

func (m cacheAuditMeta) has(key string) bool {
	value, found := m[key]
	if !found || value == nil || value == false {
		return false
	}
	if number, ok := value.(float64); ok {
		return number != 0
	}
	if items, ok := value.([]any); ok {
		return len(items) > 0
	}
	return true
}

// estimatedUsage marks usage counted locally or estimated by the gateway
// rather than reported by the upstream; its cache split is not evidence.
func (m cacheAuditMeta) estimatedUsage() bool {
	admin, _ := m["admin_info"].(map[string]any)
	if admin == nil {
		return false
	}
	path, _ := admin["usage_billing_path"].(string)
	return admin["local_count_tokens"] == true || path == "local" || strings.HasSuffix(path, "-estimated")
}

func (m cacheAuditMeta) chain() []string {
	items, _ := m["request_conversion"].([]any)
	chain := make([]string, 0, len(items))
	for _, item := range items {
		if name, ok := item.(string); ok {
			chain = append(chain, name)
		}
	}
	return chain
}

// anthropicUsage reports whether prompt_tokens was logged with Claude
// semantics (uncached input only) instead of OpenAI semantics (total input).
func (m cacheAuditMeta) anthropicUsage() bool {
	return m.text("usage_semantic") == "anthropic" || m["claude"] == true
}

// legacySplit marks old OpenAI-shaped usage that carried Claude 5m/1h cache
// splits; billing treated its prompt_tokens as already excluding cache.
func (m cacheAuditMeta) legacySplit() bool {
	return !m.anthropicUsage() && (m.tokens("cache_creation_tokens_5m") > 0 || m.tokens("cache_creation_tokens_1h") > 0)
}

func cacheAuditTokens(log *model.Log, meta cacheAuditMeta) CacheUsageTokens {
	tokens := CacheUsageTokens{
		CacheRead: meta.tokens("cache_tokens"),
		CacheWrite: max(meta.tokens("cache_write_tokens"), meta.tokens("cache_creation_tokens"),
			meta.tokens("cache_creation_tokens_5m")+meta.tokens("cache_creation_tokens_1h")),
		Output: int64(max(log.CompletionTokens, 0)),
	}
	prompt := int64(max(log.PromptTokens, 0))
	if meta.anthropicUsage() || meta.legacySplit() {
		tokens.Input = prompt
		tokens.TotalInput = prompt + tokens.CacheRead + tokens.CacheWrite
	} else {
		tokens.TotalInput = prompt
		tokens.Input = max(prompt-tokens.CacheRead-tokens.CacheWrite, 0)
	}
	return tokens
}

// classifyCacheAudit reproduces what the pre-fix converter returned as
// input_tokens. Before the fix it subtracted cache tokens only when the
// upstream reported OpenAI cache_write_tokens; logs keep a normalized cache
// write total, so requests with any cache write are left for review.
func classifyCacheAudit(row *CacheUsageAuditRow, log *model.Log, meta cacheAuditMeta, fixedAt int64) {
	tokens := row.Tokens
	row.CorrectInputTokens = tokens.Input
	row.ReportedInputTokens = tokens.Input
	chain := meta.chain()
	path := row.RequestPath
	messages := strings.HasSuffix(path, "/v1/messages") || (len(chain) > 0 && chain[0] == "Claude Messages")
	set := func(status, reason string) { row.Status, row.Reason = status, reason }
	switch {
	case log.Type == model.LogTypeRefund:
		set(CacheAuditRefund, "")
	case meta.tokens(CacheUsageWireVersionKey) >= CacheUsageWireVersion || (fixedAt > 0 && log.CreatedAt >= fixedAt):
		set(CacheAuditNotAffected, "fixed_converter")
	case meta.estimatedUsage():
		set(CacheAuditNoEvidence, "estimated_usage")
	case path == "" && len(chain) == 0:
		set(CacheAuditNoEvidence, "missing_request_path")
	case !messages:
		set(CacheAuditNotAffected, "not_messages_endpoint")
	case len(chain) == 0:
		set(CacheAuditNoEvidence, "missing_conversion_chain")
	case chain[len(chain)-1] == "Claude Messages":
		set(CacheAuditNotAffected, "native_claude_upstream")
	case meta.anthropicUsage():
		set(CacheAuditNotAffected, "anthropic_usage_preserved")
	case tokens.CacheRead == 0 && tokens.CacheWrite == 0:
		set(CacheAuditNotAffected, "no_cache")
	case meta.legacySplit():
		set(CacheAuditReview, "legacy_cache_split")
		row.DuplicateTokensMax = tokens.CacheRead + tokens.CacheWrite
	case tokens.CacheWrite > 0:
		set(CacheAuditReview, "cache_write_present")
		row.DuplicateTokensMax = tokens.CacheRead + tokens.CacheWrite
	default:
		set(CacheAuditAffected, "cache_read_in_input")
		row.ReportedInputTokens = int64(max(log.PromptTokens, 0))
		row.DuplicateTokens = row.ReportedInputTokens - tokens.Input
		row.DuplicateTokensMax = row.DuplicateTokens
	}
}

type cacheAuditQuota struct {
	input, cacheRead, cacheWrite, output decimal.Decimal
}

func (q cacheAuditQuota) total() decimal.Decimal {
	return q.input.Add(q.cacheRead).Add(q.cacheWrite).Add(q.output)
}

func (q cacheAuditQuota) scale(ratio decimal.Decimal) cacheAuditQuota {
	return cacheAuditQuota{q.input.Mul(ratio), q.cacheRead.Mul(ratio), q.cacheWrite.Mul(ratio), q.output.Mul(ratio)}
}

// cacheAuditListQuota recomputes the per-token charge from the ratios saved
// with the log, mirroring calculateTextQuotaSummary without the group ratio.
func cacheAuditListQuota(meta cacheAuditMeta, tokens CacheUsageTokens) (cacheAuditQuota, string) {
	switch {
	case meta.text("billing_mode") == "tiered_expr":
		return cacheAuditQuota{}, "tiered_billing"
	case meta.has("image") || meta.has("audio") || meta.has("audio_input_seperate_price") || meta.has("tool_surcharges"):
		return cacheAuditQuota{}, "unsupported_surcharge"
	}
	if price, _ := meta.number("model_price"); price > 0 {
		return cacheAuditQuota{}, "per_call_price"
	}
	modelRatio, okModel := meta.number("model_ratio")
	completionRatio, okCompletion := meta.number("completion_ratio")
	_, okGroup := meta.number("group_ratio")
	if !okModel || !okCompletion || !okGroup {
		return cacheAuditQuota{}, "missing_price"
	}
	cacheRatio, okCache := meta.number("cache_ratio")
	if tokens.CacheRead > 0 && !okCache {
		return cacheAuditQuota{}, "missing_cache_price"
	}
	var cacheWrite decimal.Decimal
	if tokens.CacheWrite > 0 {
		creationRatio, okCreation := meta.number("cache_creation_ratio")
		split5m, split1h := meta.tokens("cache_creation_tokens_5m"), meta.tokens("cache_creation_tokens_1h")
		if (meta.anthropicUsage() || meta.legacySplit()) && split5m+split1h > 0 {
			ratio5m, ok5m := meta.number("cache_creation_ratio_5m")
			ratio1h, ok1h := meta.number("cache_creation_ratio_1h")
			remaining := max(meta.tokens("cache_creation_tokens")-split5m-split1h, 0)
			if (split5m > 0 && !ok5m) || (split1h > 0 && !ok1h) || (remaining > 0 && !okCreation) {
				return cacheAuditQuota{}, "missing_cache_price"
			}
			cacheWrite = decimal.NewFromInt(remaining).Mul(decimal.NewFromFloat(creationRatio)).
				Add(decimal.NewFromInt(split5m).Mul(decimal.NewFromFloat(ratio5m))).
				Add(decimal.NewFromInt(split1h).Mul(decimal.NewFromFloat(ratio1h)))
		} else {
			if !okCreation {
				return cacheAuditQuota{}, "missing_cache_price"
			}
			cacheWrite = decimal.NewFromInt(tokens.CacheWrite).Mul(decimal.NewFromFloat(creationRatio))
		}
	}
	quota := cacheAuditQuota{
		input:      decimal.NewFromInt(tokens.Input),
		cacheRead:  decimal.NewFromInt(tokens.CacheRead).Mul(decimal.NewFromFloat(cacheRatio)),
		cacheWrite: cacheWrite,
		output:     decimal.NewFromInt(tokens.Output).Mul(decimal.NewFromFloat(completionRatio)),
	}
	return quota.scale(decimal.NewFromFloat(modelRatio)), ""
}

// Compensation only needs the uncached input price, so it is available even
// when other parts of the request (tiered, surcharges) cannot be repriced.
func attachCacheAuditCompensation(row *CacheUsageAuditRow, meta cacheAuditMeta, money cacheAuditMoney, prices map[string]decimal.Decimal) {
	if row.DuplicateTokensMax == 0 {
		return
	}
	downstream, hasDownstream := prices[row.Model]
	modelRatio, okModel := meta.number("model_ratio")
	groupRatio, okGroup := meta.number("group_ratio")
	perMillion := decimal.NewFromInt(1000000)
	comp := &CacheUsageCompensation{}
	if okModel && okGroup && meta.text("billing_mode") != "tiered_expr" {
		model := decimal.NewFromFloat(modelRatio)
		group := decimal.NewFromFloat(groupRatio)
		duplicate, duplicateMax := decimal.NewFromInt(row.DuplicateTokens), decimal.NewFromInt(row.DuplicateTokensMax)
		row.InputPricePerMillion = money.amount(model.Mul(perMillion))
		if cacheRatio, ok := meta.number("cache_ratio"); ok {
			row.CacheReadPricePerMillion = money.amount(model.Mul(perMillion).Mul(decimal.NewFromFloat(cacheRatio)))
		}
		comp.List, comp.ListMax = money.amount(duplicate.Mul(model)), money.amount(duplicateMax.Mul(model))
		comp.Group, comp.GroupMax = money.amount(duplicate.Mul(model).Mul(group)), money.amount(duplicateMax.Mul(model).Mul(group))
	} else if !hasDownstream {
		return
	}
	if hasDownstream {
		comp.Downstream = decimal.NewFromInt(row.DuplicateTokens).Mul(downstream).Div(perMillion).StringFixed(6)
		comp.DownstreamMax = decimal.NewFromInt(row.DuplicateTokensMax).Mul(downstream).Div(perMillion).StringFixed(6)
	}
	row.Compensation = comp
}

type cacheAuditMoney struct{ rate, unit decimal.Decimal }

func newCacheAuditMoney(currency BillingCurrency) (cacheAuditMoney, error) {
	rate, err := decimal.NewFromString(currency.Rate)
	if err != nil {
		return cacheAuditMoney{}, err
	}
	unit, err := decimal.NewFromString(currency.QuotaPerUnit)
	if err != nil || !unit.IsPositive() {
		return cacheAuditMoney{}, errors.New("invalid billing currency divisor")
	}
	return cacheAuditMoney{rate, unit}, nil
}

func (m cacheAuditMoney) amount(quota decimal.Decimal) string {
	return quota.Mul(m.rate).Div(m.unit).StringFixed(6)
}

func (m cacheAuditMoney) cost(q cacheAuditQuota) *CacheUsageCost {
	return &CacheUsageCost{Input: m.amount(q.input), CacheRead: m.amount(q.cacheRead),
		CacheWrite: m.amount(q.cacheWrite), Output: m.amount(q.output), Total: m.amount(q.total())}
}

func buildCacheUsageAuditRow(log *model.Log, money cacheAuditMoney, fixedAt int64, prices map[string]decimal.Decimal) (CacheUsageAuditRow, cacheAuditQuota, cacheAuditQuota) {
	row := CacheUsageAuditRow{
		LogID: log.Id, RequestID: log.RequestId, CreatedAt: log.CreatedAt, Type: log.Type,
		UserID: log.UserId, Username: log.Username, TokenID: log.TokenId, TokenName: log.TokenName,
		Model: log.ModelName, Group: log.Group, IsStream: log.IsStream,
		ActualQuota: int64(log.Quota),
	}
	if log.Type == model.LogTypeRefund {
		row.ActualQuota = -row.ActualQuota
	}
	row.ActualAmount = money.amount(decimal.NewFromInt(row.ActualQuota))
	meta := cacheAuditMeta{}
	if log.Other != "" && common.UnmarshalJsonStr(log.Other, &meta) != nil {
		row.Status, row.Reason, row.PriceIssue = CacheAuditNoEvidence, "invalid_metadata", "missing_price"
		return row, cacheAuditQuota{}, cacheAuditQuota{}
	}
	row.RequestPath = meta.text("request_path")
	row.UsageSemantic = meta.text("usage_semantic")
	if row.UsageSemantic == "" {
		row.UsageSemantic = "openai"
		if meta.anthropicUsage() {
			row.UsageSemantic = "anthropic"
		}
	}
	if log.Type != model.LogTypeRefund {
		row.Tokens = cacheAuditTokens(log, meta)
	}
	classifyCacheAudit(&row, log, meta, fixedAt)
	if log.Type == model.LogTypeRefund {
		return row, cacheAuditQuota{}, cacheAuditQuota{}
	}
	groupRatio, _ := meta.number("group_ratio")
	row.GroupRatio = decimal.NewFromFloat(groupRatio).String()
	attachCacheAuditCompensation(&row, meta, money, prices)
	list, issue := cacheAuditListQuota(meta, row.Tokens)
	if issue != "" {
		row.PriceIssue = issue
		return row, cacheAuditQuota{}, cacheAuditQuota{}
	}
	discounted := list.scale(decimal.NewFromFloat(groupRatio))
	row.ListCost, row.DiscountedCost = money.cost(list), money.cost(discounted)
	// Settlement rounds once and bills at least 1 quota for non-free usage.
	row.PriceMismatch = discounted.total().Round(0).Sub(decimal.NewFromInt(row.ActualQuota)).Abs().GreaterThan(decimal.NewFromInt(1))
	return row, list, discounted
}
func (t *CacheUsageAuditTotals) add(row CacheUsageAuditRow, list, discounted cacheAuditQuota) {
	t.actual = t.actual.Add(decimal.NewFromInt(row.ActualQuota))
	if row.Type == model.LogTypeRefund {
		return
	}
	t.Requests++
	switch row.Status {
	case CacheAuditAffected:
		t.AffectedRequests++
	case CacheAuditReview:
		t.ReviewRequests++
	case CacheAuditNoEvidence:
		t.NoEvidenceRequests++
	}
	t.Tokens.TotalInput += row.Tokens.TotalInput
	t.Tokens.Input += row.Tokens.Input
	t.Tokens.CacheRead += row.Tokens.CacheRead
	t.Tokens.CacheWrite += row.Tokens.CacheWrite
	t.Tokens.Output += row.Tokens.Output
	t.DuplicateTokens += row.DuplicateTokens
	t.DuplicateTokensMax += row.DuplicateTokensMax
	if row.InputPricePerMillion != "" {
		if t.InputPricePerMillion == "" {
			t.InputPricePerMillion, t.CacheReadPricePerMillion = row.InputPricePerMillion, row.CacheReadPricePerMillion
		} else if t.InputPricePerMillion != row.InputPricePerMillion || t.CacheReadPricePerMillion != row.CacheReadPricePerMillion {
			t.PriceMixed = true
		}
	}
	if row.Status == CacheAuditAffected {
		t.AffectedReportedInput += row.ReportedInputTokens
		t.AffectedCorrectInput += row.CorrectInputTokens
	}
	if comp := row.Compensation; comp != nil {
		add := func(target *decimal.Decimal, value string) {
			if value != "" {
				*target = target.Add(decimal.RequireFromString(value))
			}
		}
		if comp.List != "" {
			add(&t.compList, comp.List)
			add(&t.compListMax, comp.ListMax)
			add(&t.compGroup, comp.Group)
			add(&t.compGroupMax, comp.GroupMax)
		} else {
			t.UnpricedDuplicateTokens += row.DuplicateTokensMax
		}
		if comp.Downstream != "" {
			t.hasDownstream = true
			add(&t.compDown, comp.Downstream)
			add(&t.compDownMax, comp.DownstreamMax)
		}
	} else {
		t.UnpricedDuplicateTokens += row.DuplicateTokensMax
	}
	if row.ListCost != nil {
		t.PricedRequests++
		t.list = t.list.Add(list.total())
		t.discounted = t.discounted.Add(discounted.total())
	}
}

func (t *CacheUsageAuditTotals) finish(money cacheAuditMoney) {
	t.ActualAmount = money.amount(t.actual)
	t.ListAmount = money.amount(t.list)
	t.DiscountedAmount = money.amount(t.discounted)
	// Compensation sums are already in currency; round once at the end.
	t.Compensation = CacheUsageCompensation{
		List: t.compList.StringFixed(6), Group: t.compGroup.StringFixed(6),
		ListMax: t.compListMax.StringFixed(6), GroupMax: t.compGroupMax.StringFixed(6),
	}
	if t.hasDownstream {
		t.Compensation.Downstream, t.Compensation.DownstreamMax = t.compDown.StringFixed(6), t.compDownMax.StringFixed(6)
	}
}

type cacheAuditGroups map[string]*CacheUsageAuditTotals

func (g cacheAuditGroups) add(key string, row CacheUsageAuditRow, list, discounted cacheAuditQuota) {
	if g[key] == nil {
		g[key] = &CacheUsageAuditTotals{Key: key}
	}
	g[key].add(row, list, discounted)
}

func (g cacheAuditGroups) sorted(money cacheAuditMoney) []CacheUsageAuditTotals {
	result := make([]CacheUsageAuditTotals, 0, len(g))
	for _, totals := range g {
		totals.finish(money)
		result = append(result, *totals)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func normalizeCacheUsageAuditRequest(req CacheUsageAuditRequest) (CacheUsageAuditRequest, error) {
	req.Model = strings.TrimSpace(req.Model)
	if req.StartAt <= 0 || req.EndAt <= req.StartAt || req.EndAt-req.StartAt > cacheUsageAuditMaxPeriod {
		return req, ErrCacheUsageAuditPeriod
	}
	switch req.Status {
	case "", CacheAuditAffected, CacheAuditReview, CacheAuditNoEvidence, CacheAuditNotAffected, CacheAuditRefund:
	default:
		return req, errors.New("invalid cache usage audit status")
	}
	if req.UserID < 0 || req.TokenID < 0 || req.FixedAt < 0 {
		return req, errors.New("invalid cache usage audit filter")
	}
	return req, nil
}

// RunCacheUsageAudit scans the period once. visit, when set, receives every
// row matching req.Status (used by exports); the report keeps at most 2000.
func RunCacheUsageAudit(ctx context.Context, req CacheUsageAuditRequest, visit func(CacheUsageAuditRow) error) (*CacheUsageAuditReport, error) {
	req, err := normalizeCacheUsageAuditRequest(req)
	if err != nil {
		return nil, err
	}
	currency, err := currentBillingCurrency(true)
	if err != nil {
		return nil, err
	}
	money, err := newCacheAuditMoney(currency)
	if err != nil {
		return nil, err
	}
	report := &CacheUsageAuditReport{Request: req, Currency: currency, Total: CacheUsageAuditTotals{Key: "total"}, Rows: []CacheUsageAuditRow{}}
	byDay, byModel, byToken, byUser := cacheAuditGroups{}, cacheAuditGroups{}, cacheAuditGroups{}, cacheAuditGroups{}
	prices, err := parseCacheAuditPrices(req.DownstreamInputPrices)
	if err != nil {
		return nil, err
	}
	filter := model.CacheUsageAuditFilter{UserID: req.UserID, TokenID: req.TokenID, StartAt: req.StartAt, EndAt: req.EndAt, Model: req.Model}
	err = model.ScanCacheUsageAuditLogs(ctx, filter, func(logs []*model.Log) error {
		for _, log := range logs {
			row, list, discounted := buildCacheUsageAuditRow(log, money, req.FixedAt, prices)
			report.Total.add(row, list, discounted)
			byDay.add(time.Unix(row.CreatedAt, 0).In(billingLocation).Format("2006-01-02"), row, list, discounted)
			byModel.add(row.Model, row, list, discounted)
			byToken.add(cacheAuditTokenKey(row), row, list, discounted)
			byUser.add(row.Username+" (#"+strconv.Itoa(row.UserID)+")", row, list, discounted)
			if req.Status != "" && row.Status != req.Status {
				continue
			}
			if visit != nil {
				if err := visit(row); err != nil {
					return err
				}
			}
			if len(report.Rows) < cacheUsageAuditMaxRows {
				report.Rows = append(report.Rows, row)
			} else {
				report.Truncated = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	report.Total.finish(money)
	report.ByDay, report.ByModel, report.ByToken, report.ByUser = byDay.sorted(money), byModel.sorted(money), byToken.sorted(money), byUser.sorted(money)
	// Models are ranked by duplicated tokens: that is what customers act on.
	sort.SliceStable(report.ByModel, func(i, j int) bool {
		return report.ByModel[i].DuplicateTokensMax > report.ByModel[j].DuplicateTokensMax
	})
	return report, nil
}

func parseCacheAuditPrices(raw map[string]string) (map[string]decimal.Decimal, error) {
	if len(raw) > 200 {
		return nil, errors.New("at most 200 downstream model prices")
	}
	prices := make(map[string]decimal.Decimal, len(raw))
	for model, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		price, err := decimal.NewFromString(value)
		if err != nil || price.IsNegative() || price.GreaterThan(decimal.NewFromInt(1000000)) {
			return nil, errors.New("invalid downstream input price for " + model)
		}
		prices[model] = price
	}
	return prices, nil
}

func cacheAuditTokenKey(row CacheUsageAuditRow) string {
	if row.TokenName == "" {
		return "#" + strconv.Itoa(row.TokenID)
	}
	return row.TokenName + " (#" + strconv.Itoa(row.TokenID) + ")"
}

func SearchCacheUsageAuditKeys(ctx context.Context, userID int, keyword string) ([]model.CacheUsageAuditKey, error) {
	return model.SearchCacheUsageAuditKeys(ctx, userID, keyword)
}
