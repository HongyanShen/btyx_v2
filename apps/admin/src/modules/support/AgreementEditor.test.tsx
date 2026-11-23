import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AgreementContent } from "./AgreementEditor";

afterEach(cleanup);

describe("agreement preview", () => {
  it("keeps old agreements literal, including angle brackets and line breaks", () => {
    const text = "第一条\n<strong>旧版原文</strong>";
    const { container } = render(<AgreementContent body={text} />);
    expect(container.textContent).toBe(text);
    expect(container.querySelector("strong")).toBeNull();
  });

  it("shows the supported formatting without pasted styles or embedded media", () => {
    const { container } = render(
      <AgreementContent
        html={
          '<h2>条款</h2><ol><li><strong style="color:red">正文</strong><ins>重点</ins></li></ol><img src="test.png">'
        }
      />,
    );
    expect(container.querySelector("h2")?.textContent).toBe("条款");
    expect(container.querySelector("ol > li > strong")?.textContent).toBe(
      "正文",
    );
    expect(container.querySelector("li > ins")?.textContent).toBe("重点");
    expect(container.querySelector("[style],img")).toBeNull();
  });
});
