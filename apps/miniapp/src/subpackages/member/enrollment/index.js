const { getCurrentAgreement } = require("../../../services/agreement");
const config = require("../../../services/config");
const { EnrollmentStatus } = require("../../../services/generated/enums");
const {
  identityAgeError,
  identityPhotoEvents,
} = require("../../../services/identity");
const { enrollmentLabels } = require("../../../services/labels");
const { formatFen } = require("../../../services/money");
const { request, post } = require("../../../services/request");
const { hasSession, openLogin } = require("../../../services/session");

const xiaohongshuNumberPattern = /^[^\s:/\\<>?#：／\x00-\x1f\x7f-\x9f]+$/;

const { subscribe } = require("../../../services/subscriptions");

Page({
  data: {
    loading: true,
    signedIn: false,
    error: "",
    ageError: "",
    uploading: false,
    submitting: false,
    editable: false,
    rejected: false,
    statusLabel: "",
    user: null,
    douyin: "",
    xhs: "",
    needsXiaohongshuNumber: false,
    agreementOpen: false,
    agreementLoading: false,
    agreement: null,
    agreementError: "",
    document: {},
    referencePrice: formatFen(config.marketingReferencePriceFen),
  },

  onShow() {
    this.load();
  },

  async load() {
    if (
      this.refreshing ||
      this.data.uploading ||
      this.data.submitting ||
      this.data.agreementLoading ||
      this.data.agreementOpen
    )
      return;

    this.refreshing = true;
    try {
      const user = await request("/me");
      const rejected = user.status === EnrollmentStatus.EnrollmentRejected;
      const initialize =
        !this.data.user ||
        this.data.user.id !== user.id ||
        this.data.user.enrollment_id !== user.enrollment_id ||
        this.data.user.status !== user.status;
      const previousDocument = rejected && user.enrollment_document;
      const savedXhs = user.xhs?.trim() || "";
      const needsXiaohongshuNumber =
        !!savedXhs && !xiaohongshuNumberPattern.test(savedXhs);
      this.setData({
        user,
        signedIn: true,
        error: "",
        statusLabel: enrollmentLabels[user.status],
        rejected,
        editable:
          !user.blocked &&
          (user.status === EnrollmentStatus.EnrollmentNone || rejected),
        ...(initialize
          ? {
              douyin: user.douyin || "",
              xhs: needsXiaohongshuNumber ? "" : savedXhs,
              needsXiaohongshuNumber,
              ageError: "",
              document: previousDocument
                ? {
                    front_file_id: previousDocument.front_file_id,
                    back_file_id: previousDocument.back_file_id,
                  }
                : {},
            }
          : {}),
        ...(user.identity_verified || user.blocked
          ? { ageError: "", document: {} }
          : {}),
      });
    } catch (error) {
      const signedIn = hasSession();
      this.setData({
        error: error.message,
        signedIn,
        ...(signedIn ? {} : { user: null, document: {}, ageError: "" }),
      });
    } finally {
      this.refreshing = false;
      this.setData({ loading: false });
    }
  },

  openLogin,
  ...identityPhotoEvents,

  change(event) {
    this.setData({
      [event.currentTarget.dataset.field]: event.detail.value,
      error: "",
    });
  },

  async submit() {
    const {
      editable,
      submitting,
      uploading,
      agreementOpen,
      agreementLoading,
      user,
      document,
    } = this.data;
    if (
      !editable ||
      submitting ||
      uploading ||
      agreementOpen ||
      agreementLoading ||
      this.refreshing
    )
      return;

    const douyin = this.data.douyin.trim();
    const xhs = this.data.xhs.trim();
    if (!douyin || !xhs) {
      this.setData({ error: "请填写抖音号和小红书号" });
      return;
    }

    if (!xiaohongshuNumberPattern.test(xhs)) {
      this.setData({ error: "请填写小红书号，可在小红书「我」页面查看" });
      return;
    }

    if (!user.identity_verified) {
      if (!document.front_file_id || !document.back_file_id) {
        this.setData({ error: "请上传身份证人像面和国徽面" });
        return;
      }

      const reusingFront =
        this.data.rejected &&
        document.front_file_id === user.enrollment_document?.front_file_id;
      if (!reusingFront && (!document.name || !document.identity)) {
        this.setData({ error: "请先识别身份证人像面，可点击重试识别" });
        return;
      }

      const ageError = document.identity
        ? identityAgeError(document.identity)
        : "";
      if (ageError) {
        this.setData({ ageError });
        return;
      }
    }

    this.pendingEnrollment = {
      douyin,
      xhs,
      ...(this.data.rejected
        ? { previous_enrollment_id: user.enrollment_id }
        : {}),
      ...(user.identity_verified ? {} : document),
    };
    this.setData({
      agreement: null,
      agreementError: "",
      error: "",
      ageError: "",
    });
    await this.loadEnrollmentAgreement();
  },

  async loadEnrollmentAgreement() {
    const enrollment = this.pendingEnrollment;
    if (!enrollment || this.data.agreementLoading) return;

    this.setData({
      agreementLoading: true,
      agreement: null,
      agreementError: "",
    });
    try {
      const agreement = await getCurrentAgreement();
      if (this.pendingEnrollment !== enrollment) return;

      if (!agreement) {
        this.pendingEnrollment = null;
        this.setData({
          agreementOpen: false,
          agreementLoading: false,
          error: "暂无协议",
        });
        return;
      }

      this.setData({ agreement, agreementOpen: true });
    } catch (error) {
      if (this.pendingEnrollment === enrollment) {
        this.setData({ agreementError: error.message, agreementOpen: true });
      }
    } finally {
      if (this.pendingEnrollment === enrollment) {
        this.setData({ agreementLoading: false });
      }
    }
  },

  dismissAgreement() {
    if (this.data.submitting) return;

    this.pendingEnrollment = null;
    this.setData({
      agreementOpen: false,
      agreementLoading: false,
      agreement: null,
      agreementError: "",
    });
  },

  async confirmEnrollment() {
    const { agreementOpen, agreement, agreementLoading, submitting, user } =
      this.data;
    if (!agreementOpen || !agreement?.id || agreementLoading || submitting)
      return;

    this.setData({ submitting: true, agreementError: "" });
    try {
      if (wx.requestSubscribeMessage) await subscribe("enrollment");
      await post("/enrollment", {
        ...this.pendingEnrollment,
        agreement_id: agreement.id,
      });
      this.pendingEnrollment = null;
      this.setData({
        user: {
          ...user,
          status: EnrollmentStatus.EnrollmentPending,
          reason: "",
          consented: true,
        },
        agreementOpen: false,
        agreement: null,
        document: {},
        editable: false,
        rejected: false,
        statusLabel: enrollmentLabels[EnrollmentStatus.EnrollmentPending],
      });
      wx.showToast({ title: "报名已提交", icon: "success" });
    } catch (error) {
      if (error.status === 409) {
        await this.loadEnrollmentAgreement();
        if (!this.data.agreement) return;

        if (this.data.agreement.id !== agreement.id) {
          this.setData({
            agreementError: "协议已更新，请重新阅读并确认",
          });
          return;
        }
      }

      this.setData({ agreementError: error.message });
    } finally {
      this.setData({ submitting: false });
    }
  },
});
