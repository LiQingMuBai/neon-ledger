const apiAuth = (() => {
  async function fetchJSON(url, options = {}) {
    const headers = new Headers(options.headers || {});

    const response = await fetch(url, { ...options, headers, credentials: "same-origin" });
    if (response.status === 401) {
      window.location.href = `/login?next=${encodeURIComponent(window.location.pathname + window.location.search)}`;
    }
    return response;
  }

  return { fetchJSON };
})();

document.addEventListener("DOMContentLoaded", async () => {
  try {
    const response = await fetch("/app-config", { credentials: "same-origin" });
    if (!response.ok) {
      return;
    }
    const config = await response.json();
    if (config.api_test_enabled === false) {
      document.querySelectorAll("[data-api-test-link]").forEach((element) => {
        element.hidden = true;
      });
    }
  } catch {}
});
