import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { ConfigProvider } from "antd";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { get, getPage, post, sessionKey } from "../api/client";
import { AuditAction, EnrollmentAge } from "../generated/enums";
import { Application } from "./App";

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  get: vi.fn(),
  getPage: vi.fn(),
  post: vi.fn(),
}));

beforeEach(() => {
  sessionStorage.clear();
  window.history.replaceState(null, "", "/tasks");
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("未登录时保护运营页面，只显示登录入口", () => {
  render(<Application />);

  expect(screen.getByRole("button", { name: "进入运营后台" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "上传营销内容" })).toBeNull();
  expect(screen.getByLabelText("用户名")).toBeTruthy();
});

it("没有页面权限时隐藏菜单并阻止直接访问", async () => {
  sessionStorage.setItem(sessionKey, "test-operator-session");
  vi.mocked(get).mockResolvedValue({
    id: "operator",
    username: "reviewer",
    pages: [],
    endpoints: ["GET /tasks"],
    super_admin: false,
  });

  render(<Application />);

  expect(await screen.findByText("无权访问此页面")).toBeTruthy();
  expect(
    screen
      .getAllByRole("menuitem", { name: "任务管理" })
      .every((item) => item.getAttribute("aria-disabled") === "true"),
  ).toBe(true);
  expect(get).toHaveBeenCalledExactlyOnceWith("/auth/me");
});

it("只读运营可以查看获授权的任务页面，但没有写入按钮", async () => {
  vi.clearAllMocks();
  sessionStorage.setItem(sessionKey, "read-only-operator");
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/tasks"],
        endpoints: ["GET /tasks"],
        super_admin: false,
      };
    }
    if (path === "/tasks") {
      return [];
    }

    throw new Error(`Unpermitted request: ${path}`);
  });

  render(<Application />);

  expect(
    await screen.findByRole(
      "heading",
      { name: "营销分发 · 任务管理" },
      { timeout: 5_000 },
    ),
  ).toBeTruthy();
  expect(
    screen
      .getAllByRole("menuitem", { name: "任务管理" })
      .filter((item) => item.getAttribute("aria-disabled") !== "true"),
  ).toHaveLength(1);
  expect(screen.queryByRole("menuitem", { name: "运营账号" })).toBeNull();
  expect(screen.queryByRole("button", { name: "上传营销内容" })).toBeNull();
  expect(vi.mocked(get).mock.calls.map(([path]) => path)).toEqual([
    "/auth/me",
    "/tasks",
  ]);
});

it("新建营销任务直接进入内容池，并在池内显示指派入口", async () => {
  vi.clearAllMocks();
  sessionStorage.setItem(sessionKey, "task-operator");
  const task = {
    id: "version-one",
    task_id: "task-one",
    title: "测试营销内容",
    body: "按要求发布",
    project: "测试艺人",
    platforms: ["dy", "xhs"],
    price_fen: "750",
    online: true,
    created_at: "2026-11-19T09:00:00+08:00",
    number: 1,
  };
  let created = false;
  vi.mocked(get).mockImplementation(async (requestPath) => {
    if (requestPath === "/auth/me") {
      return {
        id: "operator",
        username: "operator",
        pages: ["/tasks"],
        endpoints: [
          "GET /tasks",
          "POST /tasks",
          "GET /users",
          "POST /tasks/:id/assign",
        ],
        super_admin: false,
      };
    }
    if (requestPath === "/tasks") return created ? [task] : [];
    if (requestPath === "/users") return [];
    throw new Error(`Unexpected request: ${requestPath}`);
  });
  vi.mocked(post).mockImplementation(async (requestPath) => {
    expect(requestPath).toBe("/tasks");
    created = true;
    return task;
  });

  render(
    <ConfigProvider theme={{ token: { motion: false } }}>
      <Application />
    </ConfigProvider>,
  );
  const upload = await screen.findByRole("button", { name: "上传营销内容" });
  const settingsSection = screen.getByRole("region", { name: "营销设置" });
  const pool = await screen.findByRole("table", { name: "营销内容" });
  expect(
    settingsSection.compareDocumentPosition(upload) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(screen.queryByRole("table", { name: "营销分发总览" })).toBeNull();
  expect(
    within(pool)
      .getAllByRole("columnheader")
      .map((header) => header.textContent),
  ).toEqual([
    "ID",
    "艺人",
    "类型 / 平台",
    "标题",
    "内容预览",
    "内容（分发语 / TAG）",
    "标签",
    "状态",
    "分发员",
    "操作",
  ]);
  fireEvent.click(upload);
  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByRole("radio", { name: "视频" })).toBeTruthy();
  expect(within(dialog).getByRole("radio", { name: "图文" })).toBeTruthy();
  expect(
    within(dialog).getByRole("radio", { name: "抖音 + 小红书" }),
  ).toBeTruthy();
  expect(within(dialog).getByLabelText("话题至少带满")).toBeTruthy();
  expect(within(dialog).queryByLabelText("从内容池选取素材")).toBeNull();
  fireEvent.change(within(dialog).getByLabelText("任务标题"), {
    target: { value: task.title },
  });
  fireEvent.change(within(dialog).getByLabelText("项目（艺人）"), {
    target: { value: task.project },
  });
  fireEvent.change(within(dialog).getByLabelText("分发语"), {
    target: { value: task.body },
  });
  fireEvent.click(within(dialog).getByRole("button", { name: "上传到内容池" }));
  expect(await screen.findByText(task.title)).toBeTruthy();
  await waitFor(() =>
    expect(screen.queryByRole("dialog", { name: /新建任务/ })).toBeNull(),
  );
  expect(within(pool).getByText("测试艺人")).toBeTruthy();
  fireEvent.change(screen.getByLabelText("艺人"), {
    target: { value: "测试艺人" },
  });
  expect(screen.getByRole("button", { name: "全部 1" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "已发布 0" }));
  expect(within(pool).getByText("没有符合筛选的内容")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "排队中 1" }));
  expect(within(pool).getByRole("button", { name: "手动指派" })).toBeTruthy();
  fireEvent.click(within(pool).getByRole("button", { name: "详情" }));
  expect(
    await screen.findByRole("dialog", { name: /测试营销内容/ }),
  ).toBeTruthy();
  expect(post).toHaveBeenCalledExactlyOnceWith(
    "/tasks",
    expect.objectContaining({
      title: task.title,
      body: `${task.body}\n话题至少带满 5 个（含必带 TAG）`,
      project: task.project,
    }),
  );
});

it("营销内容可从列表修改和预览，详情只保留关闭操作", async () => {
  vi.clearAllMocks();
  sessionStorage.setItem(sessionKey, "task-edit-operator");
  const task = {
    id: "version-one",
    task_id: "edit-task",
    title: "测试素材",
    body: "原分发语\nTAG：#原话题\n@账号：@原账号\n话题至少带满 3 个（含必带 TAG）",
    project: "测试艺人",
    platforms: ["xhs"],
    content_id: "original-content",
    files: ["video-file", "photo-file"],
    media_types: ["video/mp4", "image/png"],
    price_fen: "750",
    online: true,
    deadline: "2099-12-31",
    created_at: "2026-11-19T09:00:00+08:00",
    number: 1,
  };
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "operator",
        pages: ["/tasks"],
        endpoints: [
          "GET /tasks",
          "GET /users",
          "GET /files/:id/download",
          "POST /tasks/:id/assign",
          "POST /tasks/:id/versions",
          "POST /assignments/:id/revoke",
        ],
        super_admin: false,
      };
    }
    if (path === "/tasks") return [task];
    if (path === "/users") return [];
    if (path.startsWith("/files/") && path.endsWith("/download")) {
      const video = path.includes("video-file");
      return {
        storage: "tos",
        name: video ? "示例视频.mp4" : "示例图片.png",
        media_type: video ? "video/mp4" : "image/png",
        url: video
          ? "https://media.example/video"
          : "https://media.example/photo",
      };
    }
    throw new Error(`Unexpected request: ${path}`);
  });
  vi.mocked(post).mockResolvedValue(task);

  render(
    <ConfigProvider theme={{ token: { motion: false } }}>
      <Application />
    </ConfigProvider>,
  );
  const row = await screen.findByRole("row", { name: /测试素材/ });
  const cells = within(row).getAllByRole("cell");
  const actions = cells.at(-1)!;
  expect(
    within(actions)
      .getAllByRole("button")
      .map((button) => button.textContent),
  ).toEqual(["手动指派", "修改", "详情"]);
  expect(within(cells[2]).queryByRole("button")).toBeNull();
  const cover = within(cells[4]).getByRole("button", { name: "预览 2 个附件" });
  expect(within(cover).getByText("2")).toBeTruthy();
  fireEvent.click(cover);
  const video = await screen.findByRole("dialog", { name: "附件预览" });
  await waitFor(() =>
    expect(video.querySelector("video[controls]")?.getAttribute("src")).toBe(
      "https://media.example/video",
    ),
  );
  const thumbnails = within(video).getByRole("group", { name: "附件缩略图" });
  const firstThumbnail = within(thumbnails).getByRole("button", {
    name: "切换到附件 1",
  });
  const secondThumbnail = within(thumbnails).getByRole("button", {
    name: "切换到附件 2",
  });
  expect(firstThumbnail.getAttribute("aria-pressed")).toBe("true");
  fireEvent.click(secondThumbnail);
  expect(secondThumbnail.getAttribute("aria-pressed")).toBe("true");
  expect(video.querySelector("video[controls]")).toBeNull();
  expect(
    (
      await within(video).findByRole("img", { name: "示例图片.png" })
    ).getAttribute("src"),
  ).toBe("https://media.example/photo");
  fireEvent.click(firstThumbnail);
  await waitFor(() =>
    expect(video.querySelector("video[controls]")?.getAttribute("src")).toBe(
      "https://media.example/video",
    ),
  );
  fireEvent.click(within(video).getByRole("button", { name: "关闭" }));

  fireEvent.click(within(actions).getByRole("button", { name: "详情" }));
  const detail = await screen.findByRole("dialog", { name: /测试素材/ });
  expect(
    within(detail).queryByRole("button", { name: /内容维护|手动指派/ }),
  ).toBeNull();
  fireEvent.click(within(detail).getByRole("button", { name: "预览附件 2" }));
  const previewTitle = await screen.findByText("附件预览", { exact: true });
  const image = previewTitle.closest<HTMLElement>('[role="dialog"]');
  if (!image) throw new Error("图片预览未在弹框打开");
  expect(
    (
      await within(image).findByRole("img", { name: "示例图片.png" })
    ).getAttribute("src"),
  ).toBe("https://media.example/photo");
  fireEvent.click(within(image).getByRole("button", { name: "关闭" }));
  fireEvent.click(within(detail).getByRole("button", { name: "关闭" }));

  fireEvent.click(within(actions).getByRole("button", { name: "修改" }));
  const edit = await screen.findByRole("dialog", { name: /修改营销内容/ });
  expect(
    within(edit).getByRole<HTMLInputElement>("radio", { name: "视频" }).checked,
  ).toBe(true);
  expect(
    within(edit).getByRole<HTMLInputElement>("radio", { name: "只发小红书" })
      .checked,
  ).toBe(true);
  expect(within(edit).getByLabelText<HTMLTextAreaElement>("分发语").value).toBe(
    "原分发语",
  );
  expect(within(edit).getByLabelText<HTMLInputElement>("TAG").value).toBe(
    "#原话题",
  );
  expect(
    within(edit).getByLabelText<HTMLInputElement>("@账号（选填）").value,
  ).toBe("@原账号");
  expect(
    within(edit).getByLabelText<HTMLInputElement>("话题至少带满").value,
  ).toBe("3");
  expect(
    within(edit).getAllByRole("button", { name: /预览附件/ }),
  ).toHaveLength(2);
  fireEvent.change(within(edit).getByLabelText("任务标题"), {
    target: { value: "改后的素材" },
  });
  fireEvent.click(within(edit).getByRole("button", { name: "保存修改" }));
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      "/tasks/edit-task/versions",
      expect.objectContaining({
        title: "改后的素材",
        body: task.body,
        platforms: task.platforms,
        files: task.files,
        deadline: task.deadline,
        content_id: task.content_id,
      }),
    ),
  );
});

it("确认行内单价后显示改价记录，并保存半小时失效设置", async () => {
  vi.clearAllMocks();
  sessionStorage.setItem(sessionKey, "marketing-settings-operator");
  let settings = {
    price_fen: "750",
    expiry_mode: "fixed_duration",
    expiry_minutes: 60,
    online: true,
    price_changes: [] as {
      before_fen: string;
      after_fen: string;
      changed_at: string;
      operator: string;
    }[],
  };
  vi.mocked(get).mockImplementation(async (requestPath) => {
    if (requestPath === "/auth/me") {
      return {
        id: "operator",
        username: "operator",
        pages: ["/tasks"],
        endpoints: [
          "GET /tasks",
          "GET /marketing-settings",
          "POST /marketing-settings",
        ],
        super_admin: false,
      };
    }
    if (requestPath === "/tasks") return [];
    if (requestPath === "/marketing-settings") return settings;
    throw new Error(`Unexpected request: ${requestPath}`);
  });
  vi.mocked(post).mockImplementation(async (requestPath, input) => {
    expect(requestPath).toBe("/marketing-settings");
    const patch = input as Partial<typeof settings>;
    if (patch.price_fen) {
      settings.price_changes = [
        {
          before_fen: settings.price_fen,
          after_fen: patch.price_fen,
          changed_at: "2026-11-19T09:00:00+08:00",
          operator: "operator",
        },
      ];
    }
    settings = { ...settings, ...patch };
    return settings;
  });

  render(<Application />);
  const price = await screen.findByLabelText("每平台单价（元）");
  await waitFor(() => expect((price as HTMLInputElement).value).toBe("7.50"));
  fireEvent.change(price, { target: { value: "8.25" } });
  fireEvent.blur(price);
  const dialog = await screen.findByRole("dialog", { name: "修改营销单价" });
  fireEvent.click(within(dialog).getByRole("button", { name: "确认修改" }));
  expect(await screen.findByText(/operator.*¥7.50 → ¥8.25/)).toBeTruthy();
  const rule = screen.getByRole<HTMLSelectElement>("combobox", {
    name: "失效规则",
  });
  expect(Array.from(rule.options, (option) => option.text)).toEqual([
    "当天",
    "指定时间内",
  ]);
  const expiry = screen.getByLabelText("失效时长");
  fireEvent.change(expiry, { target: { value: "0.5" } });
  fireEvent.click(screen.getByRole("button", { name: "保存失效规则" }));
  expect(
    await screen.findByText("现在生效：指定时间内（0.5 小时）"),
  ).toBeTruthy();
  expect(post).toHaveBeenCalledWith("/marketing-settings", {
    price_fen: "825",
  });
  expect(post).toHaveBeenCalledWith("/marketing-settings", {
    expiry_mode: "fixed_duration",
    expiry_minutes: 30,
  });

  fireEvent.change(rule, { target: { value: "end_of_day" } });
  expect(screen.queryByLabelText("失效时长")).toBeNull();
  expect(screen.getByText("领取、指派或重派后")).toBeTruthy();
  expect(
    screen.getByText("北京时间 24:00 前未交发布链接，自动失效"),
  ).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "保存失效规则" }));
  expect(
    await screen.findByText("现在生效：当天（北京时间 24:00 截止）"),
  ).toBeTruthy();
  expect(post).toHaveBeenCalledWith("/marketing-settings", {
    expiry_mode: "end_of_day",
  });

  fireEvent.change(rule, { target: { value: "fixed_duration" } });
  expect(screen.getByLabelText<HTMLInputElement>("失效时长").value).toBe("0.5");
});

it("分发员管理保留搜索和分页，可查看资料并拉黑或取消拉黑", async () => {
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/#/admin/mkt-people");
  sessionStorage.setItem(sessionKey, "distributor-operator");
  const listPath = "/users?view=distributors";
  let blocked = false;
  const member = {
    id: "member-one",
    name: "测试分发员",
    nickname: "微信测试昵称",
    wechat: "test-wechat",
    phone: "138****1111",
    douyin: "test-douyin",
    xhs: "test-xhs",
    status: "approved",
    tags: [],
    identity_verified: true,
  };
  const details = {
    phone: "13800001111",
    identity: "9900**********0017",
    updated_at: "2026-11-19T09:00:00+08:00",
    today_tasks: [
      {
        id: "assignment-one",
        title: "今日测试任务",
        source: "assigned",
        status: "in_progress",
        unpublished: true,
      },
    ],
  };
  vi.mocked(get).mockImplementation(async (path) => {
    const current = {
      ...member,
      blocked,
      block_info: blocked
        ? {
            reason: "多次删除作品",
            operator: "reviewer",
            at: "2026-11-19T10:00:00+08:00",
          }
        : null,
    };
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/users"],
        endpoints: ["GET /users", "POST /users/:id/block"],
        super_admin: false,
      };
    }
    if (path === listPath) {
      return [
        current,
        ...Array.from({ length: 12 }, (_, index) => ({
          ...member,
          id: `other-${index}`,
          name: `其他分发员${index + 1}`,
          nickname: "",
          wechat: "",
          douyin: "",
          xhs: "",
          blocked: false,
        })),
      ];
    }
    if (path === `${listPath}&id=member-one`) {
      return [{ ...current, distributor_details: details }];
    }

    throw new Error(`Unexpected request: ${path}`);
  });
  vi.mocked(post).mockImplementation(async (_, body) => {
    blocked = (body as { blocked: boolean }).blocked;
    return null;
  });

  render(<Application />);

  expect(await screen.findByText(/test-wechat/)).toBeTruthy();
  expect(
    screen.getAllByRole("columnheader").map((header) => header.textContent),
  ).toEqual(["分发员", "抖音号", "小红书号", "手机号", "标签", "状态", "操作"]);
  const firstRow = screen.getByRole("row", { name: /测试分发员/ });
  const douyin = within(firstRow).getByRole("link", { name: "test-douyin" });
  expect(douyin.getAttribute("href")).toBe(
    "https://www.douyin.com/search/test-douyin?type=user",
  );
  expect(douyin.getAttribute("target")).toBe("_blank");
  const xhs = within(firstRow).getByRole("link", { name: "test-xhs" });
  expect(xhs.getAttribute("href")).toBe(
    "https://www.xiaohongshu.com/search_result?keyword=test-xhs",
  );
  expect(xhs.getAttribute("target")).toBe("_blank");
  expect(screen.getByRole("button", { name: "下一页" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "下一页" }));
  expect(await screen.findByText("其他分发员12")).toBeTruthy();
  fireEvent.change(screen.getByRole("searchbox", { name: "搜索分发员" }), {
    target: { value: "TEST-XHS" },
  });
  const row = await screen.findByRole("row", { name: /测试分发员/ });
  expect(screen.queryByText("其他分发员12")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "刷新" }));
  await waitFor(() => {
    expect(
      vi.mocked(get).mock.calls.filter(([path]) => path === listPath),
    ).toHaveLength(2);
  });

  fireEvent.click(within(row).getByRole("button", { name: "测试分发员" }));
  let dialog = await screen.findByRole("dialog");
  expect(await within(dialog).findByText("13800001111")).toBeTruthy();
  expect(within(dialog).getByText("小红书号", { exact: true })).toBeTruthy();
  expect(
    within(dialog).getByRole("link", { name: "test-xhs" }).getAttribute("href"),
  ).toBe(xhs.getAttribute("href"));
  expect(within(dialog).getByText("今日测试任务")).toBeTruthy();
  fireEvent.click(within(dialog).getByRole("button", { name: "关闭" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

  fireEvent.click(within(row).getByRole("button", { name: "拉黑" }));
  dialog = await screen.findByRole("dialog");
  expect(await within(dialog).findByText(/还有未发布的任务/)).toBeTruthy();
  expect(within(dialog).getByText("他今天的任务：")).toBeTruthy();
  const confirm = within(dialog).getByRole("button", { name: "确认拉黑" });
  const reason = within(dialog).getByLabelText("拉黑原因");
  expect(reason.tagName).toBe("INPUT");
  expect(reason.getAttribute("placeholder")).toBe("拉黑原因（必填）");
  fireEvent.change(reason, { target: { value: "   " } });
  fireEvent.click(confirm);
  expect(within(dialog).getByText("请填写拉黑原因")).toBeTruthy();
  expect(post).not.toHaveBeenCalled();
  fireEvent.change(reason, { target: { value: "  多次删除作品  " } });
  fireEvent.click(confirm);
  expect(await within(row).findByText("已拉黑", { exact: true })).toBeTruthy();
  expect(post).toHaveBeenCalledWith("/users/member-one/block", {
    blocked: true,
    reason: "多次删除作品",
    updated_at: details.updated_at,
  });

  fireEvent.click(within(row).getByRole("button", { name: "取消拉黑" }));
  dialog = await screen.findByRole("dialog");
  expect(await within(dialog).findByText("多次删除作品")).toBeTruthy();
  fireEvent.click(within(dialog).getByRole("button", { name: "确认取消拉黑" }));
  await waitFor(() => expect(within(row).getByText("正常")).toBeTruthy());
  expect(post).toHaveBeenLastCalledWith("/users/member-one/block", {
    blocked: false,
    reason: "",
    updated_at: details.updated_at,
  });
});

it("从原型实名入口查看成功和失败记录，没有人工审核操作", async () => {
  window.history.replaceState(null, "", "/#/admin/realname");
  sessionStorage.setItem(sessionKey, "identity-records-operator");
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/admin/realname"],
        endpoints: ["GET /identity-verifications"],
        super_admin: false,
      };
    }
    if (path === "/identity-verifications") {
      return {
        records: [
          {
            id: "successful",
            user_id: "user-one",
            nickname: "测试用户甲",
            action: AuditAction.AuditUserIdentityVerify,
            reason: "",
            created_at: "2026-11-18T10:00:00+08:00",
          },
          {
            id: "failed",
            user_id: "user-two",
            nickname: "测试用户乙",
            action: AuditAction.AuditEnrollmentIdentityFailed,
            reason: "姓名与身份证号不匹配",
            created_at: "2026-11-18T09:00:00+08:00",
          },
        ],
        next_before: "",
      };
    }

    throw new Error(`Unpermitted request: ${path}`);
  });

  render(<Application />);

  expect(await screen.findByText("姓名与身份证号不匹配")).toBeTruthy();
  expect(window.location.pathname).toBe("/admin/realname");
  expect(
    screen.getByRole("heading", { name: "报名审核 · 实名认证记录" }),
  ).toBeTruthy();
  expect(screen.getByRole("table").closest(".ant-card")).toBeNull();
  expect(
    screen.getAllByRole("columnheader").map((header) => header.textContent),
  ).toEqual([
    "用户",
    "姓名",
    "身份证号",
    "手机号",
    "微信号",
    "身份证照片",
    "来源",
    "提交",
    "状态",
    "失败原因",
  ]);
  expect(screen.getByRole("button", { name: "刷新" })).toBeTruthy();
  expect(screen.getByRole("menuitem", { name: "实名认证记录" })).toBeTruthy();
  expect(screen.getByText("成功")).toBeTruthy();
  expect(screen.getByText("失败")).toBeTruthy();
  expect(screen.queryByRole("button", { name: /通过|驳回/ })).toBeNull();
  expect(get).not.toHaveBeenCalledWith("/identity-verifications/documents");
});

it("有实名资料权限时显示脱敏字段和正反面缩略图，点击可分别查看大图", async () => {
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/admin/realname");
  sessionStorage.setItem(sessionKey, "identity-documents-operator");
  const front = "https://example.test/front.jpg";
  const back = "https://example.test/back.jpg";
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/admin/realname"],
        endpoints: [
          "GET /identity-verifications",
          "GET /identity-verifications/documents",
        ],
        super_admin: false,
      };
    }
    if (path === "/identity-verifications/documents") {
      return {
        records: [
          {
            id: "record",
            user_id: "user-one",
            nickname: "测试分发员",
            action: AuditAction.AuditUserIdentityVerify,
            reason: "",
            created_at: "2026-11-18T10:00:00+08:00",
            details: {
              name: "张**",
              identity: "5101**********1234",
              phone: "138****8888",
              wechat: "example_wechat",
              front_url: front,
              back_url: back,
            },
          },
          {
            id: "historical-record",
            user_id: "user-two",
            nickname: "历史核验用户",
            action: AuditAction.AuditUserIdentityVerify,
            reason: "",
            created_at: "2026-11-18T09:00:00+08:00",
            details: {
              name: "",
              identity: "",
              phone: "139****1111",
              wechat: "historical_wechat",
              front_url: "",
              back_url: "",
            },
          },
        ],
        next_before: "",
        photos_unavailable: false,
      };
    }

    throw new Error(`Unpermitted request: ${path}`);
  });

  render(<Application />);

  expect(await screen.findByText("5101**********1234")).toBeTruthy();
  expect(screen.getByText("张**")).toBeTruthy();
  expect(screen.getByText("138****8888")).toBeTruthy();
  expect(screen.getByText("example_wechat")).toBeTruthy();
  const historicalRow = screen.getByRole("row", { name: /历史核验用户/ });
  const historicalCells = within(historicalRow).getAllByRole("cell");
  expect([1, 2, 5].map((index) => historicalCells[index].textContent)).toEqual([
    "—",
    "—",
    "—",
  ]);
  expect(
    screen.getAllByRole("button", { name: /查看身份证.*大图/ }),
  ).toHaveLength(2);
  for (const [side, url] of [
    ["正面", front],
    ["反面", back],
  ]) {
    const thumbnail = screen.getByRole("button", {
      name: `查看身份证${side}大图`,
    });
    expect(thumbnail.querySelector("img")?.getAttribute("src")).toBe(url);
    fireEvent.click(thumbnail);
    expect(
      await screen.findByRole("dialog", { name: `身份证${side}` }),
    ).toBeTruthy();
    const preview = screen.getByRole("img", { name: `身份证${side}大图` });
    expect(preview.getAttribute("src")).toBe(url);
    expect(preview.getAttribute("referrerpolicy")).toBe("no-referrer");
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
  }
});

it("实名记录按游标翻页，只展示当前页并可返回首页", async () => {
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/admin/realname");
  sessionStorage.setItem(sessionKey, "identity-records-operator");
  const record = {
    user_id: "user-one",
    action: AuditAction.AuditUserIdentityVerify,
    reason: "",
    created_at: "2026-11-18T10:00:00+08:00",
  };
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/admin/realname"],
        endpoints: ["GET /identity-verifications"],
        super_admin: false,
      };
    }
    if (path === "/identity-verifications") {
      return {
        records: [{ ...record, id: "newer", nickname: "新核验记录" }],
        next_before: "newer",
      };
    }
    if (path === "/identity-verifications?before=newer") {
      return {
        records: [{ ...record, id: "older", nickname: "较早核验记录" }],
        next_before: "",
      };
    }

    throw new Error(`Unpermitted request: ${path}`);
  });

  render(<Application />);

  expect(await screen.findByText("新核验记录")).toBeTruthy();
  expect(
    screen.getByRole("navigation", { name: "实名认证记录分页" }),
  ).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "下一页" }));
  expect(await screen.findByText("较早核验记录")).toBeTruthy();
  expect(screen.queryByText("新核验记录")).toBeNull();
  expect(
    screen.getByRole("button", { name: "下一页" }).hasAttribute("disabled"),
  ).toBe(true);

  fireEvent.click(screen.getByRole("button", { name: "上一页" }));
  expect(await screen.findByText("新核验记录")).toBeTruthy();
  expect(screen.queryByText("较早核验记录")).toBeNull();
  expect(vi.mocked(get).mock.calls.map(([path]) => path)).toEqual([
    "/auth/me",
    "/identity-verifications",
    "/identity-verifications?before=newer",
  ]);
});

it("从原型入口打开独立的分发员报名审核页", async () => {
  window.history.replaceState(null, "", "/#/admin/mktapps");
  sessionStorage.setItem(sessionKey, "enrollment-review-operator");
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: [
          "/tasks",
          "/contents",
          "/users",
          "/reviews",
          "/settlements",
          "/withdrawals",
        ],
        endpoints: ["GET /tasks", "GET /users"],
        super_admin: false,
      };
    }
    throw new Error(`Unpermitted request: ${path}`);
  });
  vi.mocked(getPage).mockImplementation(async (path) => {
    if (path === "/users?view=enrollments&page=1") {
      return {
        total: 31,
        rows: [
          {
            id: "applicant",
            name: "",
            nickname: "审核中的申请人",
            douyin: "applicant-douyin",
            xhs: "https://xhslink.com/a/profile-test?xsec_token=fixture",
            status: "pending",
            enrollment_submitted_at: "2026-11-17T16:15:00Z",
            enrollment_has_photos: true,
          },
        ],
      };
    }
    if (path === "/users?view=enrollments&page=2") {
      return {
        total: 31,
        rows: [
          {
            id: "page-2",
            name: "第二页报名人",
            status: "approved",
            douyin: "https://www.douyin.com/user/MS4wLjABAAAA-profile",
            xhs: "https://xhslink.com.example.test/a/profile",
          },
        ],
      };
    }

    throw new Error(`Unpermitted request: ${path}`);
  });

  render(<Application />);

  expect(
    await screen.findByRole("heading", { name: "营销分发 · 分发员报名审核" }),
  ).toBeTruthy();
  expect(screen.getByRole("menuitem", { name: "分发员报名审核" })).toBeTruthy();
  expect(screen.getByRole("menuitem", { name: "分发员管理" })).toBeTruthy();
  const marketing = screen.getByText("营销分发", { exact: true }).closest("li");
  expect(marketing).toBeTruthy();
  expect(
    within(marketing!)
      .getAllByRole("menuitem")
      .map((item) => item.textContent),
  ).toEqual([
    "任务管理",
    "分发员报名审核",
    "分发员管理",
    "发布链接审核",
    "飞书发布提醒（示意）即将上线",
    "数据统计即将上线",
    "分发结算",
    "提现记录",
  ]);
  expect(await screen.findByText("审核中的申请人")).toBeTruthy();
  expect(
    screen.getAllByRole("columnheader").map((header) => header.textContent),
  ).toEqual([
    "报名人",
    "抖音号",
    "小红书号",
    "身份证照片",
    "提交",
    "状态",
    "人工复核",
    "标签",
    "操作",
  ]);
  expect(
    screen.getByRole("link", { name: "applicant-douyin" }).closest("td")
      ?.cellIndex,
  ).toBe(1);
  expect(screen.getByRole("link", { name: "applicant-douyin" })).toHaveProperty(
    "href",
    "https://www.douyin.com/search/applicant-douyin?type=user",
  );
  expect(screen.getByRole("combobox", { name: "按状态筛选" })).toBeTruthy();
  expect(screen.getByRole("combobox", { name: "按标签筛选" })).toBeTruthy();
  expect(screen.getByText("2026-11-18")).toBeTruthy();
  expect(screen.getByText("已上传")).toBeTruthy();
  const xhs = screen.getByRole("link", {
    name: "https://xhslink.com/a/profile-test?xsec_token=fixture",
  });
  const xhsSearch = new URL(xhs.getAttribute("href")!);
  expect(xhsSearch.origin + xhsSearch.pathname).toBe(
    "https://www.xiaohongshu.com/search_result",
  );
  expect(xhsSearch.searchParams.get("keyword")).toBe(
    "https://xhslink.com/a/profile-test",
  );
  expect(xhs.getAttribute("target")).toBe("_blank");
  expect(screen.queryByRole("button", { name: "去复核" })).toBeNull();
  expect(window.location.pathname).toBe("/users");
  expect(getPage).toHaveBeenCalledWith("/users?view=enrollments&page=1");
  fireEvent.click(
    within(screen.getByRole("navigation", { name: "报名审核分页" })).getByRole(
      "button",
      { name: /2$/ },
    ),
  );
  expect(await screen.findByText("第二页报名人")).toBeTruthy();
  expect(
    screen.getByRole("link", {
      name: "https://www.douyin.com/user/MS4wLjABAAAA-profile",
    }),
  ).toHaveProperty(
    "href",
    "https://www.douyin.com/search/https%3A%2F%2Fwww.douyin.com%2Fuser%2FMS4wLjABAAAA-profile?type=user",
  );
  expect(
    screen.getByText("https://xhslink.com.example.test/a/profile"),
  ).toBeTruthy();
  expect(
    screen.queryByRole("link", {
      name: "https://xhslink.com.example.test/a/profile",
    }),
  ).toBeNull();
  expect(getPage).toHaveBeenCalledWith("/users?view=enrollments&page=2");
});

it("报名列表点击正反面可打开新签名的大图，小红书账号可打开搜索", async () => {
  window.history.replaceState(null, "", "/users");
  sessionStorage.setItem(sessionKey, "enrollment-preview-operator");
  const front = "https://example.test/front.jpg";
  const back = "https://example.test/back.jpg";
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/users"],
        endpoints: ["GET /users", "GET /users/:id/enrollment-documents"],
        super_admin: false,
      };
    }
    if (path === "/users/applicant/enrollment-documents") {
      return {
        details: {
          front_url: front + "?signature=fresh",
          back_url: back + "?signature=fresh",
        },
        photos_unavailable: false,
      };
    }
    throw new Error(`Unexpected request: ${path}`);
  });
  vi.mocked(getPage).mockResolvedValue({
    rows: [
      {
        id: "applicant",
        name: "示例分发员",
        status: "pending",
        douyin: "dy_1",
        xhs: "xhs_2026",
        enrollment_has_photos: true,
        enrollment_photo_previews: {
          front_url: front,
          back_url: back,
          unavailable: false,
        },
      },
    ],
    total: 1,
  });

  render(<Application />);

  const row = within(await screen.findByRole("row", { name: /示例分发员/ }));
  const frontButton = row.getByRole("button", { name: "查看身份证正面大图" });
  const backButton = row.getByRole("button", { name: "查看身份证反面大图" });
  expect(frontButton.querySelector("img")?.getAttribute("src")).toBe(front);
  expect(backButton.querySelector("img")?.getAttribute("src")).toBe(back);
  expect(getPage).toHaveBeenCalledWith(
    "/users?view=enrollments&page=1&photos=1",
  );
  fireEvent.click(backButton);
  const photo = within(await screen.findByRole("dialog"));
  expect(
    await photo.findByRole("img", { name: "身份证反面大图" }),
  ).toHaveProperty("src", back + "?signature=fresh");
  fireEvent.click(photo.getByRole("button", { name: "关闭照片预览" }));

  const search = row.getByRole("link", { name: "xhs_2026" });
  expect(
    new URL(search.getAttribute("href")!).searchParams.get("keyword"),
  ).toBe("xhs_2026");
  expect(search.getAttribute("target")).toBe("_blank");
});

it("改画像弹框回显资料，新增标签后自动选中并随画像保存", async () => {
  window.history.replaceState(null, "", "/users");
  sessionStorage.setItem(sessionKey, "portrait-operator");
  const revision = "2026-11-19T02:00:00Z";
  const tag = {
    id: "fitness",
    key: "fitness-key",
    name: "健身",
    updated_at: revision,
  };
  const library = [
    { id: "style", key: "时尚", name: "时尚", updated_at: revision },
  ];
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/users"],
        endpoints: [
          "GET /users",
          "GET /marketing-tags",
          "POST /marketing-tags",
          "POST /users/:id/review",
        ],
        super_admin: false,
      };
    }
    if (path === "/marketing-tags") return library.slice();
    throw new Error(`Unexpected request: ${path}`);
  });
  vi.mocked(getPage).mockResolvedValue({
    rows: [
      {
        id: "approved",
        name: "林晓",
        status: "approved",
        tags: ["时尚"],
        xhs: "",
        douyin: "",
        enrollment_updated_at: revision,
        enrollment_recent_post: true,
        enrollment_new_account: false,
        enrollment_age_group: EnrollmentAge.EnrollmentAge25To30,
      },
      {
        id: "legacy-approved",
        name: "旧报名",
        status: "approved",
        tags: [],
        enrollment_updated_at: revision,
      },
    ],
    total: 2,
  });
  vi.mocked(post).mockImplementation(async (path) => {
    if (path === "/marketing-tags") {
      library.push(tag);
      return tag;
    }
    if (
      path === "/users/approved/review" ||
      path === "/users/legacy-approved/review"
    )
      return null;
    throw new Error(`Unexpected request: ${path}`);
  });

  render(<Application />);

  const approved = within(await screen.findByRole("row", { name: /林晓/ }));
  const approvedActions = within(approved.getAllByRole("cell").at(-1)!);
  expect(
    approvedActions.getAllByRole("button").map((button) => button.textContent),
  ).toEqual(["改画像 / 标签"]);
  fireEvent.click(
    approvedActions.getByRole("button", { name: "改画像 / 标签" }),
  );
  const dialog = within(await screen.findByRole("dialog"));
  expect(dialog.getByText("改 林晓 的画像 / 标签")).toBeTruthy();
  expect(
    dialog.getByText("打开报名人的抖音、小红书主页看", { exact: true }),
  ).toBeTruthy();
  expect(dialog.getByRole("radio", { name: "有" })).toHaveProperty(
    "checked",
    true,
  );
  expect(dialog.getByRole("radio", { name: "不是" })).toHaveProperty(
    "checked",
    true,
  );
  expect(dialog.getByRole("checkbox", { name: "#时尚" })).toHaveProperty(
    "checked",
    true,
  );
  expect(dialog.queryByRole("button", { name: "拒绝" })).toBeNull();
  const newTag = dialog.getByRole("textbox", { name: "新建画像标签" });
  fireEvent.change(newTag, { target: { value: "#健身" } });
  fireEvent.click(dialog.getByRole("button", { name: /加到标签库/ }));
  expect(await dialog.findByRole("checkbox", { name: "#健身" })).toHaveProperty(
    "checked",
    true,
  );
  expect(post).toHaveBeenCalledWith("/marketing-tags", { name: "健身" });
  fireEvent.click(dialog.getByRole("radio", { name: "没有" }));
  fireEvent.click(dialog.getByRole("button", { name: "保存" }));
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith("/users/approved/review", {
      enrollment_updated_at: revision,
      approved: true,
      portrait_only: true,
      recent_post: false,
      new_account: false,
      age_group: EnrollmentAge.EnrollmentAge25To30,
      tags: ["时尚", "fitness-key"],
    }),
  );

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  const legacy = within(await screen.findByRole("row", { name: /旧报名/ }));
  fireEvent.click(legacy.getByRole("button", { name: "改画像 / 标签" }));
  const legacyDialog = within(await screen.findByRole("dialog"));
  fireEvent.click(legacyDialog.getByRole("checkbox", { name: "#时尚" }));
  fireEvent.click(legacyDialog.getByRole("button", { name: "保存" }));
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith("/users/legacy-approved/review", {
      enrollment_updated_at: revision,
      tags_only: true,
      tags: ["时尚"],
    }),
  );
});

it("驳回保存已选标签，并可在原报名中回显", async () => {
  window.history.replaceState(null, "", "/users");
  sessionStorage.setItem(sessionKey, "rejection-operator");
  const revision = "2026-11-18T00:00:00Z";
  const row = {
    id: "applicant",
    enrollment_id: "attempt-one",
    enrollment_updated_at: revision,
    nickname: "报名人",
    status: "pending",
    tags: [] as string[],
  };
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/users"],
        endpoints: [
          "GET /users",
          "GET /marketing-tags",
          "POST /users/:id/review",
        ],
        super_admin: false,
      };
    }

    if (path === "/marketing-tags") {
      return [{ id: "photo", key: "摄影", name: "摄影", updated_at: revision }];
    }

    throw new Error(`Unexpected request: ${path}`);
  });
  vi.mocked(getPage).mockImplementation(async () => ({
    rows: [{ ...row }],
    total: 1,
  }));
  vi.mocked(post).mockImplementation(async () => {
    row.status = "rejected";
    row.tags = ["摄影"];
    return null;
  });

  render(<Application />);
  fireEvent.click(await screen.findByRole("button", { name: "去复核" }));
  const dialog = within(await screen.findByRole("dialog"));
  fireEvent.click(await dialog.findByRole("checkbox", { name: "#摄影" }));
  fireEvent.change(dialog.getByRole("textbox", { name: "驳回原因" }), {
    target: { value: "主页打不开" },
  });
  fireEvent.click(dialog.getByRole("button", { name: "拒绝" }));

  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      "/users/applicant/review",
      expect.objectContaining({
        enrollment_id: "attempt-one",
        approved: false,
        reason: "主页打不开",
        tags: ["摄影"],
      }),
    ),
  );
  const rejected = within(await screen.findByRole("row", { name: /已拒绝/ }));
  expect(rejected.getByText("#摄影", { exact: true })).toBeTruthy();
  const rejectedActions = within(rejected.getAllByRole("cell").at(-1)!);
  expect(rejectedActions.queryByRole("button")).toBeNull();
});

it("标签库使用独立图标打开改名和删除确认", async () => {
  window.history.replaceState(null, "", "/users");
  sessionStorage.setItem(sessionKey, "tag-library-operator");
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/users"],
        endpoints: [
          "GET /users",
          "GET /marketing-tags",
          "POST /marketing-tags/:id/rename",
          "POST /marketing-tags/:id/delete",
        ],
        super_admin: false,
      };
    }
    if (path === "/marketing-tags") {
      return [
        {
          id: "tag",
          key: "时尚",
          name: "时尚",
          updated_at: "2026-11-18T00:00:00Z",
        },
      ];
    }
    throw new Error(`Unpermitted request: ${path}`);
  });
  vi.mocked(getPage).mockResolvedValue({ rows: [], total: 0 });

  render(<Application />);

  const rename = await screen.findByRole("button", { name: "改名 时尚" });
  const remove = screen.getByRole("button", { name: "删除 时尚" });
  expect(rename.querySelector('[data-icon="edit"]')).toBeTruthy();
  expect(remove.querySelector('[data-icon="close"]')).toBeTruthy();

  fireEvent.click(rename);
  const renameDialog = await screen.findByRole("dialog");
  expect(within(renameDialog).getByText("重命名标签")).toBeTruthy();
  expect(
    within(renameDialog).getByRole("textbox", { name: "标签名称" }),
  ).toHaveProperty("value", "时尚");
  fireEvent.click(
    within(renameDialog).getByRole("button", { name: /取\s*消/ }),
  );
  fireEvent.click(remove);
  const deleteTitle = await screen.findByText("删除标签 #时尚");
  expect(deleteTitle.closest('[role="dialog"]')).toBeTruthy();
});

it("按设计稿显示未来业务占位，默认仍进入已授权的一期页面", async () => {
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/");
  sessionStorage.setItem(sessionKey, "workspace-operator");
  vi.mocked(get).mockImplementation(async (path) => {
    if (path === "/auth/me") {
      return {
        id: "operator",
        username: "reviewer",
        pages: ["/tasks"],
        endpoints: ["GET /tasks"],
        super_admin: false,
      };
    }
    if (path === "/tasks") {
      return [];
    }

    throw new Error(`Unpermitted request: ${path}`);
  });

  render(<Application />);

  expect(
    await screen.findByRole("heading", { name: "营销分发 · 任务管理" }),
  ).toBeTruthy();
  expect(window.location.pathname).toBe("/tasks");
  expect(screen.getByText("AI选品带货")).toBeTruthy();
  expect(screen.getByText("达人切片带货")).toBeTruthy();
  expect(screen.getByText("素人素材拍摄")).toBeTruthy();
  expect(screen.getAllByText("即将上线").length).toBeGreaterThanOrEqual(3);
  const placeholder = screen.getByRole("menuitem", { name: "达人授权申请" });
  expect(placeholder.getAttribute("aria-disabled")).toBe("true");
  fireEvent.click(placeholder);
  expect(window.location.pathname).toBe("/tasks");
  expect(vi.mocked(get).mock.calls.map(([path]) => path)).toEqual([
    "/auth/me",
    "/tasks",
  ]);
});
