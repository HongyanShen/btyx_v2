import { useEffect, useRef, useState } from "react";
import { App, Button, Modal, Spin } from "antd";
import { attachmentSource, download } from "../api/files";
import { useSession } from "../app/Auth";
import { FileMediaType } from "../generated/enums";

function useAttachment(id: string, allowLocal: boolean) {
  const [source, setSource] =
    useState<Awaited<ReturnType<typeof attachmentSource>>>();
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    let release: (() => void) | undefined;
    void attachmentSource(id, allowLocal)
      .then((result) => {
        if (!active) {
          result.release?.();
          return;
        }

        release = result.release;
        setSource(result);
      })
      .catch((failure: unknown) => {
        if (active) {
          setError(failure instanceof Error ? failure.message : "附件预览失败");
        }
      });

    return () => {
      active = false;
      release?.();
    };
  }, [id, allowLocal]);

  return { source, error, mediaFailed: () => setError("附件预览失败") };
}

function Thumbnail({
  id,
  label,
  allowLocal,
  count,
  pressed,
  onClick,
}: {
  id: string;
  label: string;
  allowLocal: boolean;
  count?: number;
  pressed?: boolean;
  onClick: () => void;
}) {
  const { source, error, mediaFailed } = useAttachment(id, allowLocal);
  const video = source?.media_type === FileMediaType.FileMP4;

  return (
    <button
      type="button"
      className="attachment-thumbnail"
      aria-label={label}
      aria-pressed={pressed}
      title={source?.name || label}
      onClick={onClick}
    >
      {source && !error ? (
        video ? (
          <>
            <video
              src={`${source.url}#t=0.001`}
              preload="metadata"
              muted
              playsInline
              aria-hidden="true"
              onError={mediaFailed}
            />
            <span className="attachment-play" aria-hidden="true">
              ▶
            </span>
          </>
        ) : (
          <img
            src={source.url}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            onError={mediaFailed}
          />
        )
      ) : (
        <span>{video ? "视频" : "附件"}</span>
      )}
      {count && (
        <span className="attachment-preview-count" aria-hidden="true">
          {count}
        </span>
      )}
    </button>
  );
}

function PreviewMedia({ id, allowLocal }: { id: string; allowLocal: boolean }) {
  const { source, error, mediaFailed } = useAttachment(id, allowLocal);
  return (
    <>
      <p className="attachment-preview-name" title={source?.name}>
        {source?.name}
      </p>
      <div className="attachment-preview-stage">
        {error ? (
          <p role="alert">{error}</p>
        ) : source ? (
          source.media_type === FileMediaType.FileMP4 ? (
            <video
              className="attachment-preview-media"
              src={source.url}
              controls
              playsInline
              preload="metadata"
              aria-label={source.name || "视频预览"}
              onError={mediaFailed}
            />
          ) : (
            <img
              className="attachment-preview-media"
              src={source.url}
              alt={source.name || "图片预览"}
              referrerPolicy="no-referrer"
              onError={mediaFailed}
            />
          )
        ) : (
          <Spin />
        )}
      </div>
    </>
  );
}

function PreviewDialog({
  ids,
  initialID,
  allowLocal,
  onClose,
}: {
  ids: string[];
  initialID: string;
  allowLocal: boolean;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState(initialID);
  const id = ids.includes(selected) ? selected : initialID;
  const gallery = ids.length > 1;
  const strip = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const list = strip.current;
    const button = list?.querySelector<HTMLElement>('[aria-pressed="true"]');
    if (!list || !button) return;

    const left = button.offsetLeft;
    if (
      left < list.scrollLeft ||
      left + button.offsetWidth > list.scrollLeft + list.clientWidth
    ) {
      list.scrollLeft = left - (list.clientWidth - button.offsetWidth) / 2;
    }
  }, [id]);

  return (
    <Modal
      title="附件预览"
      className="attachment-preview-dialog"
      centered
      open
      width={760}
      closable={false}
      onCancel={onClose}
      footer={[
        <Button
          key="download"
          onClick={() =>
            void download(id).catch((failure: Error) =>
              message.error(failure.message),
            )
          }
        >
          下载附件
        </Button>,
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
      ]}
    >
      <PreviewMedia key={id} id={id} allowLocal={allowLocal} />
      {gallery && (
        <div
          className="attachment-preview-strip"
          role="group"
          aria-label="附件缩略图"
          ref={strip}
        >
          {ids.map((fileID, index) => (
            <Thumbnail
              key={fileID}
              id={fileID}
              label={`切换到附件 ${index + 1}`}
              allowLocal={allowLocal}
              pressed={fileID === id}
              onClick={() => setSelected(fileID)}
            />
          ))}
        </div>
      )}
    </Modal>
  );
}

export function AttachmentPreviews({
  ids,
  coverOnly,
  onRemove,
}: {
  ids: string[];
  coverOnly?: boolean;
  onRemove?: (id: string) => void;
}) {
  const { canCall } = useSession();
  const canPreview = canCall("GET /files/:id/download");
  const allowLocal = canCall("GET /files/:id");
  const [selected, setSelected] = useState<string>();
  const visible = coverOnly ? ids.slice(0, 1) : ids;
  const count = coverOnly && ids.length > 1 ? ids.length : undefined;

  return (
    <div className="attachment-previews">
      {visible.map((id, index) => (
        <div
          className={
            count && canPreview
              ? "attachment-preview-item attachment-preview-stack"
              : "attachment-preview-item"
          }
          key={id}
        >
          {canPreview ? (
            <Thumbnail
              id={id}
              label={count ? `预览 ${count} 个附件` : `预览附件 ${index + 1}`}
              allowLocal={allowLocal}
              count={count}
              onClick={() => setSelected(id)}
            />
          ) : (
            <span>附件 {index + 1}</span>
          )}
          {onRemove && (
            <Button
              type="text"
              danger
              size="small"
              aria-label={`移除附件 ${index + 1}`}
              onClick={() => onRemove(id)}
            >
              移除
            </Button>
          )}
        </div>
      ))}
      {canPreview && selected && ids.includes(selected) && (
        <PreviewDialog
          key={selected}
          ids={ids}
          initialID={selected}
          allowLocal={allowLocal}
          onClose={() => setSelected(undefined)}
        />
      )}
    </div>
  );
}
