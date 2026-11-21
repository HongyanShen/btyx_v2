const {
  identityAgeError,
  identityPhotoEvents,
} = require("../../../services/identity");
const { profilePhotoURL } = require("../../../services/profile");
const { request, post, tokenKey } = require("../../../services/request");
const {
  hasSession,
  isCurrentSession,
  openLogin,
  pendingSession,
} = require("../../../services/session");

const contactPhonePattern = /^\+?[0-9]{8,15}$/;

Page({
  data: {
    loading: true,
    signedIn: false,
    uploading: false,
    submitting: false,
    verifyingPhone: false,
    user: null,
    document: {},
    savedPhotos: {},
    verificationResult: null,
    editingIdentity: false,
    wechat: "",
    wechatEdited: false,
    phone: "",
    phoneEdited: false,
    phoneReceipt: "",
    ageError: "",
    error: "",
  },

  onShow() {
    const preview = this.photoPreview;
    this.photoPreview = null;
    if (preview && isCurrentSession(preview.token)) return;

    return this.load();
  },

  async load() {
    if (
      this.refreshing ||
      this.data.uploading ||
      this.data.submitting ||
      this.data.verifyingPhone
    ) {
      return;
    }

    this.refreshing = true;
    try {
      await pendingSession();
      const token = wx.getStorageSync(tokenKey);
      if (this.profileToken && !isCurrentSession(this.profileToken)) {
        this.setData({
          user: null,
          document: {},
          savedPhotos: {},
          verificationResult: null,
          editingIdentity: false,
          wechat: "",
          wechatEdited: false,
          phone: "",
          phoneEdited: false,
          phoneReceipt: "",
          ageError: "",
        });
      }

      const user = await request("/me");
      if (!isCurrentSession(token)) return;

      this.profileToken = token;
      const initialize = !this.data.user || this.data.user.id !== user.id;
      const previousDocument = this.data.user?.identity_document;
      const savedDocument = user.identity_document;
      const resetIdentity =
        initialize ||
        user.blocked ||
        (this.data.user.identity_verified && !user.identity_verified);
      const photosChanged =
        resetIdentity ||
        savedDocument?.front_file_id !== previousDocument?.front_file_id ||
        savedDocument?.back_file_id !== previousDocument?.back_file_id;
      this.setData({
        user,
        ...(photosChanged ? { savedPhotos: {} } : {}),
        signedIn: true,
        error: "",
        ...(resetIdentity
          ? {
              document: {},
              ageError: "",
              editingIdentity: false,
              verificationResult: null,
              wechat: user.identity_verified ? user.wechat || "" : "",
              wechatEdited: false,
              phone: "",
              phoneEdited: false,
              phoneReceipt: "",
            }
          : {}),
      });
      if (
        photosChanged ||
        (savedDocument?.front_file_id && !this.data.savedPhotos.front) ||
        (savedDocument?.back_file_id && !this.data.savedPhotos.back)
      ) {
        await this.refreshSavedPhotos();
      }
    } catch (error) {
      const signedIn = hasSession();
      this.setData({
        error: error.message,
        signedIn,
        ...(signedIn
          ? {}
          : {
              user: null,
              document: {},
              savedPhotos: {},
              ageError: "",
              editingIdentity: false,
              verificationResult: null,
              wechat: "",
              wechatEdited: false,
              phone: "",
              phoneEdited: false,
              phoneReceipt: "",
            }),
      });
    } finally {
      this.refreshing = false;
      this.setData({ loading: false });
    }
  },

  openLogin,
  ...identityPhotoEvents,

  async refreshSavedPhotos() {
    const { user } = this.data;
    const saved = user?.identity_document;
    const token = this.profileToken;
    if (!user?.identity_verified || !saved || !isCurrentSession(token)) {
      return;
    }

    const [front, back] = await Promise.all([
      profilePhotoURL(saved.front_file_id),
      profilePhotoURL(saved.back_file_id),
    ]);
    const current = this.data.user;
    if (
      isCurrentSession(token) &&
      current?.id === user.id &&
      current.identity_document?.front_file_id === saved.front_file_id &&
      current.identity_document?.back_file_id === saved.back_file_id
    ) {
      this.setData({ savedPhotos: { front, back } });
    }
  },

  savedPhotoFailed(event) {
    const { side } = event.currentTarget.dataset;
    this.setData({ [`savedPhotos.${side}`]: "" });
  },

  previewSavedPhoto(event) {
    if (!isCurrentSession(this.profileToken)) return;

    const { front, back } = this.data.savedPhotos;
    const current = event.currentTarget.dataset.side === "front" ? front : back;
    if (current) {
      const preview = { token: this.profileToken };
      this.photoPreview = preview;
      wx.previewImage({
        current,
        urls: [front, back].filter(Boolean),
        fail: ({ errMsg } = {}) => {
          if (!errMsg?.includes("cancel") && this.photoPreview === preview) {
            this.photoPreview = null;
          }
        },
      });
    }
  },

  changeName(event) {
    this.setData({ "document.name": event.detail.value, error: "" });
  },

  changeIdentity(event) {
    const identity = event.detail.value.toUpperCase();
    this.setData({
      "document.identity": identity,
      ageError: identity.length === 18 ? identityAgeError(identity) : "",
      error: "",
    });
  },

  canEditContacts() {
    const { user, verificationResult, editingIdentity } = this.data;
    return (
      !!user &&
      !user.blocked &&
      !verificationResult &&
      (editingIdentity || !user.identity_verified)
    );
  },

  changeWechat(event) {
    if (!this.canEditContacts()) return;

    this.setData({ wechat: event.detail.value, wechatEdited: true, error: "" });
  },

  changePhone(event) {
    if (!this.canEditContacts()) return;

    this.setData({
      phone: event.detail.value,
      phoneEdited: true,
      phoneReceipt: "",
      error: "",
    });
  },

  async getPhone(event) {
    if (
      !this.canEditContacts() ||
      this.data.verifyingPhone ||
      this.data.submitting
    ) {
      return;
    }

    const { code } = event.detail;
    if (!code) return;

    this.setData({ verifyingPhone: true, error: "" });
    try {
      const result = await post("/me/phone", { code, use_for_contact: true });
      this.setData({
        "user.verified_phone": result.verified_phone,
        "user.contact_phone": result.verified_phone,
        phone: "",
        phoneEdited: false,
        phoneReceipt: result.contact_phone_receipt || "",
      });
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ verifyingPhone: false });
    }
  },

  async submit() {
    const {
      user,
      document,
      editingIdentity,
      verificationResult,
      submitting,
      uploading,
      verifyingPhone,
    } = this.data;
    if (
      !user ||
      user.blocked ||
      verificationResult ||
      (user.identity_verified && !editingIdentity) ||
      submitting ||
      uploading ||
      verifyingPhone
    ) {
      return;
    }

    if (!document.front_file_id || !document.back_file_id) {
      this.setData({ error: "请上传身份证人像面和国徽面" });
      return;
    }

    const name = (document.name || "").trim();
    if (!name) {
      this.setData({ error: "请填写与身份证一致的真实姓名" });
      return;
    }

    const identity = (document.identity || "").trim();
    const saved = user.identity_document;
    const keepNumber =
      !identity &&
      user.identity_verified &&
      saved?.number &&
      saved.front_file_id === document.front_file_id;
    const ageError = keepNumber ? "" : identityAgeError(identity);
    if (ageError) {
      this.setData({ ageError });
      return;
    }

    const phone = this.data.phone.trim();
    if (this.data.phoneEdited && phone && !contactPhonePattern.test(phone)) {
      this.setData({ error: "请输入有效的联系手机号" });
      return;
    }

    const { wechatEdited, phoneEdited, phoneReceipt } = this.data;
    const wechat = wechatEdited ? this.data.wechat.trim() : user.wechat || "";
    this.setData({ submitting: true, error: "" });
    try {
      await post("/me/identity", {
        name,
        front_file_id: document.front_file_id,
        back_file_id: document.back_file_id,
        ...(identity ? { identity } : {}),
        ...(phoneEdited ? { contact_phone: phone } : {}),
        ...(phoneReceipt ? { contact_phone_receipt: phoneReceipt } : {}),
        ...(wechatEdited ? { wechat } : {}),
        ...(user.identity_verified ? { correct_identity: true } : {}),
      });
      this.setData({
        user: { ...user, identity_verified: true, name, wechat },
        wechat,
        wechatEdited: false,
        phone: "",
        phoneEdited: false,
        phoneReceipt: "",
        document: {},
        savedPhotos: {},
        ageError: "",
        verificationResult: { status: "success" },
        editingIdentity: false,
      });
    } catch (error) {
      this.setData({
        verificationResult: { status: "failed", message: error.message },
      });
    } finally {
      this.setData({ submitting: false });
    }

    if (this.data.verificationResult?.status === "success") {
      await this.load();
    }
  },

  correctIdentity() {
    if (this.data.submitting || this.data.verifyingPhone) return;

    const { user, editingIdentity } = this.data;
    const saved = user?.identity_document;
    this.setData({
      ...(user?.identity_verified && !editingIdentity
        ? {
            document: {
              name: user.name || "",
              front_file_id: saved?.front_file_id || "",
              back_file_id: saved?.back_file_id || "",
            },
            ageError: "",
          }
        : {}),
      editingIdentity: true,
      verificationResult: null,
      error: "",
    });
  },

  async saveContacts() {
    const { user, submitting, verifyingPhone, phoneEdited, wechatEdited } =
      this.data;
    if (
      !this.canEditContacts() ||
      !user.identity_verified ||
      submitting ||
      verifyingPhone
    )
      return;

    const phone = this.data.phone.trim();
    if (phoneEdited && phone && !contactPhonePattern.test(phone)) {
      this.setData({ error: "请输入有效的联系手机号" });
      return;
    }

    const wechat = wechatEdited ? this.data.wechat.trim() : user.wechat || "";
    this.setData({ submitting: true, error: "" });
    try {
      await post("/me/identity", {
        ...(wechatEdited ? { wechat } : {}),
        ...(phoneEdited ? { contact_phone: phone } : {}),
      });
      this.setData({
        "user.wechat": wechat,
        wechat,
        wechatEdited: false,
        phone: "",
        phoneEdited: false,
      });
      wx.showToast({ title: "已保存", icon: "success" });
    } catch (error) {
      this.setData({ error: error.message });
      return;
    } finally {
      this.setData({ submitting: false });
    }

    if (phoneEdited) await this.load();
  },
});
