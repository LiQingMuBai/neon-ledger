const state = {
  limit: 20,
  offset: 0,
  total: 0,
  filters: {},
  items: [],
};

const tableBody = document.querySelector("#ordersTableBody");
const summaryText = document.querySelector("#summaryText");
const notice = document.querySelector("#notice");
const pageText = document.querySelector("#pageText");
const prevPage = document.querySelector("#prevPage");
const nextPage = document.querySelector("#nextPage");
const filterForm = document.querySelector("#filterForm");
const orderForm = document.querySelector("#orderForm");
const orderModal = document.querySelector("#orderModal");
const orderModalTitle = document.querySelector("#orderModalTitle");
const saveOrderButton = document.querySelector("#saveOrderButton");
const orderStatusLabels = {
  pending: "待处理",
  paid: "已支付",
  failed: "失败",
  closed: "已关闭",
};
const notifyStatusLabels = {
  pending: "待通知",
  sent: "已通知",
  failed: "通知失败",
};

document.querySelector("#refreshButton").addEventListener("click", () => loadOrders());
document.querySelector("#openCreateOrder").addEventListener("click", () => openOrderModal());
document.querySelector("#closeOrderModal").addEventListener("click", () => closeOrderModal());
document.querySelector("#cancelOrderModal").addEventListener("click", () => closeOrderModal());
document.querySelector("#resetFilters").addEventListener("click", () => {
  filterForm.reset();
  state.filters = {};
  state.offset = 0;
  loadOrders();
});

filterForm.addEventListener("submit", (event) => {
  event.preventDefault();
  state.filters = readFormValues(filterForm);
  state.offset = 0;
  loadOrders();
});

orderModal.addEventListener("click", (event) => {
  if (event.target === orderModal) {
    closeOrderModal();
  }
});

tableBody.addEventListener("click", async (event) => {
  const button = event.target.closest("button[data-action]");
  if (!button) {
    return;
  }

  const order = state.items.find((item) => item.id === button.dataset.id);
  if (!order) {
    return;
  }

  if (button.dataset.action === "edit") {
    openOrderModal(order);
    return;
  }

  if (button.dataset.action === "delete") {
    await deleteOrder(order);
  }
});

orderForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const formValues = readFormValues(orderForm);
  const id = formValues.id || "";
  delete formValues.id;

  const payload = formValues;
  payload.telegram_user_id = payload.telegram_user_id === undefined ? 0 : Number(payload.telegram_user_id);
  payload.amount = Number(payload.amount);

  setNotice("");
  setFormDisabled(orderForm, true);
  try {
    const response = await apiAuth.fetchJSON(id ? `/api/v1/orders/${encodeURIComponent(id)}` : "/api/v1/orders", {
      method: id ? "PUT" : "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "保存失败");
    }
    closeOrderModal();
    state.offset = 0;
    setNotice(`订单 ${data.platform_order_no} 已保存`);
    await loadOrders();
  } catch (error) {
    setNotice(error.message, true);
  } finally {
    setFormDisabled(orderForm, false);
  }
});

prevPage.addEventListener("click", () => {
  if (state.offset === 0) {
    return;
  }
  state.offset = Math.max(0, state.offset - state.limit);
  loadOrders();
});

nextPage.addEventListener("click", () => {
  if (state.offset + state.limit >= state.total) {
    return;
  }
  state.offset += state.limit;
  loadOrders();
});

loadOrders();

async function loadOrders() {
  tableBody.innerHTML = `<tr><td colspan="9" class="empty-cell">加载中</td></tr>`;
  setNotice("");

  const params = new URLSearchParams({
    limit: String(state.limit),
    offset: String(state.offset),
  });

  for (const [key, value] of Object.entries(state.filters)) {
    if (value !== "") {
      params.set(key, value);
    }
  }

  try {
    const response = await apiAuth.fetchJSON(`/api/v1/orders?${params.toString()}`);
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "查询失败");
    }
    state.total = data.total;
    state.items = data.items || [];
    renderOrders(data.items);
    renderPagination();
  } catch (error) {
    tableBody.innerHTML = `<tr><td colspan="9" class="empty-cell">加载失败</td></tr>`;
    setNotice(error.message, true);
  }
}

function renderOrders(items) {
  summaryText.textContent = `${state.total} 条记录`;
  if (!items.length) {
    tableBody.innerHTML = `<tr><td colspan="9" class="empty-cell">暂无订单</td></tr>`;
    return;
  }

  tableBody.innerHTML = items
    .map(
      (order) => `
        <tr>
          <td title="${escapeHtml(order.customer_order_no)}">${escapeHtml(order.customer_order_no)}</td>
          <td title="${escapeHtml(order.platform_order_no)}">${escapeHtml(order.platform_order_no)}</td>
          <td>${escapeHtml(String(order.telegram_user_id))}</td>
          <td>${formatAmount(order.amount)}</td>
          <td>${escapeHtml(order.phone)}</td>
          <td>${renderStatus("status", order.status)}</td>
          <td>${renderStatus("notify", order.notify_status)}</td>
          <td>${formatDate(order.created_at)}</td>
          <td>
            <div class="row-actions">
              <button type="button" data-action="edit" data-id="${escapeHtml(order.id)}">修改</button>
              <button class="danger-button" type="button" data-action="delete" data-id="${escapeHtml(order.id)}">删除</button>
            </div>
          </td>
        </tr>
      `,
    )
    .join("");
}

function renderPagination() {
  const page = Math.floor(state.offset / state.limit) + 1;
  const pages = Math.max(1, Math.ceil(state.total / state.limit));
  pageText.textContent = `第 ${page} / ${pages} 页`;
  prevPage.disabled = state.offset === 0;
  nextPage.disabled = state.offset + state.limit >= state.total;
}

function renderStatus(kind, value) {
  const rawValue = value || "-";
  const label = kind === "status" ? orderStatusLabels[rawValue] : notifyStatusLabels[rawValue];
  const safeValue = escapeHtml(rawValue);
  const safeLabel = escapeHtml(label || rawValue);
  return `<span class="status-pill ${kind}-${safeValue}">${safeLabel}</span>`;
}

function openOrderModal(order = null) {
  orderForm.reset();
  orderForm.elements.id.value = order?.id || "";
  orderModalTitle.textContent = order ? "修改订单" : "新增订单";
  saveOrderButton.textContent = order ? "保存修改" : "创建订单";

  if (order) {
    orderForm.elements.customer_order_no.value = order.customer_order_no || "";
    orderForm.elements.telegram_user_id.value = order.telegram_user_id || "";
    orderForm.elements.amount.value = order.amount || "";
    orderForm.elements.phone.value = order.phone || "";
    orderForm.elements.status.value = order.status || "pending";
    orderForm.elements.notify_status.value = order.notify_status || "pending";
  }

  orderModal.hidden = false;
  orderForm.elements.customer_order_no.focus();
}

function closeOrderModal() {
  orderModal.hidden = true;
  orderForm.reset();
}

async function deleteOrder(order) {
  const confirmed = window.confirm(`确认删除订单 ${order.platform_order_no}？`);
  if (!confirmed) {
    return;
  }

  setNotice("");
  try {
    const response = await apiAuth.fetchJSON(`/api/v1/orders/${encodeURIComponent(order.id)}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || "删除失败");
    }
    setNotice(`订单 ${order.platform_order_no} 已删除`);
    if (state.items.length === 1 && state.offset > 0) {
      state.offset = Math.max(0, state.offset - state.limit);
    }
    await loadOrders();
  } catch (error) {
    setNotice(error.message, true);
  }
}

function readFormValues(form) {
  const values = {};
  const data = new FormData(form);
  for (const [key, value] of data.entries()) {
    const trimmed = String(value).trim();
    if (trimmed !== "") {
      values[key] = trimmed;
    }
  }
  return values;
}

function setFormDisabled(form, disabled) {
  form.querySelectorAll("input, select, button").forEach((element) => {
    element.disabled = disabled;
  });
}

function setNotice(message, isError = false) {
  notice.textContent = message;
  notice.classList.toggle("is-visible", message !== "");
  notice.classList.toggle("is-error", isError);
}

function formatAmount(value) {
  return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
}

function formatDate(value) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
