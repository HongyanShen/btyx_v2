import { useRef, useState } from "react";
import { App, Button, Space, Upload } from "antd";
import { download, upload } from "../api/files";
import { useSession } from "../app/Auth";
import { AttachmentPreviews } from "./AttachmentPreviews";

interface AttachmentInputProps {
  value?: string[];
  onChange?: (ids: string[]) => void;
  onUploadingChange?: (uploading: boolean) => void;
  media?: "video" | "images";
}

export function AttachmentInput({
  value = [],
  onChange,
  onUploadingChange,
  media,
}: AttachmentInputProps) {
  const { canCall } = useSession();
  const canUpload =
    canCall("POST /files/uploads") && canCall("POST /files/:id/complete");
  const canDownload = canCall("GET /files/:id/download");
  const { message } = App.useApp();
  const [uploading, setUploading] = useState(false);
  const activeUpload = useRef(false);
  const imageTypes = ["image/jpeg", "image/png", "image/gif", "image/webp"];
  const allowedTypes = media === "video" ? ["video/mp4"] : imageTypes;
  const limit = media === "video" ? 1 : 9;
  const uploadLabel =
    media === "video"
      ? "＋ 上传视频"
      : media === "images"
        ? "＋ 上传图片（最多 9 张）"
        : "上传图片或视频";

  async function uploadSelection(files: File[]) {
    if (activeUpload.current) return;

    if (media && files.some((file) => !allowedTypes.includes(file.type))) {
      message.error(
        media === "video"
          ? "请选择 MP4 视频"
          : "请选择 JPG、PNG、GIF 或 WebP 图片",
      );
      return;
    }

    if (media && files.length + value.length > limit) {
      message.error(
        media === "images"
          ? `最多上传 9 张图片，还可选择 ${Math.max(0, limit - value.length)} 张`
          : "仅可上传 1 个视频",
      );
      return;
    }

    activeUpload.current = true;
    setUploading(true);
    onUploadingChange?.(true);

    try {
      const results = await Promise.allSettled(files.map(upload));
      const ids = results.flatMap((result) =>
        result.status === "fulfilled" ? [result.value] : [],
      );
      if (ids.length) onChange?.([...value, ...ids]);

      const failure = results.find((result) => result.status === "rejected");
      if (failure) {
        const detail =
          failure.reason instanceof Error ? failure.reason.message : "上传失败";
        message.error(
          files.length === 1
            ? detail
            : `已上传 ${ids.length} 张，${files.length - ids.length} 张失败。${detail}`,
        );
      } else {
        message.success(
          files.length === 1 ? "文件已上传" : `已上传 ${ids.length} 张图片`,
        );
      }
    } finally {
      activeUpload.current = false;
      setUploading(false);
      onUploadingChange?.(false);
    }
  }

  return (
    <Space orientation="vertical">
      {canUpload && (!media || value.length < limit) && (
        <Upload
          accept={media ? allowedTypes.join(",") : "image/*,video/mp4"}
          multiple={media === "images"}
          showUploadList={false}
          disabled={uploading}
          beforeUpload={async (file, selection) => {
            // Ant Design invokes this callback for every file in the selection.
            if (file.uid === selection[0]?.uid) {
              await uploadSelection(selection);
            }

            return Upload.LIST_IGNORE;
          }}
        >
          <Button
            className={media ? "marketing-upload-attachment" : undefined}
            loading={uploading}
          >
            {uploading ? "正在上传并确认…" : uploadLabel}
          </Button>
        </Upload>
      )}
      {media && value.length > 0 && (
        <AttachmentPreviews
          ids={value}
          onRemove={
            uploading
              ? undefined
              : (id) => onChange?.(value.filter((value) => value !== id))
          }
        />
      )}
      {!media &&
        value.map((id, index) => (
          <Space key={id}>
            {canDownload ? (
              <Button
                type="link"
                onClick={() =>
                  void download(id).catch((error: Error) =>
                    message.error(error.message),
                  )
                }
              >
                查看附件 {index + 1}
              </Button>
            ) : (
              <span>附件 {index + 1}</span>
            )}
            <Button
              type="text"
              danger
              disabled={uploading}
              onClick={() => onChange?.(value.filter((value) => value !== id))}
            >
              移除
            </Button>
          </Space>
        ))}
    </Space>
  );
}
