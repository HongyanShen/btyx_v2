const { request } = require("../../../services/request");
const { presentLedger } = require("../../../services/wallet-view");

Page({
  data: { tab: "commission", ledger: [], state: "idle", error: "" },

  onShow() {
    if (this.data.tab === "flow") return this.load();
  },

  onPullDownRefresh() {
    const refresh = this.data.tab === "flow" ? this.load() : Promise.resolve();
    return refresh.finally(() => wx.stopPullDownRefresh());
  },

  selectTab(event) {
    const tab = event.currentTarget.dataset.tab;
    if (tab === this.data.tab) return;
    this.setData({ tab });
    if (tab === "flow") return this.load();
  },

  async load() {
    if (this.data.state === "loading") return;
    this.setData({ state: "loading", error: "", ledger: [] });
    try {
      const entries = await request("/ledger");
      const ledger = entries.map(presentLedger);
      this.setData({ ledger, state: ledger.length ? "ready" : "empty" });
    } catch (error) {
      this.setData({ state: "error", error: error.message });
    }
  },
});
