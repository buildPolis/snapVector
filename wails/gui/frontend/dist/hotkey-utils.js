(function (root, factory) {
  if (typeof module === "object" && module.exports) {
    module.exports = factory();
  } else {
    root.SV_Hotkey = factory();
  }
})(typeof self !== "undefined" ? self : this, function () {
  const MODIFIER_RANK = { mod: 0, ctrl: 1, alt: 2, shift: 3 };
  const CONTROL_KEYS = new Set(["escape", "enter", "backspace", "tab"]);
  const DISPLAY_MAC = { mod: "⌘", ctrl: "⌃", alt: "⌥", shift: "⇧", enter: "⏎", backspace: "⌫", escape: "⎋", arrowup: "↑", arrowdown: "↓", arrowleft: "←", arrowright: "→" };
  const DISPLAY_OTHER = { mod: "Ctrl", ctrl: "Ctrl", alt: "Alt", shift: "Shift" };

  // normalize converts a KeyboardEvent-like {key, metaKey, ctrlKey, altKey, shiftKey}
  // into a canonical combo string, or "" if only modifiers are pressed.
  // isMac controls whether metaKey maps to "mod" (mac) vs ctrlKey → "mod" (others).
  function normalize(event, isMac) {
    const key = (event.key || "").toLowerCase();
    if (!key || key === "meta" || key === "control" || key === "alt" || key === "shift") {
      return "";
    }
    const mods = [];
    if (isMac) {
      if (event.metaKey) mods.push("mod");
      if (event.ctrlKey) mods.push("ctrl");
    } else {
      if (event.ctrlKey || event.metaKey) mods.push("mod");
    }
    if (event.altKey) mods.push("alt");
    if (event.shiftKey) mods.push("shift");
    return sortModifiers([...mods, key].join("+"));
  }

  function sortModifiers(combo) {
    if (!combo) return "";
    const parts = combo.split("+");
    if (parts.length === 1) return parts[0];
    const main = parts[parts.length - 1];
    const mods = parts.slice(0, -1);
    mods.sort((a, b) => (MODIFIER_RANK[a] ?? 99) - (MODIFIER_RANK[b] ?? 99));
    return [...mods, main].join("+");
  }

  function comboToDisplay(combo, isMac) {
    if (!combo) return "Unbound";
    const parts = combo.split("+");
    if (isMac) {
      return parts
        .map((p) => DISPLAY_MAC[p] || (p.length === 1 ? p.toUpperCase() : capitalize(p)))
        .join(" ");
    }
    return parts
      .map((p) => DISPLAY_OTHER[p] || (p.length === 1 ? p.toUpperCase() : capitalize(p)))
      .join("+");
  }

  // detectConflict: given current bindings and a proposed (action, combo),
  // returns the conflicting action's name, or null if no conflict.
  function detectConflict(bindings, action, combo) {
    if (!combo) return null;
    for (const b of bindings) {
      if (b.action === action) continue;
      if (b.combo === combo) return b.action;
    }
    return null;
  }

  // isControlKey returns true if the key alone should control the recorder
  // (Enter/Esc/Backspace) rather than be captured as a binding's main key.
  function isControlKey(key) {
    return CONTROL_KEYS.has((key || "").toLowerCase());
  }

  // isRecordableMainKey returns false for modifier-only inputs and control keys
  // pressed without any modifier. Control keys WITH modifiers are recordable.
  function isRecordableMainKey(event) {
    const key = (event.key || "").toLowerCase();
    if (!key || key === "meta" || key === "control" || key === "alt" || key === "shift") {
      return false;
    }
    if (key === "+") return false; // "+" is the combo separator; can't bind it
    const hasMod = event.metaKey || event.ctrlKey || event.altKey || event.shiftKey;
    if (isControlKey(key) && !hasMod) return false;
    return true;
  }

  function capitalize(s) {
    return s.charAt(0).toUpperCase() + s.slice(1);
  }

  return { normalize, sortModifiers, comboToDisplay, detectConflict, isControlKey, isRecordableMainKey };
});
