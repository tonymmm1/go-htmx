// App-wide behaviour. Loaded without defer from <head> (see layouts.Layout), so
// only the theme line touches the DOM immediately; everything else is an event
// listener on document, which keeps working after hx-boost swaps the body.
(function () {
  "use strict";

  var root = document.documentElement;
  var THEME_KEY = "theme";

  // Theme: with no saved choice, data-theme stays unset and daisyUI follows
  // the OS (dark --prefersdark in styles/input.css).
  function savedTheme() {
    try {
      return localStorage.getItem(THEME_KEY);
    } catch (e) {
      return null; // storage disabled (privacy mode, sandboxed iframe)
    }
  }

  var theme = savedTheme();
  if (theme === "light" || theme === "dark") root.dataset.theme = theme;

  document.addEventListener("click", function (event) {
    if (!event.target.closest("[data-theme-toggle]")) return;
    var dark = root.dataset.theme
      ? root.dataset.theme === "dark"
      : matchMedia("(prefers-color-scheme: dark)").matches;
    var next = dark ? "light" : "dark";
    root.dataset.theme = next;
    try {
      localStorage.setItem(THEME_KEY, next);
    } catch (e) {}
  });

  // htmx errors are reported with a toast. The message comes from the status
  // code, never from the response body, and is inserted with textContent.
  function toast(message) {
    var container = document.getElementById("toasts");
    if (!container) return;
    var alert = document.createElement("div");
    alert.className = "alert alert-error";
    alert.setAttribute("role", "alert");
    var text = document.createElement("span");
    text.textContent = message;
    var close = document.createElement("button");
    close.type = "button";
    close.className = "btn btn-ghost btn-xs";
    close.setAttribute("aria-label", "Dismiss");
    close.textContent = "✕";
    close.addEventListener("click", function () {
      alert.remove();
    });
    alert.append(text, close);
    container.append(alert);
    setTimeout(function () {
      alert.remove();
    }, 6000);
  }

  // handlesStatus reports whether elt declares hx-status for this status code
  // (exact, 50x or 5xx), in which case it decides how the response is swapped.
  function handlesStatus(elt, status) {
    var code = String(status);
    var patterns = [code, code.slice(0, 2) + "x", code[0] + "xx"];
    return !!elt && patterns.some(function (pattern) {
      return elt.hasAttribute("hx-status:" + pattern);
    });
  }

  // htmx 4 swaps 4xx/5xx responses like any other. Two cases are kept apart:
  //   - Page navigations (boosted links and forms, history restores: htmx
  //     sends HX-Request-Type: full) that receive an HTML error page, such as
  //     the 404 page, swap it in like a normal page load.
  //   - Everything else (fragment requests, or a plain-text error such as the
  //     rate limiter's 429) shows a toast and swaps nothing, so a fragment
  //     target never fills with an error body and the URL is not pushed.
  //     Elements with a matching hx-status attribute handle the response
  //     themselves.
  // This listener runs before htmx applies hx-status and noSwap, and setting
  // ctx.swap to "none" is what noSwap does internally.
  document.addEventListener("htmx:response:error", function (event) {
    var ctx = event.detail.ctx;
    var status = ctx.response.status;
    var type = ctx.response.headers.get("Content-Type") || "";
    var navigation = ctx.request.headers["HX-Request-Type"] === "full";
    if (navigation && type.indexOf("text/html") === 0) return;
    if (handlesStatus(ctx.sourceElement, status)) return;

    ctx.swap = "none";
    ctx.push = false;
    ctx.replace = false;
    toast(
      (status >= 500 ? "Something went wrong on the server" : "Request failed") +
        " (" + status + "). Please try again."
    );
  });

  // Network failures, timeouts (htmx 4 aborts requests after 60s by default)
  // and swap exceptions. Intentional aborts from hx-sync don't fire this.
  document.addEventListener("htmx:error", function (event) {
    var detail = event.detail;
    var error = detail.error;
    var timedOut = error && error.name === "AbortError" &&
      detail.ctx && detail.ctx.request.signal &&
      detail.ctx.request.signal.reason === "timeout";
    if (timedOut) {
      toast("The server took too long to respond. Please try again.");
    } else if (detail.ctx && !detail.ctx.response) {
      // fetch() rejected, so no response arrived: the server can't be reached.
      toast("Could not reach the server. Check your connection and try again.");
    } else {
      toast("Something went wrong. Please try again.");
    }
  });
})();
