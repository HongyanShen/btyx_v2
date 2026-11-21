import {
  AutoComplete,
  Button,
  Form,
  Input,
  Radio,
  Space,
  Typography,
} from "antd";
import { AttachmentInput } from "../../components/Attachments";
import {
  marketingTagOptions,
  useMarketingTags,
} from "../../components/MarketingTags";
import { Platform } from "../../generated/enums";
import { formatFen } from "../../shared/money";

type ContentKind = "video" | "images";
type PlatformChoice = Platform | "both";

export interface TaskFormValues {
  title: string;
  body: string;
  project: string;
  platformChoice?: PlatformChoice;
  tags: string[];
  files: string[];
  kind?: ContentKind;
  topics?: string;
  mention?: string;
  minimumTopics?: string;
}

const bothPlatforms = [Platform.PlatformDouyin, Platform.PlatformXiaohongshu];

export function selectedPlatforms(choice: PlatformChoice = "both"): Platform[] {
  return choice === "both" ? bothPlatforms : [choice];
}

export function platformChoice(platforms: Platform[]): PlatformChoice {
  return platforms.length === 1 ? platforms[0] : "both";
}

function MatchingTags({
  value = [],
  onChange,
}: {
  value?: string[];
  onChange?: (tags: string[]) => void;
}) {
  const tags = useMarketingTags();
  const choices = marketingTagOptions(tags.data ?? [], value);

  return (
    <Space size={6} wrap>
      {choices.length ? (
        choices.map((tag) => {
          const selected = value.includes(tag.value);
          return (
            <Button
              key={tag.value}
              shape="round"
              size="small"
              type={selected ? "primary" : "default"}
              aria-pressed={selected}
              onClick={() =>
                onChange?.(
                  selected
                    ? value.filter((key) => key !== tag.value)
                    : [...value, tag.value],
                )
              }
            >
              #{tag.label}
            </Button>
          );
        })
      ) : (
        <Typography.Text type="secondary">
          {tags.isError ? "标签库加载失败" : "暂无匹配标签"}
        </Typography.Text>
      )}
    </Space>
  );
}

export function TaskFormFields({
  projects,
  priceFen,
  uploading,
  onUploadingChange,
}: {
  projects: string[];
  priceFen?: string;
  uploading: boolean;
  onUploadingChange: (uploading: boolean) => void;
}) {
  const form = Form.useFormInstance<TaskFormValues>();
  const kind = Form.useWatch("kind", form) ?? "video";
  const files = Form.useWatch("files", form) ?? [];
  const title = (
    <Form.Item name="title" label="标题" rules={[{ required: true }]}>
      <Input
        maxLength={100}
        aria-label="任务标题"
        className="marketing-upload-title"
      />
    </Form.Item>
  );
  const project = (
    <Form.Item
      name="project"
      label="项目（艺人）"
      className="marketing-upload-inline marketing-row-project"
      extra="可选已有项目，也可以直接填新名字。项目名仅在后台显示。"
    >
      <AutoComplete
        options={projects.map((value) => ({ value }))}
        filterOption={(input, option) =>
          (option?.value ?? "").toLowerCase().includes(input.toLowerCase())
        }
        placeholder="选择或填写项目名称"
        className="marketing-upload-project"
      />
    </Form.Item>
  );

  return (
    <>
      <Form.Item name="kind" label="内容类型">
        <Radio.Group
          disabled={uploading || files.length > 0}
          options={[
            { value: "video", label: "视频" },
            { value: "images", label: "图文" },
          ]}
        />
      </Form.Item>
      {title}
      <Form.Item name="files" label={kind === "video" ? "视频文件" : "图片"}>
        <AttachmentInput media={kind} onUploadingChange={onUploadingChange} />
      </Form.Item>
      {project}
      <Form.Item
        name="platformChoice"
        label="发布平台"
        className="marketing-upload-inline marketing-row-platform"
        extra="分发员只会看到选中的平台。"
      >
        <Radio.Group
          options={[
            { value: "both", label: "抖音 + 小红书" },
            { value: Platform.PlatformDouyin, label: "只发抖音" },
            { value: Platform.PlatformXiaohongshu, label: "只发小红书" },
          ]}
        />
      </Form.Item>
      <Form.Item
        name="body"
        label="分发语"
        className="marketing-upload-inline marketing-row-copy"
        rules={[{ required: true }]}
        extra="分发员复制后发布。"
      >
        <Input.TextArea rows={2} className="marketing-upload-copy" />
      </Form.Item>
      <Form.Item
        name="topics"
        label="TAG"
        className="marketing-upload-inline marketing-row-tags"
        extra="指定要带的话题，多个话题用空格分隔。"
      >
        <Input
          placeholder="#指定话题A #指定话题B"
          className="marketing-upload-title"
        />
      </Form.Item>
      <Form.Item
        name="mention"
        label="@账号（选填）"
        className="marketing-upload-inline marketing-row-mention"
        extra="填写带 @ 的账号。"
      >
        <Input
          placeholder="例：@艺人官方号"
          className="marketing-upload-mention"
        />
      </Form.Item>
      <Form.Item
        name="tags"
        label="标签"
        extra="领取时优先匹配带有相同标签的分发员。"
      >
        <MatchingTags />
      </Form.Item>
      <Form.Item
        name="minimumTopics"
        label="话题至少带满"
        className="marketing-upload-inline marketing-row-minimum-topics"
        extra="个，包含 TAG 中的话题。"
        rules={[{ pattern: /^\d+$/, message: "请输入整数" }]}
      >
        <Input
          className="marketing-upload-topics"
          type="number"
          min={0}
          step={1}
        />
      </Form.Item>
      <Form.Item label="单价">
        <Typography.Text type="secondary">
          {priceFen
            ? `每个平台 ¥${formatFen(priceFen)}（两个平台 ¥${formatFen(String(BigInt(priceFen) * 2n))}），可在「营销设置」修改。`
            : "按营销分发当前单价计算。"}
        </Typography.Text>
      </Form.Item>
    </>
  );
}
