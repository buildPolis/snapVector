const SVG_NS = "http://www.w3.org/2000/svg";
const DEFAULT_ZOOM = 0.5;
const DEFAULT_STROKE_WIDTH = 10;
const BASELINE_ARROW_TAIL_X = 6;
const BASELINE_ARROW_TAIL_Y = 60;
const BASELINE_ARROW_TIP_X = 228;
const BASELINE_ARROW_TIP_Y = 60;
const BASELINE_ARROW_POINTS = [
  [6, 60],
  [132, 48],
  [132, 24],
  [228, 60],
  [132, 96],
  [132, 72],
];

// Fields that belong to the current active tab. When switching tabs, these
// are commit-swapped between the global `state` and the tab's own object.
// Add new per-tab fields here or they will leak across tabs.
const TAB_SCOPED_KEYS = [
  "capture", "annotations", "selectedId", "tool",
  "history", "future", "action",
  "zoom", "zoomAutoFit", "pan",
  "numberedCircle", "document",
];

function makeTabData() {
  return {
    capture: null,
    annotations: [],
    selectedId: null,
    tool: "select",
    history: [],
    future: [],
    action: null,
    zoom: DEFAULT_ZOOM,
    zoomAutoFit: true,
    pan: { x: 0, y: 0 },
    numberedCircle: {
      nextNumber: 1,
      radius: 28,
      strokeColor: "#E53935",
      outlineColor: "#FFFFFF",
      textColor: "#FFFFFF",
      strokeWidth: 6,
    },
    document: {
      path: "",
      name: "Untitled",
      dirty: false,
      savedFingerprint: "",
      menuOpen: false,
    },
  };
}

let tabSerial = 0;
const tabs = []; // [{ id, title, ...TAB_SCOPED_KEYS }]
let activeTabId = null;

function makeTab(title = "Untitled") {
  return { id: `tab-${++tabSerial}`, title, ...makeTabData() };
}

function tabById(id) { return tabs.find((t) => t.id === id); }
function activeTab() { return tabById(activeTabId); }

function commitActiveTab() {
  const t = activeTab();
  if (!t) return;
  for (const k of TAB_SCOPED_KEYS) t[k] = state[k];
}

function loadTab(id) {
  const t = tabById(id);
  if (!t) return;
  for (const k of TAB_SCOPED_KEYS) state[k] = t[k];
  activeTabId = id;
}

const state = {
  ...makeTabData(),
  // UI-level flags that are NOT per-tab:
  pointer: { x: 0, y: 0 },
  inspectorCollapsed: false,
};

const IS_MAC = /mac|iphone|ipad|ipod/i.test(navigator.platform);

const hotkeys = {
  bindings: [],              // Array<{action, combo, scope}>
  comboToAction: new Map(),  // combo → action
  suspended: false,          // true while modal is recording
};

const prefs = {
  draft: [],           // working copy during modal session
  exportDirectory: "",
  dirty: false,
  recordingAction: null,
  recordingBuffer: "",
};

const userPreferences = {
  exportDirectory: "",
};

const defaultPreferences = {
  exportDirectory: "",
};

const els = {
  captureTitle: document.getElementById("captureTitle"),
  fileMenuButton: document.getElementById("fileMenuButton"),
  fileMenu: document.getElementById("fileMenu"),
  openDocumentButton: document.getElementById("openDocumentButton"),
  saveDocumentButton: document.getElementById("saveDocumentButton"),
  saveDocumentAsButton: document.getElementById("saveDocumentAsButton"),
  documentBadge: document.getElementById("documentBadge"),
  captureButton: document.getElementById("captureButton"),
  captureRegionButton: document.getElementById("captureRegionButton"),
  captureAllDisplaysButton: document.getElementById("captureAllDisplaysButton"),
  undoButton: document.getElementById("undoButton"),
  redoButton: document.getElementById("redoButton"),
  zoomOutButton: document.getElementById("zoomOutButton"),
  zoomResetButton: document.getElementById("zoomResetButton"),
  zoomInButton: document.getElementById("zoomInButton"),
  copyButton: document.getElementById("copyButton"),
  exportButton: document.getElementById("exportButton"),
  exportFormat: document.getElementById("exportFormat"),
  toggleInspectorButton: document.getElementById("toggleInspectorButton"),
  tabStrip: document.getElementById("tabStrip"),
  canvasHost: document.getElementById("canvasHost"),
  canvasStage: document.getElementById("canvasStage"),
  captureImage: document.getElementById("captureImage"),
  annotationLayer: document.getElementById("annotationLayer"),
  htmlAnnotationLayer: document.getElementById("htmlAnnotationLayer"),
  selectionBox: document.getElementById("selectionBox"),
  draftBox: document.getElementById("draftBox"),
  emptyState: document.getElementById("emptyState"),
  selectedMeta: document.getElementById("selectedMeta"),
  geometryFields: document.getElementById("geometryFields"),
  textContent: document.getElementById("textContent"),
  textVariant: document.getElementById("textVariant"),
  textFontSize: document.getElementById("textFontSize"),
  textMaxWidth: document.getElementById("textMaxWidth"),
  blurRadius: document.getElementById("blurRadius"),
  cornerRadius: document.getElementById("cornerRadius"),
  feather: document.getElementById("feather"),
  sectionNumberedCircle: document.getElementById("sectionNumberedCircle"),
  numberedStarting: document.getElementById("numberedStarting"),
  numberedThis: document.getElementById("numberedThis"),
  numberedRadius: document.getElementById("numberedRadius"),
  numberedStrokeWidth: document.getElementById("numberedStrokeWidth"),
  numberedFillColor: document.getElementById("numberedFillColor"),
  numberedOutlineColor: document.getElementById("numberedOutlineColor"),
  numberedTextColor: document.getElementById("numberedTextColor"),
  statusX: document.getElementById("statusX"),
  statusY: document.getElementById("statusY"),
  statusZoom: document.getElementById("statusZoom"),
  statusCount: document.getElementById("statusCount"),
  statusSelected: document.getElementById("statusSelected"),
  toast: document.getElementById("toast"),
  toolButtons: Array.from(document.querySelectorAll("[data-tool]")),
  preferencesButton: document.getElementById("preferencesButton"),
  preferencesModal: document.getElementById("preferencesModal"),
  preferencesBody: document.getElementById("preferencesBody"),
  preferencesClose: document.getElementById("preferencesClose"),
  preferencesCancel: document.getElementById("preferencesCancel"),
  preferencesSave: document.getElementById("preferencesSave"),
  preferencesResetAll: document.getElementById("preferencesResetAll"),
  preferencesFilter: document.getElementById("preferencesFilter"),
  preferencesStatus: document.getElementById("preferencesStatus"),
  preferencesConflict: document.getElementById("preferencesConflict"),
};

const backend = createBackend();

async function init() {
  const firstTab = makeTab("Untitled");
  tabs.push(firstTab);
  loadTab(firstTab.id);
  bindUI();
  renderTabStrip();
  await loadPreferences();
  await loadHotkeys();
  window.addEventListener("keydown", onRecorderKeydown, true); // capture phase
  window.addEventListener("keydown", onGlobalKeydown);
  subscribeGlobalHotkeyEvents();
  // No auto-capture: the hide/show dance in the Go capture path would flash
  // the window right after it first appears. Let the user click a capture
  // button when they're ready.
  render();
}

function subscribeGlobalHotkeyEvents() {
  // Global (OS-level) hotkeys fire in Go and are relayed here via EventsEmit —
  // running them through hotkeyActions() keeps in-app and global hotkeys on
  // one code path, so the captured PNG lands in the canvas state just like
  // a button click would.
  const runtime = window.runtime;
  if (!runtime || typeof runtime.EventsOn !== "function") return;
  runtime.EventsOn("snapvector:hotkey", (action) => {
    if (typeof action !== "string" || !action) return;
    const handler = hotkeyActions()[action];
    if (!handler) {
      console.warn("global hotkey event with unknown action:", action);
      return;
    }
    handler();
  });
}

function bindUI() {
  els.toolButtons.forEach((button) => {
    button.addEventListener("click", async () => {
      const tool = button.dataset.tool;
      if (!tool) return;
      setTool(tool);
    });
  });

  els.undoButton.addEventListener("click", undo);
  els.redoButton.addEventListener("click", redo);
  els.fileMenuButton.addEventListener("click", (event) => {
    event.stopPropagation();
    toggleFileMenu();
  });
  els.openDocumentButton.addEventListener("click", openDocument);
  els.saveDocumentButton.addEventListener("click", saveDocument);
  els.saveDocumentAsButton.addEventListener("click", saveDocumentAs);
  els.zoomOutButton.addEventListener("click", () => changeZoom(-0.1));
  els.zoomInButton.addEventListener("click", () => changeZoom(0.1));
  els.zoomResetButton.addEventListener("click", () => {
    state.zoom = 1;
    state.zoomAutoFit = false;
    state.pan = { x: 0, y: 0 };
    render();
  });
  els.captureButton.addEventListener("click", () => captureScreen("fullscreen"));
  els.captureRegionButton.addEventListener("click", () => captureScreen("region"));
  els.captureAllDisplaysButton.addEventListener("click", () => captureScreen("all-displays"));
  els.exportButton.addEventListener("click", () => exportCurrent(false));
  els.copyButton.addEventListener("click", () => exportCurrent(true));
  els.toggleInspectorButton?.addEventListener("click", () => {
    state.inspectorCollapsed = !state.inspectorCollapsed;
    render();
  });

  els.canvasStage.addEventListener("pointerdown", onPointerDown);
  window.addEventListener("pointermove", onPointerMove);
  window.addEventListener("pointerup", onPointerUp);
  els.canvasHost?.addEventListener("wheel", onCanvasWheel, { passive: false });
  window.addEventListener("click", (event) => {
    if (!event.target.closest(".menu-shell")) {
      closeFileMenu();
    }
  });

  els.preferencesButton.addEventListener("click", () => {
    closeFileMenu();
    openPreferences();
  });
  els.preferencesClose.addEventListener("click", () => closePreferences());
  els.preferencesCancel.addEventListener("click", () => closePreferences());
  els.preferencesSave.addEventListener("click", () => savePreferences());
  els.preferencesFilter.addEventListener("input", () => {
    resetRecordingState();
    renderPreferences();
  });

  els.preferencesResetAll.addEventListener("click", async () => {
    if (!confirm("Reset export folder and hotkeys to defaults?")) return;
    try {
      const [nextPreferences, defaults] = await Promise.all([
        backend.resetPreferences(),
        backend.resetHotkeys(),
      ]);
      userPreferences.exportDirectory = normalizePreferencePath(nextPreferences?.exportDirectory);
      prefs.exportDirectory = userPreferences.exportDirectory;
      prefs.draft = defaults.map((b) => ({ ...b }));
      syncPreferencesDirty();
      applyHotkeyBindings(defaults);
      renderPreferences();
      showToast("已還原 export folder 與熱鍵設定");
    } catch (err) {
      setPreferencesError(`還原失敗：${err?.message || err}`);
    }
  });

  els.preferencesModal.addEventListener("click", (event) => {
    if (event.target === els.preferencesModal) closePreferences();
  });

  els.tabStrip?.addEventListener("click", (event) => {
    const closeId = event.target?.dataset?.closeTabId;
    if (closeId) { event.stopPropagation(); closeTab(closeId); return; }
    if (event.target?.dataset?.action === "new-tab") { newBlankTab(); return; }
    const btn = event.target.closest?.("[data-tab-id]");
    if (btn) activateTab(btn.dataset.tabId);
  });

  bindInspector();
  bindHintToggles();
  bindCanvasResize();
  updateDocumentUI();
}

function bindCanvasResize() {
  if (!els.canvasHost || typeof ResizeObserver !== "function") return;
  const observer = new ResizeObserver(() => {
    if (!state.capture || !state.zoomAutoFit) return;
    const next = fitToWidthZoom(state.capture.width);
    if (Math.abs(next - state.zoom) < 0.001) return;
    state.zoom = next;
    render();
  });
  observer.observe(els.canvasHost);
}

function bindHintToggles() {
  document.querySelectorAll(".hint-toggle").forEach((btn) => {
    btn.addEventListener("click", () => {
      const hint = btn.closest(".inspector-section")?.querySelector(".section-hint");
      if (!hint) return;
      const open = hint.classList.toggle("is-open");
      btn.setAttribute("aria-expanded", open ? "true" : "false");
    });
  });
}

function bindInspector() {
  els.textContent.addEventListener("input", () => {
    const ann = selectedAnnotation();
    if (!ann || ann.type !== "text") return;
    ann.text = els.textContent.value;
    syncDirtyState();
    render();
  });
  els.textVariant.addEventListener("change", () => updateSelected({ variant: els.textVariant.value }));
  els.textFontSize.addEventListener("input", () => updateSelected({ fontSize: numberValue(els.textFontSize.value, 24) }));
  els.textMaxWidth.addEventListener("input", () => updateSelected({ maxWidth: numberValue(els.textMaxWidth.value, 0) }));
  els.blurRadius.addEventListener("input", () => updateSelected({ blurRadius: numberValue(els.blurRadius.value, 12) }));
  els.cornerRadius.addEventListener("input", () => updateSelected({ cornerRadius: numberValue(els.cornerRadius.value, 18) }));
  els.feather.addEventListener("input", () => updateSelected({ feather: numberValue(els.feather.value, 12) }));

  els.numberedStarting.addEventListener("input", () => {
    state.numberedCircle.nextNumber = Math.max(0, Math.floor(numberValue(els.numberedStarting.value, 1)));
  });
  els.numberedThis.addEventListener("input", () => {
    const ann = selectedAnnotation();
    if (!ann || ann.type !== "numbered-circle") return;
    updateSelected({ number: Math.max(0, Math.floor(numberValue(els.numberedThis.value, 1))) });
  });
  els.numberedRadius.addEventListener("input", () => {
    const value = Math.max(6, Math.min(200, numberValue(els.numberedRadius.value, 20)));
    state.numberedCircle.radius = value;
    const ann = selectedAnnotation();
    if (ann && ann.type === "numbered-circle") updateSelected({ radius: value });
  });
  els.numberedStrokeWidth.addEventListener("input", () => {
    const value = Math.max(0, Math.min(20, numberValue(els.numberedStrokeWidth.value, 6)));
    state.numberedCircle.strokeWidth = value;
    const ann = selectedAnnotation();
    if (ann && ann.type === "numbered-circle") updateSelected({ strokeWidth: value });
  });
  els.numberedFillColor.addEventListener("input", () => {
    const value = els.numberedFillColor.value.toUpperCase();
    state.numberedCircle.strokeColor = value;
    const ann = selectedAnnotation();
    if (ann && ann.type === "numbered-circle") updateSelected({ strokeColor: value });
  });
  els.numberedOutlineColor.addEventListener("input", () => {
    const value = els.numberedOutlineColor.value.toUpperCase();
    state.numberedCircle.outlineColor = value;
    const ann = selectedAnnotation();
    if (ann && ann.type === "numbered-circle") updateSelected({ outlineColor: value });
  });
  els.numberedTextColor.addEventListener("input", () => {
    const value = els.numberedTextColor.value.toUpperCase();
    state.numberedCircle.textColor = value;
    const ann = selectedAnnotation();
    if (ann && ann.type === "numbered-circle") updateSelected({ textColor: value });
  });
}

const CAPTURE_MODES = {
  fullscreen: {
    title: "captured-screen.png",
    tabLabel: "Full screen",
    loadingToast: "正在擷取滑鼠所在螢幕...",
    doneToast: () => "已載入滑鼠所在螢幕",
    tool: "select",
    call: () => backend.captureScreen(),
  },
  region: {
    title: "captured-region.png",
    tabLabel: "Region",
    loadingToast: "請在桌面拖曳選取擷取範圍...",
    doneToast: (c) => `已載入所選區域 ${c.width} × ${c.height}`,
    tool: "select",
    call: () => backend.captureRegion(),
  },
  "all-displays": {
    title: "captured-all-displays.png",
    tabLabel: "All displays",
    loadingToast: "正在載入所有螢幕，接著可拖曳裁切...",
    doneToast: (c) => `已載入所有螢幕 ${c.width} × ${c.height}，拖曳框選要保留的區域`,
    tool: "crop",
    call: () => backend.captureAllDisplays(),
  },
};

function formatTabTimestamp() {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function renderTabStrip() {
  const host = els.tabStrip;
  if (!host) return;
  host.innerHTML = "";
  for (const t of tabs) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "tab-item" + (t.id === activeTabId ? " is-active" : "") + (t.document?.dirty ? " is-dirty" : "");
    btn.setAttribute("role", "tab");
    btn.setAttribute("aria-selected", t.id === activeTabId ? "true" : "false");
    btn.dataset.tabId = t.id;
    const label = document.createElement("span");
    label.className = "tab-label";
    label.textContent = t.title || "Untitled";
    btn.appendChild(label);
    const close = document.createElement("span");
    close.className = "tab-close";
    close.textContent = "×";
    close.dataset.closeTabId = t.id;
    close.setAttribute("aria-label", "Close tab");
    btn.appendChild(close);
    host.appendChild(btn);
  }
  const plus = document.createElement("button");
  plus.type = "button";
  plus.className = "tab-new";
  plus.textContent = "+";
  plus.dataset.action = "new-tab";
  plus.setAttribute("data-hotkey-action", "tab.new");
  plus.setAttribute("data-title-base", "New tab");
  plus.title = "New tab";
  host.appendChild(plus);
  if (typeof updateHotkeyAnnotatedTitles === "function") updateHotkeyAnnotatedTitles();
}

function activateTab(id) {
  if (!id || id === activeTabId) return;
  if (!tabById(id)) return;
  commitActiveTab();
  loadTab(id);
  syncToolButtons();
  renderTabStrip();
  render();
}

function activateNeighborTab(dir) {
  if (!tabs.length) return;
  const idx = tabs.findIndex((t) => t.id === activeTabId);
  if (idx < 0) return;
  const next = tabs[(idx + dir + tabs.length) % tabs.length];
  activateTab(next.id);
}

function newBlankTab() {
  commitActiveTab();
  const t = makeTab("Untitled");
  tabs.push(t);
  loadTab(t.id);
  syncToolButtons();
  renderTabStrip();
  render();
}

function closeTab(id) {
  const t = tabById(id);
  if (!t) return;
  const isActive = id === activeTabId;
  const dirty = isActive ? state.document.dirty : t.document?.dirty;
  if (dirty && !confirm("此分頁有未儲存的變更，仍要關閉？")) return;
  if (isActive) commitActiveTab();
  const idx = tabs.indexOf(t);
  tabs.splice(idx, 1);
  if (tabs.length === 0) {
    const blank = makeTab("Untitled");
    tabs.push(blank);
    loadTab(blank.id);
  } else if (isActive) {
    const next = tabs[Math.min(idx, tabs.length - 1)];
    loadTab(next.id);
  }
  syncToolButtons();
  renderTabStrip();
  render();
}

init().catch((error) => {
  console.error(error);
  showToast(String(error));
});

async function captureScreen(mode = "fullscreen") {
  const plan = CAPTURE_MODES[mode] || CAPTURE_MODES.fullscreen;
  closeFileMenu();
  showToast(plan.loadingToast);
  const capture = await plan.call();
  // First capture fills the current blank tab; subsequent captures open new tabs.
  if (state.capture != null) {
    commitActiveTab();
    const t = makeTab(plan.tabLabel || "Capture");
    tabs.push(t);
    loadTab(t.id);
  }
  state.capture = {
    base64: capture.base64,
    width: capture.captureRegion?.width ?? capture.display?.width ?? 1200,
    height: capture.captureRegion?.height ?? capture.display?.height ?? 720,
    format: capture.format,
    mimeType: capture.mimeType,
    display: capture.display || null,
    captureRegion: capture.captureRegion || null,
  };
  state.annotations = [];
  state.selectedId = null;
  state.history = [];
  state.future = [];
  state.numberedCircle.nextNumber = 1;
  state.zoom = fitToWidthZoom(state.capture.width);
  state.zoomAutoFit = true;
  state.pan = { x: 0, y: 0 };
  state.tool = plan.tool;
  syncDirtyState();
  syncToolButtons();
  const tab = activeTab();
  if (tab) tab.title = `${plan.tabLabel || "Capture"} ${formatTabTimestamp()}`;
  renderTabStrip();
  els.captureTitle.textContent = `${plan.title} · ${state.capture.width} × ${state.capture.height}`;
  showToast(plan.doneToast(state.capture));
  render();
}

function setTool(tool) {
  state.tool = tool;
  syncToolButtons();
  render();
}

function syncToolButtons() {
  els.toolButtons.forEach((button) => button.classList.toggle("is-active", button.dataset.tool === state.tool));
}

function onPointerDown(event) {
  if (!state.capture) return;

  const point = pointerPoint(event);
  state.pointer = point;

  const handle = event.target.dataset.handle;
  if (handle) {
    const ann = selectedAnnotation();
    if (!ann) return;
    pushHistory();
    state.action = { kind: "resize", handle, annotationId: ann.id, origin: point, snapshot: cloneAnnotation(ann) };
    return;
  }

  const targetNode = event.target.closest?.("[data-annotation-id]");
  const targetId = targetNode?.dataset.annotationId;
  if (targetId && targetId !== "draft") {
    state.selectedId = targetId;
    if (state.tool === "select") {
      pushHistory();
      state.action = { kind: "move", annotationId: targetId, origin: point, snapshot: cloneAnnotation(findAnnotation(targetId)) };
    }
    render();
    return;
  }

  if (state.tool === "text") {
    pushHistory();
    const id = nextId("ann-text");
    state.annotations.push({
      id,
      type: "text",
      x: point.x,
      y: point.y,
      text: "輸入文字",
      variant: "solid",
      fontSize: 24,
      width: 220,
      height: 56,
      maxWidth: 220,
    });
    state.selectedId = id;
    render();
    els.textContent.focus();
    els.textContent.select();
    return;
  }

  if (state.tool === "numbered-circle") {
    pushHistory();
    const opts = state.numberedCircle;
    const id = nextId("ann-numbered");
    state.annotations.push({
      id,
      type: "numbered-circle",
      x: point.x,
      y: point.y,
      radius: opts.radius,
      number: opts.nextNumber,
      strokeColor: opts.strokeColor,
      outlineColor: opts.outlineColor,
      textColor: opts.textColor,
      strokeWidth: opts.strokeWidth,
    });
    state.numberedCircle.nextNumber += 1;
    state.selectedId = id;
    syncDirtyState();
    render();
    return;
  }

  if (state.tool === "crop") {
    pushHistory();
    state.action = { kind: "crop", origin: point, current: point };
    render();
    return;
  }

  if (["arrow", "rectangle", "ellipse", "blur"].includes(state.tool)) {
    pushHistory();
    state.action = { kind: "draw", tool: state.tool, origin: point, current: point };
    render();
    return;
  }

  if (state.tool === "select") {
    state.selectedId = null;
    state.action = { kind: "pan", originClientX: event.clientX, originClientY: event.clientY, pan: { ...state.pan } };
    updateCanvasCursor();
    render();
    return;
  }

  state.selectedId = null;
  render();
}

function updateCanvasCursor() {
  if (!els.canvasHost) return;
  els.canvasHost.dataset.tool = state.tool;
  if (state.action && state.action.kind === "pan") {
    els.canvasHost.style.cursor = "grabbing";
    return;
  }
  els.canvasHost.style.cursor = "";
}

function onPointerMove(event) {
  if (!state.capture) return;

  const point = pointerPoint(event);
  state.pointer = point;
  if (!state.action) {
    updateStatus();
    return;
  }

  switch (state.action.kind) {
    case "draw":
    case "crop":
      state.action.current = point;
      break;
    case "move":
      moveAnnotation(state.action.annotationId, point.x - state.action.origin.x, point.y - state.action.origin.y, state.action.snapshot);
      break;
    case "resize":
      resizeAnnotation(state.action.annotationId, state.action.handle, state.action.snapshot, point);
      break;
    case "pan":
      state.pan.x = state.action.pan.x + (event.clientX - state.action.originClientX);
      state.pan.y = state.action.pan.y + (event.clientY - state.action.originClientY);
      break;
  }

  render();
}

function onPointerUp() {
  if (!state.action) return;

  const actionKind = state.action.kind;

  if (state.action.kind === "draw") {
    commitDraft(state.action.tool, state.action.origin, state.action.current);
  } else if (state.action.kind === "crop") {
    applyCrop(state.action.origin, state.action.current);
  }

  state.action = null;
  if (["draw", "crop", "move", "resize"].includes(actionKind)) {
    syncDirtyState();
  }
  render();
}

function commitDraft(tool, origin, current) {
  const rect = normalizedRect(origin, current);
  if (tool === "arrow") {
    if (origin.x === current.x && origin.y === current.y) return;
    const id = nextId("ann-arrow");
    state.annotations.push({ id, type: "arrow", x1: origin.x, y1: origin.y, x2: current.x, y2: current.y });
    state.selectedId = id;
    return;
  }
  if (rect.width < 4 || rect.height < 4) return;

  const id = nextId(`ann-${tool}`);
  const annotation = { id, type: tool, x: rect.x, y: rect.y, width: rect.width, height: rect.height };
  if (tool === "blur") {
    Object.assign(annotation, { blurRadius: 12, cornerRadius: 18, feather: 12 });
  }
  state.annotations.push(annotation);
  state.selectedId = id;
}

function applyCrop(origin, current) {
  const rect = normalizedRect(origin, current);
  if (rect.width < 20 || rect.height < 20) return;

  const img = els.captureImage;
  const canvas = document.createElement("canvas");
  canvas.width = rect.width;
  canvas.height = rect.height;
  const ctx = canvas.getContext("2d");
  ctx.drawImage(img, rect.x, rect.y, rect.width, rect.height, 0, 0, rect.width, rect.height);
  const dataURL = canvas.toDataURL("image/png");

  state.capture = {
    ...state.capture,
    base64: dataURL.split(",")[1],
    width: rect.width,
    height: rect.height,
    captureRegion: {
      x: (state.capture.captureRegion?.x || 0) + rect.x,
      y: (state.capture.captureRegion?.y || 0) + rect.y,
      width: rect.width,
      height: rect.height,
    },
  };
  state.annotations = [];
  state.selectedId = null;
  state.tool = "select";
  syncDirtyState();
  showToast(`已裁切到 ${rect.width} × ${rect.height}`);
}

function moveAnnotation(annotationId, dx, dy, snapshot) {
  const ann = findAnnotation(annotationId);
  if (!ann) return;
  Object.assign(ann, cloneAnnotation(snapshot));
  if (ann.type === "arrow") {
    ann.x1 += dx;
    ann.y1 += dy;
    ann.x2 += dx;
    ann.y2 += dy;
    return;
  }
  ann.x += dx;
  ann.y += dy;
}

function resizeAnnotation(annotationId, handle, snapshot, point) {
  const ann = findAnnotation(annotationId);
  if (!ann) return;
  Object.assign(ann, cloneAnnotation(snapshot));

  if (ann.type === "arrow") {
    if (handle === "start") {
      ann.x1 = point.x;
      ann.y1 = point.y;
    } else if (handle === "end") {
      ann.x2 = point.x;
      ann.y2 = point.y;
    }
    return;
  }

  const bounds = annotationBounds(snapshot);
  const next = { ...bounds };
  if (handle.includes("n")) {
    next.y = point.y;
    next.height = bounds.y + bounds.height - point.y;
  }
  if (handle.includes("s")) {
    next.height = point.y - bounds.y;
  }
  if (handle.includes("w")) {
    next.x = point.x;
    next.width = bounds.x + bounds.width - point.x;
  }
  if (handle.includes("e")) {
    next.width = point.x - bounds.x;
  }

  if (next.width < 8 || next.height < 8) return;
  if (ann.type === "numbered-circle") {
    const r = Math.max(6, Math.min(200, Math.min(next.width, next.height) / 2));
    ann.x = next.x + next.width / 2;
    ann.y = next.y + next.height / 2;
    ann.radius = r;
    return;
  }
  ann.x = next.x;
  ann.y = next.y;
  ann.width = next.width;
  ann.height = next.height;
  // Text renders via style.width (which falls back to maxWidth for legacy
  // docs) and also exports maxWidth through toPayload — keep the two in
  // sync so a width drag survives save/load and the SVG export matches.
  if (ann.type === "text") ann.maxWidth = next.width;
}

function updateSelected(patch) {
  const ann = selectedAnnotation();
  if (!ann) return;
  pushHistory();
  Object.assign(ann, patch);
  syncDirtyState();
  render();
}

function undo() {
  if (!state.history.length) return;
  state.future.push(snapshot());
  restoreSnapshot(state.history.pop());
  syncDirtyState();
  render();
}

function redo() {
  if (!state.future.length) return;
  state.history.push(snapshot());
  restoreSnapshot(state.future.pop());
  syncDirtyState();
  render();
}

function pushHistory() {
  state.history.push(snapshot());
  if (state.history.length > 100) state.history.shift();
  state.future = [];
}

function snapshot() {
  return {
    capture: state.capture ? { ...state.capture } : null,
    annotations: state.annotations.map(cloneAnnotation),
    selectedId: state.selectedId,
    tool: state.tool,
    zoom: state.zoom,
    pan: { ...state.pan },
    numberedNextNumber: state.numberedCircle.nextNumber,
  };
}

function restoreSnapshot(data) {
  state.capture = data.capture ? { ...data.capture } : null;
  state.annotations = data.annotations.map(cloneAnnotation);
  state.selectedId = data.selectedId;
  state.tool = data.tool;
  state.zoom = data.zoom;
  state.pan = { ...data.pan };
  if (typeof data.numberedNextNumber === "number") {
    state.numberedCircle.nextNumber = data.numberedNextNumber;
  }
}

function toggleFileMenu() {
  state.document.menuOpen = !state.document.menuOpen;
  updateDocumentUI();
}

function closeFileMenu() {
  if (!state.document.menuOpen) return;
  state.document.menuOpen = false;
  updateDocumentUI();
}

function updateDocumentUI() {
  const dirtyLabel = state.document.dirty ? "unsaved" : "saved";
  els.documentBadge.textContent = `${state.document.name} · ${dirtyLabel}`;
  els.documentBadge.classList.toggle("is-dirty", state.document.dirty);
  els.fileMenu.classList.toggle("is-hidden", !state.document.menuOpen);
  els.fileMenuButton.setAttribute("aria-expanded", state.document.menuOpen ? "true" : "false");
  els.saveDocumentButton.disabled = !state.capture;
  els.saveDocumentAsButton.disabled = !state.capture;
  els.captureTitle.textContent = state.capture
    ? `${state.document.name}${state.document.dirty ? " *" : ""} · ${state.capture.width} × ${state.capture.height}`
    : "準備擷取畫面";
}

function serializeDocument() {
  if (!state.capture) return null;
  return {
    kind: "snapvector-document",
    version: 1,
    capture: {
      base64: state.capture.base64,
      width: state.capture.width,
      height: state.capture.height,
      format: state.capture.format,
      mimeType: state.capture.mimeType,
      display: state.capture.display || null,
      captureRegion: state.capture.captureRegion || null,
    },
    annotations: state.annotations.map(toPayload),
  };
}

function documentFingerprint() {
  const doc = serializeDocument();
  return doc ? JSON.stringify(doc) : "";
}

function syncDirtyState() {
  const fingerprint = documentFingerprint();
  const prev = state.document.dirty;
  state.document.dirty = fingerprint !== "" && fingerprint !== state.document.savedFingerprint;
  if (prev !== state.document.dirty && els.tabStrip) {
    const btn = els.tabStrip.querySelector(`[data-tab-id="${activeTabId}"]`);
    if (btn) btn.classList.toggle("is-dirty", state.document.dirty);
  }
}

function defaultDocumentName() {
  if (state.document.name && state.document.name !== "Untitled") {
    return state.document.name;
  }
  return "capture.sv.json";
}

function defaultExportName(format) {
  const base = defaultDocumentName().replace(/\.sv\.json$/i, "");
  return `${base || "snapvector-export"}.${format}`;
}

async function openDocument() {
  closeFileMenu();
  try {
    const result = await backend.openDocument();
    if (!result) return;
    const parsed = JSON.parse(result.contents);
    if (parsed.kind !== "snapvector-document" || parsed.version !== 1 || !parsed.capture || !Array.isArray(parsed.annotations)) {
      throw new Error("不支援的文件格式");
    }

    commitActiveTab();
    const t = makeTab(result.name || "Untitled");
    tabs.push(t);
    loadTab(t.id);

    state.capture = {
      base64: parsed.capture.base64,
      width: parsed.capture.width,
      height: parsed.capture.height,
      format: parsed.capture.format || "png",
      mimeType: parsed.capture.mimeType || "image/png",
      display: parsed.capture.display || null,
      captureRegion: parsed.capture.captureRegion || null,
    };
    state.annotations = parsed.annotations.map(cloneAnnotation);
    state.selectedId = null;
    state.history = [];
    state.future = [];
    state.zoom = DEFAULT_ZOOM;
    state.pan = { x: 0, y: 0 };
    state.tool = "select";
    state.document.path = result.path;
    state.document.name = result.name || "Untitled";
    state.document.savedFingerprint = documentFingerprint();
    state.document.dirty = false;
    const tab = activeTab();
    if (tab) tab.title = state.document.name;
    renderTabStrip();
    render();
    showToast(`已開啟 ${state.document.name}`);
  } catch (error) {
    console.error(error);
    showToast(String(error));
  }
}

async function saveDocument() {
  closeFileMenu();
  if (!state.capture) return;
  try {
    const contents = JSON.stringify(serializeDocument(), null, 2);
    let result;
    if (state.document.path) {
      result = await backend.saveDocument(state.document.path, contents);
    } else {
      result = await backend.saveDocumentAs(defaultDocumentName(), contents);
    }
    if (!result) return;
    state.document.path = result.path;
    state.document.name = result.name || "Untitled";
    state.document.savedFingerprint = documentFingerprint();
    state.document.dirty = false;
    const tab = activeTab();
    if (tab) tab.title = state.document.name;
    renderTabStrip();
    render();
    showToast(`已儲存 ${state.document.name}`);
  } catch (error) {
    console.error(error);
    showToast(String(error));
  }
}

async function saveDocumentAs() {
  closeFileMenu();
  if (!state.capture) return;
  try {
    const result = await backend.saveDocumentAs(defaultDocumentName(), JSON.stringify(serializeDocument(), null, 2));
    if (!result) return;
    state.document.path = result.path;
    state.document.name = result.name || "Untitled";
    state.document.savedFingerprint = documentFingerprint();
    state.document.dirty = false;
    const tab = activeTab();
    if (tab) tab.title = state.document.name;
    renderTabStrip();
    render();
    showToast(`已另存為 ${state.document.name}`);
  } catch (error) {
    console.error(error);
    showToast(String(error));
  }
}

function render() {
  els.undoButton.disabled = state.history.length === 0;
  els.redoButton.disabled = state.future.length === 0;
  updateDocumentUI();

  document.querySelector(".app-body")?.classList.toggle("inspector-collapsed", state.inspectorCollapsed);

  if (!state.capture) {
    els.emptyState.classList.remove("is-hidden");
    els.canvasStage.classList.add("is-hidden");
    return;
  }

  els.emptyState.classList.add("is-hidden");
  els.canvasStage.classList.remove("is-hidden");
  els.captureImage.src = `data:image/png;base64,${state.capture.base64}`;
  els.captureImage.width = state.capture.width;
  els.captureImage.height = state.capture.height;
  els.canvasStage.style.width = `${state.capture.width}px`;
  els.canvasStage.style.height = `${state.capture.height}px`;
  els.canvasStage.style.transform = `translate(${state.pan.x}px, ${state.pan.y}px) scale(${state.zoom})`;

  renderVectorAnnotations();
  renderHTMLAnnotations();
  renderSelection();
  renderInspector();
  updateStatus();
  updateCanvasCursor();
}

function renderVectorAnnotations() {
  const svg = els.annotationLayer;
  svg.setAttribute("viewBox", `0 0 ${state.capture.width} ${state.capture.height}`);
  svg.setAttribute("width", state.capture.width);
  svg.setAttribute("height", state.capture.height);
  svg.innerHTML = "";

  state.annotations.forEach((ann) => {
    if (ann.type === "text" || ann.type === "blur") return;
    const group = document.createElementNS(SVG_NS, "g");
    group.dataset.annotationId = ann.id;
    group.classList.add("annotation-hit");
    if (ann.type === "numbered-circle") {
      const r = ann.radius ?? 28;
      const sw = ann.strokeWidth ?? 6;
      const circle = document.createElementNS(SVG_NS, "circle");
      circle.setAttribute("cx", ann.x);
      circle.setAttribute("cy", ann.y);
      circle.setAttribute("r", r);
      circle.setAttribute("fill", ann.strokeColor || "#E53935");
      circle.setAttribute("stroke", ann.outlineColor || "#FFFFFF");
      circle.setAttribute("stroke-width", sw);
      circle.setAttribute("paint-order", "stroke fill");
      group.appendChild(circle);
      const text = document.createElementNS(SVG_NS, "text");
      const fontSize = r * 1.25;
      text.setAttribute("x", ann.x);
      text.setAttribute("y", ann.y + fontSize * 0.35);
      text.setAttribute("text-anchor", "middle");
      text.setAttribute("font-size", fontSize);
      text.setAttribute("font-weight", "800");
      text.style.fontFeatureSettings = "'pnum'";
      text.setAttribute("fill", ann.textColor || "#FFFFFF");
      text.textContent = String(ann.number ?? 0);
      group.appendChild(text);
      svg.appendChild(group);
      return;
    }
    if (ann.type === "rectangle") {
      group.appendChild(svgHitRect(ann.x, ann.y, ann.width, ann.height));
      group.appendChild(svgRect(ann.x, ann.y, ann.width, ann.height, "#FFFFFF", ann.strokeWidth ?? 16));
      group.appendChild(svgRect(ann.x, ann.y, ann.width, ann.height, "#E53935", ann.strokeWidth ?? 10));
    } else if (ann.type === "ellipse") {
      group.appendChild(svgHitEllipse(ann));
      group.appendChild(svgEllipse(ann, "#FFFFFF", 16));
      group.appendChild(svgEllipse(ann, "#E53935", 10));
    } else if (ann.type === "arrow") {
      group.appendChild(svgHitArrow(ann));
      group.appendChild(svgArrow(ann, "#FFFFFF", "#FFFFFF"));
      group.appendChild(svgArrow(ann, "#E53935"));
    }
    svg.appendChild(group);
  });

  if (state.action?.kind === "draw") {
    const draft = draftVectorAnnotation(state.action.tool, state.action.origin, state.action.current);
    if (draft) {
      const group = document.createElementNS(SVG_NS, "g");
      group.dataset.annotationId = "draft";
      group.style.opacity = "0.88";
      if (draft.type === "rectangle") {
        group.appendChild(svgRect(draft.x, draft.y, draft.width, draft.height, "#FFFFFF", draft.strokeWidth ?? 16));
        group.appendChild(svgRect(draft.x, draft.y, draft.width, draft.height, "#E53935", draft.strokeWidth ?? 10));
      } else if (draft.type === "ellipse") {
        group.appendChild(svgEllipse(draft, "#FFFFFF", 16));
        group.appendChild(svgEllipse(draft, "#E53935", 10));
      } else if (draft.type === "arrow") {
        group.appendChild(svgArrow(draft, "#FFFFFF", "#FFFFFF"));
        group.appendChild(svgArrow(draft, "#E53935"));
      }
      svg.appendChild(group);
    }
  }
}

function renderHTMLAnnotations() {
  els.htmlAnnotationLayer.innerHTML = "";
  state.annotations.forEach((ann) => {
    if (ann.type === "text") {
      const div = document.createElement("div");
      div.className = `text-card ${ann.variant === "outline" ? "outline" : "solid"}`;
      div.dataset.annotationId = ann.id;
      div.style.left = `${ann.x}px`;
      div.style.top = `${ann.y}px`;
      div.style.fontSize = `${ann.fontSize || 24}px`;
      const widthPx = ann.width || ann.maxWidth || 220;
      div.style.width = `${widthPx}px`;
      if (ann.height) div.style.minHeight = `${ann.height}px`;
      div.textContent = ann.text;
      els.htmlAnnotationLayer.appendChild(div);
      return;
    }
    if (ann.type === "blur") {
      const div = document.createElement("div");
      div.className = "blur-region";
      div.dataset.annotationId = ann.id;
      div.style.left = `${ann.x}px`;
      div.style.top = `${ann.y}px`;
      div.style.width = `${ann.width}px`;
      div.style.height = `${ann.height}px`;
      div.style.borderRadius = `${ann.cornerRadius || 18}px`;
      div.style.backdropFilter = `blur(${ann.blurRadius || 12}px) saturate(0.88)`;
      const pill = document.createElement("span");
      pill.className = "blur-pill";
      pill.textContent = `blur · ${ann.blurRadius || 12}`;
      div.appendChild(pill);
      els.htmlAnnotationLayer.appendChild(div);
    }
  });
}

function renderSelection() {
  const ann = selectedAnnotation();
  const selection = els.selectionBox;
  const draft = els.draftBox;
  selection.classList.add("is-hidden");
  draft.classList.add("is-hidden");
  selection.dataset.type = "";
  delete selection.dataset.annotationId;

  if (state.action && shouldShowDraftBox(state.action)) {
    const rect = normalizedRect(state.action.origin, state.action.current);
    if (rect.width > 1 && rect.height > 1) {
      draft.classList.remove("is-hidden");
      draft.style.left = `${rect.x}px`;
      draft.style.top = `${rect.y}px`;
      draft.style.width = `${rect.width}px`;
      draft.style.height = `${rect.height}px`;
    }
  }

  if (!ann) return;

  selection.classList.remove("is-hidden");
  selection.dataset.annotationId = ann.id;
  if (ann.type === "arrow") {
    const bounds = annotationBounds(ann);
    selection.dataset.type = "arrow";
    selection.style.left = `${bounds.x}px`;
    selection.style.top = `${bounds.y}px`;
    selection.style.width = `${bounds.width}px`;
    selection.style.height = `${bounds.height}px`;
    positionEndpointHandle("start", ann.x1, ann.y1);
    positionEndpointHandle("end", ann.x2, ann.y2);
    toggleArrowHandles(true);
    return;
  }

  toggleArrowHandles(false);
  const bounds = annotationBounds(ann);
  selection.style.left = `${bounds.x}px`;
  selection.style.top = `${bounds.y}px`;
  selection.style.width = `${bounds.width}px`;
  selection.style.height = `${bounds.height}px`;
}

function positionEndpointHandle(handle, x, y) {
  const el = els.selectionBox.querySelector(`[data-handle="${handle}"]`);
  el.style.left = `${x - annotationBounds(selectedAnnotation()).x}px`;
  el.style.top = `${y - annotationBounds(selectedAnnotation()).y}px`;
}

function toggleArrowHandles(enabled) {
  els.selectionBox.querySelectorAll(".endpoint-handle").forEach((node) => node.classList.toggle("is-hidden", !enabled));
  els.selectionBox.querySelectorAll('.selection-handle:not(.endpoint-handle)').forEach((node) => node.classList.toggle("is-hidden", enabled));
}

function renderInspector() {
  const ann = selectedAnnotation();
  els.selectedMeta.innerHTML = "";

  const emptyHint = document.getElementById("inspectorEmpty");
  const sectionSelected = document.getElementById("sectionSelected");
  const sectionGeometry = document.getElementById("sectionGeometry");
  const sectionText = document.getElementById("sectionText");
  const sectionBlur = document.getElementById("sectionBlur");
  const appBody = document.querySelector(".app-body");

  if (!ann) {
    els.geometryFields.innerHTML = "";
    els.geometryFields.dataset.sig = "";
    disableInspector(true);
    emptyHint?.classList.remove("is-hidden");
    sectionSelected?.classList.add("is-hidden");
    sectionGeometry?.classList.add("is-hidden");
    sectionText?.classList.add("is-hidden");
    sectionBlur?.classList.add("is-hidden");
    els.sectionNumberedCircle?.classList.toggle("is-hidden", state.tool !== "numbered-circle");
    if (state.tool === "numbered-circle") {
      emptyHint?.classList.add("is-hidden");
      syncNumberedInspector(null);
    }
    return;
  }

  disableInspector(false);
  emptyHint?.classList.add("is-hidden");
  sectionSelected?.classList.remove("is-hidden");
  sectionGeometry?.classList.toggle("is-hidden", ann.type === "numbered-circle");
  sectionText?.classList.toggle("is-hidden", ann.type !== "text");
  sectionBlur?.classList.toggle("is-hidden", ann.type !== "blur");
  els.sectionNumberedCircle?.classList.toggle("is-hidden", ann.type !== "numbered-circle" && state.tool !== "numbered-circle");
  syncNumberedInspector(ann.type === "numbered-circle" ? ann : null);
  [ann.type, ann.id].forEach((value, index) => {
    const chip = document.createElement("span");
    chip.className = "chip";
    chip.textContent = `${index === 0 ? "type" : "id"} · ${value}`;
    els.selectedMeta.appendChild(chip);
  });

  const geometry = ann.type === "arrow"
    ? [["x1", ann.x1], ["y1", ann.y1], ["x2", ann.x2], ["y2", ann.y2]]
    : [["x", ann.x], ["y", ann.y], ["width", ann.width], ["height", ann.height]];
  syncGeometryFields(geometry);

  syncInputValue(els.textContent, ann.type === "text" ? ann.text : "");
  syncInputValue(els.textVariant, ann.type === "text" ? ann.variant || "solid" : "solid");
  syncInputValue(els.textFontSize, ann.type === "text" ? ann.fontSize || 24 : 24);
  syncInputValue(els.textMaxWidth, ann.type === "text" ? ann.maxWidth || 0 : 0);
  syncInputValue(els.blurRadius, ann.type === "blur" ? ann.blurRadius || 12 : 12);
  syncInputValue(els.cornerRadius, ann.type === "blur" ? ann.cornerRadius || 18 : 18);
  syncInputValue(els.feather, ann.type === "blur" ? ann.feather || 12 : 12);
}

function syncNumberedInspector(ann) {
  const opts = state.numberedCircle;
  syncInputValue(els.numberedStarting, opts.nextNumber);
  syncInputValue(els.numberedThis, ann ? (ann.number ?? 0) : "");
  syncInputValue(els.numberedRadius, ann ? (ann.radius ?? opts.radius) : opts.radius);
  syncInputValue(els.numberedStrokeWidth, ann ? (ann.strokeWidth ?? opts.strokeWidth) : opts.strokeWidth);
  if (els.numberedFillColor && document.activeElement !== els.numberedFillColor) {
    els.numberedFillColor.value = (ann?.strokeColor || opts.strokeColor).toLowerCase();
  }
  if (els.numberedOutlineColor && document.activeElement !== els.numberedOutlineColor) {
    els.numberedOutlineColor.value = (ann?.outlineColor || opts.outlineColor).toLowerCase();
  }
  if (els.numberedTextColor && document.activeElement !== els.numberedTextColor) {
    els.numberedTextColor.value = (ann?.textColor || opts.textColor).toLowerCase();
  }
  if (els.numberedThis) {
    els.numberedThis.disabled = !ann;
  }
}

// Skip writing to an input the user is currently editing — otherwise every
// keystroke handler's render() would clobber focus or cursor position and
// force the user to click back in between digits.
function syncInputValue(input, value) {
  if (!input) return;
  if (document.activeElement === input) return;
  if (input.value !== String(value)) input.value = value;
}

function syncGeometryFields(entries) {
  const sig = entries.map(([key]) => key).join(",");
  if (els.geometryFields.dataset.sig !== sig) {
    els.geometryFields.innerHTML = "";
    els.geometryFields.dataset.sig = sig;
    entries.forEach(([key, value]) => addField(key, value));
    return;
  }
  // Keys unchanged (same annotation type selected) — update existing input
  // values in place so focus and caret survive a re-render mid-typing.
  const inputs = els.geometryFields.querySelectorAll("input");
  entries.forEach(([, value], idx) => syncInputValue(inputs[idx], Math.round(value)));
}

function addField(key, value) {
  const wrapper = document.createElement("div");
  wrapper.className = "field";
  const label = document.createElement("label");
  label.textContent = key;
  const input = document.createElement("input");
  input.type = "number";
  input.value = Math.round(value);
  input.addEventListener("input", () => {
    const ann = selectedAnnotation();
    if (!ann) return;
    const next = numberValue(input.value, 0);
    pushHistory();
    ann[key] = next;
    syncDirtyState();
    render();
  });
  wrapper.appendChild(label);
  wrapper.appendChild(input);
  els.geometryFields.appendChild(wrapper);
}

function disableInspector(disabled) {
  [els.textContent, els.textVariant, els.textFontSize, els.textMaxWidth, els.blurRadius, els.cornerRadius, els.feather].forEach((node) => {
    node.disabled = disabled;
  });
}

function updateStatus() {
  const zoomLabel = `${Math.round(state.zoom * 100)}%`;
  els.statusX.textContent = Math.round(state.pointer.x);
  els.statusY.textContent = Math.round(state.pointer.y);
  els.statusZoom.textContent = zoomLabel;
  els.zoomResetButton.textContent = zoomLabel;
  els.statusCount.textContent = String(state.annotations.length);
  els.statusSelected.textContent = state.selectedId || "none";
}

async function exportCurrent(copyToClipboard) {
  if (!state.capture) return;
  try {
    // Copy always produces PNG — that's what downstream apps (Slack, docs,
    // issue trackers) consistently accept from the clipboard.
    const format = copyToClipboard ? "png" : els.exportFormat.value;
    const payload = JSON.stringify(state.annotations.map(toPayload));
    if (copyToClipboard) {
      await backend.exportDocument(payload, state.capture.base64, state.capture.width, state.capture.height, format, true);
      showToast(`已複製 ${format.toUpperCase()} 到剪貼簿`);
      return;
    }
    const result = await backend.exportDocumentToFile(
      payload,
      state.capture.base64,
      state.capture.width,
      state.capture.height,
      format,
      defaultExportName(format),
    );
    if (!result) {
      return;
    }
    showToast(`已匯出到 ${result.path}`);
  } catch (error) {
    console.error(error);
    showToast(String(error?.message || error));
  }
}

function showToast(message) {
  els.toast.textContent = message;
  els.toast.classList.remove("is-hidden");
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(() => els.toast.classList.add("is-hidden"), 1800);
}

function pointerPoint(event) {
  const rect = els.canvasStage.getBoundingClientRect();
  return {
    x: clamp((event.clientX - rect.left) / state.zoom, 0, state.capture.width),
    y: clamp((event.clientY - rect.top) / state.zoom, 0, state.capture.height),
  };
}

function annotationBounds(ann) {
  if (ann.type === "arrow") {
    const x = Math.min(ann.x1, ann.x2);
    const y = Math.min(ann.y1, ann.y2);
    return { x, y, width: Math.abs(ann.x2 - ann.x1), height: Math.abs(ann.y2 - ann.y1) };
  }
  if (ann.type === "numbered-circle") {
    const r = ann.radius ?? 28;
    return { x: ann.x - r, y: ann.y - r, width: r * 2, height: r * 2 };
  }
  return { x: ann.x, y: ann.y, width: ann.width || 180, height: ann.height || 64 };
}

function selectedAnnotation() {
  return state.annotations.find((ann) => ann.id === state.selectedId) || null;
}

function findAnnotation(id) {
  return state.annotations.find((ann) => ann.id === id);
}

function normalizedRect(origin, current) {
  return {
    x: Math.min(origin.x, current.x),
    y: Math.min(origin.y, current.y),
    width: Math.abs(current.x - origin.x),
    height: Math.abs(current.y - origin.y),
  };
}

function shouldShowDraftBox(action) {
  if (action.kind === "crop") return true;
  return action.kind === "draw" && ["rectangle", "blur"].includes(action.tool);
}

function fitToWidthZoom(captureWidth) {
  const host = els.canvasHost;
  if (!host || !captureWidth) return DEFAULT_ZOOM;
  // Account for the 24px padding on each side of .canvas.
  const available = Math.max(host.clientWidth - 48, 0);
  if (available <= 0) return DEFAULT_ZOOM;
  const fit = available / captureWidth;
  // Never blow the image up past DEFAULT_ZOOM on a tiny capture; keep a
  // sensible floor so a huge capture still has legible detail.
  return clamp(fit, 0.15, DEFAULT_ZOOM);
}

function changeZoom(delta) {
  state.zoom = clamp(state.zoom + delta, 0.1, 3);
  state.zoomAutoFit = false;
  render();
}

function onCanvasWheel(event) {
  if (!state.capture) return;
  event.preventDefault();
  const factor = event.deltaY < 0 ? 1.1 : 1 / 1.1;
  const nextZoom = clamp(state.zoom * factor, 0.1, 3);
  if (Math.abs(nextZoom - state.zoom) < 0.0001) return;

  const host = els.canvasHost;
  const rect = host.getBoundingClientRect();
  const cursorX = event.clientX - rect.left + host.scrollLeft;
  const cursorY = event.clientY - rect.top + host.scrollTop;
  const ratio = nextZoom / state.zoom;

  state.zoom = nextZoom;
  state.zoomAutoFit = false;
  render();

  host.scrollLeft = cursorX * ratio - (event.clientX - rect.left);
  host.scrollTop = cursorY * ratio - (event.clientY - rect.top);
}

function toPayload(ann) {
  if (ann.type === "arrow") {
    return { id: ann.id, type: ann.type, x1: round(ann.x1), y1: round(ann.y1), x2: round(ann.x2), y2: round(ann.y2) };
  }
  if (ann.type === "text") {
    return {
      id: ann.id,
      type: ann.type,
      x: round(ann.x),
      y: round(ann.y),
      text: ann.text,
      variant: ann.variant || "solid",
      fontSize: round(ann.fontSize || 24),
      maxWidth: round(ann.maxWidth || 220),
    };
  }
  if (ann.type === "blur") {
    return {
      id: ann.id,
      type: ann.type,
      x: round(ann.x),
      y: round(ann.y),
      width: round(ann.width),
      height: round(ann.height),
      blurRadius: round(ann.blurRadius || 12),
      cornerRadius: round(ann.cornerRadius || 18),
      feather: round(ann.feather || 12),
    };
  }
  if (ann.type === "numbered-circle") {
    return {
      id: ann.id,
      type: ann.type,
      x: round(ann.x),
      y: round(ann.y),
      radius: round(ann.radius ?? 28),
      number: Math.max(0, Math.floor(ann.number ?? 0)),
      strokeColor: ann.strokeColor || "#E53935",
      outlineColor: ann.outlineColor || "#FFFFFF",
      textColor: ann.textColor || "#FFFFFF",
      strokeWidth: round(ann.strokeWidth ?? 6),
    };
  }
  return { id: ann.id, type: ann.type, x: round(ann.x), y: round(ann.y), width: round(ann.width), height: round(ann.height) };
}

function svgRect(x, y, width, height, stroke, strokeWidth) {
  const rect = document.createElementNS(SVG_NS, "rect");
  rect.setAttribute("x", x);
  rect.setAttribute("y", y);
  rect.setAttribute("width", width);
  rect.setAttribute("height", height);
  rect.setAttribute("rx", 18);
  rect.setAttribute("fill", "none");
  rect.setAttribute("stroke", stroke);
  rect.setAttribute("stroke-width", strokeWidth);
  rect.setAttribute("stroke-linecap", "round");
  rect.setAttribute("stroke-linejoin", "round");
  return rect;
}

function svgHitRect(x, y, width, height) {
  const rect = document.createElementNS(SVG_NS, "rect");
  rect.setAttribute("x", x);
  rect.setAttribute("y", y);
  rect.setAttribute("width", width);
  rect.setAttribute("height", height);
  rect.setAttribute("rx", 18);
  rect.setAttribute("fill", "rgba(0,0,0,0.001)");
  rect.setAttribute("pointer-events", "all");
  return rect;
}

function svgEllipse(ann, stroke, strokeWidth) {
  const ellipse = document.createElementNS(SVG_NS, "ellipse");
  ellipse.setAttribute("cx", ann.x + ann.width / 2);
  ellipse.setAttribute("cy", ann.y + ann.height / 2);
  ellipse.setAttribute("rx", ann.width / 2);
  ellipse.setAttribute("ry", ann.height / 2);
  ellipse.setAttribute("fill", "none");
  ellipse.setAttribute("stroke", stroke);
  ellipse.setAttribute("stroke-width", strokeWidth);
  ellipse.setAttribute("stroke-linecap", "round");
  ellipse.setAttribute("stroke-linejoin", "round");
  return ellipse;
}

function svgHitEllipse(ann) {
  const ellipse = document.createElementNS(SVG_NS, "ellipse");
  ellipse.setAttribute("cx", ann.x + ann.width / 2);
  ellipse.setAttribute("cy", ann.y + ann.height / 2);
  ellipse.setAttribute("rx", ann.width / 2);
  ellipse.setAttribute("ry", ann.height / 2);
  ellipse.setAttribute("fill", "rgba(0,0,0,0.001)");
  ellipse.setAttribute("pointer-events", "all");
  return ellipse;
}

function draftVectorAnnotation(tool, origin, current) {
  if (tool === "arrow") {
    if (origin.x === current.x && origin.y === current.y) return null;
    return { type: "arrow", x1: origin.x, y1: origin.y, x2: current.x, y2: current.y };
  }

  const rect = normalizedRect(origin, current);
  if (rect.width <= 1 || rect.height <= 1) return null;
  if (tool === "rectangle" || tool === "ellipse") {
    return { type: tool, x: rect.x, y: rect.y, width: rect.width, height: rect.height };
  }
  return null;
}

function svgArrow(ann, strokeColor, outlineColor, outlineExtra) {
  const strokeWidth = ann.strokeWidth ?? DEFAULT_STROKE_WIDTH;
  const pointsStr = arrowPolygonPoints(ann).map(([x, y]) => `${x},${y}`).join(" ");

  const polygon = document.createElementNS(SVG_NS, "polygon");
  polygon.setAttribute("points", pointsStr);
  polygon.setAttribute("fill", strokeColor);
  if (outlineColor) {
    polygon.setAttribute("stroke", outlineColor);
    polygon.setAttribute("stroke-width", outlineExtra ?? (8 * (strokeWidth / DEFAULT_STROKE_WIDTH)));
    polygon.setAttribute("stroke-linejoin", "round");
  }
  return polygon;
}

function svgHitArrow(ann) {
  // Fat transparent stroke along the shaft gives users a forgiving click band
  // (the visible arrow polygon is too narrow to reliably hit after deselect).
  // pointer-events="stroke" ensures a transparent stroke still receives events.
  const line = document.createElementNS(SVG_NS, "line");
  line.setAttribute("x1", ann.x1);
  line.setAttribute("y1", ann.y1);
  line.setAttribute("x2", ann.x2);
  line.setAttribute("y2", ann.y2);
  line.setAttribute("stroke", "transparent");
  line.setAttribute("stroke-width", 28);
  line.setAttribute("stroke-linecap", "round");
  line.setAttribute("fill", "none");
  line.setAttribute("pointer-events", "stroke");
  return line;
}

function arrowPolygonPoints(ann) {
  const dx = ann.x2 - ann.x1;
  const dy = ann.y2 - ann.y1;
  const length = Math.hypot(dx, dy);
  const ux = dx / length;
  const uy = dy / length;
  const strokeWidth = ann.strokeWidth ?? DEFAULT_STROKE_WIDTH;
  const sx = length / (BASELINE_ARROW_TIP_X - BASELINE_ARROW_TAIL_X);
  let sy = strokeWidth / DEFAULT_STROKE_WIDTH;
  if (sy <= 0) sy = 1;

  const a = ux * sx;
  const b = uy * sx;
  const c = -uy * sy;
  const d = ux * sy;
  const e = ann.x1 - a * BASELINE_ARROW_TAIL_X - c * BASELINE_ARROW_TAIL_Y;
  const f = ann.y1 - b * BASELINE_ARROW_TAIL_X - d * BASELINE_ARROW_TAIL_Y;

  return BASELINE_ARROW_POINTS.map(([x, y]) => [
    a * x + c * y + e,
    b * x + d * y + f,
  ]);
}

function createBackend() {
  if (window.go?.gui?.App) {
    return {
      captureScreen: () => window.go.gui.App.CaptureScreen(),
      captureRegion: () => window.go.gui.App.CaptureRegion(),
      captureAllDisplays: () => window.go.gui.App.CaptureAllDisplays(),
      openDocument: () => window.go.gui.App.OpenDocument(),
      saveDocument: (path, contents) => window.go.gui.App.SaveDocument(path, contents),
      saveDocumentAs: (suggestedName, contents) => window.go.gui.App.SaveDocumentAs(suggestedName, contents),
      exportDocument: (payload, captureBase64, width, height, format, copy) =>
        window.go.gui.App.ExportDocument(payload, captureBase64, width, height, format, copy),
      exportDocumentToFile: (payload, captureBase64, width, height, format, suggestedName) =>
        window.go.gui.App.ExportDocumentToFile(payload, captureBase64, width, height, format, suggestedName),
      getPreferences: () => window.go.gui.App.GetPreferences(),
      defaultPreferences: () => window.go.gui.App.DefaultPreferences(),
      savePreferences: (preferences) => window.go.gui.App.SavePreferences(preferences),
      resetPreferences: () => window.go.gui.App.ResetPreferences(),
      chooseExportDirectory: (current) => window.go.gui.App.ChooseExportDirectory(current),
      getHotkeys: () => window.go.gui.App.GetHotkeys(),
      saveHotkeys: (bindings) => window.go.gui.App.SaveHotkeys(bindings),
      resetHotkeys: () => window.go.gui.App.ResetHotkeys(),
    };
  }

  const mockDocuments = new Map();
  let mockCounter = 1;
  let mockHotkeys = [];
  const mockPreferences = { exportDirectory: "/mock/Downloads" };

  return {
    async captureScreen() {
      return mockCapture(1200, 720, { id: "1", x: 1200, y: 0 });
    },
    async captureRegion() {
      return mockCapture(860, 520, { id: "1", x: 60, y: 80 });
    },
    async captureAllDisplays() {
      return mockCapture(2400, 900, { id: "all", x: -900, y: 0 });
    },
    async openDocument() {
      const entries = Array.from(mockDocuments.entries());
      if (!entries.length) {
        throw new Error("目前沒有已儲存文件");
      }
      const [path, contents] = entries[entries.length - 1];
      return { path, name: path.split("/").pop(), contents };
    },
    async saveDocument(path, contents) {
      mockDocuments.set(path, contents);
      return { path, name: path.split("/").pop() };
    },
    async saveDocumentAs(suggestedName, contents) {
      const path = `/mock/${suggestedName.replace(/\.sv\.json$/i, "")}-${mockCounter++}.sv.json`;
      mockDocuments.set(path, contents);
      return { path, name: path.split("/").pop() };
    },
    async exportDocument(payload, captureBase64, width, height, format, copy) {
      const mime = format === "svg" ? "image/svg+xml" : format === "pdf" ? "application/pdf" : format === "jpg" ? "image/jpeg" : "image/png";
      return {
        format,
        mimeType: mime,
        annotationCount: JSON.parse(payload).length,
        canvas: { width, height },
        captureRegion: { x: 0, y: 0, width, height },
        copiedToClipboard: copy,
        svg: `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}"></svg>`,
        base64: captureBase64,
      };
    },
    async exportDocumentToFile(payload, captureBase64, width, height, format, suggestedName) {
      return {
        path: `${mockPreferences.exportDirectory}/${suggestedName}`,
        name: suggestedName.split("/").pop(),
      };
    },
    async getPreferences() {
      return { ...mockPreferences };
    },
    async defaultPreferences() {
      return { exportDirectory: "/mock/Downloads" };
    },
    async savePreferences(preferences) {
      mockPreferences.exportDirectory = normalizePreferencePath(preferences?.exportDirectory);
      if (!mockPreferences.exportDirectory) {
        mockPreferences.exportDirectory = "/mock/Downloads";
      }
      return { ...mockPreferences };
    },
    async resetPreferences() {
      mockPreferences.exportDirectory = "/mock/Downloads";
      return { ...mockPreferences };
    },
    async chooseExportDirectory(current) {
      return current || "/mock/exports";
    },
    async getHotkeys() {
      return mockHotkeys.length ? mockHotkeys.slice() : defaultHotkeyBindings();
    },
    async saveHotkeys(bindings) {
      mockHotkeys = bindings.slice();
    },
    async resetHotkeys() {
      mockHotkeys = [];
      return defaultHotkeyBindings();
    },
  };
}

function mockCapture(width, height, display = { id: "1", x: 0, y: 0 }) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  ctx.fillStyle = "#1d2939";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(56, 56, Math.min(840, width - 112), Math.min(284, height - 112));
  ctx.fillRect(Math.max(56, width - 380), Math.max(56, height - 270), Math.min(312, width - 112), Math.min(214, height - 112));
  ctx.fillStyle = "#94a3b8";
  for (let i = 0; i < 10; i++) {
    ctx.globalAlpha = 0.35;
    ctx.fillRect(92, 118 + i * 24, Math.min(500 + (i % 3) * 70, width - 184), 12);
  }
  return {
    format: "png",
    mimeType: "image/png",
    base64: canvas.toDataURL("image/png").split(",")[1],
    display: { id: display.id, x: display.x, y: display.y, width, height },
    captureRegion: { x: display.x, y: display.y, width, height },
  };
}

function cloneAnnotation(ann) {
  return JSON.parse(JSON.stringify(ann));
}

function nextId(prefix) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1000)}`;
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function round(value) {
  return Math.round(value);
}

function numberValue(value, fallback) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function hotkeyActions() {
  return {
    "tool.select":          () => setTool("select"),
    "tool.arrow":           () => setTool("arrow"),
    "tool.rectangle":       () => setTool("rectangle"),
    "tool.ellipse":         () => setTool("ellipse"),
    "tool.text":            () => setTool("text"),
    "tool.blur":            () => setTool("blur"),
    "tool.numberedCircle":  () => setTool("numbered-circle"),
    "tool.crop":            () => setTool("crop"),
    "edit.undo":            () => undo(),
    "edit.redo":            () => redo(),
    "file.open":            () => openDocument(),
    "file.save":            () => saveDocument(),
    "file.saveAs":          () => saveDocumentAs(),
    "view.zoomIn":          () => changeZoom(0.1),
    "view.zoomOut":         () => changeZoom(-0.1),
    "view.zoomReset":       () => {
      state.zoom = 1;
      state.zoomAutoFit = false;
      state.pan = { x: 0, y: 0 };
      render();
    },
    "export.copy":          () => exportCurrent(true),
    "capture.fullscreen":   () => captureScreen("fullscreen"),
    "capture.region":       () => captureScreen("region"),
    "capture.allDisplays":  () => captureScreen("all-displays"),
    "tab.new":              () => newBlankTab(),
    "tab.close":            () => closeTab(activeTabId),
    "tab.next":             () => activateNeighborTab(+1),
    "tab.prev":             () => activateNeighborTab(-1),
    "app.preferences":      () => openPreferences(),
  };
}

async function loadHotkeys() {
  try {
    const bindings = await backend.getHotkeys();
    applyHotkeyBindings(bindings);
  } catch (err) {
    console.warn("loadHotkeys failed, falling back to defaults:", err);
    applyHotkeyBindings(defaultHotkeyBindings());
  }
}

async function loadPreferences() {
  try {
    const [defaults, loaded] = await Promise.all([
      backend.defaultPreferences(),
      backend.getPreferences(),
    ]);
    defaultPreferences.exportDirectory = normalizePreferencePath(defaults?.exportDirectory);
    userPreferences.exportDirectory = normalizePreferencePath(loaded?.exportDirectory);
  } catch (err) {
    console.warn("loadPreferences failed, falling back to defaults:", err);
    defaultPreferences.exportDirectory = "/Downloads";
    userPreferences.exportDirectory = defaultPreferences.exportDirectory;
  }
}

function applyHotkeyBindings(bindings) {
  hotkeys.bindings = bindings.slice();
  hotkeys.comboToAction = new Map();
  hotkeys.actionToCombo = new Map();
  for (const b of bindings) {
    if (b.combo) hotkeys.comboToAction.set(b.combo, b.action);
    hotkeys.actionToCombo.set(b.action, b.combo || "");
  }
  updateHotkeyAnnotatedTitles();
}

function updateHotkeyAnnotatedTitles() {
  if (typeof document === "undefined") return;
  const nodes = document.querySelectorAll("[data-hotkey-action]");
  nodes.forEach((node) => {
    const action = node.getAttribute("data-hotkey-action");
    const base = node.getAttribute("data-title-base") || node.getAttribute("title") || "";
    if (!node.getAttribute("data-title-base")) node.setAttribute("data-title-base", base);
    const combo = (hotkeys.actionToCombo && hotkeys.actionToCombo.get(action)) || "";
    const display = combo && typeof SV_Hotkey !== "undefined" ? SV_Hotkey.comboToDisplay(combo, IS_MAC) : "";
    node.title = display ? `${base} (${display})` : base;
  });
}

function defaultHotkeyBindings() {
  // Mirrors gui/hotkey.go DefaultHotkeys(). Used only when backend is absent
  // (e.g. the mock path when running pure HTML). Keep in sync manually.
  return [
    { action: "tool.select", combo: "v", scope: "app" },
    { action: "tool.arrow", combo: "a", scope: "app" },
    { action: "tool.rectangle", combo: "r", scope: "app" },
    { action: "tool.ellipse", combo: "o", scope: "app" },
    { action: "tool.text", combo: "t", scope: "app" },
    { action: "tool.blur", combo: "b", scope: "app" },
    { action: "tool.numberedCircle", combo: "n", scope: "app" },
    { action: "tool.crop", combo: "c", scope: "app" },
    { action: "edit.undo", combo: "mod+z", scope: "app" },
    { action: "edit.redo", combo: "mod+shift+z", scope: "app" },
    { action: "file.open", combo: "mod+o", scope: "app" },
    { action: "file.save", combo: "mod+s", scope: "app" },
    { action: "file.saveAs", combo: "mod+shift+s", scope: "app" },
    { action: "view.zoomIn", combo: "mod+=", scope: "app" },
    { action: "view.zoomOut", combo: "mod+-", scope: "app" },
    { action: "view.zoomReset", combo: "mod+0", scope: "app" },
    { action: "export.copy", combo: "mod+shift+c", scope: "app" },
    { action: "capture.fullscreen", combo: "mod+shift+q", scope: "app" },
    { action: "capture.region", combo: "mod+shift+w", scope: "app" },
    { action: "capture.allDisplays", combo: "mod+shift+e", scope: "app" },
    { action: "tab.new", combo: "mod+t", scope: "app" },
    { action: "tab.close", combo: "mod+w", scope: "app" },
    { action: "tab.next", combo: "mod+alt+right", scope: "app" },
    { action: "tab.prev", combo: "mod+alt+left", scope: "app" },
    { action: "app.preferences", combo: "mod+,", scope: "app" },
  ];
}

function isTypingTarget(target) {
  if (!target) return false;
  if (target.isContentEditable) return true;
  const tag = (target.tagName || "").toUpperCase();
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

function onGlobalKeydown(event) {
  if (typeof SV_Hotkey === "undefined" || !SV_Hotkey.normalize) return;
  if (hotkeys.suspended) return;
  if (event.isComposing) return;
  if (event.repeat) return;
  if (isTypingTarget(event.target)) return;
  const combo = SV_Hotkey.normalize(event, IS_MAC);
  if (!combo) return;
  const action = hotkeys.comboToAction.get(combo);
  if (!action) return;
  const handler = hotkeyActions()[action];
  if (!handler) return;
  event.preventDefault();
  handler();
}

const ACTION_LABELS = {
  "tool.select": "Select tool",
  "tool.arrow": "Arrow tool",
  "tool.rectangle": "Rectangle tool",
  "tool.ellipse": "Ellipse tool",
  "tool.text": "Text tool",
  "tool.blur": "Blur tool",
  "tool.numberedCircle": "Numbered circle tool",
  "tool.crop": "Crop tool",
  "edit.undo": "Undo",
  "edit.redo": "Redo",
  "file.open": "Open document",
  "file.save": "Save document",
  "file.saveAs": "Save document as…",
  "view.zoomIn": "Zoom in",
  "view.zoomOut": "Zoom out",
  "view.zoomReset": "Reset zoom",
  "export.copy": "Copy to clipboard",
  "capture.fullscreen": "Capture full screen",
  "capture.region": "Capture region",
  "capture.allDisplays": "Capture all displays",
  "tab.new": "New tab",
  "tab.close": "Close tab",
  "tab.next": "Next tab",
  "tab.prev": "Previous tab",
  "app.preferences": "Open Preferences",
};

const ACTION_GROUPS = [
  { title: "Tools", actions: ["tool.select", "tool.arrow", "tool.rectangle", "tool.ellipse", "tool.text", "tool.blur", "tool.numberedCircle", "tool.crop"] },
  { title: "Editing", actions: ["edit.undo", "edit.redo"] },
  { title: "File", actions: ["file.open", "file.save", "file.saveAs"] },
  { title: "View", actions: ["view.zoomIn", "view.zoomOut", "view.zoomReset"] },
  { title: "Export", actions: ["export.copy"] },
  { title: "Capture", actions: ["capture.fullscreen", "capture.region", "capture.allDisplays"] },
  { title: "Tabs", actions: ["tab.new", "tab.close", "tab.next", "tab.prev"] },
  { title: "App", actions: ["app.preferences"] },
];

function openPreferences() {
  prefs.draft = hotkeys.bindings.map((b) => ({ ...b }));
  prefs.exportDirectory = userPreferences.exportDirectory;
  prefs.recordingAction = null;
  prefs.recordingBuffer = "";
  syncPreferencesDirty();
  clearPreferencesStatus();
  els.preferencesFilter.value = "";
  els.preferencesModal.classList.remove("is-hidden");
  closeFileMenu();
  renderPreferences();
}

function closePreferences() {
  // Cancel / ✕ / backdrop are explicit discard intents — no confirm prompt
  // (native window.confirm is unreliable in the Wails WebView and the UX
  // matches macOS System Settings / VS Code Preferences, which just close).
  resetRecordingState();
  prefs.dirty = false;
  els.preferencesModal.classList.add("is-hidden");
  els.preferencesConflict.classList.add("is-hidden");
}

function renderPreferences() {
  const filter = els.preferencesFilter.value.trim().toLowerCase();
  const byAction = new Map(prefs.draft.map((b) => [b.action, b]));
  els.preferencesBody.innerHTML = "";
  if (shouldShowExportPreference(filter)) {
    els.preferencesBody.append(renderExportPreference());
  }
  for (const group of ACTION_GROUPS) {
    const rows = group.actions
      .map((action) => ({ action, binding: byAction.get(action) }))
      .filter(({ action, binding }) => {
        if (!filter) return true;
        const label = (ACTION_LABELS[action] || action).toLowerCase();
        const combo = (binding?.combo || "").toLowerCase();
        return label.includes(filter) || combo.includes(filter);
      });
    if (!rows.length) continue;
    const groupEl = document.createElement("div");
    groupEl.className = "hotkey-group";
    const title = document.createElement("h3");
    title.className = "hotkey-group-title";
    title.textContent = group.title;
    groupEl.append(title);
    for (const { action, binding } of rows) {
      const row = document.createElement("div");
      row.className = "hotkey-row";
      const label = document.createElement("span");
      label.textContent = ACTION_LABELS[action] || action;
      const field = document.createElement("button");
      field.type = "button";
      field.className = "hotkey-field";
      field.dataset.action = action;
      updateFieldDisplay(field, binding?.combo || "");
      field.addEventListener("click", () => startRecording(action, field));
      const clear = document.createElement("button");
      clear.type = "button";
      clear.className = "hotkey-clear";
      clear.textContent = "✕";
      clear.title = "Clear hotkey";
      clear.addEventListener("click", () => clearBinding(action));
      row.append(label, field, clear);
      groupEl.append(row);
    }
    els.preferencesBody.append(groupEl);
  }
}

function renderExportPreference() {
  const groupEl = document.createElement("div");
  groupEl.className = "hotkey-group";
  const title = document.createElement("h3");
  title.className = "hotkey-group-title";
  title.textContent = "Export";
  groupEl.append(title);

  const row = document.createElement("div");
  row.className = "preferences-setting";
  const copy = document.createElement("div");
  copy.className = "preferences-setting-copy";
  const label = document.createElement("span");
  label.className = "preferences-setting-label";
  label.textContent = "Export folder";
  const hint = document.createElement("span");
  hint.className = "preferences-setting-hint";
  hint.textContent = "預設會用各平台的 Downloads 資料夾，按 Export 時直接寫進去，不再每次跳存檔視窗。";
  copy.append(label, hint);

  const controls = document.createElement("div");
  controls.className = "preferences-setting-controls";
  const input = document.createElement("input");
  input.className = "preferences-path";
  input.type = "text";
  input.readOnly = true;
  input.value = prefs.exportDirectory;
  input.placeholder = defaultPreferences.exportDirectory || "Platform default Downloads folder";
  const choose = document.createElement("button");
  choose.type = "button";
  choose.className = "ghost-btn";
  choose.textContent = "Choose…";
  choose.addEventListener("click", () => chooseExportDirectory());
  const clear = document.createElement("button");
  clear.type = "button";
  clear.className = "ghost-btn";
  clear.textContent = "Reset";
  clear.disabled = prefs.exportDirectory === defaultPreferences.exportDirectory;
  clear.addEventListener("click", () => resetExportDirectoryToDefault());
  controls.append(input, choose, clear);

  row.append(copy, controls);
  groupEl.append(row);
  return groupEl;
}

function updateFieldDisplay(fieldEl, combo) {
  fieldEl.textContent = combo ? SV_Hotkey.comboToDisplay(combo, IS_MAC) : "Unbound";
  fieldEl.classList.toggle("is-unbound", !combo);
  fieldEl.classList.remove("is-recording");
}

function startRecording(action, fieldEl) {
  // Task 8 will implement the real recorder. For now: cancel any previous
  // recording (defensive) and mark this field active so the user sees
  // feedback. The capture listener comes in Task 8.
  if (prefs.recordingAction) cancelRecording();
  prefs.recordingAction = action;
  prefs.recordingBuffer = "";
  hotkeys.suspended = true;
  fieldEl.classList.add("is-recording");
  fieldEl.textContent = "Press keys…";
}

function resetRecordingState() {
  if (!prefs.recordingAction) return;
  prefs.recordingAction = null;
  prefs.recordingBuffer = "";
  hotkeys.suspended = false;
}

function cancelRecording() {
  if (!prefs.recordingAction) return;
  resetRecordingState();
  renderPreferences();
}

function clearBinding(action) {
  setDraftCombo(action, "");
}

function setDraftCombo(action, combo) {
  const row = prefs.draft.find((b) => b.action === action);
  if (!row) return;
  if (row.combo === combo) return;
  row.combo = combo;
  resetRecordingState();
  syncPreferencesDirty();
  renderPreferences();
}

async function savePreferences() {
  try {
    const savedPreferences = await backend.savePreferences({ exportDirectory: prefs.exportDirectory });
    await backend.saveHotkeys(prefs.draft);
    userPreferences.exportDirectory = normalizePreferencePath(savedPreferences?.exportDirectory);
    prefs.exportDirectory = userPreferences.exportDirectory;
    applyHotkeyBindings(prefs.draft);
    syncPreferencesDirty();
    closePreferences();
    showToast("已儲存 Preferences");
  } catch (err) {
    setPreferencesError(`儲存失敗：${err?.message || err}`);
  }
}

function resetExportDirectoryToDefault() {
  if (prefs.exportDirectory === defaultPreferences.exportDirectory) return;
  prefs.exportDirectory = defaultPreferences.exportDirectory;
  syncPreferencesDirty();
  renderPreferences();
}

async function chooseExportDirectory() {
  try {
    const selected = await backend.chooseExportDirectory(prefs.exportDirectory || userPreferences.exportDirectory);
    const next = normalizePreferencePath(selected);
    if (!next) return;
    prefs.exportDirectory = next;
    syncPreferencesDirty();
    renderPreferences();
  } catch (err) {
    setPreferencesError(`選擇資料夾失敗：${err?.message || err}`);
  }
}

function shouldShowExportPreference(filter) {
  if (!filter) return true;
  const haystack = ["export", "folder", "path", "directory", "save", "匯出", "資料夾", "路徑"];
  return haystack.some((token) => token.includes(filter) || filter.includes(token));
}

function normalizePreferencePath(path) {
  return typeof path === "string" ? path.trim() : "";
}

function clearPreferencesStatus() {
  els.preferencesStatus.textContent = "";
  els.preferencesStatus.classList.remove("is-error");
}

function setPreferencesError(message) {
  els.preferencesStatus.textContent = message;
  els.preferencesStatus.classList.add("is-error");
}

function syncPreferencesDirty() {
  prefs.dirty = prefs.exportDirectory !== userPreferences.exportDirectory || !sameHotkeyBindings(prefs.draft, hotkeys.bindings);
}

function sameHotkeyBindings(left, right) {
  if (left.length !== right.length) return false;
  return left.every((binding, index) =>
    binding.action === right[index]?.action &&
    binding.combo === right[index]?.combo &&
    binding.scope === right[index]?.scope,
  );
}

function onRecorderKeydown(event) {
  if (!prefs.recordingAction) return;
  // Stop both preventDefault and propagation so onGlobalKeydown (bubble phase)
  // never sees this event — otherwise committing a combo could immediately
  // dispatch the previously-bound action for the same keystroke.
  event.preventDefault();
  event.stopPropagation();

  // These keys, pressed alone, control the recorder itself.
  if (!event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey) {
    if (event.key === "Escape") {
      cancelRecording();
      return;
    }
    if (event.key === "Backspace" || event.key === "Delete") {
      const action = prefs.recordingAction;
      resetRecordingState();
      setDraftCombo(action, "");
      return;
    }
  }

  if (!SV_Hotkey.isRecordableMainKey(event)) {
    // Ignore modifier-only presses (wait for a main key).
    return;
  }

  const combo = SV_Hotkey.normalize(event, IS_MAC);
  if (!combo) return;
  commitRecording(combo);
}

function commitRecording(combo) {
  const action = prefs.recordingAction;
  if (!action) return;
  const conflict = SV_Hotkey.detectConflict(prefs.draft, action, combo);
  if (!conflict) {
    resetRecordingState();
    setDraftCombo(action, combo);
    return;
  }
  showConflictDialog(action, conflict, combo);
}

function showConflictDialog(action, conflictAction, combo) {
  const dialog = els.preferencesConflict;
  const comboDisplay = SV_Hotkey.comboToDisplay(combo, IS_MAC);
  const conflictLabel = ACTION_LABELS[conflictAction] || conflictAction;
  const actionLabel = ACTION_LABELS[action] || action;

  // Build DOM via createElement (no innerHTML) to stay XSS-safe even though
  // all values here are trusted internal labels.
  dialog.innerHTML = "";
  const warning = document.createElement("p");
  const warnPrefix = document.createTextNode("⚠️  ");
  warning.append(warnPrefix);
  const comboStrong = document.createElement("strong");
  comboStrong.textContent = comboDisplay;
  warning.append(comboStrong);
  warning.append(document.createTextNode(" is already bound to "));
  const conflictEm = document.createElement("em");
  conflictEm.textContent = conflictLabel;
  warning.append(conflictEm);
  warning.append(document.createTextNode("."));

  const question = document.createElement("p");
  question.append(document.createTextNode("Reassign it to "));
  const actionEm = document.createElement("em");
  actionEm.textContent = actionLabel;
  question.append(actionEm);
  question.append(document.createTextNode("? The previous binding will become unbound."));

  const actions = document.createElement("div");
  actions.className = "conflict-actions";
  const cancelBtn = document.createElement("button");
  cancelBtn.type = "button";
  cancelBtn.className = "ghost-btn";
  cancelBtn.dataset.conflict = "cancel";
  cancelBtn.textContent = "Cancel";
  const reassignBtn = document.createElement("button");
  reassignBtn.type = "button";
  reassignBtn.className = "ghost-btn is-primary";
  reassignBtn.dataset.conflict = "reassign";
  reassignBtn.textContent = "Reassign";
  actions.append(cancelBtn, reassignBtn);

  dialog.append(warning, question, actions);
  dialog.classList.remove("is-hidden");

  const onClick = (event) => {
    const choice = event.target.closest("[data-conflict]")?.dataset.conflict;
    if (!choice) return;
    dialog.classList.add("is-hidden");
    dialog.removeEventListener("click", onClick);
    if (choice === "reassign") {
      reassignCombo(action, conflictAction, combo);
    } else {
      cancelRecording();
    }
  };
  dialog.addEventListener("click", onClick);
}

function reassignCombo(action, conflictAction, combo) {
  const conflictRow = prefs.draft.find((b) => b.action === conflictAction);
  if (conflictRow) conflictRow.combo = "";
  resetRecordingState();
  setDraftCombo(action, combo);
}
