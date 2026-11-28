import assert from "node:assert/strict";

// Intercept TOS requests and save content only when the isolated suite passes a title.
export async function verifyDirectUpload(
  page,
  { uploadID = "browser-upload", transferStatus = 204, saveTitle } = {},
) {
  const sequence = [];
  const origin = new URL(page.url()).origin;
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
          id: uploadID,
          url: `${origin}/test-tos-upload`,
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
    await route.fulfill({ status: transferStatus });
  });

  await page.route(`**/api/v1/files/${uploadID}/complete`, async (route) => {
    sequence.push("confirmed");
    await route.fulfill({
      json: {
        code: 0,
        msg: "",
        data: {
          id: uploadID,
          name: "browser-upload.png",
          media_type: "image/png",
          size: 8,
        },
      },
    });
  });

  await page.route(`**/api/v1/files/${uploadID}/download`, (route) =>
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

  const marketingSection = page
    .locator(".sidebar details")
    .filter({ has: page.locator('a[href="/tasks"]') });
  if (!(await marketingSection.evaluate((el) => el.open)))
    await marketingSection.locator("summary").click();
  await page.getByRole("link", { name: "任务管理", exact: true }).click();
  await page.getByRole("button", { name: "上传营销内容", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新建任务 · 营销分发" });
  await dialog.getByRole("radio", { name: "图文", exact: true }).check();
  await dialog.locator('input[type="file"]').setInputFiles({
    name: "browser-upload.png",
    mimeType: "image/png",
    buffer: Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
  });
  await dialog
    .getByRole("button", { name: "预览附件 1", exact: true })
    .waitFor();
  assert.deepEqual(sequence, ["metadata", "tos", "confirmed"]);
  if (saveTitle) {
    await dialog.getByLabel("任务标题", { exact: true }).fill(saveTitle);
    await dialog.getByLabel("分发语").fill("浏览器验收分发语");
    await dialog.getByRole("button", { name: "上传到内容池" }).click();
    await page.getByText("已加入营销内容池", { exact: true }).waitFor();
  } else {
    await dialog.getByRole("button", { name: /^取\s*消$/ }).click();
    await page.unroute(`**/api/v1/files/${uploadID}/download`);
  }

  await page.unroute("**/api/v1/files/uploads");
  await page.unroute("**/test-tos-upload");
  await page.unroute(`**/api/v1/files/${uploadID}/complete`);
}
