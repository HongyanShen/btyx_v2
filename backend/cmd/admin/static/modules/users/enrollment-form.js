import { post } from "../../api/client.js";
export async function selectEnrollmentTag(button) {
  const form = button.form;
  const input = form?.querySelector("[data-enrollment-tag-input]");
  const tags = form?.querySelector("[data-enrollment-tag-options]");
  const name = input?.value
    .trim()
    .replace(/^[#＃]+/, "")
    .trim();
  if (!form || !input || !tags || !name || button.disabled) return;
  const existing = [...tags.querySelectorAll("[data-tag-name]")].find(
    (label) => label.dataset.tagName === name,
  );
  const selected = existing?.querySelector("input");
  if (selected) {
    selected.checked = true;
    input.value = "";
    return;
  }
  const submit = [...form.querySelectorAll("button[type='submit']")];
  button.disabled = true;
  submit.forEach((control) => (control.disabled = true));
  try {
    const tag = await post("/marketing-tags", { name });
    if (!form.isConnected) return;
    tags.querySelector("[data-tag-empty]")?.remove();
    const label = document.createElement("label");
    label.dataset.tagName = tag.name;
    const choice = document.createElement("input");
    choice.type = "checkbox";
    choice.name = "tags";
    choice.value = tag.key;
    choice.checked = true;
    label.append(choice, ` #${tag.name}`);
    tags.append(label);
    input.value = "";
  } finally {
    button.disabled = false;
    submit.forEach((control) => (control.disabled = false));
  }
}
