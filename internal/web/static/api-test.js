const templates = [
  {
    name: "健康检查",
    method: "GET",
    path: "/healthz",
    body: "",
  },
  {
    name: "创建订单",
    method: "POST",
    path: "/api/v1/orders",
    body: {
      customer_order_no: "C202606210001",
      telegram_user_id: 987654321,
      amount: 3599,
      phone: "+8613800138000",
      status: "pending",
      notify_status: "pending",
    },
  },
  {
    name: "查询订单",
    method: "GET",
    path: "/api/v1/orders?limit=20&offset=0",
    body: "",
  },
  {
    name: "订单号查询",
    method: "GET",
    path: "/api/v1/orders/lookup?customer_order_no=C202606210001",
    body: "",
  },
  {
    name: "修改订单",
    method: "PUT",
    path: "/api/v1/orders/{id}",
    body: {
      customer_order_no: "C202606210001",
      telegram_user_id: 987654321,
      amount: 3999,
      phone: "+8613800138000",
      status: "paid",
      notify_status: "sent",
    },
  },
  {
    name: "删除订单",
    method: "DELETE",
    path: "/api/v1/orders/{id}",
    body: "",
  },
  {
    name: "每日已支付统计",
    method: "GET",
    path: "/api/v1/orders/daily_totals",
    body: "",
  },
  {
    name: "每日状态统计",
    method: "GET",
    path: "/api/v1/orders/daily_status_totals",
    body: "",
  },
];

const templateSelect = document.querySelector("#apiTemplate");
const form = document.querySelector("#apiTestForm");
const responseMeta = document.querySelector("#responseMeta");
const responseBody = document.querySelector("#responseBody");
const apiKeyStorageKey = "bookkeeping_api_test_key";

templateSelect.innerHTML = templates
  .map((template, index) => `<option value="${index}">${template.name}</option>`)
  .join("");

templateSelect.addEventListener("change", () => applyTemplate(Number(templateSelect.value)));
form.addEventListener("submit", sendRequest);
form.elements.api_key.value = window.sessionStorage.getItem(apiKeyStorageKey) || "";

applyTemplate(0);

function applyTemplate(index) {
  const template = templates[index];
  form.elements.method.value = template.method;
  form.elements.path.value = template.path;
  form.elements.body.value = typeof template.body === "string" ? template.body : JSON.stringify(template.body, null, 2);
}

async function sendRequest(event) {
  event.preventDefault();

  const method = form.elements.method.value;
  const path = form.elements.path.value.trim();
  const rawBody = form.elements.body.value.trim();
  const apiKey = form.elements.api_key.value.trim();
  const options = { method, headers: {} };
  if (apiKey) {
    options.headers["X-API-Key"] = apiKey;
    window.sessionStorage.setItem(apiKeyStorageKey, apiKey);
  } else {
    window.sessionStorage.removeItem(apiKeyStorageKey);
  }

  if (rawBody && method !== "GET" && method !== "DELETE") {
    try {
      JSON.parse(rawBody);
    } catch (error) {
      setResponse("请求体不是合法 JSON", rawBody);
      return;
    }
    options.headers["Content-Type"] = "application/json";
    options.body = rawBody;
  }

  responseMeta.textContent = "请求中";
  responseBody.textContent = "";
  const startedAt = performance.now();

  try {
    const response = path.startsWith("/api/")
      ? await apiAuth.fetchJSON(path, options)
      : await fetch(path, { ...options, credentials: "same-origin" });
    const elapsed = Math.round(performance.now() - startedAt);
    const text = await response.text();
    setResponse(`${response.status} ${response.statusText} · ${elapsed}ms`, formatBody(text));
  } catch (error) {
    setResponse("请求失败", error.message);
  }
}

function setResponse(meta, body) {
  responseMeta.textContent = meta;
  responseBody.textContent = body || "";
}

function formatBody(text) {
  if (!text) {
    return "";
  }
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}
