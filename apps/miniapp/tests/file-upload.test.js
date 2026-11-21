const assert = require("node:assert/strict");
const { test } = require("node:test");
const { upload } = require("../src/services/file-upload");

test("小程序仅把文件上传到 TOS，业务 token 不传给对象存储", async () => {
  const calls = [];
  global.wx = {
    getStorageSync: () => "local-business-token",
    getFileSystemManager: () => ({
      stat: ({ success }) => success({ stats: { size: 8 } }),
      readFile: ({ success }) =>
        success({
          data: new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]).buffer,
        }),
    }),
    request(options) {
      calls.push({ kind: "api", options });
      const data = options.url.endsWith("/files/uploads")
        ? {
            id: "file-1",
            url: "https://private-bucket.tos-cn-beijing.volces.com/",
            fields: { key: "private/key", policy: "policy" },
            expires_at: new Date(Date.now() + 600000).toISOString(),
            completed: false,
          }
        : { id: "file-1" };
      options.success({ statusCode: 200, data: { code: 0, msg: "", data } });
    },
    uploadFile(options) {
      calls.push({ kind: "tos", options });
      options.success({ statusCode: 204, data: "" });
    },
  };

  assert.equal(await upload("wxfile://picture.png"), "file-1");
  assert.equal(calls.length, 3);
  assert.equal(calls[0].options.data.size, 8);
  assert.equal(
    calls[0].options.header.Authorization,
    "Bearer local-business-token",
  );
  assert.equal(calls[1].kind, "tos");
  assert.equal(
    calls[1].options.url,
    "https://private-bucket.tos-cn-beijing.volces.com/",
  );
  assert.equal(calls[1].options.header, undefined);
  assert.equal(calls[1].options.name, "file");
  assert.equal(calls[2].options.url.endsWith("/files/file-1/complete"), true);
});

test("带时区凭证过期后重试使用新的幂等键", async () => {
  const keys = [];
  const originalNow = Date.now;
  let now = Date.parse("2026-11-17T10:00:00+08:00");
  Date.now = () => now;
  global.wx.request = (options) => {
    if (options.url.endsWith("/files/uploads")) {
      keys.push(options.header["Idempotency-Key"]);
      options.success({
        statusCode: 200,
        data: {
          code: 0,
          msg: "",
          data: {
            id: "expired-file",
            url: "https://private-bucket.tos-cn-beijing.volces.com/",
            fields: {},
            expires_at: "2026-11-17T10:01:00+08:00",
            completed: keys.length > 1,
          },
        },
      });
    } else {
      options.success({
        statusCode: 409,
        data: { code: 409, msg: "文件核验失败", data: null },
      });
    }
  };

  try {
    await assert.rejects(upload("wxfile://expired.png"), /文件核验失败/);
    now += 60000;
    assert.equal(await upload("wxfile://expired.png"), "expired-file");
    assert.notEqual(keys[0], keys[1]);
  } finally {
    Date.now = originalNow;
  }
});
