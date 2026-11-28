import "../modules/tasks/creative.js";
import "../components/date-time-picker.js";
import { initOperatorPermissions } from "../modules/operators/permissions.js";
import "../vendor/htmx.js";
import "../modules/settlement/month-selection.js";
import "../modules/settlement/settings.js";
import { syncMarketingFilePicker } from "../modules/tasks/marketing-file-state.js";
const prototypePages = new Map([
  ["#/admin/realname", "/admin/realname"],
  ["#/admin/mkt-tasks", "/tasks"],
  ["#/admin/mktapps", "/users"],
  ["#/admin/mkt-people", "/users/manage"],
  ["#/admin/mkt-works", "/reviews"],
  ["#/admin/mktpay", "/settlements"],
  ["#/admin/mkt-wd", "/withdrawals"],
]);
function followPrototypeLink() {
  const path = prototypePages.get(window.location.hash);
  if (path) window.location.replace(path);
  return Boolean(path);
}
const dialogTriggers = new WeakMap();
const searchTimers = new WeakMap();
let activeWorkspace = null;
let lastDialogTrigger = null;
function updateTableScroll(scroller) {
  const frame = scroller.parentElement;
  if (!frame?.classList.contains("server-table-frame")) return;
  const remaining = scroller.scrollWidth - scroller.clientWidth;
  frame.toggleAttribute("data-scroll-start", scroller.scrollLeft > 0);
  frame.toggleAttribute("data-scroll-end", scroller.scrollLeft < remaining - 1);
}
const tableResize = new ResizeObserver((entries) => {
  for (const { target } of entries) updateTableScroll(target);
});
document.addEventListener(
  "scroll",
  (event) => {
    if (event.target instanceof Element) updateTableScroll(event.target);
  },
  true,
);
function initializePage(focusWorkspace = false, openDialogs = true) {
  const workspace = document.querySelector("#admin-workspace");
  if (workspace && workspace !== activeWorkspace) {
    activeWorkspace = workspace;
    if (workspace.dataset.pageTitle)
      document.title = workspace.dataset.pageTitle;
    if (focusWorkspace) document.getElementById("workspace-content")?.focus();
    const toast = workspace.querySelector(".server-toast");
    if (toast) window.setTimeout(() => toast.remove(), 3500);
  }
  if (workspace?.querySelector("[data-gallery-cover]")) {
    void import("../modules/tasks/marketing-gallery.js").then(
      ({ observeMarketingCovers }) => {
        if (workspace.isConnected) observeMarketingCovers(workspace);
      },
    );
  }
  if (!openDialogs) return;
  for (const group of document.querySelectorAll(".operator-choices")) {
    initOperatorPermissions(group);
  }
  tableResize.disconnect();
  for (const scroller of workspace?.querySelectorAll(
    ".server-table-frame > .server-table-scroll",
  ) ?? []) {
    updateTableScroll(scroller);
    tableResize.observe(scroller);
  }
  const dialogs = document.querySelectorAll(
    "[data-feedback-dialog], [data-server-dialog]",
  );
  for (const dialog of dialogs) {
    if (!dialog.open) {
      if (lastDialogTrigger) dialogTriggers.set(dialog, lastDialogTrigger);
      dialog.showModal();
    }
  }
  const agreement = workspace?.querySelector("[data-agreement-form]");
  if (agreement && !agreement.dataset.agreementLoading) {
    agreement.dataset.agreementLoading = "true";
    void import("../modules/support/agreement-form.js").then(
      ({ initAgreementForm }) => {
        if (agreement.isConnected) initAgreementForm(agreement);
      },
      () => showRequestError("编辑器暂时无法加载，请刷新后重试"),
    );
  }
}
function showRequestError(message) {
  const banner =
    document.querySelector("dialog[open] [data-request-error]") ??
    document.querySelector("[data-request-error]");
  if (banner) {
    banner.textContent = message;
    banner.hidden = false;
  }
}
function dialogPending(dialog) {
  return (
    dialog.querySelector(
      ".server-dialog-form.htmx-request, [data-file-widget][aria-busy='true']",
    ) !== null
  );
}
function openIdentityPhoto(trigger) {
  const dialog = document.querySelector("#identity-preview-dialog");
  const thumbnail = trigger.querySelector("img");
  const image = dialog?.querySelector("img");
  const title = dialog?.querySelector("h3");
  const error = dialog?.querySelector(".identity-preview-error");
  const side = trigger.dataset.identitySide;
  if (
    !dialog ||
    dialog.open ||
    !thumbnail ||
    !image ||
    !title ||
    !error ||
    !side
  )
    return;
  title.textContent = `身份证${side}`;
  image.alt = `身份证${side}大图`;
  image.hidden = false;
  error.hidden = true;
  image.onerror = () => {
    image.hidden = true;
    error.hidden = false;
  };
  dialogTriggers.set(dialog, trigger);
  dialog.showModal();
  image.src = thumbnail.src;
}
document.addEventListener("htmx:beforeSwap", (event) => {
  const detail = event.detail;
  const { xhr } = detail;
  if (
    xhr.status >= 400 &&
    (xhr.getResponseHeader("HX-Retarget") === "#admin-workspace" ||
      xhr.getResponseHeader("X-Admin-Form") !== null)
  ) {
    detail.shouldSwap = true;
    detail.isError = false;
  }
});
document.addEventListener("htmx:afterSwap", () => initializePage(true, false));
document.addEventListener("htmx:afterSettle", () => initializePage());
document.addEventListener("htmx:historyRestore", () => initializePage(true));
document.addEventListener("htmx:beforeRequest", (event) => {
  const source = event.detail.elt;
  if (
    source.matches("[data-marketing-poll]") &&
    (document.querySelector("dialog[open]") ||
      source.contains(document.activeElement))
  ) {
    event.preventDefault();
    return;
  }
  for (const banner of document.querySelectorAll("[data-request-error]")) {
    banner.hidden = true;
  }
});
document.addEventListener("htmx:afterRequest", (event) => {
  const source = event.detail.elt;
  if (
    source instanceof HTMLInputElement &&
    source.matches("[data-marketing-price]")
  )
    source.value = source.defaultValue;
});
document.addEventListener("htmx:responseError", () => {
  showRequestError("操作失败，请刷新页面后重试");
});
document.addEventListener("htmx:sendError", () => {
  showRequestError("网络连接失败，请检查网络后重试");
});
document.addEventListener("htmx:timeout", () => {
  showRequestError("请求超时，请刷新页面核对操作结果");
});
document.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof Element)) return;
  const menuButton = target.closest("#open-mobile-menu");
  if (menuButton) {
    const menu = document.querySelector("#mobile-menu");
    if (menu && !menu.open) {
      dialogTriggers.set(menu, menuButton);
      menuButton.setAttribute("aria-expanded", "true");
      menu.showModal();
    }
  }
  const feedbackTrigger = target.closest(
    'a[hx-target="#feedback-dialog-slot"], [data-open-server-dialog]',
  );
  if (feedbackTrigger) lastDialogTrigger = feedbackTrigger;
  const clearSearch = target.closest("[data-clear-distributor-search]");
  const searchField = clearSearch
    ?.closest(".distributor-search")
    ?.querySelector("input");
  if (searchField) {
    searchField.value = "";
    searchField.focus();
    searchField.dispatchEvent(new Event("input", { bubbles: true }));
  }
  const identityPhoto = target.closest("[data-identity-side]");
  if (identityPhoto) openIdentityPhoto(identityPhoto);
  const gallery = target.closest("[data-gallery-cover], [data-gallery-single]");
  if (gallery) {
    const panel = document.querySelector("#marketing-gallery-dialog");
    if (panel) {
      dialogTriggers.set(panel, gallery);
      void import("../modules/tasks/marketing-gallery.js")
        .then(({ openMarketingGallery }) =>
          openMarketingGallery(panel, gallery),
        )
        .catch(() => showRequestError("附件暂时无法预览"));
    }
  }
  const state = target.closest("[data-marketing-state]");
  const stateInput = state?.form?.querySelector("[name='state']");
  if (stateInput && state?.dataset.marketingState)
    stateInput.value = state.dataset.marketingState;
  const playback = target.closest("[data-playback-id]");
  if (playback) {
    const dialog = document.querySelector("#playback-dialog");
    if (dialog) {
      dialogTriggers.set(dialog, playback);
      void import("../modules/reviews/playback-form.js")
        .then(({ showPlaybackScreenshot }) =>
          showPlaybackScreenshot(dialog, playback),
        )
        .catch(() => showRequestError("播放量截图暂时无法预览"));
    }
  }
  const copy = target.closest("[data-copy-text]");
  if (copy?.dataset.copyText) {
    const status = copy.closest("dialog")?.querySelector("[data-copy-status]");
    void navigator.clipboard.writeText(copy.dataset.copyText).then(
      () => {
        if (status) {
          status.textContent = "已复制链接";
          status.hidden = false;
        }
      },
      () => {
        if (status) {
          status.textContent = "复制失败，请选中链接后复制";
          status.hidden = false;
        }
      },
    );
  }
  const addEnrollmentTag = target.closest("[data-enrollment-tag-add]");
  if (addEnrollmentTag) {
    void import("../modules/users/enrollment-form.js")
      .then(({ selectEnrollmentTag }) => selectEnrollmentTag(addEnrollmentTag))
      .catch((error) =>
        showRequestError(
          error instanceof Error ? error.message : "标签暂时无法添加",
        ),
      );
  }
  const download = target.closest("[data-file-download]");
  if (download?.dataset.fileDownload) {
    const id = download.dataset.fileDownload;
    void import("../api/files.js")
      .then((files) => files.download(id))
      .catch((error) =>
        showRequestError(
          error instanceof Error ? error.message : "附件暂时无法下载",
        ),
      );
  }
  const remove = target.closest("[data-file-remove]");
  const widget = remove?.closest("[data-file-widget]");
  if (remove && widget && widget.getAttribute("aria-busy") !== "true") {
    remove.closest("[data-file-item]")?.remove();
    syncMarketingFilePicker(widget);
  }
  const expand = target.closest("[data-expand-row]");
  if (expand) {
    const related = document.getElementById(
      expand.getAttribute("aria-controls") || "",
    );
    if (related) {
      related.hidden = !related.hidden;
      expand.setAttribute("aria-expanded", String(!related.hidden));
      expand.setAttribute("aria-label", related.hidden ? "展开行" : "折叠行");
    }
  }
  const passwordToggle = target.closest(".server-password-toggle");
  const password = document.getElementById("password");
  if (passwordToggle && password instanceof HTMLInputElement) {
    const show = password.type === "password";
    password.type = show ? "text" : "password";
    passwordToggle.textContent = show ? "隐藏" : "显示";
    passwordToggle.setAttribute("aria-label", show ? "隐藏密码" : "显示密码");
    passwordToggle.setAttribute("aria-pressed", String(show));
  }
  const dialog = target.closest("dialog");
  if (dialog && (target === dialog || target.closest("[data-close-dialog]"))) {
    event.preventDefault();
    if (
      !dialogPending(dialog) &&
      !(target === dialog && dialog.hasAttribute("data-close-guard"))
    )
      dialog.close();
  }
});
document.addEventListener(
  "cancel",
  (event) => {
    if (
      event.target instanceof HTMLDialogElement &&
      dialogPending(event.target)
    ) {
      event.preventDefault();
    }
  },
  true,
);
document.addEventListener("submit", (event) => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement)) return;
  window.clearTimeout(searchTimers.get(form));
  searchTimers.delete(form);
  if (form.method !== "dialog") return;
  const dialog = form.closest("dialog");
  if (dialog && dialogPending(dialog)) event.preventDefault();
});
document.addEventListener("input", (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) return;
  if (
    target instanceof HTMLInputElement &&
    target.hasAttribute("data-distributor-search") &&
    target.form
  ) {
    const form = target.form;
    window.clearTimeout(searchTimers.get(form));
    searchTimers.set(
      form,
      window.setTimeout(() => {
        if (form.isConnected) form.requestSubmit();
      }, 250),
    );
  }
  const key = target.closest("form")?.querySelector("[data-idempotency-key]");
  if (key) key.value = crypto.randomUUID();
});
document.addEventListener("change", (event) => {
  const picker = event.target;
  if (
    picker instanceof HTMLSelectElement &&
    picker.matches("[data-enrollment-filter-status]")
  ) {
    picker.form?.requestSubmit();
  }
  if (
    picker instanceof HTMLInputElement &&
    picker.matches("[data-enrollment-filter-tag]")
  ) {
    const field = picker.form?.querySelector("[name='tag']");
    if (field) {
      const term = picker.value.trim().replace(/^#/, "").toLowerCase();
      const choice = Array.from(picker.list?.options ?? []).find(
        (option) => option.value.toLowerCase() === term,
      );
      if (term && !choice) picker.value = picker.defaultValue;
      else {
        const key = choice?.dataset.tagKey ?? "";
        if (field.value !== key) {
          field.value = key;
          picker.form?.requestSubmit();
        }
      }
    }
  }
  if (
    picker instanceof HTMLSelectElement &&
    picker.id === "marketing-expiry-mode"
  ) {
    const controls = picker.closest("[data-expiry-controls]");
    const fixed = controls?.querySelectorAll("[data-expiry-fixed]");
    const day = controls?.querySelector("[data-expiry-day]");
    for (const item of fixed ?? []) item.hidden = picker.value === "end_of_day";
    if (day) day.hidden = picker.value !== "end_of_day";
  }
  if (
    picker instanceof HTMLInputElement &&
    picker.matches("[data-marketing-kind]")
  ) {
    const kind = picker.form?.querySelector("input[name='kind']");
    const widget = picker.form?.querySelector("[data-marketing-files]");
    if (kind && widget) {
      kind.value = picker.value;
      syncMarketingFilePicker(widget);
    }
  }
  if (
    picker instanceof HTMLInputElement &&
    picker.closest(".enrollment-review-fields")
  ) {
    const changed = picker.form?.querySelector("[name='portrait_changed']");
    if (changed) changed.value = "1";
  }
  if (
    !(picker instanceof HTMLInputElement) ||
    !picker.matches("[data-file-picker]") ||
    !picker.files?.length
  )
    return;
  const files = Array.from(picker.files);
  void import("../modules/content/content-files.js")
    .then(({ uploadContentFile }) => uploadContentFile(picker, files))
    .catch((error) =>
      showRequestError(
        error instanceof Error ? error.message : "上传失败，请重试",
      ),
    );
});
document.addEventListener("keydown", (event) => {
  if (event.key !== "Enter" || !(event.target instanceof HTMLElement)) return;
  if (event.target.matches("[data-marketing-price]")) {
    event.preventDefault();
    event.target.blur();
  }
  if (event.target.matches("[data-enrollment-tag-input]")) {
    event.preventDefault();
    event.target
      .closest("form")
      ?.querySelector("[data-enrollment-tag-add]")
      ?.click();
  }
});
document.addEventListener(
  "error",
  (event) => {
    if (
      !(event.target instanceof HTMLImageElement) ||
      !event.target.matches("[data-photo-error]")
    )
      return;
    const fallback = event.target
      .closest("dialog")
      ?.querySelector("[data-photo-fallback]");
    event.target.hidden = true;
    if (fallback) fallback.hidden = false;
  },
  true,
);
document.addEventListener(
  "close",
  (event) => {
    const dialog = event.target;
    if (!(dialog instanceof HTMLDialogElement)) return;
    const trigger = dialogTriggers.get(dialog);
    if (dialog.id === "mobile-menu")
      trigger?.setAttribute("aria-expanded", "false");
    if (dialog.matches("[data-feedback-dialog], [data-server-dialog]"))
      dialog.remove();
    if (dialog.id === "identity-preview-dialog") {
      const image = dialog.querySelector("img");
      if (image) {
        image.onerror = null;
        image.removeAttribute("src");
      }
    }
    trigger?.focus();
  },
  true,
);
window
  .matchMedia("(min-width: 768px)")
  .addEventListener("change", ({ matches }) => {
    if (matches) document.querySelector("#mobile-menu[open]")?.close();
  });
window.addEventListener("pageshow", ({ persisted }) => {
  if (persisted) window.location.reload();
});
window.addEventListener("hashchange", followPrototypeLink);
if (!followPrototypeLink()) initializePage();
