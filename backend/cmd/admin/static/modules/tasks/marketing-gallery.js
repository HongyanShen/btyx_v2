import { attachmentSource, download } from "../../api/files.js";
const pendingCovers = new WeakSet();
const coverReleases = new WeakMap();
const coverObserver = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    if (!entry.isIntersecting || !(entry.target instanceof HTMLButtonElement))
      continue;
    coverObserver.unobserve(entry.target);
    void loadCover(entry.target);
  }
});
export function observeMarketingCovers(root) {
  for (const button of root.querySelectorAll("[data-gallery-cover]")) {
    if (!pendingCovers.has(button)) coverObserver.observe(button);
  }
}
async function loadCover(button) {
  const id = button.dataset.galleryAsset;
  const placeholder = button.querySelector("[data-gallery-placeholder]");
  if (!id || !placeholder || pendingCovers.has(button)) return;
  pendingCovers.add(button);
  const allowLocal =
    button.closest("[data-marketing-page]")?.dataset.allowLocal === "true";
  try {
    const source = await attachmentSource(id, allowLocal);
    if (!button.isConnected) {
      source.release?.();
      return;
    }
    if (source.release) coverReleases.set(button, source.release);
    if (source.media_type === "video/mp4") {
      const video = document.createElement("video");
      video.muted = true;
      video.preload = "metadata";
      video.playsInline = true;
      video.src = source.url;
      const play = document.createElement("span");
      play.className = "attachment-play";
      play.textContent = "▶";
      placeholder.replaceWith(video, play);
    } else {
      const image = document.createElement("img");
      image.src = source.url;
      image.alt = "";
      image.referrerPolicy = "no-referrer";
      placeholder.replaceWith(image);
    }
  } catch {
    placeholder.textContent = "预览暂不可用";
  }
}
document.addEventListener("htmx:beforeCleanupElement", (event) => {
  const node = event.detail.elt;
  if (!(node instanceof HTMLElement)) return;
  coverObserver.unobserve(node);
  coverReleases.get(node)?.();
  coverReleases.delete(node);
});
export function openMarketingGallery(dialog, trigger) {
  const single = trigger.dataset.gallerySingle;
  const group = trigger.closest("[data-file-list]");
  const groupFiles = group?.querySelectorAll("input[name='files']");
  const ids =
    single && groupFiles?.length
      ? Array.from(groupFiles, (input) => input.value)
      : single
        ? [single]
        : [...trigger.querySelectorAll("[data-gallery-id]")].flatMap(
            (node) => node.dataset.galleryId || [],
          );
  const name = dialog.querySelector("[data-gallery-name]");
  const stage = dialog.querySelector("[data-gallery-stage]");
  const message = dialog.querySelector("[data-gallery-message]");
  const media = dialog.querySelector("[data-gallery-media]");
  const image = dialog.querySelector("[data-gallery-image]");
  const video = dialog.querySelector("[data-gallery-video]");
  const strip = dialog.querySelector("[data-gallery-strip]");
  const full = dialog.querySelector("[data-gallery-fullscreen]");
  const save = dialog.querySelector("[data-gallery-download]");
  if (
    !ids.length ||
    !name ||
    !stage ||
    !message ||
    !media ||
    !image ||
    !video ||
    !strip ||
    !full ||
    !save ||
    dialog.open
  )
    return;
  const controls = {
    name,
    stage,
    message,
    media,
    image,
    video,
    strip,
    full,
    save,
  };
  const abort = new AbortController();
  const listener = { signal: abort.signal };
  const thumbnails = [];
  let index = 0;
  let serial = 0;
  let release;
  let swipe = 0;
  const allowLocal = dialog.dataset.allowLocal === "true";
  const reset = () => {
    controls.video.pause();
    controls.video.removeAttribute("src");
    controls.video.load();
    controls.image.removeAttribute("src");
    controls.image.hidden = true;
    controls.video.hidden = true;
    controls.media.hidden = true;
    controls.full.hidden = true;
    controls.save.hidden = true;
    release?.();
    release = undefined;
  };
  async function show(next) {
    index = Math.max(0, Math.min(next, ids.length - 1));
    const expected = ++serial;
    reset();
    controls.name.textContent = "";
    controls.message.hidden = false;
    controls.message.textContent = "正在加载";
    for (let i = 0; i < thumbnails.length; i++)
      thumbnails[i].setAttribute("aria-pressed", String(i === index));
    try {
      const source = await attachmentSource(ids[index], allowLocal);
      if (expected !== serial || !dialog.open || !dialog.isConnected) {
        source.release?.();
        return;
      }
      release = source.release;
      controls.name.textContent = source.name;
      controls.name.title = source.name;
      controls.message.hidden = true;
      controls.media.hidden = false;
      controls.save.hidden = false;
      if (source.media_type === "video/mp4") {
        controls.video.setAttribute("aria-label", source.name);
        controls.video.hidden = false;
        controls.media.classList.add("attachment-video-preview");
        controls.video.src = source.url;
        controls.full.hidden = false;
      } else {
        controls.image.alt = source.name;
        controls.image.hidden = false;
        controls.media.classList.remove("attachment-video-preview");
        controls.image.src = source.url;
      }
    } catch (error) {
      if (expected === serial && dialog.open)
        controls.message.textContent =
          error instanceof Error ? error.message : "附件暂时无法预览";
    }
  }
  strip.replaceChildren();
  if (ids.length > 1) {
    ids.forEach((id, number) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "attachment-thumbnail";
      button.setAttribute("aria-label", `预览附件 ${number + 1}`);
      button.dataset.galleryAsset = id;
      const cover = document.createElement("span");
      cover.setAttribute("data-gallery-placeholder", "");
      cover.textContent = String(number + 1);
      button.append(cover);
      button.addEventListener("click", () => void show(number), listener);
      thumbnails.push(button);
      strip.append(button);
      coverObserver.observe(button);
    });
  }
  dialog.addEventListener(
    "keydown",
    (event) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        if (event.target === video) return;
        event.preventDefault();
        void show(index + (event.key === "ArrowLeft" ? -1 : 1));
      }
    },
    listener,
  );
  stage.addEventListener(
    "touchstart",
    (event) => {
      swipe = event.touches[0]?.clientX ?? 0;
    },
    listener,
  );
  stage.addEventListener(
    "touchend",
    (event) => {
      const delta = (event.changedTouches[0]?.clientX ?? swipe) - swipe;
      if (!video.hidden || Math.abs(delta) < 45) return;
      void show(index + (delta > 0 ? -1 : 1));
    },
    listener,
  );
  for (const element of [image, video])
    element.addEventListener(
      "error",
      () => {
        if (element.hidden) return;
        controls.media.hidden = true;
        controls.message.hidden = false;
        controls.message.textContent = "附件暂时无法预览";
      },
      listener,
    );
  full.addEventListener(
    "click",
    () => {
      const change = document.fullscreenElement
        ? document.exitFullscreen()
        : media.requestFullscreen();
      void change.catch(() => {
        controls.message.hidden = false;
        controls.message.textContent = "暂时无法切换全屏";
      });
    },
    listener,
  );
  document.addEventListener(
    "fullscreenchange",
    () => {
      const playing = document.fullscreenElement === media;
      full.textContent = playing ? "退出全屏" : "全屏播放";
      if (playing) media.append(full);
      else dialog.querySelector("footer")?.prepend(full);
      full.classList.toggle("is-fullscreen", playing);
    },
    listener,
  );
  save.addEventListener(
    "click",
    () => {
      void download(ids[index]).catch((error) => {
        controls.message.hidden = false;
        controls.message.textContent =
          error instanceof Error ? error.message : "附件暂时无法下载";
      });
    },
    listener,
  );
  dialog.addEventListener(
    "close",
    () => {
      serial++;
      abort.abort();
      reset();
      for (const button of thumbnails) {
        coverObserver.unobserve(button);
        coverReleases.get(button)?.();
      }
      dialog.querySelector("footer")?.prepend(full);
      full.classList.remove("is-fullscreen");
      full.textContent = "全屏播放";
      strip.replaceChildren();
    },
    { once: true },
  );
  dialog.showModal();
  void show(single ? ids.indexOf(single) : 0);
}
