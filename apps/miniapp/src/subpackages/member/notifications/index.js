const { request, post } = require("../../../services/request");

Page({
  data: { loading: true, error: "", items: [] },

  onShow() {
    this.load();
  },

  async load() {
    try {
      const items = await request("/notifications");
      this.setData({
        items: items.map((item) => ({
          ...item,
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
