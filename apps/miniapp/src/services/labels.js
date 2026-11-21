const {
  AssignmentStatus,
  AssignmentSource,
  EnrollmentStatus,
  Platform,
  SubmissionStatus,
  WithdrawalStatus,
} = require("./generated/enums");

const platformLabels = {
  [Platform.PlatformDouyin]: "抖音",
  [Platform.PlatformXiaohongshu]: "小红书",
};

const assignmentLabels = {
  [AssignmentStatus.AssignmentInProgress]: "进行中",
  [AssignmentStatus.AssignmentPendingReview]: "待审核",
  [AssignmentStatus.AssignmentRejected]: "需修改",
  [AssignmentStatus.AssignmentPendingSettlement]: "待结算",
  [AssignmentStatus.AssignmentSettled]: "已结算",
  [AssignmentStatus.AssignmentRevoked]: "已撤回",
  [AssignmentStatus.AssignmentExpired]: "已失效",
  [AssignmentStatus.AssignmentNotPayable]: "不予结算",
};

const sourceLabels = {
  [AssignmentSource.SourceClaim]: "自行领取",
  [AssignmentSource.SourceAssigned]: "工作人员指派",
  [AssignmentSource.SourceReissued]: "重新派发",
};

const enrollmentLabels = {
  [EnrollmentStatus.EnrollmentNone]: "尚未报名",
  [EnrollmentStatus.EnrollmentPending]: "报名审核中",
  [EnrollmentStatus.EnrollmentApproved]: "报名已通过",
  [EnrollmentStatus.EnrollmentRejected]: "报名需修改",
};

const submissionLabels = {
  [SubmissionStatus.SubmissionPending]: "审核中",
  [SubmissionStatus.SubmissionApproved]: "审核通过",
  [SubmissionStatus.SubmissionRejected]: "需修改",
  [SubmissionStatus.SubmissionInvalid]: "不予结算",
  [SubmissionStatus.SubmissionWaived]: "照常结算",
};

const withdrawalLabels = {
  [WithdrawalStatus.WithdrawalQueued]: "等待处理",
  [WithdrawalStatus.WithdrawalSubmitting]: "正在转账",
  [WithdrawalStatus.WithdrawalUnknown]: "转账结果待确认",
  [WithdrawalStatus.WithdrawalWaitingConfirmation]: "待确认收款",
  [WithdrawalStatus.WithdrawalSuccess]: "已到账微信零钱",
  [WithdrawalStatus.WithdrawalFailed]: "失败，已退回余额",
  [WithdrawalStatus.WithdrawalCancelled]: "已关闭，已退回余额",
  [WithdrawalStatus.WithdrawalBlockedConfig]: "等待开通转账通道",
};

module.exports = {
  platformLabels,
  assignmentLabels,
  sourceLabels,
  enrollmentLabels,
  submissionLabels,
  withdrawalLabels,
};
