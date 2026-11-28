import { attachmentSource } from "../../api/files.js";
export async function showPlaybackScreenshot(dialog, trigger) {
  const id = trigger.dataset.playbackId;
  const title = dialog.querySelector("h3");
  const description = dialog.querySelector("[data-playback-description]");
  const status = dialog.querySelector("[data-playback-status]");
  const zoomNote = dialog.querySelector("[data-playback-zoom-note]");
  const toggle = dialog.querySelector(".playback-screenshot-toggle");
  const image = toggle?.querySelector("img");
  if (
    !id ||
    !title ||
    !description ||
    !status ||
    !zoomNote ||
    !toggle ||
    !image ||
    dialog.open
  )
    return;
  title.textContent = trigger.dataset.playbackTitle ?? "播放量截图";
  image.alt = title.textContent;
  description.textContent = trigger.dataset.playbackNote ?? "";
  status.textContent = "正在加载";
  status.hidden = false;
  toggle.hidden = true;
  zoomNote.hidden = true;
  dialog.classList.remove("is-zoomed");
  toggle.setAttribute("aria-label", "放大截图");
  zoomNote.textContent = "点击截图放大";
  let release;
  let closed = false;
  dialog.addEventListener(
    "close",
    () => {
      closed = true;
      release?.();
      image.onload = null;
      image.onerror = null;
      image.removeAttribute("src");
    },
    { once: true },
  );
  dialog.showModal();
  try {
    const source = await attachmentSource(
      id,
      dialog.dataset.allowLocal === "true",
    );
    release = source.release;
    if (closed || !dialog.isConnected) {
      release?.();
      return;
    }
    image.onload = () => {
      status.hidden = true;
      toggle.hidden = false;
      zoomNote.hidden = false;
    };
    image.onerror = () => {
      status.textContent = "截图暂时无法预览";
      status.hidden = false;
    };
    image.src = source.url;
    toggle.onclick = () => {
      const zoomed = dialog.classList.toggle("is-zoomed");
      toggle.setAttribute("aria-label", zoomed ? "缩小截图" : "放大截图");
      zoomNote.textContent = zoomed ? "点击截图缩小" : "点击截图放大";
    };
  } catch (error) {
    if (!closed)
      status.textContent =
        error instanceof Error ? error.message : "截图暂时无法预览";
  }
}
