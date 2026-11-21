import { FileMediaType, FileStorage } from "../generated/enums";
import type {
  FileDownload,
  FileInitiateInput,
  FileObject,
  FileUploadTicket,
  FileUploadFields,
} from "../generated/models";
import { get, getBlob, post } from "./client";

const maximumSize = 25 * 1024 * 1024;
const uploadKeys = new WeakMap<File, { key: string; expiresAt: number }>();
const inFlight = new WeakMap<File, Promise<string>>();

const extensions: Record<string, FileMediaType> = {
  jpg: FileMediaType.FileJPEG,
  jpeg: FileMediaType.FileJPEG,
  png: FileMediaType.FilePNG,
  gif: FileMediaType.FileGIF,
  webp: FileMediaType.FileWEBP,
  mp4: FileMediaType.FileMP4,
};

function fileMediaType(file: File): FileMediaType {
  const declared = Object.values(FileMediaType).find(
    (type) => type === file.type,
  );
  if (declared) {
    return declared;
  }

  const inferred = extensions[file.name.split(".").at(-1)?.toLowerCase() ?? ""];
  if (!file.type && inferred) {
    return inferred;
  }

  throw new Error("仅支持 JPG、PNG、GIF、WEBP 图片及 MP4 视频");
}

async function transfer(file: File): Promise<string> {
  if (
    !Number.isSafeInteger(file.size) ||
    file.size < 1 ||
    file.size > maximumSize
  ) {
    throw new Error("请选择25MB以内的非空文件");
  }

  const input: FileInitiateInput = {
    name: file.name,
    media_type: fileMediaType(file),
    size: file.size,
  };
  let state = uploadKeys.get(file);
  if (!state || state.expiresAt <= Date.now()) {
    state = { key: crypto.randomUUID(), expiresAt: Date.now() + 600_000 };
    uploadKeys.set(file, state);
  }

  const ticket = await post<FileUploadTicket>(
    "/files/uploads",
    input,
    state.key,
  );
  if (ticket.completed) {
    uploadKeys.delete(file);

    return ticket.id;
  }

  state.expiresAt = ticket.expires_at ? Date.parse(ticket.expires_at) : 0;
  const form = new FormData();
  const fieldNames = Object.keys(ticket.fields) as (keyof FileUploadFields)[];
  for (const key of fieldNames) {
    form.append(key, ticket.fields[key]);
  }

  // TOS requires the file part last. This request must never use the business
  // Axios client, its Authorization interceptor, or browser cookies.
  form.append("file", file);
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 180_000);
  let uploadError: Error | undefined;

  try {
    const response = await fetch(ticket.url, {
      method: "POST",
      body: form,
      credentials: "omit",
      referrerPolicy: "no-referrer",
      signal: controller.signal,
    });
    if (!response.ok) {
      uploadError = new Error("文件直传失败，请检查网络后重试");
    }
  } catch {
    uploadError = new Error("文件直传结果待确认，请检查网络后重试");
  } finally {
    window.clearTimeout(timeout);
  }

  try {
    const confirmed = await post<FileObject>(
      `/files/${ticket.id}/complete`,
      {},
    );
    uploadKeys.delete(file);

    return confirmed.id;
  } catch (error) {
    throw uploadError ?? error;
  }
}

export function upload(file: File): Promise<string> {
  const existing = inFlight.get(file);
  if (existing) {
    return existing;
  }

  const pending = transfer(file).finally(() => inFlight.delete(file));
  inFlight.set(file, pending);

  return pending;
}

export async function attachmentSource(id: string, allowLocal = true) {
  const ticket = await get<FileDownload>(`/files/${id}/download`);
  if (ticket.storage !== FileStorage.FileStorageLocal) {
    return { ...ticket, release: undefined };
  }

  if (!allowLocal) throw new Error("当前账号无权预览历史本地附件");

  const blob = await getBlob(`/files/${id}`);
  const url = URL.createObjectURL(blob);
  return { ...ticket, url, release: () => URL.revokeObjectURL(url) };
}

export async function download(id: string): Promise<void> {
  const { url, release } = await attachmentSource(id);
  if (release) window.setTimeout(release, 60_000);

  const link = document.createElement("a");
  link.href = url;
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.referrerPolicy = "no-referrer";
  document.body.append(link);
  link.click();
  link.remove();
}
