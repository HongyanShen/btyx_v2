import DOMPurify from "../../vendor/purify.js";
import "../../vendor/quill.js";
const tools = [
  {
    label: "一级条款标题",
    format: "header",
    value: "2",
  },
  {
    label: "二级条款标题",
    format: "header",
    value: "3",
  },
  {
    label: "加粗",
    format: "bold",
  },
  {
    label: "斜体",
    format: "italic",
  },
  {
    label: "下划线",
    format: "underline",
  },
  {
    label: "编号列表",
    format: "list",
    value: "ordered",
  },
  {
    label: "项目列表",
    format: "list",
    value: "bullet",
  },
  {
    label: "清除格式",
    format: "clean",
  },
];
function clean(markup) {
  return DOMPurify.sanitize(markup, {
    ALLOWED_TAGS: [
      "p",
      "br",
      "h1",
      "h2",
      "h3",
      "h4",
      "h5",
      "h6",
      "strong",
      "em",
      "u",
      "ins",
      "ol",
      "ul",
      "li",
    ],
    ALLOWED_ATTR: [],
    ALLOW_DATA_ATTR: false,
    ALLOW_ARIA_ATTR: false,
  }).replaceAll("&nbsp;", " ");
}
export function initAgreementForm(form) {
  const host = form.querySelector("[data-agreement-mount]");
  const source = form.querySelector("textarea[name='body_html']");
  const title = form.querySelector("input[name='title']");
  const heading = form.querySelector(".agreement-preview article h3");
  const preview = form.querySelector(".agreement-preview .agreement-content");
  if (!host || !source || !title || !heading || !preview) return;
  const mount = document.createElement("div");
  host.append(mount);
  const editor = new globalThis.Quill(mount, {
    theme: "snow",
    placeholder: "输入或粘贴协议正文",
    formats: ["header", "bold", "italic", "underline", "list"],
    modules: {
      toolbar: [
        [{ header: 2 }, { header: 3 }],
        ["bold", "italic", "underline"],
        [{ list: "ordered" }, { list: "bullet" }],
        ["clean"],
      ],
    },
  });
  editor.root.setAttribute("role", "textbox");
  editor.root.setAttribute("aria-label", "协议正文");
  editor.root.setAttribute("aria-multiline", "true");
  for (const { label, format, value } of tools) {
    const tool = host.querySelector(
      `button.ql-${format}${value ? `[value="${value}"]` : ""}`,
    );
    tool?.setAttribute("title", label);
    tool?.setAttribute("aria-label", label);
  }
  function updatePreview() {
    if (!heading || !preview || !source || !title) return;
    heading.textContent = title.value;
    source.value = editor.getText().replace(/[\s\u200B-\u200D\uFEFF]/gu, "")
      ? clean(editor.getSemanticHTML())
      : "";
    preview.innerHTML = source.value;
  }
  editor.setContents(
    editor.clipboard.convert({ html: clean(source.value) }),
    "silent",
  );
  editor.on("text-change", updatePreview);
  title.addEventListener("input", updatePreview);
  source.required = false;
  source.hidden = true;
  host.hidden = false;
  updatePreview();
}
