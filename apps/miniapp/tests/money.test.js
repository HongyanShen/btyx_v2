const assert = require("node:assert/strict");
const test = require("node:test");
const { doubleFen, formatFen, yuanToFen } = require("../src/services/money");

test("金额超过 JS 安全整数范围仍保持精度", () => {
  assert.equal(formatFen("9007199254740993"), "90071992547409.93");
  assert.equal(yuanToFen("90071992547409.93"), "9007199254740993");
  assert.equal(formatFen(doubleFen("999999999999999999")), "19999999999999999.98");
});

test("无需浮点数和 BigInt 也能格式化分位", () => {
  assert.equal(formatFen("0"), "0.00");
  assert.equal(formatFen("5"), "0.05");
  assert.equal(formatFen("-10"), "-0.10");
  assert.equal(yuanToFen("7.5"), "750");
});

test("拒绝非法金额和不足一分的输入", () => {
  assert.throws(() => yuanToFen("1.001"));
  assert.throws(() => yuanToFen("1e3"));
  assert.throws(() => yuanToFen("-1"));
});
