package enum

// AuditAction defines the allowed wire values for this business concept.
type AuditAction string

const (
	AuditEnrollmentApply          AuditAction = "enrollment.apply"
	AuditEnrollmentApproved       AuditAction = "enrollment.approved"
	AuditEnrollmentRejected       AuditAction = "enrollment.rejected"
	AuditEnrollmentPortrait       AuditAction = "enrollment.portrait_updated"
	AuditMarketingTagCreate       AuditAction = "marketing_tag.create"
	AuditMarketingTagRename       AuditAction = "marketing_tag.rename"
	AuditMarketingTagDelete       AuditAction = "marketing_tag.delete"
	AuditUserBlock                AuditAction = "user.block"
	AuditUserProfileUpdate        AuditAction = "user.profile_update"
	AuditUserPhoneVerify          AuditAction = "user.phone_verify"
	AuditUserIdentityVerify       AuditAction = "user.identity_verify"
	AuditUserIdentityFailed       AuditAction = "user.identity_verify_failed"
	AuditEnrollmentIdentityVerify AuditAction = "enrollment.identity_verify"
	AuditEnrollmentIdentityFailed AuditAction = "enrollment.identity_verify_failed"
	AuditAgreementConsent         AuditAction = "agreement.consent"
	AuditAgreementPublish         AuditAction = "agreement.publish"
	AuditTaskCreate               AuditAction = "task.create"
	AuditTaskPublishVersion       AuditAction = "task.publish_version"
	AuditTaskVisibility           AuditAction = "task.visibility"
	AuditTaskClaim                AuditAction = "task.claim"
	AuditTaskAssign               AuditAction = "task.assigned"
	AuditTaskRevoke               AuditAction = "task.revoke"
	AuditTaskReissue              AuditAction = "task.reissue"
	AuditTaskExpire               AuditAction = "task.expire"
	AuditMarketingSettings        AuditAction = "marketing.settings"
	AuditSubmissionSubmit         AuditAction = "submission.submit"
	AuditSubmissionApproved       AuditAction = "submission.approved"
	AuditSubmissionRejected       AuditAction = "submission.rejected"
	AuditSubmissionWaive          AuditAction = "submission.waive"
	AuditSettlementCredit         AuditAction = "settlement.credit"
	AuditWithdrawalRequest        AuditAction = "withdrawal.request"
	AuditLedgerAdjust             AuditAction = "ledger.adjust"
	AuditContentCreate            AuditAction = "content.create"
	AuditContentTags              AuditAction = "content.tags_updated"
	AuditFeedbackReply            AuditAction = "feedback.reply"
	AuditOperatorCreate           AuditAction = "operator.create"
	AuditOperatorUpdate           AuditAction = "operator.update"
)

func (AuditAction) Values() []string {
	return []string{
		string(AuditEnrollmentApply),
		string(AuditEnrollmentApproved),
		string(AuditEnrollmentRejected),
		string(AuditEnrollmentPortrait),
		string(AuditMarketingTagCreate),
		string(AuditMarketingTagRename),
		string(AuditMarketingTagDelete),
		string(AuditUserBlock),
		string(AuditUserProfileUpdate),
		string(AuditUserPhoneVerify),
		string(AuditUserIdentityVerify),
		string(AuditUserIdentityFailed),
		string(AuditEnrollmentIdentityVerify),
		string(AuditEnrollmentIdentityFailed),
		string(AuditAgreementConsent),
		string(AuditAgreementPublish),
		string(AuditTaskCreate),
		string(AuditTaskPublishVersion),
		string(AuditTaskVisibility),
		string(AuditTaskClaim),
		string(AuditTaskAssign),
		string(AuditTaskRevoke),
		string(AuditTaskReissue),
		string(AuditTaskExpire),
		string(AuditMarketingSettings),
		string(AuditSubmissionSubmit),
		string(AuditSubmissionApproved),
		string(AuditSubmissionRejected),
		string(AuditSubmissionWaive),
		string(AuditSettlementCredit),
		string(AuditWithdrawalRequest),
		string(AuditLedgerAdjust),
		string(AuditContentCreate),
		string(AuditContentTags),
		string(AuditFeedbackReply),
		string(AuditOperatorCreate),
		string(AuditOperatorUpdate),
	}
}

func (value AuditAction) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *AuditAction) UnmarshalJSON(data []byte) error {
	return decode(data, value, (AuditAction("")).Values())
}
