/* Frozen Fortress — theme boot script.
 *
 * MUST run inline in <head> before any stylesheet is applied, to avoid a
 * flash-of-wrong-theme on page load. Kept tiny so embedding it inline in
 * layout.html stays cheap.
 *
 * NOTE: The actual toggle handler lives in app.js (Alpine store).
 */
(function () {
  try {
    var stored = localStorage.getItem("ff-theme");
    var mode;
    if (stored === "dark" || stored === "light") {
      mode = stored;
    } else if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
      mode = "dark";
    } else {
      mode = "light";
    }
    if (mode === "dark") {
      document.documentElement.classList.add("dark");
    }
    // Live-sync to OS preference changes when the user has not chosen a mode.
    if (!stored && window.matchMedia) {
      var mq = window.matchMedia("(prefers-color-scheme: dark)");
      var listener = function (e) {
        if (localStorage.getItem("ff-theme")) return;
        document.documentElement.classList.toggle("dark", e.matches);
      };
      if (mq.addEventListener) mq.addEventListener("change", listener);
      else if (mq.addListener) mq.addListener(listener);
    }
  } catch (_) { /* ignore — fall back to light */ }
})();
