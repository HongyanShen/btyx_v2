const { startSession } = require("./services/session");

App({
  onShow: startSession,
  globalData: {},
});
