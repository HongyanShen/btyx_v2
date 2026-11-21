const { sendRequest } = require("./http");
const config = require("./config");

const tokenKey = "btyx-session";
let loginPromise;
let startupPromise;
let renewal;

function hasSession() {
  return !!wx.getStorageSync(tokenKey);
}

function clearSession(token) {
  if (wx.getStorageSync(tokenKey) === token) {
    wx.removeStorageSync(tokenKey);
  }
}

function pendingSession() {
  return startupPromise || renewal?.pending;
}

function isCurrentSession(token) {
  const current = wx.getStorageSync(tokenKey);
  return (
    !!current &&
    (current === token ||
      (renewal?.previous === token && renewal.replacement === current))
  );
}

async function renewSession(token) {
  if (wx.getStorageSync(tokenKey) === token) {
    clearSession(token);
    const attempt = { previous: token, replacement: "" };
    renewal = attempt;
    attempt.pending = login()
      .then((replacement) => {
        attempt.replacement = replacement || "";
      })
      .catch(() => {})
      .finally(() => {
        attempt.pending = undefined;
      });
  }

  const attempt = renewal;
  if (attempt?.previous !== token) return "";

  await attempt.pending;
  return isCurrentSession(token) ? attempt.replacement : "";
}

function startSession() {
  if (!startupPromise) {
    startupPromise = restoreSession()
      .catch(() => {})
      .finally(() => {
        startupPromise = undefined;
      });
  }

  return startupPromise;
}

async function restoreSession() {
  const token = wx.getStorageSync(tokenKey);
  if (token) {
    try {
      await sendRequest("/me", {}, token);
      return;
    } catch (error) {
      if (error.status === 401) await renewSession(token);
      return;
    }
  }

  await login();
}

function openLogin() {
  wx.navigateTo({ url: "/subpackages/member/login/index" });
}

async function login() {
  if (hasSession()) {
    return;
  }

  if (!loginPromise) {
    loginPromise = new Promise((resolve, reject) => {
      wx.login({
        success: resolve,
        fail: () => reject(new Error("微信登录失败")),
      });
    })
      .then((login) =>
        sendRequest("/auth/wechat", {
          method: "POST",
          data: { code: login.code },
        }),
      )
      .then(({ token }) => {
        if (!token) {
          throw new Error("微信登录失败，请重试");
        }

        if (hasSession()) return;

        wx.setStorageSync(tokenKey, token);
        return token;
      })
      .finally(() => {
        loginPromise = undefined;
      });
  }

  return loginPromise;
}

async function logout() {
  const token = wx.getStorageSync(tokenKey);
  if (!token) {
    return;
  }

  try {
    await sendRequest("/auth/logout", { method: "POST" }, token);
  } catch (error) {
    if (error.status !== 401) {
      throw error;
    }
  }

  clearSession(token);
}

function canUseLocalDemo() {
  if (!config.allowLocalDemo) return false;

  try {
    return wx.getDeviceInfo?.()?.platform === "devtools";
  } catch {
    return false;
  }
}

async function localLogin() {
  if (!canUseLocalDemo()) {
    throw new Error("本地联调登录仅限开发者工具");
  }

  const device =
    wx.getStorageSync("btyx-local-device") || `devtools-${Date.now()}`;
  wx.setStorageSync("btyx-local-device", device);
  const result = await sendRequest("/auth/local", {
    method: "POST",
    data: { device },
  });
  wx.setStorageSync(tokenKey, result.token);
}

module.exports = {
  tokenKey,
  hasSession,
  clearSession,
  pendingSession,
  isCurrentSession,
  renewSession,
  startSession,
  openLogin,
  login,
  logout,
  canUseLocalDemo,
  localLogin,
};
