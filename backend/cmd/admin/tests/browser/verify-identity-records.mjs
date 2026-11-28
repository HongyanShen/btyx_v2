import assert from "node:assert/strict";

export async function verifyIdentityRecords(
  page,
  base,
  screenshot,
  expectNoOverflow,
) {
  const recordsURL = `${base}/admin/realname`;
  const fixtureURL = "https://images.example.test/identity-*";
  let jsonRequests = 0;
  const onRequest = (request) => {
    if (
      new URL(request.url()).pathname.startsWith(
        "/api/v1/identity-verifications",
      )
    )
      jsonRequests++;
  };
  page.on("request", onRequest);
  await page.route(fixtureURL, (route) => {
    if (route.request().url().endsWith("expired"))
      return route.fulfill({ status: 410 });
    const front = route.request().url().endsWith("front");
    return route.fulfill({
      contentType: "image/svg+xml",
      headers: { "Cache-Control": "no-store" },
      body: `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="400" viewBox="0 0 640 400"><rect width="640" height="400" rx="24" fill="${front ? "#ece5f5" : "#e2edf6"}"/><rect x="34" y="34" width="572" height="332" rx="16" fill="white" fill-opacity=".55"/><text x="320" y="170" text-anchor="middle" font-size="48" fill="#4b3f77">测试样张 · ${front ? "正面" : "反面"}</text><text x="320" y="235" text-anchor="middle" font-size="28" fill="#4b3f77">仅用于浏览器验收</text></svg>`,
    });
  });

  try {
    const response = await page.goto(recordsURL);
    assert.equal(response.status(), 200);
    const html = await response.text();
    assert.match(html, /核验留存/);
    assert.doesNotMatch(html, /data-workspace-page/);
    await page
      .getByRole("heading", { name: "报名审核 · 实名认证记录" })
      .waitFor();
    const table = page.getByRole("table");
    assert.equal(
      await table
        .locator("xpath=ancestor::*[contains(@class, 'ant-card')]")
        .count(),
      0,
    );
    assert.deepEqual(await table.getByRole("columnheader").allTextContents(), [
      "用户",
      "姓名",
      "身份证号",
      "手机号",
      "微信号",
      "身份证照片",
      "来源",
      "提交",
      "状态",
      "失败原因",
    ]);
    const recent = table.getByRole("row", { name: /新核验分发员/ });
    for (const text of [
      "张**",
      "5101**********1234",
      "138****8888",
      "example_wechat",
      "成功",
      "独立实名",
    ])
      await recent.getByText(text, { exact: true }).waitFor();
    const failed = table.getByRole("row", { name: /姓名与身份证号不匹配/ });
    await failed.getByText("失败", { exact: true }).waitFor();
    await failed.getByText("分发员报名", { exact: true }).waitFor();
    const failedCells = await failed.getByRole("cell").allTextContents();
    assert.deepEqual(
      [1, 2, 5].map((index) => failedCells[index]),
      ["—", "—", "—"],
    );
    assert.equal(
      await page.getByRole("button", { name: /通过|驳回/ }).count(),
      0,
    );
    assert.equal(
      await recent.getByRole("button", { name: /查看身份证.*大图/ }).count(),
      2,
    );
    const scroller = page.getByRole("region", { name: "实名认证记录表格" });
    const frame = page.locator(".server-table-frame");
    await frame.locator("xpath=self::*[@data-scroll-end]").waitFor();
    await scroller.evaluate((element) => {
      element.scrollLeft = element.scrollWidth;
    });
    await frame
      .locator("xpath=self::*[@data-scroll-start and not(@data-scroll-end)]")
      .waitFor();
    await scroller.evaluate((element) => {
      element.scrollLeft = 0;
    });
    await frame
      .locator("xpath=self::*[@data-scroll-end and not(@data-scroll-start)]")
      .waitFor();
    await screenshot("identity-records-desktop");

    for (const [side, file] of [
      ["正面", "front"],
      ["反面", "back"],
    ]) {
      const thumbnail = recent.getByRole("button", {
        name: `查看身份证${side}大图`,
      });
      const source = `https://images.example.test/identity-${file}`;
      assert.equal(await thumbnail.locator("img").getAttribute("src"), source);
      await thumbnail.click();
      const dialog = page.getByRole("dialog", { name: `身份证${side}` });
      const photo = dialog.getByRole("img", { name: `身份证${side}大图` });
      await photo.waitFor();
      assert.equal(await photo.getAttribute("src"), source);
      assert.equal(await photo.getAttribute("referrerpolicy"), "no-referrer");
      if (file === "front") {
        await photo.evaluate((image) => image.decode());
        await screenshot("identity-records-photo");
        await dialog.getByRole("button", { name: "关闭" }).click();
      } else {
        await page.keyboard.press("Escape");
      }
      await dialog.waitFor({ state: "hidden" });
      await thumbnail.evaluate((element) => {
        if (document.activeElement !== element)
          throw new Error("Photo trigger did not recover focus");
      });
    }

    const pager = page.getByRole("navigation", { name: "实名认证记录分页" });
    assert.equal(
      await pager.locator('[aria-current="page"]').textContent(),
      "1",
    );
    await pager.getByRole("link", { name: "第 2 页", exact: true }).waitFor();
    const next = pager.getByRole("link", { name: "第 2 页", exact: true });
    await next.click();
    await table.getByText("较早核验记录").waitFor();
    assert.equal(await recent.count(), 0);
    assert.equal(
      await pager.getByRole("button", { name: "下一页" }).isDisabled(),
      true,
    );
    await pager.getByRole("link", { name: "上一页" }).click();
    await recent.waitFor();
    assert.equal(await table.getByText("较早核验记录").count(), 0);
    await page.getByRole("button", { name: "刷新", exact: true }).click();
    await page.waitForURL(recordsURL);
    await recent.waitFor();

    const frontThumbnail = page.getByRole("button", {
      name: "查看身份证正面大图",
    });
    await frontThumbnail.locator("img").evaluate((image) => {
      image.src = "https://images.example.test/identity-expired";
    });
    await frontThumbnail.click();
    await page.getByText("照片加载失败，请重新获取预览。").waitFor();
    await page.getByRole("link", { name: "重新获取" }).click();
    await page
      .getByRole("dialog", { name: "身份证正面" })
      .waitFor({ state: "hidden" });
    await recent.waitFor();
    assert.equal(
      await frontThumbnail.locator("img").getAttribute("src"),
      "https://images.example.test/identity-front",
    );

    const desktop = page.viewportSize();
    await page.setViewportSize({ width: 390, height: 780 });
    await expectNoOverflow();
    const scrollable = page.getByRole("region", { name: "实名认证记录表格" });
    assert.equal(
      await scrollable.evaluate(
        (element) => element.scrollWidth > element.clientWidth,
      ),
      true,
    );
    await screenshot("identity-records-390");
    await frontThumbnail.click();
    assert.ok(await scrollable.evaluate((element) => element.scrollLeft > 0));
    const mobilePreview = page.getByRole("dialog", { name: "身份证正面" });
    const mobilePhoto = mobilePreview.getByRole("img", {
      name: "身份证正面大图",
    });
    await mobilePhoto.evaluate((image) => image.decode());
    assert.ok(
      await mobilePreview.evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        return (
          bounds.left >= 0 &&
          bounds.right <= innerWidth &&
          bounds.bottom <= innerHeight
        );
      }),
    );
    await screenshot("identity-records-photo-390");
    await mobilePreview.getByRole("button", { name: "关闭" }).click();
    await page.setViewportSize(desktop);
    assert.equal(jsonRequests, 0);
  } finally {
    page.off("request", onRequest);
    await page.unroute(fixtureURL);
  }
}
