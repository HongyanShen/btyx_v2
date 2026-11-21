package handler

import (
	"slices"
	"strconv"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/httpx"
	"baituber.cn/yanxuan/v2/internal/platform/logging"
	"baituber.cn/yanxuan/v2/internal/transport/httpapi"
	"github.com/gin-gonic/gin"
)

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type blockUserInput struct {
	Blocked   *bool      `json:"blocked"`
	Reason    string     `json:"reason"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (handler *Handler) login(ctx *gin.Context, input loginInput) (identity.LoginResult, error) {
	ctx.Header("Cache-Control", "private, no-store")

	return handler.services.LoginAdmin(ctx.Request.Context(), input.Username, input.Password)
}

func (handler *Handler) users(ctx *gin.Context) ([]identity.UserDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")
	if ctx.Query("view") == "distributors" {
		return handler.distributors(ctx)
	}

	if ctx.Query("view") == "enrollments" {
		if value, exists := ctx.GetQuery("page"); exists {
			page, err := strconv.Atoi(value)
			if err != nil {
				return nil, app.Invalid("分页参数无效")
			}

			withPhotos := ctx.Query("photos") == "1"
			if withPhotos && !canViewEnrollmentDocuments(ctx) {
				return nil, app.Forbidden("当前账号无权查看报名证件照片")
			}

			status := enum.EnrollmentStatus(ctx.Query("status"))
			rows, total, err := handler.services.EnrollmentPage(ctx.Request.Context(), page, status, ctx.Query("tag"))
			if err != nil {
				return nil, err
			}

			if withPhotos {
				if err := handler.attachEnrollmentPhotoPreviews(ctx, rows); err != nil {
					return nil, err
				}
			}

			ctx.Header("X-Total-Count", strconv.Itoa(total))
			return rows, nil
		}

		return handler.services.EnrollmentUsers(ctx.Request.Context(), ctx.Query("before"))
	}

	return handler.services.Users(ctx.Request.Context())
}

func canViewEnrollmentDocuments(ctx *gin.Context) bool {
	account := httpapi.CurrentPrincipal(ctx).Operator
	return account.SuperAdmin || slices.Contains(account.Endpoints, "GET /users/:id/enrollment-documents")
}

func (handler *Handler) distributors(ctx *gin.Context) ([]identity.UserDTO, error) {
	request := ctx.Request.Context()
	userID := ctx.Query("id")
	rows, err := handler.services.Distributors(request, userID, canViewEnrollmentDocuments(ctx))
	if err != nil || userID == "" {
		return rows, err
	}

	tasks, err := handler.services.TodayDistributorTasks(request, userID)
	if err != nil {
		return nil, err
	}

	rows[0].DistributorDetails.TodayTasks = tasks
	return rows, nil
}

func (handler *Handler) attachEnrollmentPhotoPreviews(ctx *gin.Context, rows []identity.UserDTO) error {
	enrollmentIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		enrollmentIDs = append(enrollmentIDs, row.EnrollmentID)
	}

	request := ctx.Request.Context()
	previews, err := handler.services.EnrollmentPhotoPreviews(request, enrollmentIDs)
	if err != nil {
		return err
	}

	for index := range rows {
		rows[index].EnrollmentPhotoPreviews = previews[rows[index].EnrollmentID]
	}

	if len(previews) > 0 {
		logging.From(request).Info().
			Str("operator_id", httpapi.CurrentUser(ctx)).
			Int("records", len(previews)).
			Msg("enrollment photo previews viewed")
	}

	return nil
}

func (handler *Handler) identityRecords(ctx *gin.Context) (audit.IdentityRecordsDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")
	return handler.services.IdentityRecords(ctx.Request.Context(), ctx.Query("before"))
}

func (handler *Handler) identityRecordsWithDocuments(ctx *gin.Context) (audit.IdentityRecordsDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")

	request := ctx.Request.Context()
	records, err := handler.services.IdentityRecordsWithDocuments(request, ctx.Query("before"))
	if err != nil {
		return records, err
	}

	logging.From(request).Info().
		Str("operator_id", httpapi.CurrentUser(ctx)).
		Int("records", len(records.Records)).
		Msg("identity documents viewed")

	return records, nil
}

func (handler *Handler) enrollmentDocuments(ctx *gin.Context) (audit.EnrollmentDocumentsDTO, error) {
	ctx.Header("Cache-Control", "private, no-store")

	request := ctx.Request.Context()
	userID := ctx.Param("id")
	documents, err := handler.services.EnrollmentDocuments(request, userID, ctx.Query("enrollment_id"))
	if err == nil {
		logging.From(request).Info().
			Str("operator_id", httpapi.CurrentUser(ctx)).
			Str("user_id", userID).
			Msg("enrollment documents viewed")
	}

	return documents, err
}

func (handler *Handler) reviewEnrollment(ctx *gin.Context, input enrollment.ReviewInput) (*httpx.NoData, error) {
	return nil, handler.services.SaveEnrollmentReview(ctx.Request.Context(), httpapi.CurrentUser(ctx), ctx.Param("id"), input)
}

func (handler *Handler) blockUser(ctx *gin.Context, input blockUserInput) (*httpx.NoData, error) {
	if input.Blocked == nil {
		return nil, app.Invalid("请选择分发员状态")
	}

	return nil, handler.services.Block(
		ctx.Request.Context(),
		httpapi.CurrentUser(ctx),
		ctx.Param("id"),
		*input.Blocked,
		input.Reason,
		input.UpdatedAt,
	)
}
