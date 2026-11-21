const config = require("./config");

function sendRequest(path, options = {}, token = "") {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${config.apiBase}${path}`,
      method: options.method || "GET",
      data: options.data,
      timeout: 20000,
      header: {
        "Content-Type": "application/json",
        Authorization: token ? `Bearer ${token}` : "",
        ...(options.key ? { "Idempotency-Key": options.key } : {}),
      },
      success(response) {
        if (response.statusCode >= 400 || response.data?.code !== 0) {
          const error = new Error(response.data?.msg || "操作失败，请重试");
          error.status = response.statusCode;
          reject(error);

          return;
        }

        resolve(response.data.data);
      },
      fail() {
        reject(new Error("网络连接失败，请检查后重试"));
      },
    });
  });
}

module.exports = { sendRequest };
