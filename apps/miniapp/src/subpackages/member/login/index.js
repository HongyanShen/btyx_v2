const {
  hasSession,
  login,
  pendingSession,
} = require("../../../services/session");

Page({
  data: { submitting: false, error: "" },

  async onLoad() {
    await pendingSession();
    if (!this.data.submitting && hasSession()) this.finish();
  },

  async submit() {
    if (this.data.submitting) return;

    this.setData({ submitting: true, error: "" });
    try {
      await pendingSession();
      await login();
      this.finish();
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ submitting: false });
    }
  },

  finish() {
    if (getCurrentPages().length > 1) {
      wx.navigateBack();
    } else {
      wx.switchTab({ url: "/pages/me/index" });
    }
  },
});
