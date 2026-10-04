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

  // htmx errors. By default htmx does not swap 4xx/5xx responses and fails
  // silently, so show a toast instead. The message comes from the status code,
  // never from the response body, and is inserted with textContent.
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

  // Boosted navigation (hx-boost links and forms) should behave like a normal
  // page load, so an HTML error page such as the 404 is swapped in and pushed
  // to history instead of being reported as a failed request.
  document.addEventListener("htmx:beforeSwap", function (event) {
    var detail = event.detail;
    var type = detail.xhr.getResponseHeader("Content-Type") || "";
    if (detail.boosted && detail.isError && type.indexOf("text/html") === 0) {
      detail.shouldSwap = true;
      detail.isError = false;
    }
  });

  document.addEventListener("htmx:responseError", function (event) {
    var status = event.detail.xhr.status;
    toast(
      (status >= 500 ? "Something went wrong on the server" : "Request failed") +
        " (" + status + "). Please try again."
    );
  });

  document.addEventListener("htmx:sendError", function () {
    toast("Could not reach the server. Check your connection and try again.");
  });

  document.addEventListener("htmx:timeout", function () {
    toast("The server took too long to respond. Please try again.");
  });
})();
