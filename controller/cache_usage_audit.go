package controller

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func cacheUsageAuditError(c *gin.Context, err error) {
	if errors.Is(err, model.ErrCacheUsageAuditTooLarge) || errors.Is(err, service.ErrCacheUsageAuditPeriod) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	billingError(c, err)
}

func bindCacheUsageAudit(c *gin.Context) (service.CacheUsageAuditRequest, bool) {
	var req service.CacheUsageAuditRequest
	if !canViewFinancialAccounting(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return req, false
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		billingError(c, errors.New("invalid cache usage audit request"))
		return req, false
	}
	return req, true
}

func QueryCacheUsageAudit(c *gin.Context) {
	req, ok := bindCacheUsageAudit(c)
	if !ok {
		return
	}
	report, err := service.RunCacheUsageAudit(c.Request.Context(), req, nil)
	if err != nil {
		cacheUsageAuditError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

func SearchCacheUsageAuditKeys(c *gin.Context) {
	if !canViewFinancialAccounting(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	userID, _ := strconv.Atoi(c.Query("user_id"))
	keys, err := service.SearchCacheUsageAuditKeys(c.Request.Context(), userID, c.Query("keyword"))
	if err != nil {
		billingError(c, err)
		return
	}
	common.ApiSuccess(c, keys)
}

var cacheUsageAuditCSVHeader = []string{
	"日志ID", "请求ID", "时间", "类型", "用户ID", "用户名", "令牌ID", "令牌名", "模型", "分组", "流式", "请求路径", "协议", "用量口径",
	"核对状态", "原因", "平台总输入(含缓存)", "普通输入", "缓存读取", "缓存写入", "输出",
	"旧返回input_tokens", "正确input_tokens", "多算token(缓存当输入)", "多算token上限",
	"分组倍率", "实际扣费", "官价-普通输入", "官价-缓存读取", "官价-缓存写入", "官价-输出", "官价合计",
	"分组价-普通输入", "分组价-缓存读取", "分组价-缓存写入", "分组价-输出", "分组价合计", "价格说明", "分组价与实扣不一致",
	"官价普通输入单价(每百万)", "官价缓存读取单价(每百万)", "应补-按官价", "应补-按分组价", "应补-按下游价", "应补上限-按官价", "应补上限-按分组价", "应补上限-按下游价",
}

// CacheUsageAuditCompensationNote explains the compensation basis in exports.
const CacheUsageAuditCompensationNote = "应补金额 = 多算token × 普通输入单价。缓存部分客户已按缓存价另行计费，" +
	"多算的只是缓存 token 又被当作普通输入收了一次，所以只需退还这部分普通输入费用。" +
	"\"已确认\"行可直接补；\"待核对\"行给出上限，需对照原始响应确认；\"证据不足\"行不计入。"

func cacheUsageAuditCSVRow(row service.CacheUsageAuditRow) []string {
	cost := func(value *service.CacheUsageCost) []string {
		if value == nil {
			return []string{"", "", "", "", ""}
		}
		return []string{value.Input, value.CacheRead, value.CacheWrite, value.Output, value.Total}
	}
	kind := "消费"
	if row.Type == model.LogTypeRefund {
		kind = "退款"
	}
	out := []string{
		strconv.Itoa(row.LogID), row.RequestID, time.Unix(row.CreatedAt, 0).In(cacheUsageAuditLocation).Format("2006-01-02 15:04:05"), kind,
		strconv.Itoa(row.UserID), row.Username, strconv.Itoa(row.TokenID), row.TokenName, row.Model, row.Group,
		strconv.FormatBool(row.IsStream), row.RequestPath, row.Protocol, row.UsageSemantic, row.Status, row.Reason,
		i64(row.Tokens.TotalInput), i64(row.Tokens.Input), i64(row.Tokens.CacheRead), i64(row.Tokens.CacheWrite), i64(row.Tokens.Output),
		i64(row.ReportedInputTokens), i64(row.CorrectInputTokens), i64(row.DuplicateTokens), i64(row.DuplicateTokensMax),
		row.GroupRatio, row.ActualAmount,
	}
	out = append(out, cost(row.ListCost)...)
	out = append(out, cost(row.DiscountedCost)...)
	out = append(out, row.PriceIssue, strconv.FormatBool(row.PriceMismatch), row.InputPricePerMillion, row.CacheReadPricePerMillion)
	if comp := row.Compensation; comp != nil {
		return append(out, comp.List, comp.Group, comp.Downstream, comp.ListMax, comp.GroupMax, comp.DownstreamMax)
	}
	return append(out, "", "", "", "", "", "")
}

// Billing documents use Asia/Shanghai regardless of the server timezone.
var cacheUsageAuditLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func i64(value int64) string { return strconv.FormatInt(value, 10) }

// ExportCacheUsageAudit streams every matching row. Once the first byte is
// written the status cannot change, so a later failure aborts the download.
func ExportCacheUsageAudit(c *gin.Context) {
	req, ok := bindCacheUsageAudit(c)
	if !ok {
		return
	}
	format := c.Query("format")
	if format != "csv" && format != "json" {
		billingError(c, errors.New("invalid export format"))
		return
	}
	name := fmt.Sprintf("cache-usage-audit-%s-%s.%s",
		time.Unix(req.StartAt, 0).In(cacheUsageAuditLocation).Format("20060102"), time.Unix(max(req.EndAt-1, req.StartAt), 0).In(cacheUsageAuditLocation).Format("20060102"), format)
	started := false
	start := func() {
		if started {
			return
		}
		started = true
		c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
		if format == "csv" {
			c.Header("Content-Type", "text/csv; charset=utf-8")
			_, _ = c.Writer.Write([]byte("\xef\xbb\xbf")) // Excel needs the BOM for UTF-8
		} else {
			c.Header("Content-Type", "application/json; charset=utf-8")
			note, _ := common.Marshal(CacheUsageAuditCompensationNote)
			_, _ = c.Writer.Write([]byte(`{"compensation_note":` + string(note) + `,"rows":[`))
		}
	}
	writer := csv.NewWriter(c.Writer)
	first := true
	visit := func(row service.CacheUsageAuditRow) error {
		start()
		if format == "csv" {
			if first {
				if err := writer.Write([]string{CacheUsageAuditCompensationNote}); err != nil {
					return err
				}
				if err := writer.Write(cacheUsageAuditCSVHeader); err != nil {
					return err
				}
			}
			first = false
			return writer.Write(cacheUsageAuditCSVRow(row))
		}
		body, err := common.Marshal(row)
		if err != nil {
			return err
		}
		if !first {
			body = append([]byte(","), body...)
		}
		first = false
		_, err = c.Writer.Write(body)
		return err
	}
	report, err := service.RunCacheUsageAudit(c.Request.Context(), req, visit)
	if err != nil {
		if !started {
			cacheUsageAuditError(c, err)
			return
		}
		common.SysError("cache usage audit export aborted: " + err.Error())
		_ = c.Error(err)
		c.Abort()
		return
	}
	start()
	if format == "csv" {
		if first {
			_ = writer.Write([]string{CacheUsageAuditCompensationNote})
			_ = writer.Write(cacheUsageAuditCSVHeader)
		}
		writer.Flush()
		return
	}
	report.Rows = nil
	summary, err := common.Marshal(report)
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, _ = c.Writer.Write([]byte(`],"summary":`))
	_, _ = c.Writer.Write(summary)
	_, _ = c.Writer.Write([]byte(`}`))
}
