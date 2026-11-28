const { getCurrentAgreement } = require("../../../services/agreement");
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
    const { agreement: previous, checked: wasChecked } = this.data;
    this.setData({
      loading: true,
      error: "",
      agreement: null,
      consented: false,
      checked: false,
    });
    try {
      const agreement = await getCurrentAgreement();
      if (!agreement) return;

      const user = await request("/me");
      this.setData({
        agreement,
        consented: user.consented,
        checked: previous?.id === agreement.id && wasChecked,
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
    const { agreement, checked, submitting } = this.data;
    if (!agreement?.id || !checked || submitting) {
      return;
    }

    this.setData({ submitting: true, error: "" });

    try {
      await post("/agreement/consent", {
        agreement_id: agreement.id,
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
