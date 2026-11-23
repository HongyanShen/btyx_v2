const { request, tokenKey } = require("../../services/request");
const { profilePhotoURL } = require("../../services/profile");
const {
  hasSession,
  isCurrentSession,
  openLogin,
  pendingSession,
} = require("../../services/session");
const config = require("../../services/config");

Page({
  data: {
    loading: true,
    signedIn: false,
    error: "",
    user: null,
    avatar: "",
    identityTag: "",
    identityAction: "去认证",
    unread: 0,
  },

  onShow() {
    this.load();
  },

  async load() {
    const loadID = (this.loadID || 0) + 1;
    this.loadID = loadID;
    this.setData({ loading: true, error: "", unread: 0 });

    await pendingSession();
    if (this.loadID !== loadID) return;

    const token = wx.getStorageSync(tokenKey);
    const isCurrent = () => this.loadID === loadID && isCurrentSession(token);
    this.setData({ signedIn: !!token, loading: !!token });
    if (!token) {
      this.setData({ user: null, avatar: "", identityAction: "去认证" });

      return;
    }

    if (this.profileToken !== token) {
      this.setData({ user: null, avatar: "", identityAction: "去认证" });
    }

    this.profileToken = token;
    try {
      const user = await request("/me");
      if (!isCurrent()) return;

      const resubmissionRequired = user.identity_resubmission_required;
      this.profileToken = wx.getStorageSync(tokenKey);
      this.setData({
        user,
        avatar:
          this.data.user?.avatar_file_id === user.avatar_file_id
            ? this.data.avatar
            : "",
        identityTag: user.identity_verified
          ? "已实名"
          : resubmissionRequired
            ? "实名待补交"
            : "未实名",
        identityAction: user.identity_verified
          ? "已认证"
          : resubmissionRequired
            ? "去补交"
            : "去认证",
      });
      await Promise.all([
        profilePhotoURL(user.avatar_file_id).then((avatar) => {
          if (isCurrent()) this.setData({ avatar });
        }),
        request("/notifications")
          .then((messages) => {
            if (isCurrent()) {
              this.setData({
                unread: messages.filter((message) => !message.read).length,
              });
            }
          })
          .catch(() => {}),
      ]);
    } catch (error) {
      if (isCurrent()) {
        this.setData({ error: error.message });
      }
    } finally {
      if (this.loadID === loadID) {
        if (!hasSession()) {
          this.setData({ signedIn: false, user: null, avatar: "", unread: 0 });
        }

        this.setData({ loading: false });
      }
    }
  },

  openLogin,

  openProfile() {
    if (!hasSession() && !pendingSession()) return openLogin();

    wx.navigateTo({ url: "/subpackages/member/profile/index" });
  },

  navigate(event) {
    if (!hasSession()) return openLogin();

    wx.navigateTo({
      url: `/subpackages/member/${event.currentTarget.dataset.page}/index`,
    });
  },

  soon() {
    wx.showToast({ title: "即将上线，敬请期待", icon: "none" });
  },

  subscribe() {
    if (!hasSession()) return openLogin();

    if (!config.subscriptionTemplates.length) {
      wx.showToast({ title: "站内消息已开启", icon: "none" });

      return;
    }

    wx.requestSubscribeMessage({ tmplIds: config.subscriptionTemplates });
  },
});
