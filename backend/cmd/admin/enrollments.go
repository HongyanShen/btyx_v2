package main

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"github.com/gin-gonic/gin"
)

var enrollmentsRoute = workspacePage{
	PermissionLabel: "分发员报名审核与管理",
	Path:            "/users",
	Label:           "分发员报名审核",
	Endpoint:        "GET /users",
}

const enrollmentReviewPermission = "POST /users/:id/review"

type enrollmentsPage struct {
	Rows       []enrollmentView
	Pager      pagination
	Total      int
	Filter     string
	Tag        string
	TagFilter  []contentTagChoice
	Statuses   []contentTagChoice
	CSRF       string
	Tags       []enrollmentTagView
	HasLibrary bool
	CanCreate  bool
	CanRename  bool
	CanDelete  bool
	Dialog     *enrollmentDialog
}

type enrollmentTagView struct {
	enrollment.MarketingTagDTO
	Revision string
}

type enrollmentView struct {
	distributorView
	StatusText        string
	Date              string
	ReviewInfo        []string
	Photos            []identityPhoto
	PhotosUnavailable bool
	ReviewText        string
	TagsOnly          bool
}

type enrollmentDialog struct {
	Row             enrollmentView
	Mode            string
	Title           string
	CSRF            string
	BackQuery       string
	Retry           string
	FrontPreview    string
	BackPreview     string
	Action          string
	Error           string
	Document        audit.EnrollmentDocumentsDTO
	Photo           identityPhoto
	CanDocuments    bool
	CanCreateTag    bool
	PortraitOnly    bool
	PortraitChanged bool
	TagOnly         bool
	Reason          string
	Revision        string
	RecentPost      string
	NewAccount      string
	Ages            []contentTagChoice
	TagChoices      []contentTagChoice
	Tag             enrollmentTagView
}

var enrollmentStatusText = map[enum.EnrollmentStatus]string{
	enum.EnrollmentPending:  "待审核",
	enum.EnrollmentApproved: "已通过",
	enum.EnrollmentRejected: "已拒绝",
	enum.EnrollmentNone:     "未报名",
}

var enrollmentAges = []contentTagChoice{
	{
		Key:   string(enum.EnrollmentAge18To24),
		Label: "18–24 岁",
	},
	{
		Key:   string(enum.EnrollmentAge25To30),
		Label: "25–30 岁",
	},
	{
		Key:   string(enum.EnrollmentAge31To40),
		Label: "31–40 岁",
	},
	{
		Key:   string(enum.EnrollmentAgeOver40),
		Label: "40 岁以上",
	},
	{
		Key:   string(enum.EnrollmentAgeUnknown),
		Label: "无法判断",
	},
}

func (admin *admin) loadEnrollments(ctx *gin.Context) (*enrollmentsPage, error) {
	account := currentOperator(ctx)
	request := ctx.Request.Context()
	pageNumber, _ := strconv.Atoi(ctx.Query("page"))
	pageNumber = max(pageNumber, 1)
	filter := ctx.Query("status")
	tagKey := ctx.Query("tag")
	users, total, err := admin.services.EnrollmentPage(request, pageNumber, enum.EnrollmentStatus(filter), tagKey)
	if err != nil {
		return nil, err
	}

	if pageNumber > 1 && len(users) == 0 {
		pageNumber = max(1, (total+29)/30)
		users, total, err = admin.services.EnrollmentPage(request, pageNumber, enum.EnrollmentStatus(filter), tagKey)
		if err != nil {
			return nil, err
		}
	}

	canDocs := canCall(account, enrollmentDocumentPermission)
	canReview := canCall(account, enrollmentReviewPermission)
	var photos map[string]*identity.EnrollmentPhotoPreviewDTO
	if canDocs && len(users) > 0 {
		ids := make([]string, 0, len(users))
		for _, user := range users {
			ids = append(ids, user.EnrollmentID)
		}

		photos, err = admin.services.EnrollmentPhotoPreviews(request, ids)
		if err != nil {
			return nil, err
		}

		if len(photos) > 0 {
			logging.Info(request).Str("operator_id", account.ID).Int("records", len(photos)).Msg("enrollment photo previews viewed")
		}
	}

	tags, err := admin.contentTagCatalog(ctx, account)
	if err != nil {
		return nil, err
	}

	pager := enrollmentPager(pageNumber, total, filter, tagKey)
	page := &enrollmentsPage{
		Pager:      pager,
		Total:      total,
		Filter:     filter,
		Tag:        tagKey,
		CSRF:       csrfToken(sessionToken(ctx)),
		HasLibrary: canCall(account, "GET /marketing-tags"),
		CanCreate:  canCall(account, "POST /marketing-tags"),
		CanRename:  canCall(account, "POST /marketing-tags/:id/rename"),
		CanDelete:  canCall(account, "POST /marketing-tags/:id/delete"),
	}
	for _, status := range []enum.EnrollmentStatus{enum.EnrollmentPending, enum.EnrollmentApproved, enum.EnrollmentRejected} {
		page.Statuses = append(page.Statuses, contentTagChoice{
			Key:      string(status),
			Label:    enrollmentStatusText[status],
			Selected: string(status) == filter,
		})
	}

	var used []string
	if tagKey != "" {
		used = append(used, tagKey)
	}

	names := make(map[string]string, len(tags))
	for _, tag := range tags {
		names[tag.Key] = tag.Name
		page.Tags = append(page.Tags, enrollmentTagView{
			MarketingTagDTO: tag,
			Revision:        tag.UpdatedAt.Format(time.RFC3339Nano),
		})
	}

	for _, user := range users {
		page.Rows = append(page.Rows, enrollmentRow(user, names, photos[user.EnrollmentID], canReview))
		used = append(used, user.Tags...)
	}

	page.TagFilter = enrollmentTagChoices(tags, used, []string{tagKey})
	if ctx.Query("dialog") != "" {
		page.Dialog, err = admin.openEnrollmentDialog(ctx, page, tags, canDocs)
	}

	return page, err
}

func enrollmentPager(current, count int, status, tag string) pagination {
	last := max(1, (count+29)/30)
	pager := pagination{
		Current:    current,
		Path:       enrollmentsRoute.Path,
		Label:      "报名审核分页",
		TotalLabel: "共 " + strconv.Itoa(count) + " 条",
		Search:     "&status=" + url.QueryEscape(status) + "&tag=" + url.QueryEscape(tag),
		Prev:       current - 1,
	}
	if current < last {
		pager.Next = current + 1
	}

	pager.Pages = pagerNumbers(current, last)

	return pager
}

func enrollmentRow(user identity.UserDTO, names map[string]string, photos *identity.EnrollmentPhotoPreviewDTO, canReview bool) enrollmentView {
	row := enrollmentView{
		distributorView: distributorRow(user, names),
		StatusText:      enrollmentStatusText[user.Status],
		Date:            "—",
	}
	row.DisplayName = label(user.Name, label(user.Nickname, "报名人"))
	if user.EnrollmentSubmittedAt != nil {
		row.Date = user.EnrollmentSubmittedAt.In(beijing).Format(time.DateOnly)
	}

	if photos != nil {
		row.Photos = identityPhotos(photos.FrontURL, photos.BackURL)
		row.PhotosUnavailable = photos.Unavailable
	}

	if user.EnrollmentRecentPost != nil {
		text := "近 30 天没发作品"
		if *user.EnrollmentRecentPost {
			text = "近 30 天有发作品"
		}

		row.ReviewInfo = append(row.ReviewInfo, text)
	}

	if user.EnrollmentNewAccount != nil {
		text := "不是新号"
		if *user.EnrollmentNewAccount {
			text = "是新号"
		}

		row.ReviewInfo = append(row.ReviewInfo, text)
	}

	for _, age := range enrollmentAges {
		if user.EnrollmentAgeGroup != nil && age.Key == string(*user.EnrollmentAgeGroup) {
			row.ReviewInfo = append(row.ReviewInfo, age.Label)
		}
	}

	if canReview {
		switch user.Status {
		case enum.EnrollmentPending:
			row.ReviewText = "去复核"
			row.TagsOnly = len(user.Tags) > 0
		case enum.EnrollmentApproved:
			row.ReviewText = "改画像 / 标签"
		}
	}

	return row
}

func identityPhotos(front, back string) []identityPhoto {
	if front == "" && back == "" {
		return nil
	}

	return []identityPhoto{
		{
			Side: "正面",
			URL:  front,
		},
		{
			Side: "反面",
			URL:  back,
		},
	}
}

func enrollmentTagChoices(tags []enrollment.MarketingTagDTO, keys, selected []string) []contentTagChoice {
	seen := make(map[string]bool)
	choices := make([]contentTagChoice, 0, len(tags)+len(keys))
	for _, tag := range tags {
		seen[tag.Key] = true
		choices = append(choices, contentTagChoice{
			Key:      tag.Key,
			Label:    tag.Name,
			Selected: slices.Contains(selected, tag.Key),
		})
	}

	for _, key := range keys {
		if !seen[key] {
			seen[key] = true
			choices = append(choices, contentTagChoice{
				Key:      key,
				Label:    key,
				Selected: slices.Contains(selected, key),
			})
		}
	}

	return choices
}

func enrollmentLink(page int, status, tag, id, record, mode string) string {
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("status", status)
	params.Set("tag", tag)
	params.Set("dialog", mode)
	params.Set("member", id)
	params.Set("record", record)
	return enrollmentsRoute.Path + "?" + params.Encode()
}

func (admin *admin) openEnrollmentDialog(ctx *gin.Context, page *enrollmentsPage, tags []enrollment.MarketingTagDTO, canDocs bool) (*enrollmentDialog, error) {
	mode := ctx.Query("dialog")
	id := ctx.Query("member")
	record := ctx.Query("record")
	dialog := &enrollmentDialog{
		Mode:      mode,
		CSRF:      page.CSRF,
		BackQuery: "page=" + strconv.Itoa(page.Pager.Current) + page.Pager.Search,
		Retry:     enrollmentLink(page.Pager.Current, page.Filter, page.Tag, id, record, mode),
	}
	if mode == "rename" || mode == "delete" {
		allowed := page.CanRename && mode == "rename" || page.CanDelete && mode == "delete"
		if !allowed || !page.HasLibrary {
			return nil, app.Forbidden("当前账号无权管理标签")
		}

		for _, tag := range page.Tags {
			if tag.ID == id {
				dialog.Tag = tag
				dialog.Action = "/users/tags/" + id + "/" + mode + "?" + dialog.BackQuery
				return dialog, nil
			}
		}

		return nil, app.NotFound()
	}

	index := slices.IndexFunc(page.Rows, func(row enrollmentView) bool {
		return row.ID == id && row.EnrollmentID == record
	})
	if index == -1 {
		return nil, app.NotFound()
	}

	dialog.Row = page.Rows[index]
	if mode == "details" || mode == "front" || mode == "back" {
		dialog.CanDocuments = canDocs
		if !canDocs && mode != "details" {
			return nil, app.Forbidden("当前账号无权查看报名证件照片")
		}

		if canDocs {
			var err error
			dialog.Document, err = admin.services.EnrollmentDocuments(ctx.Request.Context(), id, record)
			if err != nil {
				return nil, err
			}

			logging.Info(ctx.Request.Context()).Str("operator_id", currentOperator(ctx).ID).Str("user_id", id).Msg("enrollment documents viewed")
			dialog.FrontPreview = enrollmentLink(page.Pager.Current, page.Filter, page.Tag, id, record, "front")
			dialog.BackPreview = enrollmentLink(page.Pager.Current, page.Filter, page.Tag, id, record, "back")
			if mode == "front" {
				dialog.Photo.Side = "正面"
				dialog.Photo.URL = dialog.Document.Details.FrontURL
			} else if mode == "back" {
				dialog.Photo.Side = "反面"
				dialog.Photo.URL = dialog.Document.Details.BackURL
			}
		}

		return dialog, nil
	}

	if (mode != "review" && mode != "tags") || dialog.Row.ReviewText == "" {
		return nil, app.Forbidden("当前报名不可复核")
	}

	configureEnrollmentReview(dialog, page, tags)
	dialog.Action = "/users/" + id + "/review?" + dialog.BackQuery
	return dialog, nil
}

func configureEnrollmentReview(dialog *enrollmentDialog, page *enrollmentsPage, tags []enrollment.MarketingTagDTO) {
	row := dialog.Row
	dialog.TagOnly = dialog.Mode == "tags"
	dialog.PortraitOnly = row.Status == enum.EnrollmentApproved
	dialog.CanCreateTag = page.CanCreate && page.HasLibrary
	if row.EnrollmentUpdatedAt != nil {
		dialog.Revision = row.EnrollmentUpdatedAt.Format(time.RFC3339Nano)
	}

	dialog.Title = "复核 " + row.DisplayName + " 的报名"
	if dialog.TagOnly {
		dialog.Title = "编辑 " + row.DisplayName + " 的标签"
	} else if dialog.PortraitOnly {
		dialog.Title = "改 " + row.DisplayName + " 的画像 / 标签"
	}

	if row.EnrollmentRecentPost != nil {
		dialog.RecentPost = strconv.FormatBool(*row.EnrollmentRecentPost)
	}

	if row.EnrollmentNewAccount != nil {
		dialog.NewAccount = strconv.FormatBool(*row.EnrollmentNewAccount)
	}

	dialog.Ages = slices.Clone(enrollmentAges)
	for i := range dialog.Ages {
		dialog.Ages[i].Selected = row.EnrollmentAgeGroup != nil && dialog.Ages[i].Key == string(*row.EnrollmentAgeGroup)
	}

	dialog.TagChoices = enrollmentTagChoices(tags, row.Tags, row.Tags)
}

func formBool(value string) *bool {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}

	return &parsed
}

func (admin *admin) saveEnrollment(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, enrollmentsRoute, enrollmentReviewPermission) {
		return
	}

	current := currentOperator(ctx)
	revision, err := time.Parse(time.RFC3339Nano, ctx.PostForm("revision"))
	if err != nil {
		admin.failure(ctx, http.StatusBadRequest, "报名资料已失效，请刷新后重试")
		return
	}

	input := enrollment.ReviewInput{
		EnrollmentID: ctx.PostForm("record"),
		Approved:     ctx.PostForm("decision") != "reject",
		Reason:       strings.TrimSpace(ctx.PostForm("reason")),
		UpdatedAt:    &revision,
		Tags:         append([]string{}, ctx.PostFormArray("tags")...),
		RecentPost:   formBool(ctx.PostForm("recent_post")),
		NewAccount:   formBool(ctx.PostForm("new_account")),
		AgeGroup:     enum.EnrollmentAge(ctx.PostForm("age_group")),
		PortraitOnly: ctx.PostForm("portrait") == "1",
		TagsOnly:     ctx.PostForm("tags_only") == "1",
	}
	if input.PortraitOnly && ctx.PostForm("portrait_changed") == "" {
		input.TagsOnly = true
	}

	if input.TagsOnly {
		input.Approved = false
		input.PortraitOnly = false
	}

	if input.Approved && !input.TagsOnly && (input.RecentPost == nil || input.NewAccount == nil || input.AgeGroup == "") {
		err = app.Invalid("请填写完整的人工复核")
	} else if !input.Approved && !input.TagsOnly && input.Reason == "" {
		err = app.Invalid("请填写驳回原因，分发员将收到此原因")
	} else {
		err = admin.services.SaveEnrollmentReview(ctx.Request.Context(), current.ID, ctx.Param("id"), input)
	}

	success := "已保存报名复核"
	if ctx.PostForm("tags_only") == "1" {
		success = "已保存报名标签"
	}

	if err == nil {
		query := ctx.Request.URL.Query()
		query.Del("page")
		ctx.Request.URL.RawQuery = query.Encode()
		admin.finishEnrollment(ctx, success, nil, nil)
		return
	}

	query := ctx.Request.URL.Query()
	query.Set("dialog", ctx.PostForm("mode"))
	query.Set("member", ctx.Param("id"))
	query.Set("record", input.EnrollmentID)
	ctx.Request.URL.RawQuery = query.Encode()
	page, loadErr := admin.loadEnrollments(ctx)
	if loadErr != nil || page.Dialog == nil {
		admin.finishEnrollment(ctx, "", nil, err)
		return
	}

	dialog := page.Dialog
	dialog.PortraitChanged = ctx.PostForm("portrait_changed") == "1"
	dialog.Revision = ctx.PostForm("revision")
	dialog.Reason = input.Reason
	dialog.RecentPost = ctx.PostForm("recent_post")
	dialog.NewAccount = ctx.PostForm("new_account")
	for index := range dialog.Ages {
		dialog.Ages[index].Selected = dialog.Ages[index].Key == string(input.AgeGroup)
	}

	for index := range dialog.TagChoices {
		dialog.TagChoices[index].Selected = slices.Contains(input.Tags, dialog.TagChoices[index].Key)
	}

	admin.finishEnrollment(ctx, "", dialog, err)
}

func (admin *admin) updateEnrollmentTag(ctx *gin.Context) {
	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, enrollmentsRoute, "GET /marketing-tags") {
		return
	}

	account := currentOperator(ctx)
	request := ctx.Request.Context()
	action := ctx.Param("action")
	id := ctx.Param("id")
	name := ctx.PostForm("name")
	var err error
	switch action {
	case "":
		if !admin.allowPageAction(ctx, enrollmentsRoute, "POST /marketing-tags") {
			return
		}
		_, err = admin.services.CreateMarketingTag(request, account.ID, name)
	case "rename":
		if !admin.allowPageAction(ctx, enrollmentsRoute, "POST /marketing-tags/:id/rename") {
			return
		}
		err = admin.services.RenameMarketingTag(request, account.ID, id, name, ctx.PostForm("previous_name"))
	case "delete":
		if !admin.allowPageAction(ctx, enrollmentsRoute, "POST /marketing-tags/:id/delete") {
			return
		}
		revision, parseErr := time.Parse(time.RFC3339Nano, ctx.PostForm("revision"))
		if parseErr != nil {
			err = app.Invalid("标签已失效，请刷新后重试")
		} else {
			err = admin.services.DeleteMarketingTag(request, account.ID, id, revision)
		}
	default:
		admin.failure(ctx, http.StatusNotFound, "页面不存在")
		return
	}

	if err == nil {
		admin.finishEnrollment(ctx, "已更新标签库", nil, nil)
		return
	}

	if action == "" {
		admin.finishEnrollment(ctx, "", nil, err)
		return
	}

	query := ctx.Request.URL.Query()
	query.Set("dialog", action)
	query.Set("member", id)
	ctx.Request.URL.RawQuery = query.Encode()
	page, loadErr := admin.loadEnrollments(ctx)
	if loadErr != nil {
		admin.finishEnrollment(ctx, "", nil, err)
		return
	}

	if action == "rename" {
		page.Dialog.Reason = name
	}

	admin.finishEnrollment(ctx, "", page.Dialog, err)
}

func (admin *admin) finishEnrollment(ctx *gin.Context, success string, dialog *enrollmentDialog, actionErr error) {
	status := http.StatusOK
	message := ""
	if actionErr != nil {
		status, message = adminErrorMessage(ctx, actionErr)
		if dialog != nil {
			dialog.Error = message
			if partialRequest(ctx) {
				ctx.Header("X-Admin-Form", "enrollment")
				admin.render(ctx, status, "enrollment-dialog", dialog)
				return
			}
		}
	} else if !partialRequest(ctx) {
		redirect(ctx, enrollmentsRoute.Path+"?"+ctx.Request.URL.Query().Encode())
		return
	}

	data := admin.workspace(ctx, enrollmentsRoute)
	page, err := admin.loadEnrollments(ctx)
	if err != nil {
		status, data.Error = adminErrorMessage(ctx, err)
	} else {
		data.Enrollments = page
		if dialog != nil {
			page.Dialog = dialog
		} else if message != "" {
			data.Error = message
		} else {
			data.Success = success
		}
	}

	admin.renderWorkspace(ctx, status, data)
}
