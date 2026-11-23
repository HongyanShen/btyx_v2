// Code generated from Go DTOs; DO NOT EDIT.

import type * as Enums from "./enums";

export interface Agreement {
  id: string;
  title: string;
  body: string;
  body_html?: string;
  creator_id: string;
  active: boolean;
  created_at: string;
}

export interface Assignment {
  id: string;
  user_id: string;
  task_id: string;
  version_id: string;
  price_fen: string;
  title: string;
  body: string;
  files: string[];
  platforms: Enums.Platform[];
  status: Enums.AssignmentStatus;
  source: Enums.AssignmentSource;
  source_id: string;
  date: string;
  due_date: string;
  expires_at?: string | null;
  steps: Step[];
  submissions: Submission[];
}

export interface AuditEvent {
  id: string;
  actor_id: string;
  actor_username: string;
  action: Enums.AuditAction;
  reference_id: string;
  reason: string;
  created_at: string;
}

export interface Content {
  id: string;
  creator_id: string;
  created_at: string;
  title: string;
  body: string;
  project: string;
  files: string[];
  tags: string[];
}

export interface DistributorBlock {
  reason: string;
  at: string;
  operator?: string;
}

export interface DistributorDetails {
  phone: string;
  identity: string;
  updated_at: string;
  today_tasks: DistributorTask[];
}

export interface DistributorTask {
  id: string;
  title: string;
  status: Enums.AssignmentStatus;
  source: Enums.AssignmentSource;
  unpublished: boolean;
}

export interface EnrollmentDocuments {
  details: IdentityRecordDetails | null;
  photos_unavailable: boolean;
}

export interface EnrollmentPhotoPreview {
  front_url: string;
  back_url: string;
  unavailable: boolean;
}

export interface Feedback {
  id: string;
  user_id: string;
  body: string;
  reply: string;
  status: Enums.FeedbackStatus;
  created_at: string;
}

export interface FileDownload {
  storage: Enums.FileStorage;
  name: string;
  media_type: Enums.FileMediaType;
  url: string;
  expires_at: string | null;
}

export interface FileInitiateInput {
  name: string;
  media_type: Enums.FileMediaType;
  size: number;
}

export interface FileObject {
  id: string;
  name: string;
  media_type: Enums.FileMediaType;
  size: number;
}

export interface FileUploadFields {
  key: string;
  policy: string;
  "x-tos-algorithm": string;
  "x-tos-credential": string;
  "x-tos-date": string;
  "x-tos-signature": string;
  acl: string;
  "Content-Type": string;
  success_action_status: string;
  "x-tos-forbid-overwrite": string;
}

export interface FileUploadTicket {
  id: string;
  url: string;
  fields: FileUploadFields;
  expires_at: string | null;
  completed: boolean;
}

export interface IdentityRecord {
  id: string;
  user_id: string;
  nickname: string;
  action: Enums.AuditAction;
  reason: string;
  created_at: string;
  details: IdentityRecordDetails | null;
}

export interface IdentityRecordDetails {
  name: string;
  identity: string;
  phone: string;
  wechat: string;
  front_url: string;
  back_url: string;
}

export interface IdentityRecords {
  records: IdentityRecord[];
  next_before: string;
  photos_unavailable: boolean;
}

export interface LedgerEntry {
  id: string;
  user_id: string;
  kind: Enums.LedgerKind;
  amount_fen: string;
  reference_id: string;
  original_id: string;
  reason: string;
  created_at: string;
}

export interface LoginResult {
  token: string;
  user_id: string;
  role: Enums.Role;
}

export interface MarketingPriceChange {
  before_fen: string;
  after_fen: string;
  changed_at: string;
  operator: string;
}

export interface MarketingSettings {
  price_fen: string;
  expiry_mode: Enums.MarketingExpiryMode;
  expiry_minutes: number;
  online: boolean;
  price_changes: MarketingPriceChange[];
}

export interface MarketingSettingsInput {
  price_fen?: string | null;
  expiry_mode?: Enums.MarketingExpiryMode | null;
  expiry_minutes?: number | null;
  online?: boolean | null;
}

export interface MarketingTag {
  id: string;
  key: string;
  name: string;
  updated_at: string;
}

export interface Notification {
  id: string;
  title: string;
  body: string;
  reference_id: string;
  read: boolean;
  created_at: string;
}

export interface Operator {
  id: string;
  username: string;
  super_admin: boolean;
  disabled: boolean;
  pages: string[];
  endpoints: string[];
}

export interface OperatorEndpoint {
  key: string;
  label: string;
}

export interface OperatorPage {
  path: string;
  label: string;
}

export interface OperatorPermissionCatalog {
  pages: OperatorPage[];
  endpoints: OperatorEndpoint[];
}

export interface PoolAllocation {
  id: string;
  user_id: string;
  source: Enums.AssignmentSource;
  has_link: boolean;
  steps?: Enums.TaskStep[];
  links_done: boolean;
  reviewed: boolean;
  settled: boolean;
  expires_at?: string | null;
}

export interface PoolExpiry {
  user_id: string;
  source: Enums.AssignmentSource;
  expired_at?: string | null;
}

export interface SettlementBatch {
  id: string;
  through: string;
  amount_fen: string;
  count: number;
  creator_id: string;
}

export interface SettlementLine {
  submission_id: string;
  assignment_id: string;
  user_id: string;
  platform: Enums.Platform;
  amount_fen: string;
  eligible_date: string;
}

export interface SettlementPreview {
  through: string;
  amount_fen: string;
  count: number;
  lines: SettlementLine[];
}

export interface Step {
  key: Enums.TaskStep;
  label: string;
  done: boolean;
}

export interface Submission {
  id: string;
  platform: Enums.Platform;
  url: string;
  screenshot: string;
  status: Enums.SubmissionStatus;
  reason: string;
  revision: number;
  settled: boolean;
  visibility: Enums.Visibility;
  submitted_at?: string | null;
}

export interface TaskInput {
  title: string;
  body: string;
  project: string;
  content_id: string;
  files: string[];
  price_fen: string;
  platforms: Enums.Platform[];
  tags: string[];
  deadline: string;
}

export interface TaskVersion {
  id: string;
  task_id: string;
  number: number;
  creator_id: string;
  created_at: string;
  uploaded_at?: string | null;
  media_types?: Enums.FileMediaType[];
  online: boolean;
  allocation?: PoolAllocation | null;
  last_expiry?: PoolExpiry | null;
  next_in_queue?: boolean;
  title: string;
  body: string;
  project: string;
  content_id: string;
  files: string[];
  price_fen: string;
  platforms: Enums.Platform[];
  tags: string[];
  deadline: string;
}

export interface User {
  id: string;
  name: string;
  phone: string;
  nickname: string;
  avatar_file_id: string;
  verified_phone: string;
  contact_phone: string;
  wechat: string;
  identity_verified: boolean;
  identity_document: VerifiedIdentity | null;
  identity_resubmission_required: boolean;
  enrollment_id?: string;
  enrollment_document?: VerifiedIdentity | null;
  enrollment_updated_at: string | null;
  enrollment_submitted_at?: string | null;
  enrollment_has_photos?: boolean;
  enrollment_photo_previews?: EnrollmentPhotoPreview | null;
  enrollment_recent_post?: boolean | null;
  enrollment_new_account?: boolean | null;
  enrollment_age_group?: Enums.EnrollmentAge | null;
  douyin: string;
  xhs: string;
  status: Enums.EnrollmentStatus;
  reason: string;
  tags: string[];
  blocked: boolean;
  block_reason?: string;
  block_info?: DistributorBlock | null;
  distributor_details?: DistributorDetails | null;
  consented: boolean;
}

export interface VerifiedIdentity {
  number: string;
  front_file_id: string;
  back_file_id: string;
}

export interface Wallet {
  available_fen: string;
  frozen_fen: string;
  withdrawn_fen: string;
}

export interface Withdrawal {
  id: string;
  user_id: string;
  amount_fen: string;
  status: Enums.WithdrawalStatus;
  reason: string;
  package_info: string;
  merchant_id: string;
  created_at: string;
}
