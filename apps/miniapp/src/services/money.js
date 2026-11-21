function formatFen(value) {
  if (!/^-?\d+$/.test(value)) {
    throw new Error("金额必须是整数分");
  }

  const negative = value.startsWith("-");
  const digits = (negative ? value.slice(1) : value).padStart(3, "0");
  const yuan = digits.slice(0, -2).replace(/^0+(?=\d)/, "");

  return `${negative ? "-" : ""}${yuan}.${digits.slice(-2)}`;
}

function yuanToFen(value) {
  const matched = /^(0|[1-9]\d{0,15})(?:\.(\d{1,2}))?$/.exec(
    String(value).trim(),
  );
  if (!matched) {
    throw new Error("请输入最多两位小数的金额");
  }

  const digits = `${matched[1]}${(matched[2] || "").padEnd(2, "0")}`;

  return digits.replace(/^0+(?=\d)/, "");
}

function doubleFen(value) {
  if (!/^-?\d+$/.test(value)) {
    throw new Error("金额必须是整数分");
  }

  const negative = value.startsWith("-");
  const digits = negative ? value.slice(1) : value;
  let doubled = "";
  let carry = 0;

  for (let index = digits.length - 1; index >= 0; index--) {
    const digit = (Number(digits[index]) << 1) + carry;
    doubled = `${digit % 10}${doubled}`;
    carry = digit >= 10 ? 1 : 0;
  }

  if (carry) {
    doubled = `1${doubled}`;
  }

  doubled = doubled.replace(/^0+(?=\d)/, "");
  return negative && doubled !== "0" ? `-${doubled}` : doubled;
}

module.exports = { doubleFen, formatFen, yuanToFen };
