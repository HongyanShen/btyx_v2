import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { get, post } from "../static/api/client.js";
import { upload } from "../static/api/files.js";
import type { FileUploadTicket } from "./generated/models";

vi.mock("../static/api/client.js", () => ({
  get: vi.fn(),
  post: vi.fn(),
  getBlob: vi.fn(),
}));

const ticket: FileUploadTicket = {
  id: "upload-1",
  url: "https://private-bucket.tos-cn-beijing.volces.com/",
  fields: {
    key: "btyx-v2/dev/uploads/upload-1.png",
    policy: "signed-policy",
    "x-tos-signature": "signature",
    "x-tos-algorithm": "TOS4-HMAC-SHA256",
    "x-tos-credential": "test-credential",
    "x-tos-date": "20261117T020000Z",
    acl: "private",
    "Content-Type": "image/png",
    success_action_status: "204",
    "x-tos-forbid-overwrite": "true",
  },
  completed: false,
  expires_at: new Date(Date.now() + 600_000).toISOString(),
};

beforeEach(() => {
  vi.resetAllMocks();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("TOS 前端直传", () => {
  it("带时区凭证过期后重试使用新的幂等键", async () => {
    const file = new File(["png data"], "expired.png", { type: "image/png" });
    const now = Date.parse("2026-11-17T10:00:00+08:00");
    const clock = vi.spyOn(Date, "now").mockReturnValue(now);
    vi.mocked(post)
      .mockResolvedValueOnce({
        ...ticket,
        expires_at: "2026-11-17T10:01:00+08:00",
      })
      .mockRejectedValueOnce(new Error("文件核验失败"));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

    try {
      await expect(upload(file)).rejects.toThrow("文件核验失败");
      const originalKey = vi.mocked(post).mock.calls[0][2];
      clock.mockReturnValue(now + 60_000);
      vi.mocked(post).mockResolvedValueOnce({ id: ticket.id, completed: true });

      expect(await upload(file)).toBe(ticket.id);
      expect(vi.mocked(post).mock.calls[2][2]).not.toBe(originalKey);
    } finally {
      clock.mockRestore();
    }
  });

  it("业务 API 只收元数据，文件直接发往 TOS 且不携带业务 token", async () => {
    const file = new File(["png data"], "test.png", { type: "image/png" });
    vi.mocked(post)
      .mockResolvedValueOnce(ticket)
      .mockResolvedValueOnce({ id: ticket.id });
    const fetchUpload = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchUpload);

    expect(await upload(file)).toBe(ticket.id);
    expect(post).toHaveBeenNthCalledWith(
      1,
      "/files/uploads",
      {
        name: "test.png",
        media_type: "image/png",
        size: 8,
      },
      expect.any(String),
    );
    expect(fetchUpload).toHaveBeenCalledWith(
      ticket.url,
      expect.objectContaining({
        method: "POST",
        credentials: "omit",
      }),
    );
    const options = fetchUpload.mock.calls[0][1] as RequestInit;
    expect(options.headers).toBeUndefined();
    const form = options.body as FormData;
    expect([...form.keys()].at(-1)).toBe("file");
    expect(form.get("policy")).toBe("signed-policy");
    expect(post).toHaveBeenLastCalledWith("/files/upload-1/complete", {});
    expect(get).not.toHaveBeenCalled();
  });

  it("TOS 超时后仍尝试服务端确认，避免把已上传对象当成失败重传", async () => {
    const file = new File(["png data"], "retry.png", { type: "image/png" });
    vi.mocked(post)
      .mockResolvedValueOnce(ticket)
      .mockResolvedValueOnce({ id: ticket.id });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new Error("network timeout")),
    );

    expect(await upload(file)).toBe(ticket.id);
  });

  it("服务端未确认时不返回可引用的文件 ID", async () => {
    const file = new File(["png data"], "pending.png", { type: "image/png" });
    vi.mocked(post)
      .mockResolvedValueOnce(ticket)
      .mockRejectedValueOnce(new Error("文件核验失败"));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

    await expect(upload(file)).rejects.toThrow("文件核验失败");
  });
});
