export function formatFen(value: string): string {
  if (!/^-?\d+$/.test(value)) {
    throw new Error("金额必须是整数分");
  }

  const amount = BigInt(value);
  const negative = amount < 0n;
  const absolute = negative ? -amount : amount;
  const yuan = absolute / 100n;
  const fen = (absolute % 100n).toString().padStart(2, "0");

  return `${negative ? "-" : ""}${yuan}.${fen}`;
}

export function yuanToFen(value: string): string {
  const matched = /^(0|[1-9]\d{0,15})(?:\.(\d{1,2}))?$/.exec(value.trim());
  if (!matched) {
    throw new Error("请输入最多两位小数的金额");
  }

  const yuan = BigInt(matched[1]);
  const fen = BigInt((matched[2] ?? "").padEnd(2, "0"));

  return (yuan * 100n + fen).toString();
}

export function signedYuanToFen(value: string): string {
  const text = value.trim();
  const negative = text.startsWith("-");
  const amount = yuanToFen(negative ? text.slice(1) : text);

  return negative && amount !== "0" ? `-${amount}` : amount;
}
