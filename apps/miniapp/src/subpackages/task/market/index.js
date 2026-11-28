const { loadMarketingTasks } = require("../../../services/marketing-tasks");
const { formatFen } = require("../../../services/money");
Page({
  data: {
    loading: true,
    error: "",
    dailyLimit: 1,
    used: 0,
    creativeOnline: false,
  },
  onShow() {
    return this.load();
  },
  onPullDownRefresh() {
    return this.load().finally(() => wx.stopPullDownRefresh());
  },
  async load() {
    try {
      const o = await loadMarketingTasks();
      this.setData({
        dailyLimit: o.dailyLimit || 1,
        used: o.used || 0,
        today: o.today || null,
        creativeOnline: o.creativeOnline,
        price: o.priceFen ? formatFen(o.priceFen) : "",
        hasContent: o.hasContent,
        error: "",
      });
    } catch (e) {
      this.setData({ error: e.message });
    } finally {
      this.setData({ loading: false });
    }
  },
});
