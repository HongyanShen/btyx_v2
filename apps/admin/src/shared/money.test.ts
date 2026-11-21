import { describe, expect, it } from "vitest";
import { formatFen, signedYuanToFen, yuanToFen } from "./money";

describe("金额字符串转换", () => {
  it("保留超过安全整数范围的金额精度", () => {
    expect(formatFen("9007199254740993")).toBe("90071992547409.93");
    expect(yuanToFen("90071992547409.93")).toBe("9007199254740993");
  });

  it("补齐分位，避免浮点运算", () => {
    expect(yuanToFen("7.5")).toBe("750");
    expect(formatFen("750")).toBe("7.50");
    expect(formatFen("-10")).toBe("-0.10");
  });

  it("拒绝不足一分和非数字输入", () => {
    expect(() => yuanToFen("0.001")).toThrow();
    expect(() => yuanToFen("1e3")).toThrow();
  });

  it("调整金额支持扣减并保持精度", () => {
    expect(signedYuanToFen("-7.50")).toBe("-750");
    expect(signedYuanToFen("7.50")).toBe("750");
    expect(() => signedYuanToFen("-0.001")).toThrow();
  });
});
