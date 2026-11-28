import assert from "node:assert/strict";

export async function verifyDateTimePicker(page, screenshot) {
  const start = page.getByRole("textbox", {
    name: "开始时间（北京时间）",
    exact: true,
  });
  const end = page.getByLabel("结束时间（北京时间）", { exact: true });
  const panel = page.getByRole("dialog", { name: "开始时间（北京时间）" });
  await start.fill("2028-01-31T08:15");
  await start.click();
  await panel.getByRole("button", { name: "下个月" }).click();
  await panel.getByRole("button", { name: "2028-02-29" }).click();
  await panel.getByLabel("小时").selectOption("23");
  await panel.getByLabel("分钟").selectOption("59");
  await screenshot("date-time-picker", panel);
  assert.equal(await start.inputValue(), "2028-01-31T08:15");
  await panel.getByRole("button", { name: "确定" }).click();
  assert.equal(await start.inputValue(), "2028-02-29T23:59");
  assert.equal(await end.inputValue(), "");

  const original = page.viewportSize();
  await page.setViewportSize({ width: 320, height: 568 });
  try {
    await start.press("Enter");
    const box = await panel.boundingBox();
    assert.ok(box && box.x >= 0 && box.x + box.width <= 320);
    await panel.getByRole("button", { name: "上个月" }).click();
    await page.keyboard.press("Escape");
    assert.equal(await start.inputValue(), "2028-02-29T23:59");
    await start.click();
    await panel.getByRole("button", { name: "清空" }).click();
    assert.equal(await start.inputValue(), "");
  } finally {
    await page.setViewportSize(original);
  }
}

export async function verifyCreativeDateTime(page) {
  await page.getByRole("button", { name: "新建创作任务" }).click();
  for (const label of ["投稿开始", "投稿截止", "兑奖时间"]) {
    const name = label + "（北京时间）";
    const field = page.getByLabel(name, { exact: true });
    assert.equal(
      await field.evaluate((input) => input.validity.valueMissing),
      true,
    );
    await field.click();
    const panel = page.getByRole("dialog", { name, exact: true });
    await panel.getByRole("button", { name: "此刻" }).click();
    await panel.getByRole("button", { name: "确定" }).click();
    assert.equal(await field.inputValue(), "2026-12-01T00:05");
    assert.equal(await field.evaluate((input) => input.validity.valid), true);
  }
  await page.getByRole("button", { name: "关闭", exact: true }).click();
}
