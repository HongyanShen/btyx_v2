import { upload } from "../../api/files.js";
import { syncMarketingFilePicker } from "../tasks/marketing-file-state.js";
export async function uploadContentFile(picker, files) {
  const widget = picker.closest("[data-file-widget]");
  const list = widget?.querySelector("[data-file-list]");
  const status = widget?.querySelector("[data-file-status]");
  const submit = picker.form?.querySelector("button[type='submit']");
  if (!widget || !list || !status || !submit || picker.disabled) return;
  const media = widget.dataset.media;
  const images = ["image/jpeg", "image/png", "image/gif", "image/webp"];
  const limit = media === "images" ? 9 : 1;
  if (
    media &&
    (files.length + list.children.length > limit ||
      files.some((file) =>
        media === "images"
          ? !images.includes(file.type)
          : file.type !== "video/mp4",
      ))
  ) {
    status.hidden = false;
    status.textContent =
      media === "images"
        ? "请选择 JPG、PNG、GIF 或 WebP 图片，最多 9 张"
        : "仅可上传 1 个 MP4 视频";
    picker.value = "";
    return;
  }
  picker.disabled = true;
  submit.disabled = true;
  widget.setAttribute("aria-busy", "true");
  syncMarketingFilePicker(widget);
  status.hidden = false;
  status.textContent = "正在上传并确认…";
  try {
    const results = await Promise.allSettled(files.map(upload));
    if (widget.isConnected) {
      let count = 0;
      let failure = "";
      for (const result of results) {
        if (result.status === "fulfilled") {
          list.append(
            attachmentItem(
              result.value,
              list.children.length + 1,
              widget.dataset.canDownload === "true",
              !!media,
            ),
          );
          count++;
        } else {
          failure ||=
            result.reason instanceof Error
              ? result.reason.message
              : "上传失败，请重试";
        }
      }
      status.textContent = failure
        ? files.length === 1
          ? failure
          : `已上传 ${count} 张，${files.length - count} 张失败。${failure}`
        : count === 1
          ? "文件已上传"
          : `已上传 ${count} 张图片`;
    }
  } catch (error) {
    status.textContent =
      error instanceof Error ? error.message : "上传失败，请重试";
  } finally {
    widget.removeAttribute("aria-busy");
    picker.value = "";
    picker.disabled = false;
    submit.disabled = false;
    syncMarketingFilePicker(widget);
  }
}
function attachmentItem(id, number, canDownload, gallery) {
  const row = document.createElement("div");
  row.setAttribute("data-file-item", "");
  const reference = document.createElement("input");
  reference.type = "hidden";
  reference.name = "files";
  reference.value = id;
  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "server-plain-button";
  remove.setAttribute("data-file-remove", "");
  remove.textContent = "移除";
  const attachment = canDownload
    ? document.createElement("button")
    : document.createElement("span");
  if (attachment instanceof HTMLButtonElement) {
    attachment.type = "button";
    attachment.className = "server-plain-button";
    if (gallery) attachment.dataset.gallerySingle = id;
    else attachment.dataset.fileDownload = id;
  }
  attachment.textContent = `${canDownload ? (gallery ? "预览附件" : "查看附件") : "附件"} ${number}`;
  row.append(reference, attachment, remove);
  return row;
}
