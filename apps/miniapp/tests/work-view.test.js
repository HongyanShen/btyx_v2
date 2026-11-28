const { test } = require("node:test");
const assert = require("node:assert/strict");
const {
  AssignmentSource,
  AssignmentStatus,
  Platform,
  SettlementFrequency,
  SubmissionStatus,
  TaskStep,
} = require("../src/services/generated/enums");
const {
  workCard,
  workTabs,
  playbackWeeks,
  activePlaybackRows,
} = require("../src/services/work-view");

function assignment(id, date, extra = {}) {
  return {
    id,
    date,
    due_date: date,
    title: id,
    source: AssignmentSource.SourceClaim,
    status: AssignmentStatus.AssignmentInProgress,
    platforms: [Platform.PlatformDouyin],
    steps: [],
    submissions: [],
    ...extra,
  };
}

function submission(platform, extra = {}) {
  return {
    platform,
    url: "https://www.douyin.com/video/123",
    status: SubmissionStatus.SubmissionApproved,
    ...extra,
  };
}

test("work filters count visible tasks by the displayed status", () => {
  const records = [
    assignment("fresh", "2026-11-23"),
    assignment("started", "2026-11-23", {
      steps: [{ key: TaskStep.StepContent, done: true }],
    }),
    assignment("waived", "2026-11-17", {
      status: AssignmentStatus.AssignmentPendingSettlement,
      submissions: [
        submission(Platform.PlatformDouyin, {
          status: SubmissionStatus.SubmissionWaived,
        }),
      ],
    }),
    assignment("revoked", "2026-11-23", {
      status: AssignmentStatus.AssignmentRevoked,
    }),
  ];
  const cards = records.map((record) => workCard(record)).filter(Boolean);
  assert.deepEqual(
    cards.map(({ statusLabel }) => statusLabel),
    ["进行中", "进行中", "待交数据"],
  );
  assert.equal(
    workTabs(cards).find(({ label }) => label === "待交数据").count,
    1,
  );
  assert.equal(workTabs(cards)[0].count, 3);
});

test("playback weeks use Beijing dates, preserve overdue work and exclude ineligible platforms", () => {
  const records = [
    assignment("older", "2026-11-09", {
      submissions: [submission(Platform.PlatformDouyin)],
    }),
    assignment("last", "2026-11-22", {
      playback_deadline: "2026-11-23T12:00:00+08:00",
      submissions: [
        submission(Platform.PlatformDouyin, {
          screenshot: "saved",
          published_on: "2026-11-23",
        }),
        submission(Platform.PlatformXiaohongshu, {
          published_on: "2026-11-22",
        }),
      ],
    }),
    assignment("current", "2026-11-23", {
      submissions: [submission(Platform.PlatformDouyin)],
    }),
    assignment("pending-review", "2026-11-20", {
      status: AssignmentStatus.AssignmentPendingReview,
      submissions: [
        submission(Platform.PlatformDouyin, {
          status: SubmissionStatus.SubmissionPending,
          screenshot: "saved-before-review",
        }),
      ],
    }),
    assignment("invalid", "2026-11-20", {
      submissions: [
        submission(Platform.PlatformDouyin, {
          status: SubmissionStatus.SubmissionInvalid,
        }),
        submission(Platform.PlatformXiaohongshu, {
          status: SubmissionStatus.SubmissionRejected,
        }),
      ],
    }),
    assignment("expired", "2026-11-20", {
      status: AssignmentStatus.AssignmentExpired,
      submissions: [submission(Platform.PlatformDouyin)],
    }),
  ];
  const early = playbackWeeks(records, Date.parse("2026-11-23T03:59:59Z"));
  assert.deepEqual(
    early.last.rows.map(({ id }) => id),
    ["last"],
  );
  assert.deepEqual(
    early.older.map(({ id }) => id),
    ["older"],
  );
  assert.deepEqual(
    early.current.rows.map(({ id }) => id),
    ["current"],
  );
  assert.equal(early.last.rows[0].platforms[0].screenshot, "saved");
  assert.equal(early.last.rows[0].dateLabel, "11/22 发布");
  assert.equal(early.older[0].dateLabel, "");
  assert.equal(early.pendingTasks, 2);
  assert.equal(early.last.rows[0].late, false);
  assert.equal(
    playbackWeeks(records, Date.parse("2026-11-23T04:00:00Z")).last.rows[0].late,
    true,
  );
});

test("daily settlement lets current-week tasks submit and shows the configured deferral", () => {
  const weeks = playbackWeeks(
    [
      assignment("today", "2026-11-26", {
        settlement_frequency: SettlementFrequency.SettlementDaily,
        playback_deadline: "2026-11-27T09:30:00+08:00",
        playback_settlement_at: "2026-11-29T04:15:00Z",
        submissions: [submission(Platform.PlatformDouyin)],
      }),
    ],
    Date.parse("2026-11-27T09:30:00+08:00"),
  );
  const [row] = activePlaybackRows(weeks);
  assert.equal(weeks.pendingTasks, 1);
  assert.equal(row.deadlineLabel, "2026-11-27 09:30");
  assert.equal(row.nextSettlement, "2026-11-29 12:15");
  assert.equal(row.late, true);
});

test("workbench shows the playback entry only after a platform has been reviewed", async () => {
  const platformSubmission = submission(Platform.PlatformDouyin, {
    status: SubmissionStatus.SubmissionPending,
  });
  const task = assignment("last", new Date().toISOString().slice(0, 10), {
    status: AssignmentStatus.AssignmentPendingReview,
    submissions: [platformSubmission],
  });
  const previous = { Page: global.Page, wx: global.wx };
  let page;
  global.Page = (definition) => {
    page = {
      ...definition,
      data: structuredClone(definition.data),
      setData(patch) {
        Object.assign(this.data, patch);
      },
    };
  };
  global.wx = {
    getStorageSync: () => "test-session",
    request(options) {
      const data = options.url.endsWith("/assignments")
        ? [structuredClone(task)]
        : { status: "approved" };
      options.success({ statusCode: 200, data: { code: 0, data } });
    },
  };

  try {
    require("../src/pages/work/index");
    await page.load();
    assert.equal(page.data.showPlayback, false);
    assert.equal(page.data.tasks[0].statusLabel, "待审核");

    platformSubmission.status = SubmissionStatus.SubmissionApproved;
    task.status = AssignmentStatus.AssignmentPendingSettlement;
    await page.load();
    assert.equal(page.data.showPlayback, true);
    assert.equal(page.data.tasks[0].statusLabel, "待交数据");
  } finally {
    Object.assign(global, previous);
  }
});

test("playback page previews uploaded screenshots and reloads them after submission", async () => {
  const past = new Date(Date.now() - 7 * 86400000).toISOString().slice(0, 10);
  const records = [
    assignment("last", past, {
      status: AssignmentStatus.AssignmentPendingReview,
      submissions: [
        submission(Platform.PlatformDouyin),
        submission(Platform.PlatformXiaohongshu, {
          url: "https://xhslink.cn/o/example",
          status: SubmissionStatus.SubmissionPending,
        }),
      ],
    }),
  ];
  const attempts = [];
  const ticketRequests = [];
  const previewed = [];
  let ticketExpired = false;
  let imagePath;
  const previous = { Page: global.Page, wx: global.wx };
  let page;
  let createPage;
  global.Page = (definition) => {
    createPage = () => ({
      ...definition,
      data: structuredClone(definition.data),
      setData(patch) {
        Object.assign(this.data, patch);
      },
    });
    page = createPage();
  };
  global.wx = {
    getStorageSync: () => "test-session",
    showToast() {},
    previewImage(options) {
      previewed.push(options.current);
    },
    chooseMedia(options) {
      options.success({ tempFiles: [{ tempFilePath: imagePath }] });
    },
    getFileSystemManager() {
      const png = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
      return {
        stat: ({ success }) => success({ stats: { size: png.length } }),
        readFile: ({ success }) =>
          success({ data: Uint8Array.from(png).buffer }),
      };
    },
    request(options) {
      const success = (data) =>
        options.success({ statusCode: 200, data: { code: 0, data } });
      if (options.url.endsWith("/files/uploads")) {
        return success({ id: options.data.name, completed: true });
      }
      const fileId = options.url.match(/\/files\/([^/]+)\/download$/)?.[1];
      if (fileId) {
        ticketRequests.push(fileId);
        return success({
          storage: "tos",
          media_type: "image/png",
          url: "https://tos.example/" + fileId,
          expires_at: new Date(
            Date.now() + (ticketExpired ? -1000 : 60000),
          ).toISOString(),
        });
      }
      if (options.method === "GET") return success(structuredClone(records));

      const { platform, screenshot } = options.data;
      attempts.push(platform);
      if (attempts.length === 2) {
        options.success({
          statusCode: 409,
          data: { code: 1, msg: "提交冲突" },
        });
        return;
      }
      records[0].submissions.find(
        (item) => item.platform === platform,
      ).screenshot = screenshot;
      success(null);
    },
  };

  try {
    require("../src/subpackages/task/playback/index");
    await page.load();
    assert.deepEqual(page.pending.map(({ key }) => key), ["dy"]);
    records[0].submissions[1].status = SubmissionStatus.SubmissionApproved;
    records[0].status = AssignmentStatus.AssignmentPendingSettlement;
    await page.load();
    assert.deepEqual(page.pending.map(({ key }) => key), ["dy", "xhs"]);
    await page.submit();
    assert.match(page.data.submitError, /请先上传/);
    assert.deepEqual(attempts, []);

    for (const { slot } of page.pending) {
      imagePath = "wxfile://tmp/" + slot + ".png";
      await page.chooseScreenshot({ currentTarget: { dataset: { slot } } });
      assert.equal(page.data.drafts[slot].preview, imagePath);
    }
    await page.previewScreenshot({
      currentTarget: { dataset: { src: imagePath } },
    });
    assert.deepEqual(previewed, [imagePath]);
    await page.submit();
    assert.match(page.data.submitError, /小红书.*提交冲突/);
    assert.equal(page.data.readyCount, 1);
    assert.equal(page.data.pendingCount, 1);
    assert.equal(page.data.drafts.last_xhs.preview, "wxfile://tmp/last_xhs.png");
    assert.equal(
      page.data.screenshotPreviews["last_dy.png"].src,
      "wxfile://tmp/last_dy.png",
    );

    await page.submit();
    assert.equal(page.data.pendingCount, 0);
    assert.deepEqual(attempts, ["dy", "xhs", "xhs"]);

    const reopened = createPage();
    ticketExpired = true;
    await reopened.load();
    assert.deepEqual(ticketRequests, ["last_dy.png", "last_xhs.png"]);
    assert.equal(
      reopened.data.screenshotPreviews["last_xhs.png"].src,
      "https://tos.example/last_xhs.png",
    );
    ticketExpired = false;
    await reopened.previewScreenshot({
      currentTarget: {
        dataset: {
          fileId: "last_xhs.png",
          src: "https://tos.example/last_xhs.png",
        },
      },
    });
    assert.deepEqual(ticketRequests, [
      "last_dy.png",
      "last_xhs.png",
      "last_xhs.png",
    ]);
  } finally {
    Object.assign(global, previous);
  }
});

test("screenshot picker supports WeChat runtimes that only expose chooseImage", async () => {
  const { chooseImage } = require("../src/services/files");
  const previous = global.wx;
  global.wx = {
    chooseImage(options) {
      assert.equal(options.count, 1);
      options.success({ tempFilePaths: ["wxfile://shot.png"] });
    },
  };

  try {
    assert.equal(await chooseImage(), "wxfile://shot.png");
  } finally {
    global.wx = previous;
  }
});
