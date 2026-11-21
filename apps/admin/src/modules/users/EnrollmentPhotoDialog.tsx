import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Modal } from "antd";
import { get } from "../../api/client";
import { QueryState } from "../../components/Page";
import type { EnrollmentDocuments } from "../../generated/models";
import type { IdentityPhoto } from "./IdentityPhotoThumbnails";

export function enrollmentDocumentsPath(userID: string, enrollmentID?: string) {
  const path = `/users/${userID}/enrollment-documents`;
  return enrollmentID
    ? `${path}?enrollment_id=${encodeURIComponent(enrollmentID)}`
    : path;
}

export function EnrollmentPhotoDialog({
  userID,
  enrollmentID,
  side,
  onClose,
}: {
  userID: string;
  enrollmentID?: string;
  side: IdentityPhoto["side"];
  onClose: () => void;
}) {
  const [failedURL, setFailedURL] = useState("");
  const query = useQuery({
    queryKey: ["enrollment-photo-preview", userID, enrollmentID],
    queryFn: () =>
      get<EnrollmentDocuments>(enrollmentDocumentsPath(userID, enrollmentID)),
    gcTime: 0,
    retry: false,
  });
  const details = query.data?.details;
  const url = side === "正面" ? details?.front_url : details?.back_url;

  function retry() {
    void query.refetch().then(() => setFailedURL(""));
  }

  return (
    <Modal
      title={`身份证${side}`}
      open
      centered
      width={860}
      closable={{ "aria-label": "关闭照片预览" }}
      onCancel={onClose}
      footer={null}
    >
      <QueryState pending={query.isPending} error={query.error} retry={retry}>
        {url && url !== failedURL ? (
          <img
            src={url}
            className="identity-photo-preview"
            alt={`身份证${side}大图`}
            referrerPolicy="no-referrer"
            onError={() => setFailedURL(url)}
          />
        ) : (
          <Alert
            type="warning"
            title="该面照片暂时无法预览"
            action={
              <Button onClick={retry} loading={query.isFetching}>
                重新获取
              </Button>
            }
          />
        )}
      </QueryState>
    </Modal>
  );
}
