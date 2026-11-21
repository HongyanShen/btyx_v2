import type { Submission } from "../../generated/models";

const beijingTime = new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Shanghai",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

export function reviewDisplayID(id: string) {
  return id.slice(-4);
}

export function SubmissionLink({
  submission,
  label,
}: {
  submission: Submission;
  label?: string;
}) {
  const url = submission.url;
  if (!/^https?:\/\//i.test(url)) return <span>{label ?? url}</span>;

  return (
    <a href={url} target="_blank" rel="noreferrer">
      {label ?? url}
    </a>
  );
}

export function SubmissionTime({ submissions }: { submissions: Submission[] }) {
  let latest: Date | undefined;
  for (const submission of submissions) {
    if (!submission.submitted_at) continue;

    const time = new Date(submission.submitted_at);
    if (Number.isFinite(time.getTime()) && (!latest || time > latest))
      latest = time;
  }

  if (!latest) return <span className="review-note">—</span>;

  const parts = beijingTime.formatToParts(latest);
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((entry) => entry.type === type)?.value;
  return (
    <time dateTime={latest.toISOString()} title={latest.toISOString()}>
      {part("month")}-{part("day")} {part("hour")}:{part("minute")}
    </time>
  );
}
