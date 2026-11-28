const twoDigits = (value) => String(value).padStart(2, "0");
const beijingClock = new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Shanghai",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

function beijingNow() {
  return beijingClock.format(new Date()).replace(" ", "T");
}

function calendarDate(day) {
  return new Date(day + "T00:00:00Z");
}

function dateKey(date) {
  return date.toISOString().slice(0, 10);
}

function placePanel(panel, field) {
  const rect = field.getBoundingClientRect();
  const gap = 6;
  const edge = 8;
  const below = rect.bottom + gap;
  const above = rect.top - panel.offsetHeight - gap;
  const lastTop =
    document.documentElement.clientHeight - panel.offsetHeight - edge;
  panel.style.top =
    Math.max(edge, below <= lastTop ? below : Math.min(above, lastTop)) + "px";
  panel.style.left =
    Math.max(
      edge,
      Math.min(
        rect.left,
        document.documentElement.clientWidth - panel.offsetWidth - edge,
      ),
    ) + "px";
}

function openCalendar(field, title) {
  const now = beijingNow();
  const today = now.slice(0, 10);
  const initial = field.value || now;
  let selected = initial.slice(0, 10);
  let month = calendarDate(selected.slice(0, 7) + "-01");
  const panel = document.createElement("div");
  panel.className = "datetime-panel";
  panel.popover = "auto";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-label", title);
  panel.innerHTML = `
    <div class="datetime-heading">
      <strong aria-live="polite"></strong>
      <button type="button" data-month="-1" aria-label="上个月">‹</button>
      <button type="button" data-month="1" aria-label="下个月">›</button>
    </div>
    <div class="datetime-week" aria-hidden="true"></div>
    <div class="datetime-days"></div>
    <div class="datetime-time"><span>时间</span><div>
      <select aria-label="小时"></select><b aria-hidden="true">:</b><select aria-label="分钟"></select>
    </div></div>
    <div class="datetime-actions">
      <button type="button" data-action="clear">清空</button>
      <button type="button" data-action="now">此刻</button>
      <button type="button" data-action="confirm">确定</button>
    </div>`;
  const [hours, minutes] = panel.querySelectorAll("select");
  for (const [select, count] of [
    [hours, 24],
    [minutes, 60],
  ]) {
    for (let index = 0; index < count; index++) {
      select.add(new Option(twoDigits(index), twoDigits(index)));
    }
  }
  function setTime(value) {
    hours.value = value.slice(11, 13);
    minutes.value = value.slice(14, 16);
  }
  setTime(initial);
  const week = panel.querySelector(".datetime-week");
  for (const day of "一二三四五六日") {
    const label = document.createElement("span");
    label.textContent = day;
    week.append(label);
  }
  const grid = panel.querySelector(".datetime-days");
  function render(focused = "") {
    panel.querySelector("strong").textContent =
      month.getUTCFullYear() + "年" + (month.getUTCMonth() + 1) + "月";
    const first = new Date(month);
    first.setUTCDate(1 - ((first.getUTCDay() + 6) % 7));
    const tabDay =
      focused ||
      (selected.slice(0, 7) === dateKey(month).slice(0, 7)
        ? selected
        : dateKey(month));
    const days = document.createDocumentFragment();
    for (let index = 0; index < 42; index++) {
      const key = dateKey(first);
      const day = document.createElement("button");
      day.type = "button";
      day.dataset.day = key;
      day.disabled =
        first.getUTCFullYear() < 1 || first.getUTCFullYear() > 9999;
      day.textContent = first.getUTCDate();
      day.setAttribute("aria-label", key);
      day.setAttribute("aria-pressed", String(key === selected));
      if (key === today) day.setAttribute("aria-current", "date");
      day.toggleAttribute(
        "data-adjacent",
        first.getUTCMonth() !== month.getUTCMonth(),
      );
      day.tabIndex = key === tabDay ? 0 : -1;
      days.append(day);
      first.setUTCDate(first.getUTCDate() + 1);
    }
    grid.replaceChildren(days);
    if (focused) grid.querySelector('[tabindex="0"]').focus();
  }
  render();
  function showDay(key) {
    month = calendarDate(key.slice(0, 7) + "-01");
    render(key);
  }
  grid.addEventListener("keydown", (event) => {
    const steps = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };
    if (!Object.hasOwn(steps, event.key)) return;
    event.preventDefault();
    const date = calendarDate(event.target.dataset.day);
    date.setUTCDate(date.getUTCDate() + steps[event.key]);
    if (date.getUTCFullYear() >= 1 && date.getUTCFullYear() <= 9999)
      showDay(dateKey(date));
  });
  panel.addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button) return;
    if (button.dataset.month) {
      const next = new Date(month);
      next.setUTCMonth(next.getUTCMonth() + Number(button.dataset.month));
      if (next.getUTCFullYear() < 1 || next.getUTCFullYear() > 9999) return;
      month = next;
      render();
    } else if (button.dataset.day) {
      selected = button.dataset.day;
      showDay(selected);
    } else if (button.dataset.action === "now") {
      const current = beijingNow();
      selected = current.slice(0, 10);
      setTime(current);
      showDay(selected);
    } else {
      const value =
        button.dataset.action === "clear"
          ? ""
          : selected + "T" + hours.value + ":" + minutes.value;
      if (field.value !== value) {
        field.value = value;
        field.dispatchEvent(new Event("input", { bubbles: true }));
        field.dispatchEvent(new Event("change", { bubbles: true }));
      }
      panel.hidePopover();
      field.focus({ preventScroll: true });
    }
  });

  (field.closest("dialog, #admin-workspace") || document.body).append(panel);
  panel.showPopover({ source: field });
  field.setAttribute("aria-expanded", "true");
  const reposition = () => placePanel(panel, field);
  reposition();
  grid.querySelector('[tabindex="0"]').focus({ preventScroll: true });
  const listeners = new AbortController();
  window.addEventListener("resize", reposition, { signal: listeners.signal });
  document.addEventListener("scroll", reposition, {
    capture: true,
    signal: listeners.signal,
  });
  function cleanup() {
    listeners.abort();
    field.setAttribute("aria-expanded", "false");
    panel.remove();
  }
  panel.addEventListener("beforetoggle", (event) => {
    if (event.newState === "closed") cleanup();
  });
  return cleanup;
}

class AdminDateTime extends HTMLElement {
  connectedCallback() {
    const field = this.querySelector('input[type="datetime-local"]');
    if (!field || !HTMLElement.prototype.showPopover) return;
    const title = field.labels[0]?.textContent.trim() || "选择日期和时间";
    const display = document.createElement("span");
    display.className = "datetime-display";
    display.setAttribute("aria-hidden", "true");
    this.append(display);
    const refresh = () => {
      display.dataset.text = field.value.replace("T", " ") || "选择日期和时间";
      display.toggleAttribute("data-empty", !field.value);
    };
    refresh();
    field.setAttribute("aria-haspopup", "dialog");
    field.setAttribute("aria-expanded", "false");
    const handlers = new AbortController();
    const options = { signal: handlers.signal };
    field.addEventListener("input", refresh, options);
    field.addEventListener("change", refresh, options);
    field.form?.addEventListener(
      "reset",
      () => queueMicrotask(refresh),
      options,
    );
    const open = (event) => {
      event.preventDefault();
      if (
        field.getAttribute("aria-expanded") === "true" ||
        field.readOnly ||
        field.disabled
      )
        return;
      this.closeCalendar = openCalendar(field, title);
    };
    field.addEventListener("click", open, options);
    field.addEventListener(
      "keydown",
      (event) => {
        if (
          event.key === "Enter" ||
          event.key === " " ||
          (event.altKey && event.key === "ArrowDown")
        )
          open(event);
      },
      options,
    );
    this.teardown = () => {
      handlers.abort();
      this.closeCalendar?.();
      display.remove();
    };
  }
  disconnectedCallback() {
    this.teardown?.();
  }
}
customElements.define("admin-datetime", AdminDateTime);
