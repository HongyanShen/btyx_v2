const { request } = require("../../../services/request");
const {
  hasSession,
  openLogin,
  pendingSession,
} = require("../../../services/session");
const {
  presentWithdrawal,
  requestWithdrawalReceipt,
} = require("../../../services/withdrawals");

Page({
  data: { state: "loading", records: [], error: "", confirming: false },

  onShow() {
    this.visible = true;
    clearInterval(this.refreshTimer);
    this.refreshTimer = setInterval(() => {
      if (
        this.data.records.some(({ frozen }) => frozen) &&
        !this.data.confirming
      )
        this.load();
    }, 15000);
    return this.load();
  },

  onHide() {
    this.visible = false;
    clearInterval(this.refreshTimer);
  },

  onUnload() {
    this.onHide();
  },

  onPullDownRefresh() {
    return this.load().finally(() => wx.stopPullDownRefresh());
  },

  openLogin,

  load() {
    if (this.refreshing) return this.refreshing;
    this.refreshing = this.loadRecords().finally(() => {
      this.refreshing = null;
    });
    return this.refreshing;
  },

  async loadRecords() {
    if (this.data.state !== "ready")
      this.setData({ state: "loading", error: "" });
    try {
      await pendingSession();
      if (!hasSession()) {
        this.setData({ state: "guest", records: [], error: "" });
        return;
      }
      const entries = await request("/withdrawals");
      const records = entries.map(presentWithdrawal);
      this.setData({
        records,
        state: records.length ? "ready" : "empty",
        error: "",
      });
    } catch (error) {
      if (!hasSession()) {
        this.setData({ state: "guest", records: [], error: "" });
      } else {
        this.setData({
          state: this.data.records.length ? "ready" : "error",
          error: error.message,
        });
      }
    }
  },

  async confirmReceipt(event) {
    if (this.data.confirming || this.data.error) return;
    const record = this.data.records.find(
      ({ id }) => id === event.currentTarget.dataset.id,
    );
    if (!record?.waitingConfirmation) return;

    this.setData({ confirming: true });
    try {
      await requestWithdrawalReceipt(record);
    } catch {
      wx.showToast({ title: "微信收款页面打开失败，请重试", icon: "none" });
    } finally {
      this.setData({ confirming: false });
      if (this.refreshing) await this.refreshing;
      if (this.visible) await this.load();
    }
  },
});
