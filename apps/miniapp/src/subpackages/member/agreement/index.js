const { request, post } = require("../../../services/request");

Page({
  data: {
    loading: true,
    error: "",
    agreement: null,
    consented: false,
    checked: false,
    submitting: false,
  },

  async onShow() {
    const previous = this.data.agreement;
    this.setData({
      loading: true,
      error: "",
      agreement: null,
      consented: false,
    });
    try {
      const [agreement, user] = await Promise.all([
        request("/agreement"),
        request("/me"),
      ]);
      this.setData({
        agreement,
        consented: user.consented,
        checked: previous?.id === agreement.id && this.data.checked,
      });
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ loading: false });
    }
  },

  toggle(event) {
    this.setData({ checked: event.detail.value.length > 0 });
  },

  async confirm() {
    if (!this.data.agreement || !this.data.checked || this.data.submitting) {
      return;
    }

    this.setData({ submitting: true, error: "" });

    try {
      await post("/agreement/consent", {
        agreement_id: this.data.agreement.id,
      });
      this.setData({ consented: true });
      wx.showToast({ title: "协议已确认", icon: "success" });
    } catch (error) {
      this.setData({ error: error.message });
    } finally {
      this.setData({ submitting: false });
    }
  },
});
