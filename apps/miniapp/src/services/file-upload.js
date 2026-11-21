const { post, newKey } = require("./request");
const { fileInfo } = require("./file-info");

const uploadKeys = new Map();
const inFlight = new Map();

function uploadToTOS(ticket, filePath) {
  return new Promise((resolve, reject) => {
    wx.uploadFile({
      url: ticket.url,
      filePath,
      name: "file",
      formData: ticket.fields,
      timeout: 180000,
      // Only the signed form travels to TOS. Never forward our API Bearer token.
      success(response) {
        if (response.statusCode >= 200 && response.statusCode < 300) {
          resolve();

          return;
        }

        reject(new Error("文件直传失败，请重试"));
      },
      fail() {
        reject(new Error("文件直传结果待确认，请检查网络后重试"));
      },
    });
  });
}

async function transfer(filePath) {
  const input = await fileInfo(filePath);
  let state = uploadKeys.get(filePath);
  if (!state || state.expiresAt <= Date.now()) {
    state = { key: newKey(), expiresAt: Date.now() + 600000 };
    uploadKeys.set(filePath, state);
  }

  const ticket = await post("/files/uploads", input, state.key);
  if (ticket.completed) {
    uploadKeys.delete(filePath);

    return ticket.id;
  }

  state.expiresAt = ticket.expires_at ? Date.parse(ticket.expires_at) : 0;
  let uploadError;

  try {
    await uploadToTOS(ticket, filePath);
  } catch (error) {
    uploadError = error;
  }

  try {
    const confirmed = await post(`/files/${ticket.id}/complete`, {});
    uploadKeys.delete(filePath);

    return confirmed.id;
  } catch (error) {
    throw uploadError || error;
  }
}

function upload(filePath) {
  if (inFlight.has(filePath)) {
    return inFlight.get(filePath);
  }

  const pending = transfer(filePath).finally(() => inFlight.delete(filePath));
  inFlight.set(filePath, pending);

  return pending;
}

module.exports = { upload };
