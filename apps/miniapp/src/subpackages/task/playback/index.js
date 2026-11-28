const { request, post } = require("../../../services/request");
const {
  hasSession,
  openLogin,
  pendingSession,
} = require("../../../services/session");
const {
  chooseImage,
  materialTicket,
  upload,
} = require("../../../services/files");
const {
  activePlaybackRows,
  playbackWeeks,
} = require("../../../services/work-view");

const { subscribe } = require("../../../services/subscriptions");

Page({
  data: {
    loading: true,
    signedIn: false,
    error: "",
    submitError: "",
    uploading: "",
    submitting: false,
    submittedEarlier: false,
    drafts: {},
    screenshotPreviews: {},
  },

  onShow() {
    return this.load();
  },

  onPullDownRefresh() {
    return this.load().finally(() => wx.stopPullDownRefresh());
  },

  async load() {
    if (this.refreshing || this.data.submitting || this.data.uploading) return;
    this.refreshing = true;
    this.setData({ loading: true, error: "" });
    try {
      await pendingSession();
      this.setData({ signedIn: hasSession() });
      if (hasSession()) {
        this.present(await request("/assignments"));
        return this.loadScreenshotPreviews();
      }

      this.screenshotIds = new Set();
      this.setData({ drafts: {}, screenshotPreviews: {} });
    } catch (error) {
      this.setData({ signedIn: hasSession(), error: error.message });
    } finally {
      this.refreshing = false;
      this.setData({ loading: false });
    }
  },

  present(
    records,
    drafts = this.data.drafts,
    previews = this.data.screenshotPreviews,
  ) {
    const weeks = playbackWeeks(records);
    const rows = activePlaybackRows(weeks);
    const pending = rows.flatMap((row) =>
      row.platforms
        .filter(({ screenshot }) => !screenshot)
        .map((platform) => ({
          ...platform,
          taskId: row.id,
          title: row.title,
          slot: row.id + "_" + platform.key,
        })),
    );
    const activeDrafts = {};
    for (const { slot, url } of pending) {
      if (drafts[slot]?.url === url) activeDrafts[slot] = drafts[slot];
    }

    const screenshots = rows.flatMap(({ platforms }) =>
      platforms.map(({ screenshot }) => screenshot).filter(Boolean),
    );
    this.screenshotIds = new Set(screenshots);
    this.pending = pending;
    this.setData({
      ...weeks,
      drafts: activeDrafts,
      screenshotPreviews: Object.fromEntries(
        Object.entries(previews).filter(([id]) => this.screenshotIds.has(id)),
      ),
      pendingCount: pending.length,
      readyCount: Object.keys(activeDrafts).length,
    });
  },

  loadScreenshotPreviews() {
    return Promise.all(
      [...this.screenshotIds]
        .filter((id) => !this.data.screenshotPreviews[id]?.src)
        .map((id) => this.loadScreenshotPreview(id)),
    );
  },

  setScreenshotPreview(id, preview) {
    if (!this.screenshotIds.has(id)) return;

    this.setData({
      screenshotPreviews: { ...this.data.screenshotPreviews, [id]: preview },
    });
  },

  async loadScreenshotPreview(id) {
    if (
      !this.screenshotIds?.has(id) ||
      this.data.screenshotPreviews[id]?.loading
    ) {
      return;
    }

    this.setScreenshotPreview(id, { loading: true });
    try {
      const ticket = await materialTicket(id);
      const src = ticket.previewPath || ticket.url;
      this.setScreenshotPreview(id, {
        src: src || "",
        expiresAt: ticket.expires_at || "",
      });
    } catch {
      this.setScreenshotPreview(id, { src: "" });
    }
  },

  retryScreenshotPreview(event) {
    return this.loadScreenshotPreview(event.currentTarget.dataset.fileId);
  },

  screenshotPreviewError(event) {
    const { fileId, src } = event.currentTarget.dataset;
    if (fileId && this.data.screenshotPreviews[fileId]?.src === src) {
      this.setScreenshotPreview(fileId, { src: "" });
    }
  },

  async previewScreenshot(event) {
    const { fileId, src } = event.currentTarget.dataset;
    const preview = this.data.screenshotPreviews[fileId];
    let previewSrc = src;
    if (Date.parse(preview?.expiresAt) <= Date.now()) {
      await this.loadScreenshotPreview(fileId);
      previewSrc = this.data.screenshotPreviews[fileId]?.src;
    }

    if (previewSrc) wx.previewImage({ urls: [previewSrc], current: previewSrc });
  },

  async chooseScreenshot(event) {
    const { slot } = event.currentTarget.dataset;
    const platform = this.pending.find((item) => item.slot === slot);
    if (
      !platform ||
      this.data.loading ||
      this.data.submitting ||
      this.data.uploading
    )
      return;

    this.setData({ uploading: slot, submitError: "" });
    try {
      const path = await chooseImage();
      const screenshot = await upload(path);
      const drafts = {
        ...this.data.drafts,
        [slot]: { screenshot, url: platform.url, preview: path },
      };
      this.setData({ drafts, readyCount: Object.keys(drafts).length });
    } catch (error) {
      if (error.message !== "未选择图片") {
        wx.showToast({ title: error.message, icon: "none" });
      }
    } finally {
      this.setData({ uploading: "" });
    }
  },

  async submit() {
    if (
      this.data.loading ||
      this.data.submitting ||
      this.data.uploading ||
      !this.pending?.length
    )
      return;
    const drafts = { ...this.data.drafts };
    const selected = this.pending.filter(({ slot }) => drafts[slot]);
    if (!selected.length) {
      this.setData({ submitError: "请先上传要提交的播放量截图" });
      return;
    }

    this.setData({ submitting: true, submitError: "" });
    try {
      if (wx.requestSubscribeMessage) await subscribe("screenshots");
      const records = await request("/assignments");
      this.present(records, drafts);
      const current = new Map(
        this.pending.map((platform) => [platform.slot, platform]),
      );
      const errors = [];
      const submitted = new Map();
      const uploadedPreviews = {};
      for (const platform of selected) {
        const { slot, label, title, taskId, key, url } = platform;
        const latest = current.get(slot);
        if (!latest) continue;
        if (latest.url !== url) {
          errors.push(title + " · " + label + "：链接已更新，请重新上传截图");
          continue;
        }

        try {
          const { screenshot, preview } = drafts[slot];
          await post(
            "/assignments/" + encodeURIComponent(taskId) + "/submissions",
            {
              platform: key,
              url,
              screenshot,
            },
          );
          submitted.set(slot, screenshot);
          uploadedPreviews[screenshot] = { src: preview };
          delete drafts[slot];
        } catch (error) {
          errors.push(title + " · " + label + "：" + error.message);
        }
      }
      let updated = records.map((task) => ({
        ...task,
        submissions: task.submissions.map((item) => {
          const screenshot = submitted.get(task.id + "_" + item.platform);
          return screenshot
            ? { ...item, screenshot, settlement_eligible_at: "" }
            : item;
        }),
      }));
      if (submitted.size) {
        try {
          updated = await request("/assignments");
        } catch {
          // Keep completed submissions when only the refresh failed.
        }
      }
      this.present(updated, drafts, {
        ...this.data.screenshotPreviews,
        ...uploadedPreviews,
      });
      void this.loadScreenshotPreviews();
      if (submitted.size && !this.data.pendingCount) {
        this.setData({ submittedEarlier: true });
      }
      if (errors.length) this.setData({ submitError: errors.join("\n") });
      if (submitted.size)
        wx.showToast({
          title: errors.length ? "部分截图已提交" : "播放数据已提交",
          icon: errors.length ? "none" : "success",
        });
    } catch (error) {
      this.setData({ submitError: error.message });
    } finally {
      this.setData({ submitting: false });
    }
  },

  openLogin,

  goWork() {
    wx.switchTab({ url: "/pages/work/index" });
  },

  goWallet() {
    wx.switchTab({ url: "/pages/wallet/index" });
  },
});
