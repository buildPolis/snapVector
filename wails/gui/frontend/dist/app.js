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

const state = {
  capture: null,
  annotations: [],
  selectedId: null,
  tool: "select",
  history: [],
  future: [],
  action: null,
  pointer: { x: 0, y: 0 },
  zoom: DEFAULT_ZOOM,
  pan: { x: 0, y: 0 },
  document: {
    path: "",
    name: "Untitled",
    dirty: false,
    savedFingerprint: "",
    menuOpen: false,
  },
};

const IS_MAC = /mac|iphone|ipad|ipod/i.test(navigator.platform);

const hotkeys = {
  bindings: [],              // Array<{action, combo, scope}>
  comboToAction: new Map(),  // combo → action
  suspended: false,          // true while modal is recording
};

const prefs = {
  draft: [],           // working copy during modal session
  dirty: false,
  recordingAction: null,
  recordingBuffer: "",
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
  bindUI();
  await loadHotkeys();
  window.addEventListener("keydown", onRecorderKeydown, true); // capture phase
  window.addEventListener("keydown", onGlobalKeydown);
  // No auto-capture: the hide/show dance in the Go capture path would flash
  // the window right after it first appears. Let the user click a capture
  // button when they're ready.
  render();
}

function bindUI() {
  els.toolButtons.forEach((button) => {
    button.addEventListener("click", async () => {
      const tool = button.dataset.tool;
      if (tool === "capture") {
        await captureScreen();
        return;
      }
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

  els.canvasStage.addEventListener("pointerdown", onPointerDown);
  window.addEventListener("pointermove", onPointerMove);
  window.addEventListener("pointerup", onPointerUp);
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
}

const CAPTURE_MODES = {
  fullscreen: {
    title: "captured-screen.png",
    loadingToast: "正在擷取滑鼠所在螢幕...",
    doneToast: () => "已載入滑鼠所在螢幕",
    tool: "select",
    call: () => backend.captureScreen(),
  },
  region: {
    title: "captured-region.png",
    loadingToast: "請在桌面拖曳選取擷取範圍...",
    doneToast: (c) => `已載入所選區域 ${c.width} × ${c.height}`,
    tool: "select",
    call: () => backend.captureRegion(),
  },
  "all-displays": {
    title: "captured-all-displays.png",
    loadingToast: "正在載入所有螢幕，接著可拖曳裁切...",
    doneToast: (c) => `已載入所有螢幕 ${c.width} × ${c.height}，拖曳框選要保留的區域`,
    tool: "crop",
    call: () => backend.captureAllDisplays(),
  },
};

init().catch((error) => {
  console.error(error);
  showToast(String(error));
});

async function captureScreen(mode = "fullscreen") {
  const plan = CAPTURE_MODES[mode] || CAPTURE_MODES.fullscreen;
  closeFileMenu();
  showToast(plan.loadingToast);
  const capture = await plan.call();
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
  state.zoom = fitToWidthZoom(state.capture.width);
  state.zoomAutoFit = true;
  state.pan = { x: 0, y: 0 };
  state.tool = plan.tool;
  syncDirtyState();
  syncToolButtons();
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
      maxWidth: 220,
    });
    state.selectedId = id;
    render();
    els.textContent.focus();
    els.textContent.select();
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

  if (state.tool === "select" && state.zoom > 1) {
    state.action = { kind: "pan", originClientX: event.clientX, originClientY: event.clientY, pan: { ...state.pan } };
    return;
  }

  state.selectedId = null;
  render();
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
  ann.x = next.x;
  ann.y = next.y;
  ann.width = next.width;
  ann.height = next.height;
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
  };
}

function restoreSnapshot(data) {
  state.capture = data.capture ? { ...data.capture } : null;
  state.annotations = data.annotations.map(cloneAnnotation);
  state.selectedId = data.selectedId;
  state.tool = data.tool;
  state.zoom = data.zoom;
  state.pan = { ...data.pan };
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
  state.document.dirty = fingerprint !== "" && fingerprint !== state.document.savedFingerprint;
}

function defaultDocumentName() {
  if (state.document.name && state.document.name !== "Untitled") {
    return state.document.name;
  }
  return "capture.sv.json";
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

  if (!state.capture) {
    els.emptyState.classList.remove("is-hidden");
    els.canvasStage.classList.add("is-hidden");
    document.querySelector(".app-body")?.classList.add("inspector-collapsed");
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
      group.appendChild(svgArrow(ann));
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
        group.appendChild(svgArrow(draft));
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
      div.style.maxWidth = `${ann.maxWidth || 220}px`;
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
  els.geometryFields.innerHTML = "";

  const emptyHint = document.getElementById("inspectorEmpty");
  const sectionSelected = document.getElementById("sectionSelected");
  const sectionGeometry = document.getElementById("sectionGeometry");
  const sectionText = document.getElementById("sectionText");
  const sectionBlur = document.getElementById("sectionBlur");
  const appBody = document.querySelector(".app-body");

  if (!ann) {
    disableInspector(true);
    appBody?.classList.add("inspector-collapsed");
    emptyHint?.classList.add("is-hidden");
    sectionSelected?.classList.add("is-hidden");
    sectionGeometry?.classList.add("is-hidden");
    sectionText?.classList.add("is-hidden");
    sectionBlur?.classList.add("is-hidden");
    return;
  }

  disableInspector(false);
  appBody?.classList.remove("inspector-collapsed");
  emptyHint?.classList.add("is-hidden");
  sectionSelected?.classList.remove("is-hidden");
  sectionGeometry?.classList.remove("is-hidden");
  sectionText?.classList.toggle("is-hidden", ann.type !== "text");
  sectionBlur?.classList.toggle("is-hidden", ann.type !== "blur");
  [ann.type, ann.id].forEach((value, index) => {
    const chip = document.createElement("span");
    chip.className = "chip";
    chip.textContent = `${index === 0 ? "type" : "id"} · ${value}`;
    els.selectedMeta.appendChild(chip);
  });

  if (ann.type === "arrow") {
    [["x1", ann.x1], ["y1", ann.y1], ["x2", ann.x2], ["y2", ann.y2]].forEach(([key, value]) => addField(key, value));
  } else {
    [["x", ann.x], ["y", ann.y], ["width", ann.width], ["height", ann.height]].forEach(([key, value]) => addField(key, value));
  }

  els.textContent.value = ann.type === "text" ? ann.text : "";
  els.textVariant.value = ann.type === "text" ? ann.variant || "solid" : "solid";
  els.textFontSize.value = ann.type === "text" ? ann.fontSize || 24 : 24;
  els.textMaxWidth.value = ann.type === "text" ? ann.maxWidth || 0 : 0;
  els.blurRadius.value = ann.type === "blur" ? ann.blurRadius || 12 : 12;
  els.cornerRadius.value = ann.type === "blur" ? ann.cornerRadius || 18 : 18;
  els.feather.value = ann.type === "blur" ? ann.feather || 12 : 12;
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
  // Copy always produces PNG — that's what downstream apps (Slack, docs,
  // issue trackers) consistently accept from the clipboard.
  const format = copyToClipboard ? "png" : els.exportFormat.value;
  const payload = JSON.stringify(state.annotations.map(toPayload));
  const result = await backend.exportDocument(payload, state.capture.base64, state.capture.width, state.capture.height, format, copyToClipboard);
  if (copyToClipboard) {
    showToast(`已複製 ${format.toUpperCase()} 到剪貼簿`);
    return;
  }
  downloadResult(result);
  showToast(`已匯出 ${format.toUpperCase()}`);
}

function downloadResult(result) {
  const link = document.createElement("a");
  if (result.format === "svg") {
    const blob = new Blob([result.svg], { type: result.mimeType });
    link.href = URL.createObjectURL(blob);
  } else {
    link.href = `data:${result.mimeType};base64,${result.base64}`;
  }
  link.download = `snapvector-export.${result.format}`;
  link.click();
  if (link.href.startsWith("blob:")) {
    setTimeout(() => URL.revokeObjectURL(link.href), 1000);
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

function svgArrow(ann) {
  const strokeWidth = ann.strokeWidth ?? DEFAULT_STROKE_WIDTH;
  const polygon = document.createElementNS(SVG_NS, "polygon");
  polygon.setAttribute("points", arrowPolygonPoints(ann).map(([x, y]) => `${x},${y}`).join(" "));
  polygon.setAttribute("fill", "#E53935");
  polygon.setAttribute("stroke", "#FFFFFF");
  polygon.setAttribute("stroke-width", 6 * (strokeWidth / DEFAULT_STROKE_WIDTH));
  polygon.setAttribute("stroke-linejoin", "round");
  polygon.setAttribute("paint-order", "stroke fill");
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
      getHotkeys: () => window.go.gui.App.GetHotkeys(),
      saveHotkeys: (bindings) => window.go.gui.App.SaveHotkeys(bindings),
      resetHotkeys: () => window.go.gui.App.ResetHotkeys(),
    };
  }

  const mockDocuments = new Map();
  let mockCounter = 1;
  let mockHotkeys = [];

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

function applyHotkeyBindings(bindings) {
  hotkeys.bindings = bindings.slice();
  hotkeys.comboToAction = new Map();
  for (const b of bindings) {
    if (b.combo) hotkeys.comboToAction.set(b.combo, b.action);
  }
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
  "app.preferences": "Open Preferences",
};

const ACTION_GROUPS = [
  { title: "Tools", actions: ["tool.select", "tool.arrow", "tool.rectangle", "tool.ellipse", "tool.text", "tool.blur", "tool.crop"] },
  { title: "Editing", actions: ["edit.undo", "edit.redo"] },
  { title: "File", actions: ["file.open", "file.save", "file.saveAs"] },
  { title: "View", actions: ["view.zoomIn", "view.zoomOut", "view.zoomReset"] },
  { title: "Export", actions: ["export.copy"] },
  { title: "Capture", actions: ["capture.fullscreen", "capture.region", "capture.allDisplays"] },
  { title: "App", actions: ["app.preferences"] },
];

function openPreferences() {
  prefs.draft = hotkeys.bindings.map((b) => ({ ...b }));
  prefs.dirty = false;
  prefs.recordingAction = null;
  prefs.recordingBuffer = "";
  els.preferencesStatus.textContent = "";
  els.preferencesStatus.classList.remove("is-error");
  els.preferencesFilter.value = "";
  els.preferencesModal.classList.remove("is-hidden");
  closeFileMenu();
  renderPreferences();
}

function closePreferences(force = false) {
  if (!force && prefs.dirty) {
    const ok = confirm("You have unsaved hotkey changes. Discard them?");
    if (!ok) return;
  }
  resetRecordingState();
  els.preferencesModal.classList.add("is-hidden");
  els.preferencesConflict.classList.add("is-hidden");
}

function renderPreferences() {
  const filter = els.preferencesFilter.value.trim().toLowerCase();
  const byAction = new Map(prefs.draft.map((b) => [b.action, b]));
  els.preferencesBody.innerHTML = "";
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
  prefs.dirty = true;
  resetRecordingState();
  renderPreferences();
}

async function savePreferences() {
  try {
    await backend.saveHotkeys(prefs.draft);
    applyHotkeyBindings(prefs.draft);
    prefs.dirty = false;
    closePreferences(true);
    showToast("已儲存熱鍵設定");
  } catch (err) {
    els.preferencesStatus.textContent = `儲存失敗：${err?.message || err}`;
    els.preferencesStatus.classList.add("is-error");
  }
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
