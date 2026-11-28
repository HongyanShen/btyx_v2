package main

import (
	"net/url"
	"strconv"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
)

const identityRecordsPath = "/admin/realname"

type identityRecordsPage struct {
	Rows              []identityRecordRow
	CanViewDetails    bool
	PhotosUnavailable bool
	CurrentURL        string
	Pager             pagination
}

type identityRecordRow struct {
	Nickname string
	Name     string
	Identity string
	Phone    string
	Wechat   string
	Photos   []identityPhoto
	Source   string
	Date     string
	Success  bool
	Reason   string
}

type identityPhoto struct {
	Side string
	URL  string
}

func (admin *admin) loadIdentityRecords(ctx *gin.Context, account identity.OperatorDTO) (*identityRecordsPage, error) {
	request := ctx.Request.Context()
	before := ctx.Query("before")
	after := ctx.Query("after")
	number, _ := strconv.Atoi(ctx.Query("page"))
	canViewDetails := canCall(account, "GET /identity-verifications/documents")
	records, err := admin.services.IdentityRecordsPage(request, before, after, number, canViewDetails)
	if err != nil {
		return nil, err
	}

	if canViewDetails {
		logging.Info(request).
			Str("operator_id", account.ID).
			Int("records", len(records.Records)).
			Msg("identity documents viewed")
	}

	page := &identityRecordsPage{
		Rows:              make([]identityRecordRow, 0, len(records.Records)),
		CanViewDetails:    canViewDetails,
		PhotosUnavailable: records.PhotosUnavailable,
		CurrentURL:        identityRecordsPath,
		Pager:             identityRecordPager(records.Number, records.NextBefore != ""),
	}
	if records.Number > 1 {
		page.CurrentURL = identityRecordsPath + "?page=" + strconv.Itoa(records.Number)
	}

	if before != "" {
		page.CurrentURL = identityRecordsPath + "?before=" + url.QueryEscape(before)
	}

	if after != "" {
		page.CurrentURL = identityRecordsPath + "?after=" + url.QueryEscape(after)
	}

	for _, record := range records.Records {
		page.Rows = append(page.Rows, identityRecordView(record))
	}

	return page, nil
}

func identityRecordPager(current int, hasNext bool) pagination {
	pager := pagination{
		Current: current,
		Path:    identityRecordsPath,
		Label:   "实名认证记录分页",
		Prev:    current - 1,
	}
	last := current
	if hasNext {
		last++
		pager.Next = last
	}

	pager.Pages = pagerNumbers(current, last)

	return pager
}

func identityRecordView(record audit.IdentityRecordDTO) identityRecordRow {
	row := identityRecordRow{
		Nickname: label(record.Nickname, "分发员 "+record.UserID[max(0, len(record.UserID)-6):]),
		Name:     "—",
		Identity: "—",
		Phone:    "—",
		Wechat:   "—",
		Source:   "独立实名",
		Date:     dateText(record.CreatedAt),
		Success:  record.Action == enum.AuditUserIdentityVerify || record.Action == enum.AuditEnrollmentIdentityVerify,
		Reason:   label(record.Reason, "—"),
	}
	if record.Action == enum.AuditEnrollmentIdentityVerify || record.Action == enum.AuditEnrollmentIdentityFailed {
		row.Source = "分发员报名"
	}

	if details := record.Details; details != nil {
		row.Name = label(details.Name, "—")
		row.Identity = label(details.Identity, "—")
		row.Phone = label(details.Phone, "—")
		row.Wechat = label(details.Wechat, "—")
		if details.FrontURL != "" || details.BackURL != "" {
			row.Photos = []identityPhoto{
				{
					Side: "正面",
					URL:  details.FrontURL,
				},
				{
					Side: "反面",
					URL:  details.BackURL,
				},
			}
		}
	}

	return row
}
