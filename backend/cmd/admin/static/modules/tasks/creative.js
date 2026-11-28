import { post } from "../../api/client.js";
import { upload, download } from "../../api/files.js";

function fen(raw) {
  const value = String(raw).trim();
  if (!/^\d+(\.\d{1,2})?$/.test(value))
    throw new Error("金额请填写非负元数，最多两位小数");
  const [whole, part = ""] = value.split(".");
  return (whole + part.padEnd(2, "0")).replace(/^0+(?=\d)/, "");
}

async function createTask(data) {
  const amounts = data.getAll("tier_reward");
  const tiers = data.getAll("tier_likes").map((likes, i) => ({
    likes: Number(likes),
    reward_fen: fen(amounts[i]),
  }));
  const files = [];
  for (const file of data.getAll("materials")) {
    if (file.size) files.push(await upload(file));
  }
  return post("/creative/tasks", {
    title: data.get("title"),
    requirements: data.get("requirements"),
    copy_text: data.get("copy_text"),
    topics: data.get("topics"),
    media_type: data.get("media_type"),
    files,
    platforms: data.getAll("platforms"),
    creation_fen: fen(data.get("fee")),
    tiers,
    combined_likes: data.has("combined"),
    publish_days: Number(data.get("days")),
    starts_at: data.get("start") + ":00+08:00",
    ends_at: data.get("end") + ":00+08:00",
    reward_at: data.get("reward") + ":00+08:00",
  });
}

document.addEventListener("submit", async (event) => {
  const form = event.target.closest("form[data-creative-action]");
  if (!form) return;
  event.preventDefault();
  const button = event.submitter;
  if (form.dataset.busy || !button) return;
  if (button.dataset.confirm && !window.confirm(button.dataset.confirm)) return;
  const output = form.querySelector("output");
  const data = new FormData(form);
  form.dataset.busy = "true";
  button.disabled = true;
  output.textContent = "正在提交";
  const id = form.dataset.id;
  try {
    switch (form.dataset.creativeAction) {
      case "create":
        await createTask(data);
        break;
      case "visibility":
        await post("/creative/tasks/" + id + "/visibility", {
          online: form.dataset.online === "true",
        });
        break;
      case "review":
        await post("/creative/works/" + id + "/review", {
          revision: Number(form.dataset.revision),
          approve: button.value === "approve",
          reason: button.value === "approve" ? "" : data.get("reason"),
        });
        break;
      case "award": {
        const inputs = [...form.querySelectorAll("input[data-platform]")];
        if (inputs.some((input) => !input.dataset.shot || !input.value))
          throw new Error("请先收齐截图并逐个平台填写点赞数");
        await post("/creative/works/" + id + "/award", {
          links: inputs.map((input) => ({
            platform: input.dataset.platform,
            screenshot: input.dataset.shot,
            likes: Number(input.value),
          })),
        });
        break;
      }
    }
    location.reload();
  } catch (error) {
    output.textContent = error.message;
  } finally {
    delete form.dataset.busy;
    button.disabled = false;
  }
});
document.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-creative-file]");
  if (!button) return;
  try {
    await download(button.dataset.creativeFile);
  } catch (error) {
    window.alert(error.message);
  }
});
document.addEventListener("click", async (e) => {
  const b = e.target.closest("[data-return-evidence]");
  if (!b) return;
  const reason = window.prompt("请输入退回原因，分发员将收到补交截图通知。");
  if (!reason?.trim()) return;
  b.disabled = true;
  try {
    await post("/creative/works/" + b.dataset.work + "/return", {
      platform: b.dataset.platform,
      screenshot: b.dataset.shot,
      reason,
    });
    location.reload();
  } catch (error) {
    window.alert(error.message);
    b.disabled = false;
  }
});
document.addEventListener("click", (e) => {
  if (e.target.closest("[data-creative-create]"))
    document.querySelector(".creative-editor").showModal();
});

document.addEventListener("click", (event) => {
  const group = event.target.closest("[data-creative-tiers]");
  if (!group) return;
  if (event.target.closest("[data-add-tier]")) {
    const row = group.querySelector("[data-creative-tier]").cloneNode(true);
    for (const input of row.querySelectorAll("input")) input.value = "";
    group.insertBefore(row, group.querySelector("[data-add-tier]"));
    row.querySelector("input").focus();
  } else if (event.target.closest("[data-remove-tier]")) {
    event.target.closest("[data-creative-tier]").remove();
  } else return;
  const buttons = group.querySelectorAll("[data-remove-tier]");
  for (const button of buttons) button.disabled = buttons.length === 1;
});
