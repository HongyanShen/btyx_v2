package business

import (
	"context"

	"baituber.cn/yanxuan/v2/internal/integrations/tencentfaceid"
	"baituber.cn/yanxuan/v2/internal/integrations/tencentocr"
	tosstorage "baituber.cn/yanxuan/v2/internal/integrations/tos"
	"baituber.cn/yanxuan/v2/internal/integrations/wechatmini"
	"baituber.cn/yanxuan/v2/internal/modules/agreement"
	"baituber.cn/yanxuan/v2/internal/modules/audit"
	"baituber.cn/yanxuan/v2/internal/modules/content"
	"baituber.cn/yanxuan/v2/internal/modules/enrollment"
	"baituber.cn/yanxuan/v2/internal/modules/feedback"
	"baituber.cn/yanxuan/v2/internal/modules/files"
	"baituber.cn/yanxuan/v2/internal/modules/identity"
	"baituber.cn/yanxuan/v2/internal/modules/notification"
	"baituber.cn/yanxuan/v2/internal/modules/settlement"
	"baituber.cn/yanxuan/v2/internal/modules/submission"
	"baituber.cn/yanxuan/v2/internal/modules/task"
	"baituber.cn/yanxuan/v2/internal/modules/wallet"
	"baituber.cn/yanxuan/v2/internal/platform/app"
	"baituber.cn/yanxuan/v2/internal/platform/config"
	"baituber.cn/yanxuan/v2/internal/platform/objectstore"
)

type IdentityService = identity.Service

type EnrollmentService = enrollment.Service

type AgreementService = agreement.Service

type TaskService = task.Service

type SubmissionService = submission.Service

type SettlementService = settlement.Service

type WalletService = wallet.Service

type ContentService = content.Service

type FeedbackService = feedback.Service

type FilesService = files.Service

type NotificationService = notification.Service

type AuditService = audit.Service

type LoginResult = identity.LoginResult

type Principal = identity.Principal

type UserDTO = identity.UserDTO

type ApplyInput = enrollment.ApplyInput

type TaskInput = task.TaskInput

type VersionDTO = task.VersionDTO

type StepDTO = task.StepDTO

type SubmissionDTO = task.SubmissionDTO

type AssignmentDTO = task.AssignmentDTO

type SubmitInput = submission.SubmitInput

type SettlementInput = settlement.SettlementInput

type BatchDTO = settlement.BatchDTO

type WalletDTO = wallet.WalletDTO

type WithdrawalDTO = wallet.WithdrawalDTO

type LedgerDTO = wallet.LedgerDTO

// Service composes modules for process bootstrap and cross-module integration tests.
type Service struct {
	*app.Runtime
	*IdentityService
	*EnrollmentService
	*AgreementService
	*TaskService
	*SubmissionService
	*SettlementService
	*WalletService
	*ContentService
	*FeedbackService
	*FilesService
	*NotificationService
	*AuditService
}

func New(ctx context.Context, cfg config.Config) (*Service, error) {
	runtime, err := app.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	storage, err := tosstorage.New(cfg.Storage)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}

	var fileStorage objectstore.Gateway
	if storage != nil {
		fileStorage = storage
	}

	identityService := &identity.Service{
		Runtime:     runtime,
		WechatPhone: wechatmini.NewPhoneClient(cfg.WechatAppID, cfg.WechatAppSecret),
		Store:       fileStorage,
	}
	ocrClient, err := tencentocr.New(cfg.IdentityOCRSecretID, cfg.IdentityOCRSecretKey)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}

	if ocrClient != nil {
		identityService.IdentityOCR = ocrClient
	}

	enrollmentService := &enrollment.Service{Runtime: runtime}
	verifier, err := tencentfaceid.New(cfg.IdentityOCRSecretID, cfg.IdentityOCRSecretKey)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}

	if verifier != nil {
		identityService.Verifier = verifier
		enrollmentService.Verifier = verifier
	}

	taskService := &task.Service{
		Runtime: runtime,
	}
	return &Service{
		Runtime: runtime,

		IdentityService: identityService,

		EnrollmentService: enrollmentService,

		AgreementService: &agreement.Service{
			Runtime: runtime,
		},

		TaskService: taskService,

		SubmissionService: &submission.Service{
			Runtime:     runtime,
			TaskQueries: taskService,
		},

		SettlementService: &settlement.Service{
			Runtime: runtime,
		},

		WalletService: &wallet.Service{
			Runtime: runtime,
		},

		ContentService: &content.Service{
			Runtime: runtime,
		},

		FeedbackService: &feedback.Service{
			Runtime: runtime,
		},

		FilesService: &files.Service{
			Runtime: runtime,
			Store:   fileStorage,
		},

		NotificationService: &notification.Service{
			Runtime: runtime,
		},

		AuditService: &audit.Service{
			Runtime: runtime,
			Store:   fileStorage,
		},
	}, nil
}

func NewID() string {
	return app.NewID()
}
