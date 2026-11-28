const { sendRequest } = require("./http");
const {
  tokenKey,
  clearSession,
  pendingSession,
  renewSession,
} = require("./session");

function request(path, options = {}) {
  if (options.auth === false) {
    return sendRequest(path, options);
  }

  const pending = pendingSession();
  if (pending) return pending.then(() => authenticatedRequest(path, options));

  return authenticatedRequest(path, options);
}

function authenticatedRequest(path, options) {
  const token = wx.getStorageSync(tokenKey);
  if (!token) {
    return Promise.reject(new Error("请先登录"));
  }

  return sendRequest(path, options, token).catch(async (error) => {
    if (error.status !== 401) throw error;

    // The mini API rejects expired sessions before running the protected handler.
    const replacement = await renewSession(token);
    if (!replacement) throw error;

    return sendRequest(path, options, replacement).catch((retryError) => {
      if (retryError.status === 401) clearSession(replacement);

      throw retryError;
    });
  });
}

function post(path, data, key) {
  return request(path, { method: "POST", data, key });
}

function newKey() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}

module.exports = { request, authenticatedRequest, post, newKey, tokenKey };
