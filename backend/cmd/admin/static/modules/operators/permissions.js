export function initOperatorPermissions(group) {
  const all = group.querySelector("[data-select-permissions]");
  const choices = [
    ...group.querySelectorAll(".operator-permission-options input"),
  ];

  function syncSelection() {
    const count = choices.filter((choice) => choice.checked).length;
    all.disabled = choices.length === 0;
    all.checked = count > 0 && count === choices.length;
    all.indeterminate = count > 0 && count < choices.length;
  }

  group.onchange = (event) => {
    if (event.target === all) {
      for (const choice of choices) choice.checked = all.checked;
    }
    syncSelection();
  };
  syncSelection();
}
