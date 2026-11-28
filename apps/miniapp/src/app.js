const { startSession } = require("./services/session");

App({
  onShow() {
    return startSession().then(() => require("./services/subscriptions").flushPending()).catch(() => {});
  },
  globalData: {},
});
