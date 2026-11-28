package main

import (
	"net/http"
	"slices"
	"strconv"

	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"github.com/gin-gonic/gin"
)

var contentRoute = workspacePage{
	Path:     "/contents",
	Label:    "内容池",
	Endpoint: "GET /contents",
}

const contentCreatePermission = "POST /contents"
const contentTagsPermission = "POST /contents/:id/tags"

type contentPage struct {
	Rows        []contentViewRow
	Pager       pagination
	CanCreate   bool
	CanEditTags bool
	CanDownload bool
	Dialog      *contentDialog
}

type contentViewRow struct {
	content.DTO
	Date       string
	TagLabels  []string
	Attachment []contentAttachment
}

type contentDialog struct {
	content.DTO
	CSRF         string
	Page         int
	Error        string
	CanUpload    bool
	CanDownload  bool
	TagsEnabled  bool
	PreviousTags []string
	Choices      []contentTagChoice
	Attachments  []contentAttachment
}

type contentAttachment struct {
	ID     string
	Number int
}

type contentTagChoice struct {
	Key      string
	Label    string
	Selected bool
}

func (admin *admin) loadContent(ctx *gin.Context) (*contentPage, error) {
	account := currentOperator(ctx)
	entries, err := admin.services.Contents(ctx.Request.Context())
	if err != nil {
		return nil, err
	}

	tags, err := admin.contentTagCatalog(ctx, account)
	if err != nil {
		return nil, err
	}

	labels := make(map[string]string, len(tags))
	for _, tag := range tags {
		labels[tag.Key] = tag.Name
	}

	start, end, pager := listPageBounds(ctx, contentRoute.Path, len(entries), 10)
	page := &contentPage{
		Pager:       pager,
		CanCreate:   canCall(account, contentCreatePermission),
		CanEditTags: canCall(account, contentTagsPermission),
		CanDownload: canCall(account, "GET /files/:id/download"),
	}
	for _, item := range entries[start:end] {
		row := contentViewRow{
			DTO:        item,
			Date:       item.CreatedAt.In(beijing).Format("2006/1/2"),
			Attachment: contentAttachments(item.Files),
		}
		for _, key := range item.Tags {
			row.TagLabels = append(row.TagLabels, label(labels[key], key))
		}

		page.Rows = append(page.Rows, row)
	}

	if ctx.Query("new") == "1" && page.CanCreate {
		page.Dialog = newContentDialog(ctx, account, pager.Current, content.DTO{}, nil, tags)
	} else if id := ctx.Query("edit"); id != "" && page.CanEditTags {
		index := slices.IndexFunc(entries, func(item content.DTO) bool { return item.ID == id })
		if index >= 0 {
			item := entries[index]
			page.Dialog = newContentDialog(ctx, account, pager.Current, item, item.Tags, tags)
		}
	}

	return page, nil
}

func (admin *admin) contentTagCatalog(ctx *gin.Context, account identity.OperatorDTO) ([]enrollment.MarketingTagDTO, error) {
	if !canCall(account, "GET /marketing-tags") {
		return nil, nil
	}

	return admin.services.MarketingTags(ctx.Request.Context())
}

func contentAttachments(ids []string) []contentAttachment {
	items := make([]contentAttachment, 0, len(ids))
	for index, id := range ids {
		items = append(items, contentAttachment{
			ID:     id,
			Number: index + 1,
		})
	}

	return items
}

func newContentDialog(ctx *gin.Context, account identity.OperatorDTO, page int, item content.DTO, previous []string, tags []enrollment.MarketingTagDTO) *contentDialog {
	dialog := &contentDialog{
		DTO:          item,
		CSRF:         csrfToken(sessionToken(ctx)),
		Page:         page,
		CanUpload:    canCall(account, "POST /files/uploads") && canCall(account, "POST /files/:id/complete"),
		CanDownload:  canCall(account, "GET /files/:id/download"),
		TagsEnabled:  canCall(account, "GET /marketing-tags"),
		PreviousTags: previous,
		Attachments:  contentAttachments(item.Files),
	}
	for _, tag := range tags {
		dialog.Choices = append(dialog.Choices, contentTagChoice{
			Key:      tag.Key,
			Label:    tag.Name,
			Selected: slices.Contains(item.Tags, tag.Key),
		})
	}

	for _, key := range item.Tags {
		if slices.ContainsFunc(dialog.Choices, func(choice contentTagChoice) bool { return choice.Key == key }) {
			continue
		}

		dialog.Choices = append(dialog.Choices, contentTagChoice{
			Key:      key,
			Label:    key,
			Selected: true,
		})
	}

	return dialog
}

func (admin *admin) saveContent(ctx *gin.Context) {
	id := ctx.Param("id")
	permission := contentCreatePermission
	if id != "" {
		permission = contentTagsPermission
	}

	if !admin.allowForm(ctx) || !admin.allowPageAction(ctx, contentRoute, permission) {
		return
	}

	account := currentOperator(ctx)
	request := ctx.Request.Context()
	page, _ := strconv.Atoi(ctx.Query("page"))
	page = max(page, 1)
	input := content.Input{
		Title:   ctx.PostForm("title"),
		Project: ctx.PostForm("project"),
		Body:    ctx.PostForm("body"),
		Files:   append([]string{}, ctx.PostFormArray("files")...),
		Tags:    append([]string{}, ctx.PostFormArray("tags")...),
	}
	previous := ctx.PostFormArray("previous_tags")
	var err error
	success := "素材已加入内容池"
	if id == "" {
		_, err = admin.services.CreateContent(request, account.ID, input)
	} else {
		err = admin.services.UpdateContentTags(request, account.ID, id, content.TagsInput{
			Tags:         input.Tags,
			PreviousTags: previous,
		})
		success = "已保存素材标签"
	}

	status := http.StatusOK
	var dialog *contentDialog
	if err != nil {
		tags, catalogErr := admin.contentTagCatalog(ctx, account)
		if catalogErr != nil {
			status, message := adminErrorMessage(ctx, catalogErr)
			admin.failure(ctx, status, message)
			return
		}

		item := content.DTO{
			ID:    id,
			Input: input,
		}
		dialog = newContentDialog(ctx, account, page, item, previous, tags)
		status, dialog.Error = adminErrorMessage(ctx, err)
		if partialRequest(ctx) {
			ctx.Header("X-Admin-Form", "content")
			admin.render(ctx, status, "content-dialog", dialog)
			return
		}
	} else if !partialRequest(ctx) {
		redirect(ctx, contentRoute.Path+"?page="+strconv.Itoa(page))
		return
	}

	data := admin.workspace(ctx, contentRoute)
	data.Content, err = admin.loadContent(ctx)
	if err != nil {
		status, data.Error = adminErrorMessage(ctx, err)
	} else if dialog != nil {
		data.Content.Dialog = dialog
	} else {
		data.Success = success
	}

	admin.renderWorkspace(ctx, status, data)
}
