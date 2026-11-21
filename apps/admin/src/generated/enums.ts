// Code generated from Go enum definitions; DO NOT EDIT.

export enum AssignmentSource {
  SourceClaim = "claim",
  SourceAssigned = "assigned",
  SourceReissued = "reissued",
}

export enum AssignmentStatus {
  AssignmentInProgress = "in_progress",
  AssignmentPendingReview = "pending_review",
  AssignmentRejected = "rejected",
  AssignmentPendingSettlement = "pending_settlement",
  AssignmentSettled = "settled",
  AssignmentRevoked = "revoked",
  AssignmentExpired = "expired",
  AssignmentNotPayable = "not_payable",
}

export enum AuditAction {
  AuditEnrollmentApply = "enrollment.apply",
  AuditEnrollmentApproved = "enrollment.approved",
  AuditEnrollmentRejected = "enrollment.rejected",
  AuditEnrollmentPortrait = "enrollment.portrait_updated",
  AuditMarketingTagCreate = "marketing_tag.create",
  AuditMarketingTagRename = "marketing_tag.rename",
  AuditMarketingTagDelete = "marketing_tag.delete",
  AuditUserBlock = "user.block",
  AuditUserProfileUpdate = "user.profile_update",
  AuditUserPhoneVerify = "user.phone_verify",
  AuditUserIdentityVerify = "user.identity_verify",
  AuditUserIdentityFailed = "user.identity_verify_failed",
  AuditEnrollmentIdentityVerify = "enrollment.identity_verify",
  AuditEnrollmentIdentityFailed = "enrollment.identity_verify_failed",
  AuditAgreementConsent = "agreement.consent",
  AuditAgreementPublish = "agreement.publish",
  AuditTaskCreate = "task.create",
  AuditTaskPublishVersion = "task.publish_version",
  AuditTaskVisibility = "task.visibility",
  AuditTaskClaim = "task.claim",
  AuditTaskAssign = "task.assigned",
  AuditTaskRevoke = "task.revoke",
  AuditTaskReissue = "task.reissue",
  AuditTaskExpire = "task.expire",
  AuditMarketingSettings = "marketing.settings",
  AuditSubmissionSubmit = "submission.submit",
  AuditSubmissionApproved = "submission.approved",
  AuditSubmissionRejected = "submission.rejected",
  AuditSubmissionWaive = "submission.waive",
  AuditSettlementCredit = "settlement.credit",
  AuditWithdrawalRequest = "withdrawal.request",
  AuditLedgerAdjust = "ledger.adjust",
  AuditContentCreate = "content.create",
  AuditContentTags = "content.tags_updated",
  AuditFeedbackReply = "feedback.reply",
  AuditOperatorCreate = "operator.create",
  AuditOperatorUpdate = "operator.update",
}

export enum EnrollmentAge {
  EnrollmentAge18To24 = "18_24",
  EnrollmentAge25To30 = "25_30",
  EnrollmentAge31To40 = "31_40",
  EnrollmentAgeOver40 = "over_40",
  EnrollmentAgeUnknown = "unknown",
}

export enum EnrollmentStatus {
  EnrollmentNone = "none",
  EnrollmentPending = "pending",
  EnrollmentApproved = "approved",
  EnrollmentRejected = "rejected",
}

export enum ErrorCode {
  ErrorInvalidArgument = "INVALID_ARGUMENT",
  ErrorConflict = "CONFLICT",
  ErrorForbidden = "FORBIDDEN",
  ErrorNotFound = "NOT_FOUND",
  ErrorUnauthorized = "UNAUTHORIZED",
  ErrorInternal = "INTERNAL_ERROR",
  ErrorUnavailable = "UNAVAILABLE",
}

export enum FeedbackStatus {
  FeedbackOpen = "open",
  FeedbackReplied = "replied",
  FeedbackClosed = "closed",
}

export enum FileMediaType {
  FileJPEG = "image/jpeg",
  FilePNG = "image/png",
  FileGIF = "image/gif",
  FileWEBP = "image/webp",
  FileMP4 = "video/mp4",
}

export enum FileStatus {
  FilePending = "pending",
  FileReady = "ready",
}

export enum FileStorage {
  FileStorageLocal = "local",
  FileStorageTOS = "tos",
}

export enum JobKind {
  JobTransfer = "transfer",
  JobTransferQuery = "transfer_query",
  JobNotification = "notification",
  JobVisibilityCheck = "visibility_check",
}

export enum JobStatus {
  JobQueued = "queued",
  JobRunning = "running",
  JobDone = "done",
  JobRetry = "retry",
  JobFailed = "failed",
  JobBlockedConfig = "blocked_config",
}

export enum LedgerKind {
  LedgerSettlementCredit = "settlement_credit",
  LedgerWithdrawalFreeze = "withdrawal_freeze",
  LedgerWithdrawalSuccess = "withdrawal_success",
  LedgerWithdrawalRefund = "withdrawal_refund",
  LedgerAdjustment = "adjustment",
}

export enum MarketingExpiryMode {
  MarketingExpiryFixedDuration = "fixed_duration",
  MarketingExpiryEndOfDay = "end_of_day",
}

export enum Platform {
  PlatformDouyin = "dy",
  PlatformXiaohongshu = "xhs",
}

export enum Role {
  RoleAdmin = "admin",
  RoleUser = "user",
}

export enum SubmissionStatus {
  SubmissionPending = "pending",
  SubmissionApproved = "approved",
  SubmissionRejected = "rejected",
  SubmissionInvalid = "invalid",
  SubmissionWaived = "waived",
}

export enum TaskStep {
  StepContent = "content",
  StepPublish = "publish",
  StepLinks = "links",
  StepData = "data",
  StepSettle = "settle",
}

export enum Visibility {
  VisibilityUnchecked = "unchecked",
  VisibilityManualVerified = "manual_verified",
  VisibilityVisible = "visible",
  VisibilityUnavailable = "unavailable",
  VisibilityUnknown = "unknown",
}

export enum WechatTransferState {
  WechatTransferAccepted = "ACCEPTED",
  WechatTransferProcessing = "PROCESSING",
  WechatTransferWaitConfirm = "WAIT_USER_CONFIRM",
  WechatTransferTransferring = "TRANSFERING",
  WechatTransferCancelling = "CANCELING",
  WechatTransferSuccess = "SUCCESS",
  WechatTransferFailed = "FAIL",
  WechatTransferCancelled = "CANCELLED",
}

export enum WithdrawalStatus {
  WithdrawalQueued = "queued",
  WithdrawalSubmitting = "submitting",
  WithdrawalUnknown = "unknown",
  WithdrawalWaitingConfirmation = "waiting_confirmation",
  WithdrawalSuccess = "success",
  WithdrawalFailed = "failed",
  WithdrawalCancelled = "cancelled",
  WithdrawalBlockedConfig = "blocked_config",
}
