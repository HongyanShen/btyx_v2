import "./account-search-link.css";

export function AccountSearchLink({
  value,
  href,
}: {
  value: string;
  href?: string;
}) {
  return href ? (
    <a
      className="account-search-link"
      href={href}
      title={value}
      target="_blank"
      rel="noreferrer"
    >
      {value}
    </a>
  ) : (
    value || "—"
  );
}
