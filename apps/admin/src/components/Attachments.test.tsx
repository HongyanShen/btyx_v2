import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { App } from "antd";
import { afterEach, expect, it, vi } from "vitest";
import { upload } from "../api/files";
import { AttachmentInput } from "./Attachments";

vi.mock("../api/files", () => ({ upload: vi.fn() }));
vi.mock("../app/Auth", () => ({
  useSession: () => ({
    canCall: (permission: string) => permission.startsWith("POST /files/"),
  }),
}));

afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

function selectImages(input: HTMLInputElement, ...names: string[]) {
  fireEvent.change(input, {
    target: {
      files: names.map(
        (name) => new File(["test"], name, { type: "image/png" }),
      ),
    },
  });
}

it("批量选择后按顺序追加成功图片，单张失败不影响其他图片", async () => {
  let finishFirst: (id: string) => void = () => {};
  const firstUpload = new Promise<string>((resolve) => {
    finishFirst = resolve;
  });
  vi.mocked(upload)
    .mockReturnValueOnce(firstUpload)
    .mockRejectedValueOnce(new Error("第二张图片上传失败"))
    .mockResolvedValueOnce("last-photo");

  const onChange = vi.fn();
  const onUploadingChange = vi.fn();
  const { container } = render(
    <App>
      <AttachmentInput
        media="images"
        value={["existing-photo"]}
        onChange={onChange}
        onUploadingChange={onUploadingChange}
      />
    </App>,
  );
  const input =
    container.querySelector<HTMLInputElement>('input[type="file"]')!;
  expect(input.multiple).toBe(true);
  selectImages(input, "first.png", "failed.png", "last.png");
  await waitFor(() => expect(upload).toHaveBeenCalledTimes(3));
  expect(onUploadingChange.mock.calls).toEqual([[true]]);
  expect(onChange).not.toHaveBeenCalled();

  finishFirst("first-photo");
  await waitFor(() =>
    expect(onChange).toHaveBeenCalledExactlyOnceWith([
      "existing-photo",
      "first-photo",
      "last-photo",
    ]),
  );
  expect(onUploadingChange.mock.calls).toEqual([[true], [false]]);
  expect(await screen.findByText(/第二张图片上传失败/)).toBeTruthy();
});

it("按已有图片数量校验剩余名额", async () => {
  const existing = Array.from({ length: 8 }, (_, index) => `photo-${index}`);
  const onChange = vi.fn();
  vi.mocked(upload).mockResolvedValue("ninth-photo");
  const { container } = render(
    <App>
      <AttachmentInput media="images" value={existing} onChange={onChange} />
    </App>,
  );
  const input =
    container.querySelector<HTMLInputElement>('input[type="file"]')!;
  selectImages(input, "ninth.png", "tenth.png");
  expect(await screen.findByText(/还可选择 1 张/)).toBeTruthy();
  expect(upload).not.toHaveBeenCalled();
  expect(onChange).not.toHaveBeenCalled();

  selectImages(
    container.querySelector<HTMLInputElement>('input[type="file"]')!,
    "ninth.png",
  );
  await waitFor(() =>
    expect(onChange).toHaveBeenCalledExactlyOnceWith([
      ...existing,
      "ninth-photo",
    ]),
  );
});
