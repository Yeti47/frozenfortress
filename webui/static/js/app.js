/* Frozen Fortress — global client helpers.
 * Loaded by views/common/layout.html on every page. Functions are attached
 * to window so inline Alpine expressions and event handlers can use them.
 */
(function () {
  "use strict";

  // ---------------------------------------------------------------------------
  // Date / time / size formatting
  // ---------------------------------------------------------------------------

  function normalizeTimestamp(ts) {
    if (!ts) return null;
    var s = String(ts).trim();
    if (!s) return null;
    // Treat plain "YYYY-MM-DD HH:MM:SS" as UTC.
    if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(s)) {
      s = s.replace(" ", "T") + "Z";
    } else if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(s)) {
      s = s + "Z";
    }
    var d = new Date(s);
    return isNaN(d.getTime()) ? null : d;
  }

  window.formatTimestamp = function (ts) {
    var d = normalizeTimestamp(ts);
    if (!d) return ts || "";
    return new Intl.DateTimeFormat(undefined, {
      year: "numeric", month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit", hour12: false,
    }).format(d);
  };

  window.formatDate = function (ts) {
    var d = normalizeTimestamp(ts);
    if (!d) return ts || "";
    return new Intl.DateTimeFormat(undefined, {
      year: "numeric", month: "2-digit", day: "2-digit",
    }).format(d);
  };

  window.formatFileSize = function (bytes) {
    var n = Number(bytes);
    if (!isFinite(n) || n < 0) return "—";
    if (n < 1024) return n + " B";
    var units = ["KB", "MB", "GB", "TB"];
    var u = -1;
    do { n /= 1024; u++; } while (n >= 1024 && u < units.length - 1);
    return n.toFixed(n >= 10 ? 0 : 1) + " " + units[u];
  };

  // Auto-format any element with [data-ts] / [data-date] on page load.
  function autoFormat() {
    document.querySelectorAll("[data-ts]").forEach(function (el) {
      var formatted = window.formatTimestamp(el.getAttribute("data-ts"));
      if (formatted) el.textContent = formatted;
    });
    document.querySelectorAll("[data-date]").forEach(function (el) {
      var formatted = window.formatDate(el.getAttribute("data-date"));
      if (formatted) el.textContent = formatted;
    });
    document.querySelectorAll("[data-size]").forEach(function (el) {
      var formatted = window.formatFileSize(el.getAttribute("data-size"));
      if (formatted) el.textContent = formatted;
    });
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", autoFormat);
  } else {
    autoFormat();
  }

  // ---------------------------------------------------------------------------
  // Sign-out — clear browser history then GET /logout
  // ---------------------------------------------------------------------------

  window.signOut = function (event) {
    if (event) event.preventDefault();
    try { history.replaceState(null, "", "/login"); } catch (_) {}
    window.location.replace("/logout");
  };

  // ---------------------------------------------------------------------------
  // Toast notification system
  // ---------------------------------------------------------------------------

  (function setupToasts() {
    // Create the fixed toast container once.
    var container = document.createElement("div");
    container.id = "ff-toast-container";
    container.setAttribute("aria-live", "polite");
    container.setAttribute("aria-atomic", "false");
    container.style.cssText = [
      "position:fixed",
      "bottom:1.25rem",
      "right:1.25rem",
      "z-index:9999",
      "display:flex",
      "flex-direction:column",
      "align-items:flex-end",
      "gap:0.5rem",
      "pointer-events:none",
      "max-width:min(22rem,calc(100vw - 2rem))",
    ].join(";");

    function mount() { document.body.appendChild(container); }
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", mount);
    } else {
      mount();
    }

    // Icon SVG paths keyed by type.
    var icons = {
      success: '<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>',
      error:   '<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>',
      warning: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
      info:    '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>',
    };
    var colors = {
      success: "var(--color-success-500,#22c55e)",
      error:   "var(--color-danger-500,#ef4444)",
      warning: "var(--color-warning-500,#f59e0b)",
      info:    "var(--color-brand-500,#3b82f6)",
    };

    window.ffToast = function (message, type, durationMs) {
      type = type || "info";
      durationMs = durationMs !== undefined ? durationMs : 4000;

      var toast = document.createElement("div");
      toast.style.cssText = [
        "pointer-events:auto",
        "display:flex",
        "align-items:flex-start",
        "gap:0.5rem",
        "padding:0.6rem 0.85rem",
        "border-radius:0.5rem",
        "border:1px solid color-mix(in srgb," + colors[type] + " 30%,transparent)",
        "background:var(--color-surface,#1e293b)",
        "box-shadow:0 4px 16px rgba(0,0,0,.25)",
        "font-size:0.85rem",
        "color:var(--color-text,#f8fafc)",
        "max-width:100%",
        "opacity:0",
        "transform:translateY(6px)",
        "transition:opacity .22s ease,transform .22s ease",
      ].join(";");

      var iconSvg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="' +
        colors[type] + '" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" ' +
        'style="width:1rem;height:1rem;flex-shrink:0;margin-top:1px">' +
        (icons[type] || "") + "</svg>";

      var msgEl = document.createElement("span");
      msgEl.style.cssText = "flex:1;min-width:0;word-break:break-word";
      msgEl.textContent = message;

      var dismissBtn = document.createElement("button");
      dismissBtn.type = "button";
      dismissBtn.setAttribute("aria-label", "Dismiss");
      dismissBtn.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width:0.85rem;height:0.85rem"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';
      dismissBtn.style.cssText = [
        "flex-shrink:0",
        "display:inline-flex",
        "align-items:center",
        "justify-content:center",
        "margin-top:1px",
        "padding:2px",
        "border:none",
        "background:transparent",
        "color:var(--color-text-subtle,#94a3b8)",
        "cursor:pointer",
        "border-radius:3px",
        "line-height:1",
        "opacity:0.7",
      ].join(";");
      dismissBtn.onmouseover = function () { dismissBtn.style.opacity = "1"; };
      dismissBtn.onmouseout  = function () { dismissBtn.style.opacity = "0.7"; };

      toast.innerHTML = iconSvg;
      toast.appendChild(msgEl);
      toast.appendChild(dismissBtn);

      container.appendChild(toast);

      function dismiss() {
        clearTimeout(autoTimer);
        toast.style.opacity = "0";
        toast.style.transform = "translateY(4px)";
        setTimeout(function () { if (toast.parentNode) toast.parentNode.removeChild(toast); }, 260);
      }

      dismissBtn.addEventListener("click", dismiss);

      // Animate in.
      requestAnimationFrame(function () {
        requestAnimationFrame(function () {
          toast.style.opacity = "1";
          toast.style.transform = "translateY(0)";
        });
      });

      // Auto-dismiss.
      var autoTimer = null;
      if (durationMs > 0) {
        autoTimer = setTimeout(dismiss, durationMs);
      }

      return toast;
    };
  })();

  // ---------------------------------------------------------------------------
  // Copy-to-clipboard helper
  // ---------------------------------------------------------------------------

  window.copyToClipboard = async function (text, label) {
    if (!text) return false;
    var ok = false;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        ok = true;
      } else {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "");
        ta.style.position = "absolute";
        ta.style.left = "-9999px";
        document.body.appendChild(ta);
        ta.select();
        ok = document.execCommand("copy");
        document.body.removeChild(ta);
      }
    } catch (_) {
      ok = false;
    }
    if (ok) {
      var msg = label ? label + " copied to clipboard" : "Copied to clipboard";
      window.ffToast(msg, "success", 2500);
    } else {
      window.ffToast("Failed to copy to clipboard", "error", 3500);
    }
    return ok;
  };

  // ---------------------------------------------------------------------------
  // Convert server-rendered flash messages to toasts on page load
  // ---------------------------------------------------------------------------

  function convertFlashToToasts() {
    var typeMap = {
      "ff-flash-success": "success",
      "ff-flash-error":   "error",
      "ff-flash-warning": "warning",
      "ff-flash-info":    "info",
    };
    document.querySelectorAll(".ff-flash").forEach(function (el) {
      var toastType = "info";
      Object.keys(typeMap).forEach(function (cls) {
        if (el.classList.contains(cls)) toastType = typeMap[cls];
      });
      var textEl = el.querySelector("span.flex-1") || el;
      var msg = textEl.textContent.trim();
      if (msg) {
        var duration = el.hasAttribute("data-persist") ? 0 : 6000;
        window.ffToast(msg, toastType, duration);
      }
      el.remove();
    });
    // Also remove the wrapper div if it's now empty.
    document.querySelectorAll(".ff-flash").forEach(function (el) { el.remove(); });
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", convertFlashToToasts);
  } else {
    convertFlashToToasts();
  }

  // ---------------------------------------------------------------------------
  // Alpine.js global store: theme + UI flags
  // Registered before Alpine boots via the 'alpine:init' event.
  // ---------------------------------------------------------------------------

  document.addEventListener("alpine:init", function () {
    /* global Alpine */
    Alpine.store("theme", {
      // Initialize from <html class="dark"> already set by theme.js boot script.
      mode: document.documentElement.classList.contains("dark") ? "dark" : "light",
      toggle: function () {
        this.mode = this.mode === "dark" ? "light" : "dark";
        document.documentElement.classList.toggle("dark", this.mode === "dark");
        try { localStorage.setItem("ff-theme", this.mode); } catch (_) {}
      },
      isDark: function () { return this.mode === "dark"; },
    });
  });
})();
