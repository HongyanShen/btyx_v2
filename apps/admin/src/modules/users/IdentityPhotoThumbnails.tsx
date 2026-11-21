export interface IdentityPhoto {
  side: "正面" | "反面";
  url: string;
}

export function IdentityPhotoThumbnails({
  details,
  onPreview,
  onImageError,
}: {
  details: { front_url: string; back_url: string } | null;
  onPreview: (photo: IdentityPhoto) => void;
  onImageError?: () => void;
}) {
  if (!details?.front_url && !details?.back_url) {
    return "—";
  }

  const photos: IdentityPhoto[] = [
    { side: "正面", url: details.front_url },
    { side: "反面", url: details.back_url },
  ];

  return (
    <div className="identity-photo-thumbnails">
      {photos.map((photo) => (
        <div key={photo.side}>
          {photo.url ? (
            <button
              className="identity-photo-thumbnail"
              type="button"
              aria-label={`查看身份证${photo.side}大图`}
              onClick={() => onPreview(photo)}
            >
              <img
                src={photo.url}
                alt=""
                loading="lazy"
                referrerPolicy="no-referrer"
                onError={onImageError}
              />
              {photo.side}
            </button>
          ) : (
            <span className="identity-photo-missing">{photo.side} —</span>
          )}
        </div>
      ))}
    </div>
  );
}
