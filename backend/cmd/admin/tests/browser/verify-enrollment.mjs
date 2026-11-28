import assert from "node:assert/strict";

export async function verifyEnrollment(page, screenshot) {
  const applicant = page.getByRole("row").filter({ hasText: "报名回归用户" });
  const xhs = applicant.getByRole("link", { name: "xhs-fixture" });
  assert.equal(await xhs.getAttribute("target"), "_blank");
  const search = new URL(await xhs.getAttribute("href"));
  assert.equal(search.hostname, "www.xiaohongshu.com");
  assert.equal(search.searchParams.get("keyword"), "xhs-fixture");

  for (const [side, suffix] of [
    ["正面", "front"],
    ["反面", "back"],
  ]) {
    const preview = applicant.getByRole("button", {
      name: "查看身份证" + side + "大图",
    });
    const imageURL = "https://images.example.test/enrollment-" + suffix;
    assert.equal(await preview.locator("img").getAttribute("src"), imageURL);
    const response = page.waitForResponse((result) => {
      const url = new URL(result.url());
      return (
        url.pathname === "/users" && url.searchParams.get("dialog") === suffix
      );
    });
    await preview.click();
    assert.equal((await response).status(), 200);
    const photo = page.getByRole("dialog", { name: "身份证" + side });
    assert.equal(await photo.locator("img").getAttribute("src"), imageURL);
    await photo.getByRole("button", { name: "关闭照片预览" }).click();
    await photo.waitFor({ state: "hidden" });
  }

  const approved = page.getByRole("row").filter({ hasText: "指派验收分发员" });
  const edit = approved.getByRole("button", { name: "改画像 / 标签" });
  await edit.click();
  const portrait = page.getByRole("dialog", { name: /画像 \/ 标签/ });
  assert.ok(
    await portrait
      .locator('input[name="recent_post"][value="true"]')
      .isChecked(),
  );
  assert.ok(
    await portrait
      .locator('input[name="new_account"][value="false"]')
      .isChecked(),
  );
  assert.ok(
    await portrait
      .locator('input[name="age_group"][value="25_30"]')
      .isChecked(),
  );
  assert.equal(
    await portrait.getByRole("button", { name: "拒绝", exact: true }).count(),
    0,
  );
  await portrait.getByLabel("新建画像标签").fill("浏览器画像标签");
  await portrait.getByRole("button", { name: /加到标签库/ }).click();
  const newTag = portrait.getByRole("checkbox", { name: "#浏览器画像标签" });
  await newTag.waitFor();
  assert.ok(await newTag.isChecked());
  await portrait.getByRole("button", { name: "保存", exact: true }).click();
  await approved.getByText("#浏览器画像标签").waitFor();
  await edit.click();
  await newTag.waitFor();
  assert.ok(await newTag.isChecked());
  await portrait.getByRole("button", { name: "取消" }).click();

  const review = applicant.getByRole("button", { name: "去复核" });
  await review.click();
  const decision = page.getByRole("dialog", {
    name: "复核 报名回归用户 的报名",
  });
  for (const [question, answer] of [
    [/^① 近 30 天发过作品/, "有"],
    [/^② 是不是新号/, "不是"],
    [/^③ 年龄层/, "18–24 岁"],
  ]) {
    const prompt = await decision.getByText(question).boundingBox();
    const chip = decision.getByText(answer, { exact: true });
    const option = await chip.boundingBox();
    assert.ok(
      option.x >= prompt.x + prompt.width - 1 &&
        option.y < prompt.y + prompt.height,
      "复核问题与首个选项应在同一行",
    );
    await chip.click();
    assert.equal(
      await chip.evaluate((el) => getComputedStyle(el).backgroundColor),
      "rgb(139, 130, 190)",
    );
  }
  await screenshot("enrollment-review", decision);
  const viewport = page.viewportSize();
  await page.setViewportSize({ width: 320, height: 480 });
  for (const action of [/加到标签库/, /^拒绝$/]) {
    const button = decision.getByRole("button", { name: action });
    assert.ok(
      await button.evaluate((el) => {
        const range = document.createRange();
        range.selectNodeContents(el);
        return range.getClientRects().length === 1;
      }),
      "窄屏操作按钮的文字应保持单行",
    );
  }
  const footer = await decision.locator("footer").boundingBox();
  assert.ok(footer.y + footer.height <= 480);
  assert.ok(await decision.evaluate((el) => el.scrollWidth <= el.clientWidth));
  await page.setViewportSize(viewport);
  await decision.getByRole("button", { name: "拒绝", exact: true }).click();
  await decision
    .getByRole("alert")
    .filter({ hasText: "请填写驳回原因" })
    .waitFor();
  const reason = "浏览器验收驳回原因";
  await decision.getByRole("checkbox", { name: "#浏览器验收标签" }).check();
  await decision.getByLabel("驳回原因").fill(reason);
  await decision.getByRole("button", { name: "拒绝", exact: true }).click();
  await applicant.getByText(reason).waitFor();
  await applicant.getByText("已拒绝", { exact: true }).waitFor();
  await applicant.getByText("#浏览器验收标签").waitFor();
  assert.equal(await review.count(), 0);
  const previousWorkspace = await page
    .locator("#admin-workspace")
    .elementHandle();
  await page.getByRole("button", { name: "刷新", exact: true }).click();
  await page.waitForFunction(
    (workspace) => !workspace.isConnected,
    previousWorkspace,
  );
  await previousWorkspace.dispose();
  await applicant.getByText(reason).waitFor();
  await applicant.getByText("#浏览器验收标签").waitFor();

  const library = page.getByRole("region", { name: "标签库" });
  await library.getByLabel("新标签", { exact: true }).fill("待删除验收标签");
  await library.getByRole("button", { name: "＋ 添加" }).click();
  await library.getByRole("button", { name: "改名 待删除验收标签" }).click();
  const rename = page.getByRole("dialog", { name: "重命名标签" });
  await rename.getByLabel("标签名称").fill("已改名验收标签");
  await rename.getByRole("button", { name: "保存" }).click();
  await library.getByRole("button", { name: "删除 已改名验收标签" }).click();
  const remove = page.getByRole("dialog", { name: "删除标签 #已改名验收标签" });
  await remove.getByText(/删除前请先解除/).waitFor();
  await remove.getByRole("button", { name: "删除", exact: true }).click();
  await library
    .getByText("#已改名验收标签", { exact: true })
    .waitFor({ state: "hidden" });
}
