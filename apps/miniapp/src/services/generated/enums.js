// Code generated from Go enum definitions; DO NOT EDIT.

const AssignmentSource = Object.freeze({
  SourceClaim: "claim",
  SourceAssigned: "assigned",
  SourceReissued: "reissued",
});

const AssignmentStatus = Object.freeze({
  AssignmentInProgress: "in_progress",
  AssignmentPendingReview: "pending_review",
  AssignmentRejected: "rejected",
  AssignmentPendingSettlement: "pending_settlement",
  AssignmentSettled: "settled",
  AssignmentRevoked: "revoked",
  AssignmentExpired: "expired",
  AssignmentNotPayable: "not_payable",
});

const AuditAction = Object.freeze({
  AuditEnrollmentApply: "enrollment.apply",
  AuditEnrollmentApproved: "enrollment.approved",
  AuditEnrollmentRejected: "enrollment.rejected",
  AuditEnrollmentPortrait: "enrollment.portrait_updated",
  AuditMarketingTagCreate: "marketing_tag.create",
  AuditMarketingTagRename: "marketing_tag.rename",
  AuditMarketingTagDelete: "marketing_tag.delete",
  AuditUserBlock: "user.block",
  AuditUserProfileUpdate: "user.profile_update",
  AuditUserPhoneVerify: "user.phone_verify",
  AuditUserIdentityVerify: "user.identity_verify",
  AuditUserIdentityFailed: "user.identity_verify_failed",
  AuditEnrollmentIdentityVerify: "enrollment.identity_verify",
  AuditEnrollmentIdentityFailed: "enrollment.identity_verify_failed",
  AuditAgreementConsent: "agreement.consent",
  AuditAgreementPublish: "agreement.publish",
  AuditTaskCreate: "task.create",
  AuditTaskPublishVersion: "task.publish_version",
  AuditTaskVisibility: "task.visibility",
  AuditTaskClaim: "task.claim",
  AuditTaskAssign: "task.assigned",
  AuditTaskRevoke: "task.revoke",
  AuditTaskReissue: "task.reissue",
  AuditTaskExpire: "task.expire",
  AuditMarketingSettings: "marketing.settings",
  AuditSubmissionSubmit: "submission.submit",
  AuditSubmissionApproved: "submission.approved",
  AuditSubmissionRejected: "submission.rejected",
  AuditSubmissionWaive: "submission.waive",
  AuditSettlementCredit: "settlement.credit",
  AuditWithdrawalRequest: "withdrawal.request",
  AuditLedgerAdjust: "ledger.adjust",
  AuditContentCreate: "content.create",
  AuditContentTags: "content.tags_updated",
  AuditFeedbackReply: "feedback.reply",
  AuditOperatorCreate: "operator.create",
  AuditOperatorUpdate: "operator.update",
});

const EnrollmentAge = Object.freeze({
  EnrollmentAge18To24: "18_24",
  EnrollmentAge25To30: "25_30",
  EnrollmentAge31To40: "31_40",
  EnrollmentAgeOver40: "over_40",
  EnrollmentAgeUnknown: "unknown",
});

const EnrollmentStatus = Object.freeze({
  EnrollmentNone: "none",
  EnrollmentPending: "pending",
  EnrollmentApproved: "approved",
  EnrollmentRejected: "rejected",
});

const ErrorCode = Object.freeze({
  ErrorInvalidArgument: "INVALID_ARGUMENT",
  ErrorConflict: "CONFLICT",
  ErrorForbidden: "FORBIDDEN",
  ErrorNotFound: "NOT_FOUND",
  ErrorUnauthorized: "UNAUTHORIZED",
  ErrorInternal: "INTERNAL_ERROR",
  ErrorUnavailable: "UNAVAILABLE",
});

const FeedbackStatus = Object.freeze({
  FeedbackOpen: "open",
  FeedbackReplied: "replied",
  FeedbackClosed: "closed",
});

const FileMediaType = Object.freeze({
  FileJPEG: "image/jpeg",
  FilePNG: "image/png",
  FileGIF: "image/gif",
  FileWEBP: "image/webp",
  FileMP4: "video/mp4",
});

const FileStatus = Object.freeze({
  FilePending: "pending",
  FileReady: "ready",
});

const FileStorage = Object.freeze({
  FileStorageLocal: "local",
  FileStorageTOS: "tos",
});

const JobKind = Object.freeze({
  JobTransfer: "transfer",
  JobTransferQuery: "transfer_query",
  JobNotification: "notification",
  JobVisibilityCheck: "visibility_check",
});

const JobStatus = Object.freeze({
  JobQueued: "queued",
  JobRunning: "running",
  JobDone: "done",
  JobRetry: "retry",
  JobFailed: "failed",
  JobBlockedConfig: "blocked_config",
});

const LedgerKind = Object.freeze({
  LedgerSettlementCredit: "settlement_credit",
  LedgerWithdrawalFreeze: "withdrawal_freeze",
  LedgerWithdrawalSuccess: "withdrawal_success",
  LedgerWithdrawalRefund: "withdrawal_refund",
  LedgerAdjustment: "adjustment",
});

const MarketingExpiryMode = Object.freeze({
  MarketingExpiryFixedDuration: "fixed_duration",
  MarketingExpiryEndOfDay: "end_of_day",
});

const Platform = Object.freeze({
  PlatformDouyin: "dy",
  PlatformXiaohongshu: "xhs",
});

const Role = Object.freeze({
  RoleAdmin: "admin",
  RoleUser: "user",
});

const SubmissionStatus = Object.freeze({
  SubmissionPending: "pending",
  SubmissionApproved: "approved",
  SubmissionRejected: "rejected",
  SubmissionInvalid: "invalid",
  SubmissionWaived: "waived",
});

const TaskStep = Object.freeze({
  StepContent: "content",
  StepPublish: "publish",
  StepLinks: "links",
  StepData: "data",
  StepSettle: "settle",
});

const Visibility = Object.freeze({
  VisibilityUnchecked: "unchecked",
  VisibilityManualVerified: "manual_verified",
  VisibilityVisible: "visible",
  VisibilityUnavailable: "unavailable",
  VisibilityUnknown: "unknown",
});

const WechatTransferState = Object.freeze({
  WechatTransferAccepted: "ACCEPTED",
  WechatTransferProcessing: "PROCESSING",
  WechatTransferWaitConfirm: "WAIT_USER_CONFIRM",
  WechatTransferTransferring: "TRANSFERING",
  WechatTransferCancelling: "CANCELING",
  WechatTransferSuccess: "SUCCESS",
  WechatTransferFailed: "FAIL",
  WechatTransferCancelled: "CANCELLED",
});

const WithdrawalStatus = Object.freeze({
  WithdrawalQueued: "queued",
  WithdrawalSubmitting: "submitting",
  WithdrawalUnknown: "unknown",
  WithdrawalWaitingConfirmation: "waiting_confirmation",
  WithdrawalSuccess: "success",
  WithdrawalFailed: "failed",
  WithdrawalCancelled: "cancelled",
  WithdrawalBlockedConfig: "blocked_config",
});

module.exports = Object.freeze({
  AssignmentSource,
  AssignmentStatus,
  AuditAction,
  EnrollmentAge,
  EnrollmentStatus,
  ErrorCode,
  FeedbackStatus,
  FileMediaType,
  FileStatus,
  FileStorage,
  JobKind,
  JobStatus,
  LedgerKind,
  MarketingExpiryMode,
  Platform,
  Role,
  SubmissionStatus,
  TaskStep,
  Visibility,
  WechatTransferState,
  WithdrawalStatus,
});
