export function syncMarketingFilePicker(widget) {
  const picker = widget.querySelector("[data-file-picker]");
  const pickerLabel = widget.querySelector("[data-media-picker-label]");
  const fieldLabel = widget.querySelector("[data-media-field-label]");
  const kind = widget.closest("form")?.querySelector("input[name='kind']");
  if (!widget.matches("[data-marketing-files]") || !kind) return;
  const images = kind.value === "images";
  const limit = images ? 9 : 1;
  const items = [...widget.querySelectorAll("[data-file-item]")];
  const busy = widget.getAttribute("aria-busy") === "true";
  widget.dataset.media = kind.value;
  if (picker) {
    picker.multiple = images;
    picker.accept = images
      ? "image/jpeg,image/png,image/gif,image/webp"
      : "video/mp4";
    picker.disabled = busy || items.length >= limit;
  }
  if (pickerLabel) {
    pickerLabel.hidden = items.length >= limit;
    pickerLabel.textContent = images
      ? "＋ 上传图片（最多 9 张）"
      : "＋ 上传视频";
  }
  if (fieldLabel) fieldLabel.textContent = images ? "图片" : "视频文件";
  for (const choice of widget
    .closest("form")
    ?.querySelectorAll("[data-marketing-kind]") ?? [])
    choice.disabled = busy || items.length > 0;
  items.forEach((item, index) => {
    const preview = item.querySelector("[data-gallery-single]");
    if (preview) {
      preview.textContent = `预览附件 ${index + 1}`;
      preview.setAttribute("aria-label", preview.textContent);
    }
  });
}
