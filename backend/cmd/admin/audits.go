package main

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"github.com/gin-gonic/gin"
)

type auditRow struct {
	HTTPStatus int
	Platform   string
	Action     string
	Actor      string
	Reason     string
	Ref        string
	Date       string
}

type auditPage struct {
	ActorName string
	Platform  enum.AuditPlatform
	Endpoint  string
	Endpoints []string
	Rows      []auditRow
	Pager     pagination
	Start     string
	End       string
	Error     string
}

func (admin *admin) loadAudits(ctx *gin.Context) (*auditPage, error) {
	request := ctx.Request.Context()
	page := &auditPage{
		ActorName: strings.TrimSpace(ctx.Query("actor_name")),
		Platform:  enum.AuditPlatform(ctx.Query("platform")),
		Endpoint:  ctx.Query("endpoint"),
		Start:     ctx.Query("start"),
		End:       ctx.Query("end"),
	}
	endpoints, err := admin.services.AuditEndpoints(request)
	if err != nil {
		return nil, err
	}

	if page.Endpoint != "" && page.Endpoint != audit.MissingEndpoint && !slices.Contains(endpoints, page.Endpoint) {
		endpoints = append(endpoints, page.Endpoint)
	}

	slices.Sort(endpoints)
	page.Endpoints = endpoints
	filter := audit.Filter{
		ActorName: page.ActorName,
		Platform:  page.Platform,
		Endpoint:  page.Endpoint,
	}
	if page.Platform != "" && !page.Platform.Valid() {
		page.Error = "无效的操作平台"
	}

	if page.Start != "" {
		start, err := time.ParseInLocation("2006-01-02T15:04", page.Start, beijing)
		if err != nil {
			page.Error = "请输入有效的开始时间"
		} else {
			filter.Start = start
		}
	}

	if page.End != "" {
		end, parseErr := time.ParseInLocation("2006-01-02T15:04", page.End, beijing)
		if parseErr != nil {
			page.Error = "请输入有效的结束时间"
		} else {
			filter.Before = end.Add(time.Minute)
		}
	}

	if !filter.Before.IsZero() && !filter.Start.Before(filter.Before) {
		page.Error = "结束时间不能早于开始时间"
	}

	if page.Error != "" {
		return page, app.Invalid(page.Error)
	}

	number, _ := strconv.Atoi(ctx.Query("page"))
	result, err := admin.services.AuditPage(request, filter, number)
	if err != nil {
		return nil, err
	}

	last := max(1, (result.Total+audit.PageSize-1)/audit.PageSize)
	values := url.Values{}
	if page.ActorName != "" {
		values.Set("actor_name", page.ActorName)
	}

	if page.Platform != "" {
		values.Set("platform", string(page.Platform))
	}

	if page.Endpoint != "" {
		values.Set("endpoint", page.Endpoint)
	}

	if page.Start != "" {
		values.Set("start", page.Start)
	}

	if page.End != "" {
		values.Set("end", page.End)
	}

	page.Pager = pagination{
		Path:       "/audits",
		Label:      "操作审计分页",
		TotalLabel: fmt.Sprintf("共 %d 条", result.Total),
		Current:    result.Number,
		Pages:      pagerNumbers(result.Number, last),
		Prev:       result.Number - 1,
	}
	if result.Number < last {
		page.Pager.Next = result.Number + 1
	}

	if len(values) > 0 {
		page.Pager.Search = "&" + values.Encode()
	}

	for _, row := range result.Rows {
		page.Rows = append(page.Rows, auditRow{
			HTTPStatus: row.HTTPStatus,
			Platform:   label(auditPlatformLabels[row.Platform], "未记录"),
			Action:     label(auditLabels[row.Action], string(row.Action)),
			Actor:      row.ActorName,
			Reason:     label(row.Reason, "—"),
			Ref:        label(row.ReferenceID, "—"),
			Date:       dateText(row.CreatedAt),
		})
	}

	return page, nil
}

var auditPlatformLabels = map[enum.AuditPlatform]string{
	enum.AuditPlatformAdmin:   "管理后台",
	enum.AuditPlatformMiniapp: "小程序",
}
