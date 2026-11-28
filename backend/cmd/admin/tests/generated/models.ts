// Code generated from Go DTOs; DO NOT EDIT.

import type * as Enums from './enums';

export interface Agreement {
  "id": string;
  "title": string;
  "body": string;
  "body_html"?: string;
  "creator_name": string;
  "creator_id": string;
  "active": boolean;
  "created_at": string;
}

export interface Assignment {
  "task_code"?: string;
  "id": string;
  "user_id": string;
  "task_id": string;
  "version_id": string;
  "price_fen": string;
  "title": string;
  "body": string;
  "files": string[];
  "platforms": Enums.Platform[];
  "status": Enums.AssignmentStatus;
  "source": Enums.AssignmentSource;
  "source_id": string;
  "date": string;
  "due_date": string;
  "expires_at"?: string | null;
  "playback_deadline"?: string | null;
  "playback_settlement_at"?: string | null;
  "settlement_frequency"?: Enums.SettlementFrequency;
  "steps": Step[];
  "submissions": Submission[];
}

export interface AuditEvent {
  "http_status"?: number;
  "platform"?: Enums.AuditPlatform;
  "actor_name": string;
  "id": string;
  "actor_id": string;
  "actor_username": string;
  "action": Enums.AuditAction;
  "reference_id": string;
  "reason": string;
  "created_at": string;
}

export interface Content {
  "id": string;
  "creator_id": string;
  "created_at": string;
  "title": string;
  "body": string;
  "project": string;
  "files": string[];
  "tags": string[];
}

export interface CreativeCatalog {
  "tasks": CreativeTaskDTO[];
  "mine": CreativeWorkDTO[];
  "online": boolean;
  "limit": number;
  "used": number;
}

export interface CreativeEarning {
  "likes_reward": boolean;
  "amount_fen": string;
  "eligible_at": string;
  "batch_id": string;
}

export interface CreativeLink {
  "reason"?: string;
  "platform": Enums.Platform;
  "url": string;
  "screenshot": string;
  "likes": number | null;
}

export interface CreativeLinksInput {
  "links": CreativeLink[];
}

export interface CreativeReviewInput {
  "revision": number;
  "approve": boolean;
  "reason": string;
}

export interface CreativeSubmitInput {
  "revision": number;
  "copy_text": string;
  "files": string[];
}

export interface CreativeTaskDTO {
  "title": string;
  "requirements": string;
  "copy_text": string;
  "topics": string;
  "files": string[];
  "platforms": Enums.Platform[];
  "media_type": Enums.FileMediaType;
  "creation_fen": string;
  "combined_likes": boolean;
  "publish_days": number;
  "starts_at": string;
  "ends_at": string;
  "reward_at": string;
  "tiers": CreativeTier[];
  "task_code": string;
  "id": string;
  "online": boolean;
  "publish_fen": string;
}

export interface CreativeTaskInput {
  "title": string;
  "requirements": string;
  "copy_text": string;
  "topics": string;
  "files": string[];
  "platforms": Enums.Platform[];
  "media_type": Enums.FileMediaType;
  "creation_fen": string;
  "combined_likes": boolean;
  "publish_days": number;
  "starts_at": string;
  "ends_at": string;
  "reward_at": string;
  "tiers": CreativeTier[];
}

export interface CreativeTier {
  "likes": number;
  "reward_fen": string;
}

export interface CreativeWorkDTO {
  "id": string;
  "user_id": string;
  "task": CreativeTaskDTO;
  "status": Enums.CreativeWorkStatus;
  "revision": number;
  "copy_text": string;
  "files": string[];
  "reason": string;
  "links": CreativeLink[];
  "earnings": CreativeEarning[];
  "business_date": string;
}

export interface DistributionSettings {
  "expiry_mode": Enums.MarketingExpiryMode;
  "expiry_minutes": number;
  "schedule": MarketingSchedule;
}

export interface DistributorBlock {
  "reason": string;
  "at": string;
  "operator"?: string;
}

export interface DistributorDetails {
  "phone": string;
  "identity": string;
  "updated_at": string;
  "today_tasks": DistributorTask[];
}

export interface DistributorTask {
  "id": string;
  "title": string;
  "status": Enums.AssignmentStatus;
  "source": Enums.AssignmentSource;
  "unpublished": boolean;
}

export interface EnrollmentDocuments {
  "details": IdentityRecordDetails | null;
  "photos_unavailable": boolean;
}

export interface EnrollmentPhotoPreview {
  "front_url": string;
  "back_url": string;
  "unavailable": boolean;
}

export interface Feedback {
  "id": string;
  "user_id": string;
  "body": string;
  "reply": string;
  "status": Enums.FeedbackStatus;
  "created_at": string;
}

export interface FileDownload {
  "storage": Enums.FileStorage;
  "name": string;
  "media_type": Enums.FileMediaType;
  "url": string;
  "expires_at": string | null;
}

export interface FileInitiateInput {
  "name": string;
  "media_type": Enums.FileMediaType;
  "size": number;
}

export interface FileObject {
  "id": string;
  "name": string;
  "media_type": Enums.FileMediaType;
  "size": number;
}

export interface FileUploadFields {
  "key": string;
  "policy": string;
  "x-tos-algorithm": string;
  "x-tos-credential": string;
  "x-tos-date": string;
  "x-tos-signature": string;
  "acl": string;
  "Content-Type": string;
  "success_action_status": string;
  "x-tos-forbid-overwrite": string;
}

export interface FileUploadTicket {
  "id": string;
  "url": string;
  "fields": FileUploadFields;
  "expires_at": string | null;
  "completed": boolean;
}

export interface IdentityRecord {
  "id": string;
  "user_id": string;
  "nickname": string;
  "action": Enums.AuditAction;
  "reason": string;
  "created_at": string;
  "details": IdentityRecordDetails | null;
}

export interface IdentityRecordDetails {
  "name": string;
  "identity": string;
  "phone": string;
  "wechat": string;
  "front_url": string;
  "back_url": string;
}

export interface IdentityRecords {
  "records": IdentityRecord[];
  "next_before": string;
  "photos_unavailable": boolean;
}

export interface LedgerEntry {
  "id": string;
  "user_id": string;
  "kind": Enums.LedgerKind;
  "amount_fen": string;
  "reference_id": string;
  "original_id": string;
  "reason": string;
  "created_at": string;
}

export interface LoginResult {
  "token": string;
  "user_id": string;
  "role": Enums.Role;
}

export interface MarketingPriceChange {
  "before_fen": string;
  "after_fen": string;
  "changed_at": string;
  "operator": string;
}

export interface MarketingSchedule {
  "settlement_frequency": Enums.SettlementFrequency;
  "settlement_weekday": number;
  "settlement_month_day": number;
  "settlement_clock": string;
  "settlement_delay_minutes": number;
  "playback_cutoff_clock": string;
  "playback_cutoff_minutes": number;
  "playback_deferral_periods": number;
}

export interface MarketingSettings {
  "daily_limit": number;
  "creative_limit": number;
  "creative_online": boolean;
  "price_fen": string;
  "expiry_mode": Enums.MarketingExpiryMode;
  "expiry_minutes": number;
  "online": boolean;
  "price_changes": MarketingPriceChange[];
}

export interface MarketingSettingsInput {
  "daily_limit"?: number | null;
  "creative_limit"?: number | null;
  "creative_online"?: boolean | null;
  "price_fen"?: string | null;
  "expiry_mode"?: Enums.MarketingExpiryMode | null;
  "expiry_minutes"?: number | null;
  "online"?: boolean | null;
}

export interface MarketingTag {
  "id": string;
  "key": string;
  "name": string;
  "updated_at": string;
}

export interface Notification {
  "id": string;
  "title": string;
  "body": string;
  "reference_id": string;
  "delivery_state": Enums.DeliveryState;
  "delivery_reason": string;
  "read": boolean;
  "created_at": string;
}

export interface Operator {
  "id": string;
  "username": string;
  "super_admin": boolean;
  "disabled": boolean;
  "all_pages": boolean;
  "all_endpoints": boolean;
  "pages": string[];
  "endpoints": string[];
}

export interface OperatorEndpoint {
  "key": string;
  "label": string;
}

export interface OperatorPage {
  "path": string;
  "label": string;
}

export interface OperatorPermissionCatalog {
  "pages": OperatorPage[];
  "endpoints": OperatorEndpoint[];
}

export interface PoolAllocation {
  "id": string;
  "user_id": string;
  "source": Enums.AssignmentSource;
  "has_link": boolean;
  "steps"?: Enums.TaskStep[];
  "links_done": boolean;
  "reviewed": boolean;
  "settled": boolean;
  "expires_at"?: string | null;
}

export interface PoolExpiry {
  "user_id": string;
  "source": Enums.AssignmentSource;
  "expired_at"?: string | null;
}

export interface SettlementBatch {
  "id": string;
  "through": string;
  "amount_fen": string;
  "count": number;
  "creator_id": string;
}

export interface SettlementLine {
  "creative"?: boolean;
  "submission_id": string;
  "assignment_id": string;
  "user_id": string;
  "platform": Enums.Platform;
  "amount_fen": string;
  "eligible_date": string;
}

export interface SettlementMonth {
  "today": string;
  "withdrawn_fen": string;
  "withdrawals": number;
  "weeks": SettlementWeek[];
  "distributors": User[];
}

export interface SettlementPreview {
  "through": string;
  "amount_fen": string;
  "count": number;
  "lines": SettlementLine[];
}

export interface SettlementWeek {
  "through": string;
  "start": string;
  "end": string;
  "review_token": string;
  "weekly": boolean;
  "due": boolean;
  "lines": SettlementWeekLine[];
}

export interface SettlementWeekLine {
  "creative"?: boolean;
  "id": string;
  "assignment_id": string;
  "user_id": string;
  "project": string;
  "title": string;
  "published_on": string;
  "task_date": string;
  "platform": Enums.Platform;
  "url": string;
  "screenshot": string;
  "has_screenshot": boolean;
  "status": Enums.SubmissionStatus;
  "visibility": Enums.Visibility;
  "amount_fen": string;
  "due": boolean;
  "eligible_at"?: string | null;
  "credited_at": string | null;
  "credited_by": string;
}

export interface Step {
  "key": Enums.TaskStep;
  "label": string;
  "done": boolean;
}

export interface Submission {
  "id": string;
  "platform": Enums.Platform;
  "url": string;
  "screenshot": string;
  "status": Enums.SubmissionStatus;
  "reason": string;
  "revision": number;
  "settled": boolean;
  "visibility": Enums.Visibility;
  "published_on"?: string;
  "submitted_at"?: string | null;
  "settlement_eligible_at"?: string | null;
}

export interface SubscriptionChoice {
  "template_id": string;
  "result": Enums.SubscriptionResult;
  "remembered": string;
}

export interface SubscriptionChoiceBatch {
  "attempt": string;
  "scene": string;
  "observed_at": string;
  "main_switch": boolean | null;
  "error_code": number;
  "choices": SubscriptionChoice[];
}

export interface TaskInput {
  "title": string;
  "body": string;
  "project": string;
  "content_id": string;
  "files": string[];
  "price_fen": string;
  "platforms": Enums.Platform[];
  "tags": string[];
  "deadline": string;
}

export interface TaskVersion {
  "task_code": string;
  "id": string;
  "task_id": string;
  "number": number;
  "creator_id": string;
  "created_at": string;
  "uploaded_at"?: string | null;
  "media_types"?: Enums.FileMediaType[];
  "online": boolean;
  "allocation"?: PoolAllocation | null;
  "last_expiry"?: PoolExpiry | null;
  "next_in_queue"?: boolean;
  "title": string;
  "body": string;
  "project": string;
  "content_id": string;
  "files": string[];
  "price_fen": string;
  "platforms": Enums.Platform[];
  "tags": string[];
  "deadline": string;
}

export interface User {
  "id": string;
  "name": string;
  "phone": string;
  "nickname": string;
  "avatar_file_id": string;
  "verified_phone": string;
  "contact_phone": string;
  "wechat": string;
  "identity_verified": boolean;
  "identity_document": VerifiedIdentity | null;
  "identity_resubmission_required": boolean;
  "enrollment_id"?: string;
  "enrollment_document"?: VerifiedIdentity | null;
  "enrollment_updated_at": string | null;
  "enrollment_submitted_at"?: string | null;
  "enrollment_has_photos"?: boolean;
  "enrollment_photo_previews"?: EnrollmentPhotoPreview | null;
  "enrollment_recent_post"?: boolean | null;
  "enrollment_new_account"?: boolean | null;
  "enrollment_age_group"?: Enums.EnrollmentAge | null;
  "douyin": string;
  "xhs": string;
  "status": Enums.EnrollmentStatus;
  "reason": string;
  "tags": string[];
  "blocked": boolean;
  "block_reason"?: string;
  "block_info"?: DistributorBlock | null;
  "distributor_details"?: DistributorDetails | null;
  "consented": boolean;
}

export interface VerifiedIdentity {
  "number": string;
  "front_file_id": string;
  "back_file_id": string;
}

export interface Wallet {
  "available_fen": string;
  "frozen_fen": string;
  "withdrawn_fen": string;
  "earned_fen": string;
  "month_earned_fen": string;
  "pending_fen": string;
  "min_withdrawal_fen": string;
  "max_withdrawal_fen": string;
}

export interface Withdrawal {
  "id": string;
  "user_id": string;
  "amount_fen": string;
  "status": Enums.WithdrawalStatus;
  "reason": string;
  "package_info": string;
  "merchant_id": string;
  "created_at": string;
}

