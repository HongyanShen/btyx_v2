import { getByRole, getByText } from "@testing-library/dom";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { upload } from "../static/api/files.js";
import { uploadContentFile } from "../static/modules/content/content-files.js";
import { syncMarketingFilePicker } from "../static/modules/tasks/marketing-file-state.js";

vi.mock("../static/api/files.js", () => ({ upload: vi.fn() }));

beforeEach(() => {
  vi.resetAllMocks();
});

afterEach(() => {
  document.body.replaceChildren();
});

function attachmentPicker(existing: string[]) {
  document.body.innerHTML = [
    '<form><input type="hidden" name="kind" value="images">',
    "<div data-file-widget data-marketing-files>",
    '<input type="file" data-file-picker><div data-file-list>',
    ...existing.map(
      (id) =>
        '<div data-file-item><input type="hidden" name="files" value="' +
        id +
        '"></div>',
    ),
    "</div><p data-file-status hidden></p></div>",
    '<button type="submit">保存</button></form>',
  ].join("");

  const widget = document.querySelector<HTMLElement>("[data-file-widget]")!;
  const picker = widget.querySelector<HTMLInputElement>("[data-file-picker]")!;
  const submit = getByRole<HTMLButtonElement>(document.body, "button", {
    name: "保存",
  });
  const selected = () =>
    [...widget.querySelectorAll<HTMLInputElement>("input[name=files]")].map(
      ({ value }) => value,
    );

  syncMarketingFilePicker(widget);
  return { widget, picker, submit, selected };
}

function images(...names: string[]) {
  return names.map((name) => new File(["test"], name, { type: "image/png" }));
}

it("批量上传保留原有图片及成功图片的顺序，单张失败不影响其他图片", async () => {
  const first = Promise.withResolvers<string>();
  vi.mocked(upload)
    .mockReturnValueOnce(first.promise)
    .mockRejectedValueOnce(new Error("第二张图片上传失败"))
    .mockResolvedValueOnce("last-photo");

  const { widget, picker, submit, selected } = attachmentPicker([
    "existing-photo",
  ]);
  expect(picker.multiple).toBe(true);
  const pending = uploadContentFile(
    picker,
    images("first.png", "failed.png", "last.png"),
  );
  expect(upload).toHaveBeenCalledTimes(3);
  expect(widget.getAttribute("aria-busy")).toBe("true");
  expect(submit.disabled).toBe(true);
  expect(selected()).toEqual(["existing-photo"]);

  first.resolve("first-photo");
  await pending;
  expect(selected()).toEqual(["existing-photo", "first-photo", "last-photo"]);
  expect(widget.hasAttribute("aria-busy")).toBe(false);
  expect(picker.disabled).toBe(false);
  expect(submit.disabled).toBe(false);
  expect(
    getByText(widget, /已上传 2 张，1 张失败。第二张图片上传失败/).hidden,
  ).toBe(false);
});

it("按已有图片数量校验剩余名额", async () => {
  const existing = Array.from({ length: 8 }, (_, index) => "photo-" + index);
  const { widget, picker, selected } = attachmentPicker(existing);
  vi.mocked(upload).mockResolvedValue("ninth-photo");

  await uploadContentFile(picker, images("ninth.png", "tenth.png"));
  expect(getByText(widget, /最多 9 张/).hidden).toBe(false);
  expect(upload).not.toHaveBeenCalled();
  expect(selected()).toEqual(existing);

  await uploadContentFile(picker, images("ninth.png"));
  expect(upload).toHaveBeenCalledTimes(1);
  expect(selected()).toEqual([...existing, "ninth-photo"]);
  expect(picker.disabled).toBe(true);
});
