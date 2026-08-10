// pbccreate front-end — self-hosted vanilla JS, no frameworks or CDNs.
"use strict";

// Copy-to-clipboard: a [data-copy="<id>"] button copies that element's value.
document.addEventListener("click", function (e) {
  const btn = e.target.closest("[data-copy]");
  if (!btn) return;
  const el = document.getElementById(btn.getAttribute("data-copy"));
  if (!el) return;

  el.focus();
  el.select();
  const done = function () {
    const prev = btn.textContent;
    btn.textContent = "Copied";
    setTimeout(function () {
      btn.textContent = prev;
    }, 1500);
  };

  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(el.value).then(done, function () {
      if (document.execCommand("copy")) done();
    });
  } else if (document.execCommand("copy")) {
    done();
  }
});

// Confirm-before-submit: a form with [data-confirm="message"] asks first. Keeps
// the warning CSP-safe (no inline onsubmit handlers).
document.addEventListener("submit", function (e) {
  const form = e.target.closest("form[data-confirm]");
  if (!form) return;
  if (!window.confirm(form.getAttribute("data-confirm"))) {
    e.preventDefault();
  }
});

// Content editor tabs: group the [data-tab] sections into an accessible tabbed
// interface so the (long) content detail page is less busy. Progressive
// enhancement — without this script the sections simply render stacked. The
// active tab is remembered across the page's post/redirect reloads.
(function () {
  const TABS = [
    { key: "plan", label: "Plan" },
    { key: "media", label: "Media" },
    { key: "metadata", label: "Metadata" },
    { key: "rights", label: "Rights" },
    { key: "release", label: "Release" },
  ];
  const STORE_KEY = "pbccreate:contentTab";

  function initContentTabs() {
    const sections = Array.prototype.slice.call(
      document.querySelectorAll("section[data-tab]")
    );
    if (sections.length === 0) return;

    const main = sections[0].parentNode;
    // A stable insertion point: the sections get moved into panels below.
    const marker = document.createComment("content-tabs");
    main.insertBefore(marker, sections[0]);

    // Group present sections by tab key, preserving document order.
    const groups = {};
    sections.forEach(function (sec) {
      const key = sec.getAttribute("data-tab");
      (groups[key] = groups[key] || []).push(sec);
    });

    const tablist = document.createElement("div");
    tablist.className = "tabbar";
    tablist.setAttribute("role", "tablist");
    tablist.setAttribute("aria-label", "Content sections");

    const tabs = [];
    const panels = [];

    TABS.forEach(function (t) {
      const secs = groups[t.key];
      if (!secs || secs.length === 0) return; // skip tabs with nothing in them

      const panel = document.createElement("div");
      panel.className = "tabpanel";
      panel.id = "panel-" + t.key;
      panel.setAttribute("role", "tabpanel");
      panel.setAttribute("aria-labelledby", "tab-" + t.key);
      panel.setAttribute("tabindex", "0");
      secs.forEach(function (sec) {
        sec.removeAttribute("data-tab");
        panel.appendChild(sec); // moves the node out of <main> into the panel
      });

      const tab = document.createElement("button");
      tab.type = "button";
      tab.className = "tab";
      tab.id = "tab-" + t.key;
      tab.setAttribute("role", "tab");
      tab.setAttribute("aria-controls", panel.id);
      tab.setAttribute("data-tab-key", t.key);
      tab.textContent = t.label;

      tablist.appendChild(tab);
      tabs.push(tab);
      panels.push(panel);
    });

    if (tabs.length === 0) return;

    main.insertBefore(tablist, marker);
    panels.forEach(function (p) {
      main.insertBefore(p, marker);
    });
    main.removeChild(marker);

    function select(key, moveFocus) {
      tabs.forEach(function (tab) {
        const active = tab.getAttribute("data-tab-key") === key;
        tab.setAttribute("aria-selected", active ? "true" : "false");
        tab.tabIndex = active ? 0 : -1;
        if (active && moveFocus) tab.focus();
      });
      panels.forEach(function (p) {
        p.hidden = p.id !== "panel-" + key;
      });
      try {
        localStorage.setItem(STORE_KEY, key);
      } catch (e) {
        /* storage unavailable — tab state simply is not remembered */
      }
    }

    tabs.forEach(function (tab) {
      tab.addEventListener("click", function () {
        select(tab.getAttribute("data-tab-key"), false);
      });
    });

    // Arrow / Home / End keyboard navigation over the tablist.
    tablist.addEventListener("keydown", function (e) {
      const idx = tabs.indexOf(document.activeElement);
      if (idx === -1) return;
      let next = -1;
      if (e.key === "ArrowRight" || e.key === "ArrowDown") next = (idx + 1) % tabs.length;
      else if (e.key === "ArrowLeft" || e.key === "ArrowUp") next = (idx - 1 + tabs.length) % tabs.length;
      else if (e.key === "Home") next = 0;
      else if (e.key === "End") next = tabs.length - 1;
      if (next === -1) return;
      e.preventDefault();
      select(tabs[next].getAttribute("data-tab-key"), true);
    });

    // Restore the last-used tab (survives post/redirect reloads); fall back to
    // the first tab when the stored one is not present for this item.
    let initial = tabs[0].getAttribute("data-tab-key");
    try {
      const saved = localStorage.getItem(STORE_KEY);
      if (saved && tabs.some(function (t) { return t.getAttribute("data-tab-key") === saved; })) {
        initial = saved;
      }
    } catch (e) {
      /* ignore */
    }
    select(initial, false);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initContentTabs);
  } else {
    initContentTabs();
  }
})();
