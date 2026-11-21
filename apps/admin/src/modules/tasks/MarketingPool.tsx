import { Pagination, Space, Tag } from "antd";
import { useState, type ReactNode } from "react";
import { AttachmentPreviews } from "../../components/AttachmentPreviews";
import { FileMediaType, TaskStep } from "../../generated/enums";
import type { PoolAllocation, TaskVersion } from "../../generated/models";
import { platformLabels } from "../../shared/status";

const beijingDate = new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Shanghai",
  dateStyle: "short",
});

export function marketingDate(date = new Date()) {
  return beijingDate.format(date);
}

export function marketingDisplayID(id: string) {
  return id.slice(-4);
}

function daysBefore(day: string, count: number) {
  const date = new Date(`${day}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() - count);
  return date.toISOString().slice(0, 10);
}

export function poolStatus(task: TaskVersion, today: string): string {
  if (task.allocation?.has_link) return "已发布";
  if (task.allocation) return "已领取";
  if (!task.online) return "已下架";
  if (task.deadline && task.deadline < today) return "已截止";
  return "排队中";
}

const columns = [
  { label: "ID", width: 48 },
  { label: "艺人", width: 58 },
  { label: "类型 / 平台", width: 104 },
  { label: "标题", width: "14%" },
  { label: "内容预览", width: 88 },
  { label: "内容（分发语 / TAG）" },
  { label: "标签", width: "7%" },
  { label: "状态", width: "10%" },
  { label: "分发员", width: "8%" },
  { label: "操作", width: 188 },
];
const stateOptions = ["全部", "排队中", "已领取", "已发布"] as const;
const dateOptions = {
  all: "全部日期",
  today: "今天",
  yesterday: "昨天",
  week: "近 7 天",
  pick: "指定日期",
};
const pageSize = 10;

export function mediaLabel(task: TaskVersion) {
  const video = task.media_types?.includes(FileMediaType.FileMP4);
  const image = task.media_types?.some(
    (type) => type !== FileMediaType.FileMP4,
  );
  if (video && image) return "图文、视频";
  if (video) return "视频";
  if (image) return "图文";
  return task.files?.length ? "附件" : "文案";
}

export function platformText(task: TaskVersion) {
  const platforms = task.platforms
    ?.map((platform) => platformLabels[platform])
    .join(" + ");
  return platforms && task.platforms.length === 1
    ? `只发${platforms}`
    : platforms;
}

export function marketingBody(body: string) {
  const copy: string[] = [];
  let tags = "";
  let mention = "";
  let minimum = "";
  for (const line of body.split("\n")) {
    if (/^TAG[：:]/i.test(line)) {
      tags = line.slice(4).trim();
    } else if (/^@账号[：:]/.test(line)) {
      mention = line.slice(4).trim();
    } else if (/^话题至少带满 \d+ 个（含必带 TAG）\s*$/.test(line)) {
      minimum = line;
    } else {
      copy.push(line);
    }
  }

  return { copy: copy.join("\n").trim(), tags, mention, minimum };
}

export function MarketingProgress({
  allocation,
}: {
  allocation: PoolAllocation;
}) {
  const steps = [
    { label: "拿内容", done: allocation.steps?.includes(TaskStep.StepContent) },
    { label: "发布", done: allocation.steps?.includes(TaskStep.StepPublish) },
    { label: "交链接", done: allocation.links_done },
    { label: "审核", done: allocation.reviewed },
    { label: "结算", done: allocation.settled },
  ];
  const current = steps.findIndex((step) => !step.done);

  return (
    <div className="marketing-progress" aria-label="任务进度">
      <ol>
        {steps.map((step, index) => (
          <li
            key={step.label}
            title={step.label}
            aria-label={`${step.label}${step.done ? "已完成" : "未完成"}`}
            className={
              step.done
                ? "is-complete"
                : index === current
                  ? "is-current"
                  : undefined
            }
          />
        ))}
      </ol>
      <span className="marketing-note">
        {steps[current]?.label ?? "已结算"}
      </span>
    </div>
  );
}

function uploadDate(task: TaskVersion) {
  return marketingDate(new Date(task.uploaded_at ?? task.created_at));
}

export function MarketingPoolTable({
  tasks,
  today,
  open,
  tagNames,
  renderRecipient,
  renderActions,
  canPreview,
}: {
  tasks: TaskVersion[];
  today: string;
  open: boolean;
  tagNames: Map<string, string>;
  renderRecipient: (task: TaskVersion) => ReactNode;
  renderActions: (task: TaskVersion) => ReactNode;
  canPreview: boolean;
}) {
  const [page, setPage] = useState(1);
  const [artist, setArtist] = useState("");
  const [dateMode, setDateMode] = useState<keyof typeof dateOptions>("all");
  const [pickedDate, setPickedDate] = useState("");
  const [state, setState] = useState<(typeof stateOptions)[number]>("全部");
  const artists = [...new Set(tasks.map((task) => task.project || "未分项目"))];
  const byArtistAndDate = tasks.filter((task) => {
    if (artist && (task.project || "未分项目") !== artist) return false;

    const date = uploadDate(task);
    if (dateMode === "today") return date === today;
    if (dateMode === "yesterday") return date === daysBefore(today, 1);
    if (dateMode === "week")
      return date >= daysBefore(today, 6) && date <= today;
    if (dateMode === "pick" && pickedDate) return date === pickedDate;
    return true;
  });
  const rows =
    state === "全部"
      ? byArtistAndDate
      : byArtistAndDate.filter((task) => poolStatus(task, today) === state);
  const currentPage = Math.min(
    page,
    Math.max(1, Math.ceil(rows.length / pageSize)),
  );
  const start = (currentPage - 1) * pageSize;
  const next = open
    ? tasks.find(
        (task) => task.next_in_queue && poolStatus(task, today) === "排队中",
      )?.task_id
    : undefined;

  return (
    <>
      <div className="marketing-filters">
        <div className="marketing-filter-field">
          <label htmlFor="marketing-filter-artist">艺人</label>
          <select
            id="marketing-filter-artist"
            value={artist}
            onChange={(event) => {
              setArtist(event.target.value);
              setPage(1);
            }}
          >
            <option value="">全部艺人</option>
            {artists.map((name) => (
              <option value={name} key={name}>
                {name}
              </option>
            ))}
          </select>
        </div>
        <div className="marketing-filter-field">
          <label htmlFor="marketing-filter-date">上传日期</label>
          <select
            id="marketing-filter-date"
            value={dateMode}
            onChange={(event) => {
              setDateMode(event.target.value as keyof typeof dateOptions);
              setPage(1);
            }}
          >
            {Object.entries(dateOptions).map(([value, label]) => (
              <option value={value} key={value}>
                {label}
              </option>
            ))}
          </select>
        </div>
        {dateMode === "pick" && (
          <input
            type="date"
            aria-label="指定上传日期"
            value={pickedDate}
            onChange={(event) => {
              setPickedDate(event.target.value);
              setPage(1);
            }}
          />
        )}
        <div className="marketing-status-filter">
          <span>状态</span>
          <div role="group" aria-label="按状态筛选">
            {stateOptions.map((option) => (
              <button
                type="button"
                key={option}
                aria-pressed={state === option}
                onClick={() => {
                  setState(option);
                  setPage(1);
                }}
              >
                {option}{" "}
                {option === "全部"
                  ? byArtistAndDate.length
                  : byArtistAndDate.filter(
                      (task) => poolStatus(task, today) === option,
                    ).length}
              </button>
            ))}
          </div>
        </div>
      </div>
      <div
        className="marketing-table-scroll"
        tabIndex={0}
        aria-label="横向滚动查看营销内容"
      >
        <table
          className="marketing-table marketing-pool-table"
          aria-label="营销内容"
        >
          <colgroup>
            {columns.map(({ label, width }) => (
              <col key={label} style={{ width }} />
            ))}
          </colgroup>
          <thead>
            <tr>
              {columns.map(({ label }) => (
                <th scope="col" key={label}>
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.slice(start, start + pageSize).map((task) => {
              const status = poolStatus(task, today);
              const body = marketingBody(task.body);
              return (
                <tr key={task.task_id} className="marketing-content-row">
                  <td className="marketing-id" title={task.task_id}>
                    {marketingDisplayID(task.task_id)}
                  </td>
                  <td>
                    <span className="marketing-artist" title={task.project}>
                      {task.project || "未分项目"}
                    </span>
                  </td>
                  <td>
                    {mediaLabel(task)}
                    <div className="marketing-note marketing-platform">
                      {platformText(task)}
                    </div>
                  </td>
                  <td>
                    {task.title}
                    <div className="marketing-note">
                      上传 {uploadDate(task).slice(5).replace("-", "/")}
                    </div>
                  </td>
                  <td>
                    {canPreview && task.files?.length ? (
                      <AttachmentPreviews ids={task.files} coverOnly />
                    ) : (
                      <span className="marketing-note">—</span>
                    )}
                  </td>
                  <td className="marketing-summary">
                    <div className="marketing-pool-requirements">
                      {body.copy || "—"}
                    </div>
                    {body.tags && (
                      <div>
                        <span className="marketing-note">TAG</span> {body.tags}
                      </div>
                    )}
                  </td>
                  <td>
                    <Space size={4} wrap>
                      {task.tags?.map((tag) => (
                        <Tag className="platform-tag" key={tag}>
                          #{tagNames.get(tag) ?? tag}
                        </Tag>
                      ))}
                    </Space>
                  </td>
                  <td>
                    <Tag
                      className={
                        status === "已发布"
                          ? "task-online-tag"
                          : task.task_id === next
                            ? "marketing-next"
                            : undefined
                      }
                      title={
                        task.task_id === next
                          ? "没有匹配标签时按队列优先取这条"
                          : undefined
                      }
                    >
                      {task.task_id === next ? "下一条分配" : status}
                    </Tag>
                    {task.allocation && (
                      <MarketingProgress allocation={task.allocation} />
                    )}
                  </td>
                  <td>{renderRecipient(task)}</td>
                  <td>{renderActions(task)}</td>
                </tr>
              );
            })}
            {!rows.length && (
              <tr>
                <td colSpan={columns.length} className="marketing-empty-cell">
                  没有符合筛选的内容
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {rows.length > pageSize && (
        <Pagination
          className="marketing-pool-pagination"
          current={currentPage}
          total={rows.length}
          pageSize={pageSize}
          onChange={setPage}
          showSizeChanger={false}
        />
      )}
    </>
  );
}
