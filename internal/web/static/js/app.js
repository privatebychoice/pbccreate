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
