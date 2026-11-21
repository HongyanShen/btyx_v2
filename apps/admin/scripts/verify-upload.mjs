import assert from "node:assert/strict";

// Browser-side protocol verification only. No real TOS request or business
// content write occurs; the normal login and other menus still use local APIs.
export async function verifyDirectUpload(page) {
  const sequence = [];
  await page.route("**/api/v1/files/uploads", async (route) => {
    const input = route.request().postDataJSON();
    assert.equal(input.name, "browser-upload.png");
    assert.equal(input.media_type, "image/png");
    assert.equal(input.size, 8);
    sequence.push("metadata");
    await route.fulfill({
      json: {
        code: 0,
        msg: "",
        data: {
          id: "browser-upload",
          url: "http://127.0.0.1:5173/test-tos-upload",
          fields: { key: "test/uploads/source.png", policy: "mock-policy" },
          completed: false,
          expires_at: new Date(Date.now() + 600_000).toISOString(),
        },
      },
    });
  });

  await page.route("**/test-tos-upload", async (route) => {
    const headers = route.request().headers();
    assert.ok(!headers.authorization, "Business authorization leaked to TOS");
    assert.ok(!headers.cookie, "Business cookies leaked to TOS");
    const form = route.request().postDataBuffer().toString("latin1");
    assert.ok(form.includes('name="policy"'));
    assert.ok(form.indexOf('name="file"') > form.indexOf('name="policy"'));
    sequence.push("tos");
    await route.fulfill({ status: 204 });
  });

  await page.route("**/api/v1/files/browser-upload/complete", async (route) => {
    sequence.push("confirmed");
    await route.fulfill({
      json: {
        code: 0,
        msg: "",
        data: {
          id: "browser-upload",
          name: "browser-upload.png",
          media_type: "image/png",
          size: 8,
        },
      },
    });
  });

  await page.route("**/api/v1/files/browser-upload/download", (route) =>
    route.fulfill({
      json: {
        code: 0,
        data: {
          name: "browser-upload.png",
          media_type: "image/png",
          storage: "tos",
          url: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jQa0AAAAASUVORK5CYII=",
        },
      },
    }),
  );

  await page
    .getByRole("menuitem", { name: "任务管理", exact: true })
    .and(page.locator(":not([aria-disabled=true])"))
    .click();
  await page.getByRole("button", { name: "上传营销内容", exact: true }).click();
  await page.getByRole("radio", { name: "图文", exact: true }).check();
  await page.locator('input[type="file"]').setInputFiles({
    name: "browser-upload.png",
    mimeType: "image/png",
    buffer: Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
  });
  await page.getByRole("button", { name: "预览附件 1", exact: true }).waitFor();
  assert.deepEqual(sequence, ["metadata", "tos", "confirmed"]);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /^取\s*消$/ })
    .click();
  await page.unroute("**/api/v1/files/uploads");
  await page.unroute("**/test-tos-upload");
  await page.unroute("**/api/v1/files/browser-upload/complete");
  await page.unroute("**/api/v1/files/browser-upload/download");
}
