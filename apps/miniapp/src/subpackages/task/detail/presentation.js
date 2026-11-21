const {
  FileMediaType,
  FileStorage,
  Platform,
  TaskStep,
} = require("../../../services/generated/enums");

const shareLinks = {
  [Platform.PlatformDouyin]: {
    hosts: ["v.douyin.com", "www.douyin.com"],
    example: "复制打开抖音，看看【xxx的作品】…\nhttps://v.douyin.com/xxxx/",
  },
  [Platform.PlatformXiaohongshu]: {
    hosts: ["xhslink.cn", "xhslink.com", "www.xiaohongshu.com"],
    example: "我发布了一篇笔记：xxx…\nhttps://xhslink.cn/o/xxxx",
  },
};

function sharePlaceholder(platform, label) {
  return `直接粘贴${label}的分享文字，例：${shareLinks[platform]?.example || ""}`;
}

function publicationLink(platform, text) {
  const hosts = shareLinks[platform]?.hosts || [];
  // Share text may escape the colon in the URL scheme.
  const candidates =
    text.match(/https?\\?:\/\/[^\s<>"'，。；！？、：…（）【】《》「」“”]+/gi) ||
    [];
  for (const candidate of candidates) {
    const link = candidate.replace(/[.,;!]+$/, "");
    const match = /^https?\\?:\/\/([a-z\d.-]+)(\/[^/?#]+[^\s]*)$/i.exec(link);
    if (match && hosts.includes(match[1].toLowerCase())) {
      return `https://${match[1].toLowerCase()}${match[2]}`;
    }
  }

  return "";
}

function executionTimeline(progress, platforms) {
  const rewardState =
    progress.items[4].state === "waiting"
      ? progress.items[3].state === "current"
        ? "current"
        : "waiting"
      : progress.items[4].state;
  const definitions = [
    [TaskStep.StepContent, "拿内容和要求", progress.items[0].state],
    [
      TaskStep.StepPublish,
      `发${platforms.join("和")}`,
      progress.items[1].state,
    ],
    [
      TaskStep.StepLinks,
      platforms.length === 2 ? "交两条链接" : "交链接",
      progress.items[2].state,
    ],
    [TaskStep.StepSettle, "奖励记账", rewardState],
  ];
  const items = definitions.map(([key, title, state], index) => ({
    key,
    title,
    state,
    mark: state === "done" ? "✓" : index + 1,
  }));
  const current = items.find(
    ({ state }) => state === "current" || state === "rejected",
  );

  return { items, active: current?.key || TaskStep.StepSettle };
}

function presentMaterials(files, tickets = []) {
  const images = [];
  const videos = [];
  const attachments = [];

  files.forEach((id, index) => {
    const ticket = tickets[index];
    const name = ticket?.name || `素材 ${index + 1}`;
    if (ticket?.media_type === FileMediaType.FileMP4) {
      videos.push({ id, name, title: name.replace(/\.mp4$/i, "") });
    } else if (ticket?.media_type?.startsWith("image/")) {
      images.push({
        id,
        name,
        imageURL:
          ticket.storage === FileStorage.FileStorageTOS
            ? ticket.url || ""
            : ticket.previewPath || "",
        expiresAt: ticket.expires_at || "",
      });
    } else {
      attachments.push({ id, name });
    }
  });

  return { images, videos, attachments };
}

function taskRequirements(body = "") {
  const copy = [];
  let tags = "";
  let mention = "";
  let topicNote = "";

  for (const line of body.split("\n")) {
    if (/^TAG[：:]/i.test(line)) {
      tags = line.slice(4).trim();
    } else if (/^@账号[：:]/.test(line)) {
      mention = line.slice(4).trim();
    } else if (/^话题至少带满 \d+ 个（含必带 TAG）\s*$/.test(line)) {
      topicNote = line.trim();
    } else {
      copy.push(line);
    }
  }

  const fields = [
    { label: "分发语", value: copy.join("\n").trim() },
    { label: "TAG", value: tags },
    { label: "@账号", value: mention },
  ].filter(({ value }) => value);

  return { fields, topicNote };
}

module.exports = {
  executionTimeline,
  presentMaterials,
  publicationLink,
  sharePlaceholder,
  taskRequirements,
};
