const { request, post } = require("../../../services/request");

const deliveryLabels = {
  pending: "等待发送", sending: "正在发送", sent: "微信已受理",
  skipped: "已跳过", failed: "发送失败", unknown: "结果待核实",
};

Page({
  data: { loading: true, error: "", items: [] },

  onShow() {
    this.load();
  },

  async load() {
    try {
      const items = await request("/notifications");
      this.setData({
        error: "",
        items: items.map((item) => ({
          ...item,
          delivery: deliveryLabels[item.delivery_state] ? "微信提醒 · " + (item.delivery_reason || deliveryLabels[item.delivery_state]) : "",
          time: new Date(item.created_at).toLocaleString(),
        })),
      });
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ loading: false });
    }
  },

  async read(event) {
    try {
      await post(`/notifications/${event.currentTarget.dataset.id}/read`, {});
      await this.load();
    } catch (error) {
      this.setData({ error: error.message });
    }
  },
});
