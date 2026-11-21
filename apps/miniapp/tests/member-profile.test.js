const assert = require("node:assert/strict");
const test = require("node:test");
const { tokenKey, request } = require("../src/services/request");
const session = require("../src/services/session");

let application;
global.App = (definition) => {
  application = definition;
};
require("../src/app");
delete global.App;

function mockWx({ failRequests = false } = {}) {
  const storage = new Map();
  const requests = [];
  const navigations = [];
  let loginCalls = 0;
  global.wx = {
    getStorageSync: (key) => storage.get(key),
    setStorageSync: (key, value) => storage.set(key, value),
    removeStorageSync: (key) => storage.delete(key),
    login: ({ success }) => {
      loginCalls++;
      success({ code: "wx-login-code" });
    },
    navigateTo: (options) => navigations.push(options.url),
    request: (options) => {
      requests.push(options);
      if (failRequests) options.fail();
    },
  };
  return { storage, requests, navigations, loginCalls: () => loginCalls };
}

function ok(request, data) {
  request.success({ statusCode: 200, data: { code: 0, data } });
}

function expired(request) {
  request.success({
    statusCode: 401,
    data: { code: 401, msg: "登录已失效" },
  });
}

function flushRequests() {
  return new Promise(setImmediate);
}

function page(path) {
  global.Page = (definition) => {
    global.definition = definition;
  };
  delete require.cache[require.resolve(path)];
  require(path);
  const definition = global.definition;
  return {
    ...definition,
    data: { ...definition.data },
    setData(values) {
      for (const [key, value] of Object.entries(values)) {
        const [field, property] = key.split(".");
        if (property) {
          this.data[field] = { ...this.data[field], [property]: value };
        } else {
          this.data[field] = value;
        }
      }
    },
  };
}

test("disabled local demo login does not need a device information API", async () => {
  const fake = mockWx();
  await assert.rejects(session.localLogin(), /本地联调登录仅限开发者工具/);
  assert.deepEqual(fake.requests, []);
});

test("home retains its network error when device information is unavailable", async () => {
  const fake = mockWx({ failRequests: true });
  fake.storage.set(tokenKey, "saved-session");
  const home = page("../src/pages/home/index");

  await home.load();

  assert.match(home.data.error, /网络连接失败/);
  assert.equal(home.data.localDemo, false);
  assert.equal(home.data.loading, false);
});

test("app startup signs in once and 我的 opens the profile after login", async () => {
  const fake = mockWx();
  const startup = application.onShow();
  const me = page("../src/pages/me/index");
  const load = me.load();
  const privateRequest = request("/wallet");
  await flushRequests();

  assert.equal(fake.loginCalls(), 1);
  assert.equal(fake.requests.length, 1);
  assert.match(fake.requests[0].url, /\/auth\/wechat$/);
  ok(fake.requests[0], { token: "signed-in" });
  await flushRequests();

  const fetch = (path) =>
    fake.requests.find((request) => request.url.endsWith(path));
  const profile = fetch("/me");
  const wallet = fetch("/wallet");
  assert.equal(profile.header.Authorization, "Bearer signed-in");
  assert.equal(wallet.header.Authorization, "Bearer signed-in");
  ok(profile, { nickname: "微信用户Awdx" });
  ok(wallet, {});
  await flushRequests();
  ok(fetch("/notifications"), []);
  await Promise.all([startup, load, privateRequest]);

  assert.equal(me.data.user.nickname, "微信用户Awdx");
  me.openProfile();
  assert.deepEqual(fake.navigations, ["/subpackages/member/profile/index"]);
});

test("an expired stored session is renewed automatically at startup", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "old-session");
  const startup = application.onShow();
  assert.match(fake.requests[0].url, /\/me$/);
  expired(fake.requests[0]);
  await flushRequests();

  assert.equal(fake.loginCalls(), 1);
  assert.match(fake.requests[1].url, /\/auth\/wechat$/);
  ok(fake.requests[1], { token: "renewed-session" });
  await startup;
  assert.equal(fake.storage.get(tokenKey), "renewed-session");
});

test("我的 distinguishes missing saved identity from a first-time verification", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "saved-session");
  const me = page("../src/pages/me/index");
  const loading = me.load();
  await flushRequests();
  ok(fake.requests[0], {
    identity_verified: false,
    identity_resubmission_required: true,
  });
  await flushRequests();
  ok(fake.requests[1], []);
  await loading;

  assert.equal(me.data.identityTag, "实名待补交");
  assert.equal(me.data.identityAction, "去补交");
});

test("failed automatic login keeps the manual login entrances available", async () => {
  const fake = mockWx({ failRequests: true });
  const startup = application.onShow();
  const me = page("../src/pages/me/index");
  await Promise.all([startup, me.load()]);
  assert.equal(me.data.user, null);
  assert.equal(fake.loginCalls(), 1);
  assert.equal(fake.requests.length, 1);
  me.openProfile();
  me.openLogin();
  assert.deepEqual(fake.navigations, [
    "/subpackages/member/login/index",
    "/subpackages/member/login/index",
  ]);
});

test("protected requests stay logged out and explicit login is shared across taps", async () => {
  const fake = mockWx();
  await assert.rejects(request("/me"), /请先登录/);
  assert.equal(fake.loginCalls(), 0);

  const first = session.login();
  const second = session.login();
  await Promise.resolve();
  assert.equal(fake.loginCalls(), 1);
  assert.deepEqual(fake.requests[0].data, { code: "wx-login-code" });
  ok(fake.requests[0], { token: "signed-in" });
  await Promise.all([first, second]);
  assert.equal(fake.storage.get(tokenKey), "signed-in");

  const logout = session.logout();
  assert.match(fake.requests[1].url, /\/auth\/logout$/);
  assert.equal(fake.requests[1].header.Authorization, "Bearer signed-in");
  ok(fake.requests[1], null);
  await logout;
  assert.equal(fake.storage.has(tokenKey), false);
});

test("an expired response from an older request leaves a newer session intact", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "old-session");
  const pending = request("/me");
  fake.storage.set(tokenKey, "new-session");
  expired(fake.requests[0]);
  await assert.rejects(pending, /登录已失效/);
  assert.equal(fake.storage.get(tokenKey), "new-session");
});

test("foreground expiry renews once and retries concurrent profile requests", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "old-session");
  const fetchProfile = request("/me");
  const saveProfile = request("/me", {
    method: "PUT",
    data: { nickname: "新昵称", avatar_file_id: "ready-avatar" },
  });
  const completed = Promise.allSettled([fetchProfile, saveProfile]);

  expired(fake.requests[0]);
  expired(fake.requests[1]);
  await flushRequests();
  assert.equal(fake.loginCalls(), 1);
  assert.match(fake.requests[2].url, /\/auth\/wechat$/);
  ok(fake.requests[2], { token: "renewed-session" });
  await flushRequests();

  assert.equal(fake.requests.length, 5);
  assert.equal(fake.requests[3].header.Authorization, "Bearer renewed-session");
  assert.equal(fake.requests[4].header.Authorization, "Bearer renewed-session");
  assert.equal(fake.requests[4].method, "PUT");
  assert.deepEqual(fake.requests[4].data, fake.requests[1].data);
  ok(fake.requests[3], { nickname: "微信用户Awdx" });
  ok(fake.requests[4], null);
  assert.deepEqual(await completed, [
    { status: "fulfilled", value: { nickname: "微信用户Awdx" } },
    { status: "fulfilled", value: null },
  ]);
});

test("a second unauthorized response ends recovery without a retry loop", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "old-session");
  const result = assert.rejects(request("/me"), /登录已失效/);
  expired(fake.requests[0]);
  await flushRequests();
  assert.equal(fake.loginCalls(), 1);
  ok(fake.requests[1], { token: "renewed-session" });
  await flushRequests();
  expired(fake.requests[2]);
  await result;

  assert.equal(fake.requests.length, 3);
  assert.equal(fake.storage.has(tokenKey), false);
});

test("我的 shows the returned profile when refreshing its session in the foreground", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "old-session");
  const me = page("../src/pages/me/index");
  const load = me.load();
  await flushRequests();

  expired(fake.requests[0]);
  await flushRequests();
  ok(fake.requests[1], { token: "renewed-session" });
  await flushRequests();
  ok(fake.requests[2], { nickname: "联调兔", verified_phone: "138****0000" });
  await flushRequests();
  ok(fake.requests[3], []);
  await load;

  assert.equal(me.data.user.nickname, "联调兔");
  assert.equal(me.data.user.verified_phone, "138****0000");
  assert.equal(me.data.signedIn, true);
});

test("个人信息 submits the native form nickname and exchanges only the WeChat phone code", async () => {
  const fake = mockWx();
  fake.storage.set(tokenKey, "signed-in");
  const profile = page("../src/subpackages/member/profile/index");
  profile.setData({ user: { nickname: "微信用户Awdx" }, loading: false });
  const phone = profile.getPhone({ detail: { code: "one-time-phone-code" } });
  assert.deepEqual(fake.requests[0].data, { code: "one-time-phone-code" });
  ok(fake.requests[0], { verified_phone: "138****0000" });
  await phone;
  assert.equal(profile.data.user.verified_phone, "138****0000");

  let wentBack = false;
  wx.navigateBack = () => {
    wentBack = true;
  };
  wx.showToast = () => {};
  const save = profile.save({
    detail: { value: { nickname: "我的微信昵称" } },
  });
  assert.deepEqual(fake.requests[1].data, { nickname: "我的微信昵称" });
  ok(fake.requests[1], null);
  await save;
  assert.equal(wentBack, true);
});
