import {
  AssignmentSource,
  AssignmentStatus,
  EnrollmentStatus,
  Platform,
  SubmissionStatus,
  WithdrawalStatus,
} from "../generated/enums";

export const platformLabels: Record<Platform, string> = {
  [Platform.PlatformDouyin]: "抖音",
  [Platform.PlatformXiaohongshu]: "小红书",
};

export const enrollmentLabels: Record<EnrollmentStatus, string> = {
  [EnrollmentStatus.EnrollmentNone]: "尚未报名",
  [EnrollmentStatus.EnrollmentPending]: "待审核",
  [EnrollmentStatus.EnrollmentApproved]: "已通过",
  [EnrollmentStatus.EnrollmentRejected]: "已驳回",
};

export const assignmentLabels: Record<AssignmentStatus, string> = {
  [AssignmentStatus.AssignmentInProgress]: "进行中",
  [AssignmentStatus.AssignmentPendingReview]: "待审核",
  [AssignmentStatus.AssignmentRejected]: "需修改",
  [AssignmentStatus.AssignmentPendingSettlement]: "待结算",
  [AssignmentStatus.AssignmentSettled]: "已结算",
  [AssignmentStatus.AssignmentRevoked]: "已撤回",
  [AssignmentStatus.AssignmentExpired]: "已过期",
  [AssignmentStatus.AssignmentNotPayable]: "不予结算",
};

export const sourceLabels: Record<AssignmentSource, string> = {
  [AssignmentSource.SourceClaim]: "自行领取",
  [AssignmentSource.SourceAssigned]: "工作人员指派",
  [AssignmentSource.SourceReissued]: "重新派发",
};

export const submissionLabels: Record<SubmissionStatus, string> = {
  [SubmissionStatus.SubmissionPending]: "待审核",
  [SubmissionStatus.SubmissionApproved]: "已通过",
  [SubmissionStatus.SubmissionRejected]: "已驳回",
  [SubmissionStatus.SubmissionInvalid]: "不予结算",
  [SubmissionStatus.SubmissionWaived]: "素材问题，照常结算",
};

export const withdrawalLabels: Record<WithdrawalStatus, string> = {
  [WithdrawalStatus.WithdrawalQueued]: "等待处理",
  [WithdrawalStatus.WithdrawalSubmitting]: "正在发起转账",
  [WithdrawalStatus.WithdrawalUnknown]: "转账结果待确认",
  [WithdrawalStatus.WithdrawalWaitingConfirmation]: "待用户确认收款",
  [WithdrawalStatus.WithdrawalSuccess]: "已到账",
  [WithdrawalStatus.WithdrawalFailed]: "失败，已退回余额",
  [WithdrawalStatus.WithdrawalCancelled]: "已关闭，已退回余额",
  [WithdrawalStatus.WithdrawalBlockedConfig]: "等待开通转账通道",
};
