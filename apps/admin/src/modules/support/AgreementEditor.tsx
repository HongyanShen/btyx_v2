import { useEffect, useRef } from "react";
import Quill from "quill";
import DOMPurify from "dompurify";
import "quill/dist/quill.snow.css";
import "./agreements.css";

const allowedTags = [
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
];

function cleanMarkup(markup: string) {
  return DOMPurify.sanitize(markup, {
    ALLOWED_TAGS: allowedTags,
    ALLOWED_ATTR: [],
    ALLOW_DATA_ATTR: false,
    ALLOW_ARIA_ATTR: false,
  }).replaceAll("&nbsp;", " ");
}

export function AgreementContent({
  body,
  html,
}: {
  body?: string;
  html?: string;
}) {
  if (!html)
    return <div className="agreement-content agreement-plain">{body}</div>;
  return (
    <div
      className="agreement-content"
      dangerouslySetInnerHTML={{ __html: cleanMarkup(html) }}
    />
  );
}

const toolLabels: Record<string, string> = {
  "header:2": "一级条款标题",
  "header:3": "二级条款标题",
  bold: "加粗",
  italic: "斜体",
  underline: "下划线",
  "list:ordered": "编号列表",
  "list:bullet": "项目列表",
  clean: "清除格式",
};

interface Props {
  id?: string;
  value?: string;
  onChange?: (value: string) => void;
}

export function AgreementEditor({ id, value = "", onChange }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const quill = useRef<Quill | null>(null);
  const emitted = useRef("");
  const notify = useRef(onChange);

  useEffect(() => {
    notify.current = onChange;
  }, [onChange]);

  useEffect(() => {
    const container = host.current!;
    const mount = document.createElement("div");
    container.appendChild(mount);
    const editor = new Quill(mount, {
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
    if (id) editor.root.id = id;
    editor.root.setAttribute("role", "textbox");
    editor.root.setAttribute("aria-label", "协议正文");
    editor.root.setAttribute("aria-multiline", "true");
    container
      .querySelectorAll<HTMLButtonElement>(".ql-toolbar button")
      .forEach((button) => {
        const format = [...button.classList]
          .find((name) => name.startsWith("ql-"))
          ?.slice(3);
        const label =
          toolLabels[button.value ? format + ":" + button.value : format || ""];
        if (label) {
          button.title = label;
          button.setAttribute("aria-label", label);
        }
      });
    quill.current = editor;
    const changed = () => {
      emitted.current = editor
        .getText()
        .replace(/[\s\u200B-\u200D\uFEFF]/gu, "")
        ? cleanMarkup(editor.getSemanticHTML())
        : "";
      notify.current?.(emitted.current);
    };
    editor.on("text-change", changed);
    return () => {
      editor.off("text-change", changed);
      quill.current = null;
      emitted.current = "";
      container.replaceChildren();
    };
  }, [id]);

  useEffect(() => {
    const editor = quill.current;
    if (editor && value !== emitted.current) {
      editor.setContents(
        editor.clipboard.convert({ html: cleanMarkup(value) }),
        "silent",
      );
      emitted.current = value;
    }
  }, [value, id]);

  return <div className="agreement-editor" ref={host} />;
}
