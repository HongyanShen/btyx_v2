let active;
function updateYear(picker, year) {
  const latest = picker.dataset.latest ?? "";
  picker.dataset.year = String(year);
  const heading = picker.querySelector("[data-month-heading]");
  if (heading) heading.textContent = `${year} 年`;
  const previous = picker.querySelector("[data-month-year='-1']");
  const next = picker.querySelector("[data-month-year='1']");
  if (previous) previous.disabled = year <= 1;
  if (next) next.disabled = year >= Number(latest.slice(0, 4));
  for (const choice of picker.querySelectorAll("[data-month-number]")) {
    const number = choice.dataset.monthNumber ?? "";
    choice.value = `${String(year).padStart(4, "0")}-${number.padStart(2, "0")}`;
    choice.disabled = choice.value > latest;
    choice.setAttribute(
      "aria-pressed",
      String(choice.value === picker.dataset.month),
    );
  }
}
function closePicker(restoreFocus = false) {
  const picker = active;
  active = undefined;
  const popup = picker?.querySelector(".month-picker-popover");
  const button = picker?.querySelector(".month-picker-trigger");
  if (popup) popup.hidden = true;
  if (button) {
    button.setAttribute("aria-expanded", "false");
    if (restoreFocus) button.focus({ preventScroll: true });
  }
}
function openPicker(picker, trigger) {
  const popup = picker.querySelector(".month-picker-popover");
  if (!popup) return;
  const current = picker.dataset.month ?? "";
  const latest = picker.dataset.latest ?? "";
  const opening = current > latest ? latest : current;
  updateYear(picker, Number(opening.slice(0, 4)));
  popup.hidden = false;
  trigger.setAttribute("aria-expanded", "true");
  active = picker;
  const width = popup.offsetWidth;
  const right = trigger.getBoundingClientRect().right;
  const left = Math.max(8, Math.min(right - width, innerWidth - width - 8));
  popup.style.left = `${left - picker.getBoundingClientRect().left}px`;
  const month = Array.from(popup.querySelectorAll("[data-month-number]")).find(
    (button) => button.value === opening,
  );
  month?.focus({ preventScroll: true });
}
document.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof Element)) return;
  const picker = target.closest("[data-month-picker]");
  const trigger = target.closest(".month-picker-trigger");
  if (picker && trigger) {
    if (active === picker) closePicker(true);
    else {
      closePicker();
      openPicker(picker, trigger);
    }
    return;
  }
  const yearButton = target.closest("[data-month-year]");
  if (picker && yearButton && !yearButton.disabled) {
    updateYear(
      picker,
      Number(picker.dataset.year) + Number(yearButton.dataset.monthYear),
    );
    if (yearButton.disabled)
      picker
        .querySelector("[data-month-heading]")
        ?.focus({ preventScroll: true });
    return;
  }
  if (active && (active !== picker || target.closest("[data-month-number]")))
    closePicker();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && active) {
    event.preventDefault();
    closePicker(true);
  }
});
