const { marketingBusinessDate } = require("../../../services/marketing-tasks");
const day = (value) => marketingBusinessDate(Date.parse(value));
const { request, post, newKey } = require("../../../services/request");
const { formatFen } = require("../../../services/money");
const {
  chooseImage,
  upload,
  saveMaterial,
  materialTicket,
} = require("../../../services/files");
const { publicationLink } = require("../detail/presentation");
const labels = {
  draft: "已领取",
  pending: "待审作品",
  rejected: "已驳回",
  approved: "可以发布",
  linked: "待核点赞",
  rewarded: "奖励已核定",
};
const platformName = (p) => (p === "dy" ? "抖音" : "小红书");
function taskView(t) {
  return {
    ...t,
    fee: formatFen(t.creation_fen),
    rate: formatFen(t.publish_fen),
    platformNames: t.platforms.map(platformName).join("、"),
    tiers: t.tiers.map((x) => ({ ...x, amount: formatFen(x.reward_fen) })),
    start: day(t.starts_at),
    end: day(t.ends_at),
    award: day(t.reward_at),
  };
}
function workView(w) {
  const paid =
    w.status === "rewarded" &&
    (w.earnings || []).every((e) => e.amount_fen === "0" || e.batch_id);
  return {
    ...w,
    task: taskView(w.task),
    label: paid ? "已结算" : labels[w.status],
    links: (w.links || []).map((l) => {
      const tier =
        l.likes === null || w.task.combined_likes
          ? undefined
          : [...w.task.tiers].reverse().find((t) => t.likes <= l.likes);
      return {
        ...l,
        label: platformName(l.platform),
        reward:
          l.likes === null || w.task.combined_likes
            ? ""
            : tier
              ? "奖励 ¥" + formatFen(tier.reward_fen)
              : "未达奖励档",
      };
    }),
    earnings: (w.earnings || []).map((e) => ({
      ...e,
      amount: formatFen(e.amount_fen),
      date: day(e.eligible_at),
    })),
  };
}
const { subscribe } = require("../../../services/subscriptions");

Page({
  data: {
    loading: true,
    error: "",
    busy: false,
    work: null,
    task: null,
    mine: [],
    tasks: [],
    files: [],
    copy: "",
    urls: {},
    platformRows: [],
    mineOnly: false,
    steps: [],
  },
  onLoad(query) {
    this.query = query;
    this.setData({ mineOnly: query.mine === "1" });
  },
  onShow() {
    return this.load();
  },
  onPullDownRefresh() {
    return this.load().finally(() => wx.stopPullDownRefresh());
  },
  async load() {
    try {
      const catalog = await request("/creative/catalog");
      const mine = catalog.mine.map(workView),
        tasks = catalog.tasks.map(taskView);
      const w =
        mine.find(
          (w) => w.id === this.query.work || w.task.id === this.query.task,
        ) || null;
      const t = w?.task || tasks.find((t) => t.id === this.query.task) || null;
      if ((this.query.work || this.query.task) && !t)
        throw new Error("任务已下架或无法访问");
      this.setData({
        tasks,
        mine,
        work: w,
        task: t,
        online: catalog.online,
        limit: catalog.limit,
        used: catalog.used,
        error: "",
        platformRows: (t?.platforms || []).map((p) => ({
          id: p,
          label: platformName(p),
        })),
        steps: ["领取", "交作品", "审核作品", "发布交链接", "兑奖", "结算"],
        stage: w
          ? {
              draft: 1,
              pending: 2,
              rejected: 1,
              approved: 3,
              linked: 4,
              rewarded: 5,
            }[w.status]
          : 0,
      });
      if (w && this.loadedRevision !== w.id + ":" + w.revision) {
        this.setData({ files: w.files || [], copy: w.copy_text || "" });
        this.loadedRevision = w.id + ":" + w.revision;
      }
    } catch (e) {
      this.setData({ error: e.message });
    } finally {
      this.setData({ loading: false });
    }
  },
  select(e) {
    const { task, work } = e.currentTarget.dataset;
    wx.navigateTo({
      url:
        "/subpackages/task/creative/index?" +
        (work ? "work=" + work : "task=" + task),
    });
  },
  async act(fn) {
    if (this.data.busy) return;
    this.setData({ busy: true, actionError: "" });
    try {
      await fn();
      await this.load();
    } catch (e) {
      this.setData({ actionError: e.message });
    } finally {
      this.setData({ busy: false });
    }
  },
  claim() {
    return this.act(async () => {
      this.key = this.key || newKey();
      this.query.work = await post(
        "/creative/tasks/" + this.data.task.id + "/claim",
        {},
        this.key,
      );
    });
  },
  editCopy(e) {
    this.setData({ copy: e.detail.value });
  },
  editUrl(e) {
    this.setData({
      ["urls." + e.currentTarget.dataset.platform]: e.detail.value,
    });
  },
  async selectFiles() {
    if (this.data.busy) return;
    this.setData({ busy: true, actionError: "" });
    try {
      const selected = await new Promise((resolve, reject) =>
        wx.chooseMedia({
          count: this.data.task.media_type === "video/mp4" ? 1 : 9,
          mediaType: [
            this.data.task.media_type === "video/mp4" ? "video" : "image",
          ],
          success: resolve,
          fail: () => reject(new Error("未选择作品")),
        }),
      );
      const ids = [];
      for (const file of selected.tempFiles) {
        ids.push(await upload(file.tempFilePath));
      }
      this.setData({ files: ids });
    } catch (e) {
      this.setData({ actionError: e.message });
    } finally {
      this.setData({ busy: false });
    }
  },
  submit() {
    return this.act(async () => {
      if (wx.requestSubscribeMessage) await subscribe("creative");
      return post("/creative/works/" + this.data.work.id + "/submit", {
        revision: this.data.work.revision,
        copy_text: this.data.copy,
        files: this.data.files,
      });
    });
  },
  submitLinks() {
    return this.act(async () => {
      const links = this.data.task.platforms.map((platform) => {
        const url = publicationLink(platform, this.data.urls[platform] || "");
        if (!url)
          throw new Error("请填写" + platformName(platform) + "的作品链接");
        return { platform, url };
      });
      if (wx.requestSubscribeMessage) await subscribe("screenshots");
      await post("/creative/works/" + this.data.work.id + "/links", { links });
    });
  },
  evidence(e) {
    const platform = e.currentTarget.dataset.platform;
    return this.act(async () => {
      if (wx.requestSubscribeMessage) await subscribe("creative");
      const screenshot = await upload(await chooseImage());
      await post("/creative/works/" + this.data.work.id + "/evidence", {
        platform,
        screenshot,
      });
    });
  },
  async preview(e) {
    try {
      const ticket = await materialTicket(e.currentTarget.dataset.id);
      const src = ticket.previewPath || ticket.url;
      wx.previewImage({ urls: [src] });
    } catch (e) {
      this.setData({ actionError: e.message });
    }
  },
  async save(e) {
    try {
      await saveMaterial(e.currentTarget.dataset.id);
    } catch (e) {
      this.setData({ actionError: e.message });
    }
  },
  distribution() {
    const pages = getCurrentPages();
    if (
      [
        "subpackages/task/today/index",
        "subpackages/task/market/index",
      ].includes(pages[pages.length - 2]?.route)
    )
      wx.navigateBack();
    else wx.redirectTo({ url: "/subpackages/task/today/index" });
  },
});
