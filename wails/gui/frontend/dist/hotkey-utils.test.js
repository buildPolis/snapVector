const test = require("node:test");
const assert = require("node:assert/strict");
const { normalize, sortModifiers, comboToDisplay, detectConflict, isRecordableMainKey } = require("./hotkey-utils.js");

function ev(key, opts = {}) {
  return { key, metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, ...opts };
}

test("normalize: mac metaKey becomes mod", () => {
  assert.equal(normalize(ev("z", { metaKey: true }), true), "mod+z");
});

test("normalize: non-mac ctrlKey becomes mod", () => {
  assert.equal(normalize(ev("z", { ctrlKey: true }), false), "mod+z");
});

test("normalize: non-mac metaKey (Super/Win) also maps to mod", () => {
  assert.equal(normalize(ev("z", { metaKey: true }), false), "mod+z");
});

test("normalize: mac ctrlKey is distinct from mod", () => {
  assert.equal(normalize(ev("z", { ctrlKey: true }), true), "ctrl+z");
});

test("normalize: modifier order is canonical", () => {
  assert.equal(normalize(ev("Q", { metaKey: true, shiftKey: true, altKey: true }), true), "mod+alt+shift+q");
});

test("normalize: modifier-only returns empty", () => {
  assert.equal(normalize(ev("Meta", { metaKey: true }), true), "");
  assert.equal(normalize(ev("Shift", { shiftKey: true }), true), "");
});

test("sortModifiers is idempotent", () => {
  assert.equal(sortModifiers("mod+shift+z"), "mod+shift+z");
  assert.equal(sortModifiers("shift+mod+z"), "mod+shift+z");
  assert.equal(sortModifiers("z"), "z");
  assert.equal(sortModifiers(""), "");
});

test("comboToDisplay mac", () => {
  assert.equal(comboToDisplay("mod+shift+q", true), "⌘ ⇧ Q");
  assert.equal(comboToDisplay("v", true), "V");
  assert.equal(comboToDisplay("", true), "Unbound");
});

test("comboToDisplay non-mac", () => {
  assert.equal(comboToDisplay("mod+shift+q", false), "Ctrl+Shift+Q");
  assert.equal(comboToDisplay("v", false), "V");
});

test("detectConflict finds other action", () => {
  const bindings = [
    { action: "tool.select", combo: "v" },
    { action: "edit.undo", combo: "mod+z" },
  ];
  assert.equal(detectConflict(bindings, "tool.arrow", "mod+z"), "edit.undo");
  assert.equal(detectConflict(bindings, "tool.arrow", "a"), null);
});

test("detectConflict ignores same action", () => {
  const bindings = [{ action: "edit.undo", combo: "mod+z" }];
  assert.equal(detectConflict(bindings, "edit.undo", "mod+z"), null);
});

test("detectConflict ignores empty combo", () => {
  const bindings = [{ action: "tool.select", combo: "" }];
  assert.equal(detectConflict(bindings, "tool.arrow", ""), null);
});

test("isRecordableMainKey rejects modifier-only and bare control keys", () => {
  assert.equal(isRecordableMainKey(ev("Meta", { metaKey: true })), false);
  assert.equal(isRecordableMainKey(ev("Escape")), false);
  assert.equal(isRecordableMainKey(ev("Enter")), false);
  assert.equal(isRecordableMainKey(ev("Enter", { metaKey: true })), true);
  assert.equal(isRecordableMainKey(ev("v")), true);
});

test("isRecordableMainKey rejects '+' to avoid split ambiguity", () => {
  assert.equal(isRecordableMainKey(ev("+", { metaKey: true, shiftKey: true })), false);
});
