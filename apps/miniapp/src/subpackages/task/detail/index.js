const { request, post } = require("../../../services/request");
const { takeClaimedAssignment } = require("../../../services/marketing-tasks");
const { doubleFen, formatFen } = require("../../../services/money");
const { isPlaybackReviewed, workCard } = require("../../../services/work-view");
const {
  chooseImage,
  materialTicket,
  upload,
  saveMaterial,
} = require("../../../services/files");
const {
  platformLabels,
  assignmentLabels,
  sourceLabels,
  submissionLabels,
} = require("../../../services/labels");
const {
  AssignmentSource,
  AssignmentStatus,
  SubmissionStatus,
  TaskStep,
} = require("../../../services/generated/enums");
const {
  isClosed,
  taskDeadline,
  beijingDateTime,
  taskProgress,
} = require("../../../services/task-progress");
const {
  executionTimeline,
  presentMaterials,
  publicationLink,
  sharePlaceholder,
  taskRequirements,
} = require("./presentation");

const { subscribe } = require("../../../services/subscriptions");

Page({
  data: {
    loading: true,
    error: "",
    task: null,
    platforms: [],
    links: {},
    screenshots: {},
    screenshotsOpen: false,
    saving: "",
    uploading: "",
    completing: false,
    savingFile: "",
    savedFiles: {},
    savedFileCount: 0,
    deadline: null,
    steps: TaskStep,
    openStage: "",
    materials: presentMaterials([]),
    loadingMaterials: false,
    galleryOpen: false,
    galleryIndex: 0,
    refreshingImage: "",
  },

  onLoad(options) {
    this.id = options.id;
    this.reissueOf = options.reissueOf;
    this.reissuePlatform = options.platform;
    this.section = options.section;
    this.drafts = {};
    this.submittedShareTexts = {};
  },

  onShow() {
    this.visible = true;
    this.load();
  },

  onHide() {
    this.visible = false;
    clearInterval(this.deadlineTimer);
  },

  onUnload() {
    this.onHide();
    this.unloaded = true;
  },

  async load() {
    if (this.refreshing) {
      return;
    }

    this.refreshing = true;
    clearInterval(this.deadlineTimer);
    this.setData({ loading: true, error: "" });
    try {
      const { task, original } = await this.loadAssignment();
      if (this.unloaded || !task) return;
      const closed = isClosed(task);
      const reissued = task.source === AssignmentSource.SourceReissued;
      const files = [...(task.files || [])];
      const links = {};
      const screenshots = {};
      const platforms = Array.from(task.platforms, (platform) => {
        const submission = task.submissions.find(
          (submission) => submission.platform === platform,
        );
        const invalid =
          submission?.status === SubmissionStatus.SubmissionInvalid;
        const readOnly = closed || !!submission?.settled || invalid;
        const reviewed = isPlaybackReviewed(submission?.status);
        const linkLocked = readOnly || reviewed;
        const draft = this.drafts[platform];
        const submittedURL = submission?.url || "";
        const share = this.submittedShareTexts[platform];
        const submittedText =
          share?.url === submittedURL ? share.text : submittedURL;
        links[platform] =
          (!linkLocked ? draft?.text : undefined) ?? submittedText;
        screenshots[platform] =
          (!readOnly && reviewed ? draft?.screenshot : undefined) ??
          submission?.screenshot ??
          "";

        return {
          key: platform,
          label: platformLabels[platform],
          placeholder: sharePlaceholder(platform, platformLabels[platform]),
          submittedURL,
          submittedText,
          submittedScreenshot: submission?.screenshot || "",
          status: submission?.settled
            ? "已结算"
            : submission
              ? submissionLabels[submission.status]
              : "",
          reason: submission?.reason || "",
          rejected: submission?.status === SubmissionStatus.SubmissionRejected,
          reissued: invalid,
          readOnly,
          linkLocked,
          showPlayback: reviewed && !!submittedURL,
        };
      });
      const deadline = taskDeadline(task);
      const summary = workCard(task);
      const progress = taskProgress(task);
      const platformNames = platforms.map((platform) => platform.label);
      const timeline = executionTimeline(progress, platformNames);
      const hasPlaybackPlatforms = platforms.some(
        ({ showPlayback }) => showPlayback,
      );
      const openScreenshots =
        this.section === "screenshots" && hasPlaybackPlatforms;
      const openStage = openScreenshots
        ? TaskStep.StepLinks
        : this.data.task?.timeline?.active === timeline.active
          ? this.data.openStage
          : timeline.active;

      this.setData({
        task: {
          ...task,
          summary,
          files,
          price: formatFen(task.price_fen),
          dualPlatformPrice:
            platforms.length === 2 ? formatFen(doubleFen(task.price_fen)) : "",
          statusLabel: assignmentLabels[task.status],
          sourceLabel: sourceLabels[task.source],
          showSource: task.source !== AssignmentSource.SourceClaim,
          reissued,
          expired: task.status === AssignmentStatus.AssignmentExpired,
          closed,
          platformText: platformNames.join(" + "),
          appNames: platformNames.join("、"),
          requirements: taskRequirements(task.body),
          contentDone: progress.items[0].state === "done",
          publishDone: progress.items[1].state === "done",
          canSubmitLinks: platforms.some(({ linkLocked }) => !linkLocked),
          hasPlaybackPlatforms,
          playbackDeadline: beijingDateTime(task.playback_deadline),
          progress,
          timeline,
          filesUpdated:
            !!original &&
            JSON.stringify(files) !== JSON.stringify(original.files || []),
          bodyUpdated: !!original && task.body !== original.body,
        },
        platforms,
        links,
        screenshots,
        screenshotsOpen:
          hasPlaybackPlatforms && (openScreenshots || this.data.screenshotsOpen),
        deadline,
        openStage,
      });
      if (!closed && openStage === TaskStep.StepContent) {
        this.loadMaterials(files);
      }
      if (this.visible && deadline && deadline.remaining !== "等待系统确认") {
        this.deadlineTimer = setInterval(() => {
          const current = taskDeadline(task);
          this.setData({ deadline: current });
          if (current?.remaining === "等待系统确认") {
            clearInterval(this.deadlineTimer);
            this.load();
          }
        }, 1000);
      }
    } catch (error) {
      if (!this.unloaded) this.setData({ error: error.message });
    } finally {
      this.refreshing = false;
      if (this.unloaded) return;
      this.setData({ loading: false }, () => {
        const target =
          this.section === "screenshots" && this.data.screenshotsOpen
            ? "screenshots"
            : this.scrollToActiveStage && this.data.openStage
              ? `stage-${this.data.openStage}`
              : "";
        this.section = "";
        this.scrollToActiveStage = false;
        if (target && !this.data.error) {
          wx.pageScrollTo({ selector: `#${target}`, duration: 0 });
        }
      });
    }
  },

  async loadMaterials(files) {
    if (!files.length || this.data.loadingMaterials) {
      return;
    }

    this.setData({ loadingMaterials: true });
    this.materialMetadata ??= new Map();
    const tickets = await Promise.all(
      files.map(async (id) => {
        try {
          const ticket = await materialTicket(id);
          this.materialMetadata.set(id, {
            name: ticket.name,
            media_type: ticket.media_type,
            storage: ticket.storage,
          });
          return ticket;
        } catch {
          return this.materialMetadata.get(id);
        }
      }),
    );
    if (!this.unloaded) {
      const materials = presentMaterials(files, tickets);
      const galleryIndex = Math.min(
        this.data.galleryIndex,
        Math.max(materials.images.length - 1, 0),
      );
      this.setData({
        materials,
        galleryOpen: this.data.galleryOpen && !!materials.images.length,
        galleryIndex,
        loadingMaterials: false,
      });
    }
  },

  toggleStage(event) {
    const selected = event.currentTarget.dataset;
    if (selected.state === "waiting") {
      return;
    }

    const openStage =
      this.data.openStage === selected.stage ? "" : selected.stage;
    this.setData({ openStage });
    if (openStage === TaskStep.StepContent) {
      this.loadMaterials(this.data.task.files);
    }
  },

  goToStage(event) {
    const { stage } = event.currentTarget.dataset;
    this.setData({ openStage: stage }, () => {
      wx.pageScrollTo({ selector: `#stage-${stage}`, duration: 0 });
    });
  },

  imageUnavailable(event) {
    const { index } = event.currentTarget.dataset;
    if (this.data.materials.images[index]?.imageURL) {
      this.setData({ [`materials.images[${index}].imageURL`]: "" });
    }
  },

  openGallery() {
    const { images } = this.data.materials;
    if (!images.length) {
      return;
    }

    this.setData({ galleryOpen: true, galleryIndex: 0 });
    const now = Date.now();
    if (
      this.data.task?.files &&
      images.some(
        ({ imageURL, expiresAt }) => !imageURL || Date.parse(expiresAt) <= now,
      )
    ) {
      return this.loadMaterials(this.data.task.files);
    }
  },

  closeGallery() {
    this.setData({ galleryOpen: false });
  },

  selectGalleryImage(event) {
    if (event.detail?.source === "") {
      return;
    }

    const index = event.detail?.current ?? event.currentTarget?.dataset?.index;
    if (this.data.materials.images[index] && index !== this.data.galleryIndex) {
      this.setData({ galleryIndex: index });
    }
  },

  async retryGalleryImage(event) {
    const { index } = event.currentTarget.dataset;
    const image = this.data.materials.images[index];
    if (!image || this.data.refreshingImage) {
      return;
    }

    this.setData({ refreshingImage: image.id });
    try {
      const ticket = await materialTicket(image.id);
      const refreshed = presentMaterials([image.id], [ticket]).images[0];
      if (!refreshed?.imageURL) {
        throw new Error("图片暂时无法预览");
      }

      if (
        !this.unloaded &&
        this.data.materials.images[index]?.id === image.id
      ) {
        this.setData({
          [`materials.images[${index}].imageURL`]: refreshed.imageURL,
          [`materials.images[${index}].expiresAt`]: refreshed.expiresAt,
        });
      }
    } catch (error) {
      if (!this.unloaded) {
        wx.showToast({ title: error.message, icon: "none" });
      }
    } finally {
      if (!this.unloaded) {
        this.setData({ refreshingImage: "" });
      }
    }
  },

  changeLink(event) {
    const platform = event.currentTarget.dataset.platform;
    const text = event.detail.value;
    this.drafts[platform] = { ...this.drafts[platform], text };
    this.setData({ [`links.${platform}`]: text });
  },

  async complete(event) {
    if (!this.data.task || this.data.task.closed || this.data.completing) {
      return;
    }

    this.setData({ completing: true });
    try {
      await post(`/assignments/${this.id}/steps`, {
        step: event.currentTarget.dataset.step,
      });
      this.scrollToActiveStage = true;
      await this.load();
    } catch (error) {
      wx.showToast({ title: error.message, icon: "none" });
    } finally {
      this.setData({ completing: false });
    }
  },

  async uploadScreenshot(event) {
    const platform = this.data.platforms.find(
      (item) => item.key === event.currentTarget.dataset.platform,
    );
    if (
      !platform?.showPlayback ||
      platform.readOnly ||
      this.data.uploading ||
      this.data.saving
    ) {
      return;
    }

    this.setData({ uploading: platform.key });
    try {
      const path = await chooseImage();
      wx.showLoading({ title: "正在上传并确认", mask: true });
      const id = await upload(path);
      this.drafts[platform.key] = {
        ...this.drafts[platform.key],
        screenshot: id,
      };
      this.setData({ [`screenshots.${platform.key}`]: id });
      wx.showToast({ title: "截图已上传，请提交", icon: "none" });
    } catch (error) {
      wx.showToast({ title: error.message, icon: "none" });
    } finally {
      this.setData({ uploading: "" });
      wx.hideLoading();
    }
  },

  async submitLinks() {
    const { platforms, links, screenshots, saving, uploading, loading } =
      this.data;
    if (saving || uploading || loading) {
      return;
    }

    const editable = platforms.filter(
      ({ key, linkLocked }) => !linkLocked && links[key]?.trim(),
    );
    const selected = editable.filter(
      ({ key, rejected, submittedText }) =>
        links[key].trim() !== submittedText.trim() ||
        (rejected && this.drafts[key]?.text !== undefined),
    );

    // A single rejected work may be corrected in the platform app at the same URL.
    if (!selected.length) {
      const rejected = editable.filter((platform) => platform.rejected);
      if (rejected.length === 1) {
        selected.push(rejected[0]);
      }
    }

    if (!selected.length) {
      wx.showToast({ title: "请粘贴要提交的平台发布链接", icon: "none" });
      return;
    }

    const records = [];
    for (const platform of selected) {
      const text = links[platform.key].trim();
      const url =
        text === platform.submittedURL
          ? platform.submittedURL
          : publicationLink(platform.key, text);
      if (!url) {
        wx.showToast({
          title: `请粘贴${platform.label}的作品分享链接`,
          icon: "none",
        });
        return;
      }

      records.push({
        platform,
        url,
        text: links[platform.key],
        screenshot: screenshots[platform.key] || "",
      });
    }

    this.setData({ saving: "links" });
    if (wx.requestSubscribeMessage) await subscribe("links");
    const errors = [];
    try {
      for (const { platform, url, text, screenshot } of records) {
        try {
          await this.saveSubmission(platform.key, url, screenshot);
          this.submittedShareTexts[platform.key] = { url, text };
          delete this.drafts[platform.key];
        } catch (error) {
          errors.push(`${platform.label}：${error.message}`);
        }
      }

      if (errors.length < records.length) {
        await this.load();
      }
      if (errors.length) {
        wx.showToast({ title: errors.join("\n"), icon: "none" });
      } else {
        wx.showToast({ title: "已提交审核", icon: "success" });
      }
    } finally {
      this.setData({ saving: "" });
    }
  },

  async submitScreenshot(event) {
    const platform = this.data.platforms.find(
      (item) => item.key === event.currentTarget.dataset.platform,
    );
    if (
      !platform?.showPlayback ||
      platform.readOnly ||
      this.data.loading ||
      this.data.saving ||
      this.data.uploading
    ) {
      return;
    }

    const screenshot = this.data.screenshots[platform.key] || "";
    if (!screenshot || screenshot === platform.submittedScreenshot) {
      return;
    }

    this.setData({ saving: platform.key });
    try {
      if (wx.requestSubscribeMessage) await subscribe("screenshots");
      await this.saveSubmission(
        platform.key,
        platform.submittedURL,
        screenshot,
      );
      delete this.drafts[platform.key]?.screenshot;
      wx.showToast({ title: "播放量截图已提交", icon: "success" });
      await this.load();
    } catch (error) {
      wx.showToast({ title: error.message, icon: "none" });
    } finally {
      this.setData({ saving: "" });
    }
  },

  saveSubmission(platform, url, screenshot) {
    return post(`/assignments/${this.id}/submissions`, {
      platform,
      url,
      screenshot,
    });
  },

  save(event) {
    return this.saveFile(event.currentTarget.dataset.id);
  },

  saveGalleryImage() {
    return this.saveFile(
      this.data.materials.images[this.data.galleryIndex]?.id,
    );
  },

  async saveFile(id) {
    if (!id || this.data.savingFile) {
      return;
    }

    this.setData({ savingFile: id });
    try {
      await saveMaterial(id);
      const savedFiles = { ...this.data.savedFiles, [id]: true };
      this.setData({
        savedFiles,
        savedFileCount: Object.keys(savedFiles).length,
      });
      wx.showToast({ title: "已保存到相册", icon: "success" });
    } catch (error) {
      wx.showToast({ title: error.message, icon: "none" });
    } finally {
      this.setData({ savingFile: "" });
    }
  },

  copy(event) {
    const { value } = event.currentTarget.dataset;
    wx.setClipboardData({ data: value });
  },

  async loadAssignment() {
    let task;
    let original;
    if (this.id) {
      task = takeClaimedAssignment(this.id) || (await request(`/assignments/${this.id}`));
    } else if (this.reissueOf) {
      const assignments = await request("/assignments");
      task = assignments.find(
        (item) =>
          item.source === AssignmentSource.SourceReissued &&
          item.source_id === this.reissueOf &&
          item.platforms.includes(this.reissuePlatform),
      );
      if (!task) {
        if (this.visible) wx.switchTab({ url: "/pages/work/index" });
        return {};
      }
      this.id = task.id;
      original = assignments.find((item) => item.id === this.reissueOf);
    } else {
      throw new Error("缺少任务信息");
    }
    if (!original && task.source === AssignmentSource.SourceReissued && task.source_id) {
      original = await request(`/assignments/${task.source_id}`).catch(() => null);
    }
    return { task, original };
  },

  openReissue(event) {
    const platform = event.currentTarget.dataset.platform;
    wx.redirectTo({
      url: `/subpackages/task/detail/index?reissueOf=${encodeURIComponent(this.id)}&platform=${encodeURIComponent(platform)}`,
      fail: () => wx.showToast({ title: "任务打开失败，请重试", icon: "none" }),
    });
  },

  openPlayback() {
    wx.navigateTo({ url: "/subpackages/task/playback/index" });
  },

  goWork() {
    wx.switchTab({ url: "/pages/work/index" });
  },

  goHome() {
    wx.switchTab({ url: "/pages/home/index" });
  },

  goWallet() {
    wx.switchTab({ url: "/pages/wallet/index" });
  },
});
