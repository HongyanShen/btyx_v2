const { upload } = require("../../../services/file-upload");
const { profilePhotoURL } = require("../../../services/profile");
const { request, post } = require("../../../services/request");
const {
  hasSession,
  isCurrentSession,
  openLogin,
  pendingSession,
  tokenKey,
} = require("../../../services/session");

Page({
  data: {
    loading: true,
    signedIn: false,
    saving: false,
    verifying: false,
    error: "",
    user: null,
    avatar: "",
  },

  onShow() {
    if (!this.data.user) return this.load();
  },

  onUnload() {
    this.unloaded = true;
  },

  async load() {
    if (this.refreshing) return;
    this.refreshing = true;
    this.setData({ loading: true, error: "" });
    try {
      await pendingSession();
      if (this.unloaded) return;
      const token = wx.getStorageSync(tokenKey);
      if (!token) {
        this.setData({ signedIn: false, loading: false });
        return;
      }
      const user = await request("/me");
      if (this.unloaded || !isCurrentSession(token)) return;
      this.setData({ user, signedIn: true, loading: false });
      const avatar = await profilePhotoURL(user.avatar_file_id);
      if (!this.unloaded && !this.avatarPath && isCurrentSession(token)) {
        this.setData({ avatar });
      }
    } catch (error) {
      if (!this.unloaded) this.handleError(error);
    } finally {
      this.refreshing = false;
      if (!this.unloaded && this.data.loading) this.setData({ loading: false });
    }
  },

  openLogin,

  chooseAvatar(event) {
    const filePath = event.detail.avatarUrl;
    if (!filePath || this.data.saving) return;

    this.avatarPath = filePath;
    this.avatarID = "";
    this.setData({ avatar: filePath, error: "" });
  },

  async getPhone(event) {
    if (this.data.verifying || this.data.saving) return;

    const { code } = event.detail;
    if (!code) {
      this.setData({ error: "未获取手机号，可重新授权" });

      return;
    }

    this.setData({ verifying: true, error: "" });
    try {
      const result = await post("/me/phone", { code });
      this.setData({ "user.verified_phone": result.verified_phone });
    } catch (error) {
      this.handleError(error);
    } finally {
      this.setData({ verifying: false });
    }
  },

  async save(event) {
    if (this.data.saving || this.data.verifying || !this.data.user) return;

    const nickname = (event.detail.value.nickname || "").trim();
    if (!nickname) {
      this.setData({ error: "请输入有效的昵称" });

      return;
    }

    this.setData({ saving: true, error: "" });
    try {
      const changes = {};
      if (nickname !== this.data.user.nickname) changes.nickname = nickname;
      if (this.avatarPath) {
        this.avatarID = this.avatarID || (await upload(this.avatarPath));
        changes.avatar_file_id = this.avatarID;
      }

      if (Object.keys(changes).length) {
        await request("/me", { method: "PUT", data: changes });
      }

      wx.showToast({ title: "资料已保存", icon: "success" });
      wx.navigateBack();
    } catch (error) {
      this.handleError(error);
    } finally {
      this.setData({ saving: false });
    }
  },

  handleError(error) {
    const signedIn = hasSession();
    if (!signedIn) {
      this.avatarPath = "";
      this.avatarID = "";
      this.setData({ user: null, avatar: "" });
    }

    this.setData({ signedIn, error: error.message });
  },
});
