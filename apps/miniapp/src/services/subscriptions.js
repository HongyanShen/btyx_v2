const { authenticatedRequest, newKey, tokenKey } = require("./request");
const { isCurrentSession } = require("./session");

const scenes = require("./generated/subscription-scenes");

function readPending(key) {
  const value = wx.getStorageSync(key);
  return Array.isArray(value) ? value : [];
}

function sendReceipt(batch) {
  return authenticatedRequest("/notifications/subscriptions", {
    method: "POST",
    data: batch,
  });
}

async function flushPending() {
  const owner = wx.getStorageSync("btyx-member-id");
  const token = wx.getStorageSync(tokenKey);
  if (!owner || !token) return;
  const key = "btyx-subscription-receipts-" + owner;
  for (const batch of readPending(key)) {
    if (
      !isCurrentSession(token) ||
      wx.getStorageSync("btyx-member-id") !== owner
    )
      return;
    await sendReceipt(batch);
    wx.setStorageSync(
      key,
      readPending(key).filter((row) => row.attempt !== batch.attempt),
    );
  }
}

// Keep offline reporting from holding a business action until the network timeout.
function reportWithoutBlocking(report) {
  return new Promise((resolve) => {
    const timer = setTimeout(resolve, 1200);
    const finish = () => {
      clearTimeout(timer);
      resolve();
    };
    report.then(finish, finish);
  });
}

function settings() {
  return new Promise((resolve) => {
    if (!wx.getSetting) return resolve({});
    wx.getSetting({
      withSubscriptions: true,
      success: (res) => resolve(res.subscriptionsSetting || {}),
      fail: () => resolve({}),
    });
  });
}

async function subscribe(scene, save) {
  const ids = scenes[scene];
  const token = wx.getStorageSync(tokenKey);
  const owner = wx.getStorageSync("btyx-member-id");
  if (!token || !ids || !wx.requestSubscribeMessage) return;
  try {
    const attempt = newKey();
    const response = await new Promise((resolve) => {
      wx.requestSubscribeMessage({
        tmplIds: ids,
        success: (result) => resolve({ result }),
        fail: (error) => resolve({ error }),
      });
    });
    const observed = new Date().toISOString();
    const prefs = await settings();
    const choices = ids.map((template_id) => ({
      template_id,
      result: response.error
        ? "error"
        : response.result[template_id] || "unknown",
      remembered: prefs.itemSettings?.[template_id] || "",
    }));
    const batch = {
      attempt,
      scene,
      observed_at: observed,
      main_switch: prefs.mainSwitch ?? null,
      error_code: response.error?.errCode || 0,
      choices,
    };
    if (!isCurrentSession(token)) return;
    if (save) await save(batch);
    else if (owner) {
      const key = "btyx-subscription-receipts-" + owner;
      wx.setStorageSync(key, [...readPending(key), batch]);
      await reportWithoutBlocking(flushPending());
    } else await reportWithoutBlocking(sendReceipt(batch));
    return choices;
  } catch {
    // Cached receipts are retried when this member opens the app again.
    return undefined;
  }
}

async function subscribeTasks() {
  const choices = await subscribe("tasks");
  const accepted = choices?.filter((row) => row.result === "accept").length;
  const title = accepted
    ? "已订阅 " + accepted + " 项提醒"
    : choices
      ? "未开启，可在微信设置中修改"
      : "订阅结果暂未确认";
  wx.showToast({ title, icon: "none" });
}

module.exports = { subscribe, subscribeTasks, scenes, flushPending };
