// pbccreate thumbnail canvas editor — self-hosted vanilla JS, no dependencies.
// Renders the canvas model (background + text layers), supports select/drag and
// a properties panel, and serializes canvas_json back into the save form. The
// server-side render (render.png) remains the authoritative export.
"use strict";

(function () {
  const canvas = document.getElementById("thumb-canvas");
  if (!canvas) return; // only on the thumbnail editor page

  const ctx = canvas.getContext("2d");
  const jsonField = document.getElementById("canvas-json-field");
  const el = {
    bg: document.getElementById("ctl-bg"),
    add: document.getElementById("ctl-add"),
    text: document.getElementById("ctl-text"),
    color: document.getElementById("ctl-color"),
    size: document.getElementById("ctl-size"),
    bold: document.getElementById("ctl-bold"),
    del: document.getElementById("ctl-del"),
    guides: document.getElementById("ctl-guides"),
  };

  let state = parseCanvas();
  let selected = null;
  let drag = null;
  let guidesOn = false;

  function parseCanvas() {
    try {
      const c = JSON.parse(canvas.dataset.canvas || "{}");
      return { background: c.background || "#101418", layers: Array.isArray(c.layers) ? c.layers : [] };
    } catch (e) {
      return { background: "#101418", layers: [] };
    }
  }

  function clamp(v, lo, hi) {
    return v < lo ? lo : v > hi ? hi : v;
  }
  function lineHeight(fs) {
    return Math.round((fs || 100) * 1.2);
  }
  function fontSpec(l) {
    return (l.bold ? "bold " : "") + (l.fontSize || 100) + "px 'PBC Go', system-ui, sans-serif";
  }

  function sync() {
    jsonField.value = JSON.stringify(state);
  }

  function measure(l) {
    ctx.font = fontSpec(l);
    const lines = String(l.text || "").split("\n");
    let w = 0;
    for (const line of lines) w = Math.max(w, ctx.measureText(line).width);
    return { w: w, h: lines.length * lineHeight(l.fontSize), lines: lines };
  }

  function draw() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = state.background;
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.textBaseline = "top";

    state.layers.forEach(function (l, i) {
      if (l.type !== "text") return;
      const m = measure(l);
      ctx.font = fontSpec(l);
      ctx.fillStyle = l.color || "#ffffff";
      m.lines.forEach(function (line, li) {
        ctx.fillText(line, l.x || 0, (l.y || 0) + li * lineHeight(l.fontSize));
      });
      if (i === selected) {
        ctx.save();
        ctx.strokeStyle = "#5b9dff";
        ctx.lineWidth = 3;
        ctx.setLineDash([10, 6]);
        ctx.strokeRect((l.x || 0) - 6, (l.y || 0) - 6, m.w + 12, m.h + 12);
        ctx.restore();
      }
    });

    if (guidesOn) drawGuides();
  }

  function drawGuides() {
    ctx.save();
    ctx.strokeStyle = "rgba(91,157,255,0.8)";
    ctx.lineWidth = 2;
    ctx.setLineDash([12, 8]);
    const m = Math.round(canvas.width * 0.05); // ~title-safe margin
    ctx.strokeRect(m, m, canvas.width - 2 * m, canvas.height - 2 * m);
    // Approximate YouTube duration-badge zone (bottom-right).
    ctx.setLineDash([]);
    ctx.fillStyle = "rgba(0,0,0,0.4)";
    const bw = 180, bh = 64, bm = 24;
    ctx.fillRect(canvas.width - bw - bm, canvas.height - bh - bm, bw, bh);
    ctx.restore();
  }

  function toCanvasCoords(ev) {
    const rect = canvas.getBoundingClientRect();
    return {
      x: (ev.clientX - rect.left) * (canvas.width / rect.width),
      y: (ev.clientY - rect.top) * (canvas.height / rect.height),
    };
  }

  function hitTest(x, y) {
    for (let i = state.layers.length - 1; i >= 0; i--) {
      const l = state.layers[i];
      if (l.type !== "text") continue;
      const m = measure(l);
      const lx = l.x || 0, ly = l.y || 0;
      if (x >= lx - 6 && x <= lx + m.w + 6 && y >= ly - 6 && y <= ly + m.h + 6) return i;
    }
    return null;
  }

  // --- Pointer interaction (mouse + touch) ---
  canvas.addEventListener("pointerdown", function (ev) {
    const p = toCanvasCoords(ev);
    selected = hitTest(p.x, p.y);
    if (selected !== null) {
      const l = state.layers[selected];
      drag = { dx: p.x - (l.x || 0), dy: p.y - (l.y || 0) };
      canvas.setPointerCapture(ev.pointerId);
    }
    refreshPanel();
    draw();
  });
  canvas.addEventListener("pointermove", function (ev) {
    if (drag === null || selected === null) return;
    const p = toCanvasCoords(ev);
    const l = state.layers[selected];
    l.x = Math.round(clamp(p.x - drag.dx, 0, canvas.width));
    l.y = Math.round(clamp(p.y - drag.dy, 0, canvas.height));
    sync();
    draw();
  });
  canvas.addEventListener("pointerup", function () {
    drag = null;
  });

  // --- Properties panel ---
  function selectedLayer() {
    return selected !== null ? state.layers[selected] : null;
  }

  el.bg.addEventListener("input", function () {
    state.background = el.bg.value;
    sync();
    draw();
  });

  [el.text, el.color, el.size, el.bold].forEach(function (input) {
    input.addEventListener("input", function () {
      const l = selectedLayer();
      if (!l) return;
      l.text = el.text.value;
      l.color = el.color.value;
      l.fontSize = clamp(parseInt(el.size.value, 10) || 100, 8, 400);
      l.bold = el.bold.checked;
      sync();
      draw();
    });
  });

  el.add.addEventListener("click", function () {
    state.layers.push({ type: "text", text: "New text", x: 100, y: 100, fontSize: 100, color: "#ffffff", bold: false });
    selected = state.layers.length - 1;
    sync();
    refreshPanel();
    draw();
  });

  el.del.addEventListener("click", function () {
    if (selected === null) return;
    state.layers.splice(selected, 1);
    selected = null;
    sync();
    refreshPanel();
    draw();
  });

  el.guides.addEventListener("change", function () {
    guidesOn = el.guides.checked;
    draw();
  });

  function refreshPanel() {
    const l = selectedLayer();
    const has = !!l;
    [el.text, el.color, el.size, el.bold, el.del].forEach(function (i) {
      i.disabled = !has;
    });
    if (has) {
      el.text.value = l.text || "";
      el.color.value = l.color || "#ffffff";
      el.size.value = l.fontSize || 100;
      el.bold.checked = !!l.bold;
    }
    el.bg.value = state.background;
  }

  // --- Init: load matching fonts, then draw ---
  refreshPanel();
  sync();
  draw();
  if (document.fonts && document.fonts.load) {
    Promise.all([
      document.fonts.load("100px 'PBC Go'"),
      document.fonts.load("bold 100px 'PBC Go'"),
    ]).then(draw).catch(function () {});
    document.fonts.ready.then(draw);
  }
})();
