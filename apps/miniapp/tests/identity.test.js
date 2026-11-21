const assert = require("node:assert/strict");
const test = require("node:test");
const { identityAgeError } = require("../src/services/identity");
const { addIdentityWatermark } = require("../src/services/identity-watermark");

test("identity age uses the exact eighteenth birthday in Beijing", () => {
  const beforeMidnight = new Date("2026-11-17T15:59:59Z");
  const midnight = new Date("2026-11-17T16:00:00Z");
  assert.match(
    identityAgeError("990000200811180013", beforeMidnight),
    /须年满 18 周岁/,
  );
  assert.equal(identityAgeError("990000200811180013", midnight), "");
  assert.match(
    identityAgeError("990000200811190019", midnight),
    /须年满 18 周岁/,
  );
  assert.match(identityAgeError("990000200811180014", midnight), /有效/);
});

let identityDefinition;
function identityPage(initial = {}) {
  if (!identityDefinition) {
    global.Page = (options) => {
      identityDefinition = options;
    };
    require("../src/subpackages/member/identity/index");
    delete global.Page;
  }

  return {
    ...identityDefinition,
    data: {
      ...identityDefinition.data,
      ...initial,
    },
    setData(updates) {
      for (const [key, value] of Object.entries(updates)) {
        const [parent, field] = key.split(".");
        if (field) {
          this.data[parent][field] = value;
        } else {
          this.data[parent] = value;
        }
      }
    },
  };
}

test("identity displays the verification outcome and retains editable fields after failure", async () => {
  const page = identityPage({
    user: { id: "owner", identity_verified: false },
    document: { front_file_id: "front", back_file_id: "back" },
  });
  let requests = 0;
  let readbacks = 0;
  global.wx = {
    getStorageSync: () => "test-session",
    request({ method, url, data, success }) {
      if (method === "GET") {
        assert.match(url, /\/me$/);
        readbacks++;
        success({
          statusCode: 200,
          data: {
            code: 0,
            data: {
              id: "owner",
              identity_verified: true,
              name: requests === 2 ? "更正姓名" : "更新姓名",
              identity_document: { number: "9900**********0017" },
            },
          },
        });
        return;
      }

      requests++;
      assert.equal(
        data.name,
        ["识别姓名", "更正姓名", "更新姓名"][requests - 1],
      );
      assert.equal(!!data.correct_identity, requests === 3);
      success(
        requests === 1
          ? {
              statusCode: 400,
              data: { code: "INVALID_ARGUMENT", msg: "姓名和身份证号不一致" },
            }
          : { statusCode: 200, data: { code: 0 } },
      );
    },
  };

  try {
    page.identityRecognized({
      detail: { name: "识别姓名", identity: "990000199001010017" },
    });
    assert.equal(page.data.document.name, "识别姓名");
    assert.equal(page.data.document.identity, "990000199001010017");
    await page.submit();
    assert.deepEqual(page.data.verificationResult, {
      status: "failed",
      message: "姓名和身份证号不一致",
    });
    assert.equal(page.data.user.identity_verified, false);

    await page.submit();
    assert.equal(requests, 1);
    page.correctIdentity();
    assert.equal(page.data.verificationResult, null);
    assert.equal(page.data.document.front_file_id, "front");
    assert.equal(page.data.document.name, "识别姓名");
    page.changeName({ detail: { value: "更正姓名" } });
    await page.submit();
    assert.equal(page.data.verificationResult.status, "success");
    assert.equal(page.data.user.identity_verified, true);
    assert.deepEqual(page.data.document, {});

    page.correctIdentity();
    assert.equal(page.data.verificationResult, null);
    assert.equal(page.data.editingIdentity, true);
    page.photoChanged({ detail: { side: "front", fileID: "corrected-front" } });
    page.photoChanged({ detail: { side: "back", fileID: "corrected-back" } });
    page.identityRecognized({
      detail: { name: "更新姓名", identity: "990000199001010017" },
    });
    await page.submit();
    assert.equal(requests, 3);
    assert.equal(readbacks, 2);
    assert.equal(page.data.user.identity_document.number, "9900**********0017");
    assert.equal(page.data.verificationResult.status, "success");
  } finally {
    delete global.wx;
  }
});

test("identity reloads both saved photos and previews the selected side", async () => {
  const page = identityPage();
  const calls = [];
  const previews = [];
  let cancelPreview;
  const photos = {
    front: "https://example.test/front.jpg",
    back: "https://example.test/back.jpg",
  };
  global.wx = {
    getStorageSync: () => "test-session",
    request({ url, success }) {
      const route = new URL(url).pathname;
      calls.push(route);
      const data = route.endsWith("/me")
        ? {
            id: "owner",
            identity_verified: true,
            name: "测试实名",
            identity_document: {
              number: "9900**********0017",
              front_file_id: "front",
              back_file_id: "back",
            },
          }
        : { url: route.includes("/front/") ? photos.front : photos.back };
      success({ statusCode: 200, data: { code: 0, data } });
    },
    previewImage({ current, urls, fail }) {
      previews.push({ current, urls });
      cancelPreview = fail;
    },
  };

  try {
    await page.load();
    assert.equal(page.data.user.name, "测试实名");
    assert.equal(page.data.user.identity_document.number, "9900**********0017");
    assert.deepEqual(page.data.savedPhotos, photos);
    assert.deepEqual(
      calls.map((path) => path.replace(/^\/api\/v1/, "")),
      ["/me", "/files/front/download", "/files/back/download"],
    );
    await page.previewSavedPhoto({
      currentTarget: { dataset: { side: "back" } },
    });
    assert.deepEqual(previews, [
      { current: photos.back, urls: [photos.front, photos.back] },
    ]);
    const savedPhotos = page.data.savedPhotos;
    cancelPreview({ errMsg: "previewImage:fail cancel" });
    await page.onShow();
    assert.equal(calls.length, 3);
    assert.equal(page.data.savedPhotos, savedPhotos);
    await page.onShow();
    assert.equal(calls.length, 4);
    assert.equal(page.data.savedPhotos, savedPhotos);
  } finally {
    delete global.wx;
  }
});

test("correction uses the saved name and photo references without submitting the masked number", async () => {
  const user = {
    id: "owner",
    identity_verified: true,
    name: "留存姓名",
    wechat: "saved-wechat",
    identity_document: {
      number: "9900**********0017",
      front_file_id: "saved-front",
      back_file_id: "saved-back",
    },
  };
  const page = identityPage({ user, wechat: user.wechat });
  const submissions = [];
  global.wx = {
    getStorageSync: () => "test-session",
    request({ method, url, data, success }) {
      let result;
      if (method === "POST") {
        submissions.push(data);
      } else {
        result = url.endsWith("/me") ? { ...user, name: "更正姓名" } : {};
      }

      success({ statusCode: 200, data: { code: 0, data: result } });
    },
  };

  try {
    page.correctIdentity();
    assert.equal(page.data.document.name, "留存姓名");
    assert.equal(page.data.document.front_file_id, "saved-front");
    assert.equal(page.data.document.back_file_id, "saved-back");
    assert.ok(!page.data.document.identity);
    page.changeName({ detail: { value: "更正姓名" } });
    await page.submit();
    assert.deepEqual(submissions, [
      {
        name: "更正姓名",
        front_file_id: "saved-front",
        back_file_id: "saved-back",
        correct_identity: true,
      },
    ]);
  } finally {
    delete global.wx;
  }
});

test("saved identity contacts become editable only after entering correction", async () => {
  const user = {
    id: "owner",
    identity_verified: true,
    verified_phone: "138****3333",
    contact_phone: "137****1111",
    wechat: "saved-wechat",
  };
  const page = identityPage({ user, wechat: user.wechat });
  const submissions = [];
  global.wx = {
    getStorageSync: () => "test-session",
    showToast() {},
    request({ method, url, data, success }) {
      let result;
      if (method === "POST") {
        submissions.push([
          new URL(url).pathname.replace(/^\/api\/v1/, ""),
          data,
        ]);
        result = url.endsWith("/me/phone")
          ? { verified_phone: user.verified_phone }
          : null;
      } else {
        result = {
          ...user,
          contact_phone: "139****2222",
          wechat: "other-wechat",
        };
      }

      success({ statusCode: 200, data: { code: 0, data: result } });
    },
  };

  try {
    page.changePhone({ detail: { value: "13900002222" } });
    page.changeWechat({ detail: { value: "other-wechat" } });
    await page.saveContacts();
    await page.getPhone({ detail: { code: "verified-again" } });
    assert.deepEqual(submissions, []);
    assert.equal(page.data.phone, "");
    assert.equal(page.data.wechat, "saved-wechat");

    page.correctIdentity();
    page.changePhone({ detail: { value: "13900002222" } });
    page.changeWechat({ detail: { value: "other-wechat" } });
    await page.saveContacts();
    assert.equal(page.data.user.contact_phone, "139****2222");
    assert.equal(page.data.user.wechat, "other-wechat");
    assert.equal(page.data.user.verified_phone, "138****3333");
    await page.getPhone({ detail: { code: "verified-again" } });
    assert.equal(page.data.user.contact_phone, "138****3333");
    assert.equal(page.data.user.verified_phone, "138****3333");
    assert.deepEqual(submissions, [
      [
        "/me/identity",
        { wechat: "other-wechat", contact_phone: "13900002222" },
      ],
      ["/me/phone", { code: "verified-again", use_for_contact: true }],
    ]);
  } finally {
    delete global.wx;
  }
});

test("new verification omits saved contacts and includes an explicit WeChat phone selection", async () => {
  const page = identityPage();
  const submissions = [];
  const user = {
    id: "owner",
    identity_verified: false,
    wechat: "old-wechat",
    verified_phone: "139****1111",
    contact_phone: "138****2222",
  };
  global.wx = {
    getStorageSync: () => "test-session",
    request({ method, url, data, success }) {
      let result;
      if (url.endsWith("/me/phone")) {
        result = {
          verified_phone: "138****3333",
          contact_phone_receipt: "selection-receipt",
        };
      } else if (method === "POST") {
        submissions.push(data);
      } else {
        result = { ...user, identity_verified: submissions.length > 0 };
      }

      success({ statusCode: 200, data: { code: 0, data: result } });
    },
  };

  try {
    await page.load();
    assert.equal(page.data.wechat, "");
    assert.equal(page.data.phoneReceipt, "");
    await page.getPhone({ detail: { code: "explicit-selection" } });
    page.setData({
      document: {
        name: "实名姓名",
        identity: "990000199001010017",
        front_file_id: "front",
        back_file_id: "back",
      },
    });
    await page.submit();
    assert.deepEqual(submissions, [
      {
        name: "实名姓名",
        identity: "990000199001010017",
        front_file_id: "front",
        back_file_id: "back",
        contact_phone_receipt: "selection-receipt",
      },
    ]);
  } finally {
    delete global.wx;
  }
});

test("identity image export uses the watermarked canvas and keeps the photo ratio", async () => {
  const painted = [];
  const canvas = {
    createImage() {
      return {
        width: 3200,
        height: 2000,
        set src(value) {
          painted.push(value);
          this.onload();
        },
      };
    },
    getContext() {
      return {
        fillRect() {},
        drawImage: () => painted.push("photo"),
        save() {},
        translate() {},
        rotate() {},
        restore() {},
        fillText: (text) => painted.push(text),
      };
    },
  };

  global.wx = {
    canvasToTempFilePath({ canvas: input, fileType, success }) {
      assert.equal(input, canvas);
      assert.equal(fileType, "jpg");
      assert.deepEqual([input.width, input.height], [1600, 1000]);
      assert.deepEqual(painted.slice(0, 2), ["original", "photo"]);
      assert.ok(painted.slice(2).includes("仅供白兔严选实名认证使用"));
      success({ tempFilePath: "watermarked.jpg" });
    },
  };

  try {
    assert.equal(
      await addIdentityWatermark(canvas, "original"),
      "watermarked.jpg",
    );
  } finally {
    delete global.wx;
  }
});
