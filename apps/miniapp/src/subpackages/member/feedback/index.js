const { request, post, newKey } = require("../../../services/request");

Page({
  data: { loading: true, error: "", body: "", items: [], submitting: false },

  onShow() {
    this.load();
  },

  async load() {
    try {
      const items = await request("/feedback");
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

  change(event) {
    this.setData({ body: event.detail.value });
    this.requestKey = undefined;
  },

  async submit() {
    if (this.data.submitting || !this.data.body.trim()) {
      return;
    }

    this.setData({ submitting: true, error: "" });

    try {
      this.requestKey = this.requestKey || newKey();
      await post("/feedback", { body: this.data.body }, this.requestKey);
      this.requestKey = undefined;
      this.setData({ body: "" });
      await this.load();
      wx.showToast({ title: "反馈已提交", icon: "success" });
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ submitting: false });
    }
  },
});
