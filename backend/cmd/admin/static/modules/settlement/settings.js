document.addEventListener("change", ({ target }) => {
  if (!(target instanceof HTMLSelectElement)) return;
  const form = target.closest("[data-distribution-settings]");
  if (!form) return;

  if (target.name === "settlement_frequency") {
    for (const field of form.querySelectorAll("[data-schedule-modes]")) {
      field.hidden = !field.dataset.scheduleModes
        .split(" ")
        .includes(target.value);
    }
  }
  if (target.name === "expiry_mode") {
    form.querySelector("[data-expiry-duration]").hidden =
      target.value !== "fixed_duration";
  }
});
