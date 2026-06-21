const totalAmount = document.querySelector("#totalAmount");
const totalOrders = document.querySelector("#totalOrders");
const averageAmount = document.querySelector("#averageAmount");
const chartSummary = document.querySelector("#chartSummary");
const chart = document.querySelector("#lineChart");
const notice = document.querySelector("#notice");
const filterForm = document.querySelector("#statsFilterForm");
const orderStatuses = ["pending", "paid", "failed", "closed"];
const statusColors = {
  pending: "#9a6700",
  paid: "#137333",
  failed: "#b42318",
  closed: "#475467",
};
const statusLabels = {
  pending: "待处理",
  paid: "已支付",
  failed: "失败",
  closed: "已关闭",
};

document.querySelector("#refreshButton").addEventListener("click", () => loadStats());

filterForm.addEventListener("submit", (event) => {
  event.preventDefault();
  loadStats();
});

loadStats();

async function loadStats() {
  setNotice("");
  chart.innerHTML = `<div class="chart-empty">加载中</div>`;

  const filters = readFormValues(filterForm);
  const days = Math.max(1, Math.min(90, Number(filters.days || 30)));
  const end = new Date();
  const start = new Date(end);
  start.setDate(end.getDate() - days + 1);
  start.setHours(0, 0, 0, 0);

  const params = new URLSearchParams({
    start_time: start.toISOString(),
    end_time: end.toISOString(),
  });
  if (filters.notify_status) {
    params.set("notify_status", filters.notify_status);
  }

  try {
    const response = await apiAuth.fetchJSON(`/api/v1/orders/daily_status_totals?${params.toString()}`);
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "统计加载失败");
    }
    const series = fillDateSeries(start, days, data.items || []);
    renderSummary(series, days);
    renderLineChart(series);
  } catch (error) {
    chart.innerHTML = `<div class="chart-empty">加载失败</div>`;
    setNotice(error.message, true);
  }
}

function renderSummary(series, days) {
  const amount = series.reduce(
    (sum, item) => sum + orderStatuses.reduce((statusSum, status) => statusSum + item.statuses[status].total_amount, 0),
    0,
  );
  const count = series.reduce(
    (sum, item) => sum + orderStatuses.reduce((statusSum, status) => statusSum + item.statuses[status].order_count, 0),
    0,
  );
  totalAmount.textContent = formatAmount(amount);
  totalOrders.textContent = formatAmount(count);
  averageAmount.textContent = formatAmount(Math.round(amount / Math.max(1, days)));
  chartSummary.textContent = `最近 ${days} 天`;
}

function renderLineChart(series) {
  const max = Math.max(
    1,
    ...series.flatMap((item) => orderStatuses.map((status) => item.statuses[status].total_amount)),
  );
  const width = Math.max(900, series.length * 44);
  const height = 340;
  const padding = { top: 28, right: 28, bottom: 44, left: 54 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;
  const pointGap = series.length > 1 ? chartWidth / (series.length - 1) : 0;

  const pointsByStatus = Object.fromEntries(
    orderStatuses.map((status) => [
      status,
      series.map((item, index) => {
        const amount = item.statuses[status].total_amount;
        const x = padding.left + index * pointGap;
        const y = padding.top + chartHeight - (amount / max) * chartHeight;
        return {
          date: item.date,
          status,
          total_amount: amount,
          order_count: item.statuses[status].order_count,
          x,
          y,
        };
      }),
    ]),
  );
  const labelEvery = Math.max(1, Math.ceil(series.length / 8));
  const gridLines = [0, 0.25, 0.5, 0.75, 1]
    .map((ratio) => {
      const y = padding.top + chartHeight - ratio * chartHeight;
      const value = Math.round(max * ratio);
      return `
        <line class="line-grid" x1="${padding.left}" y1="${y}" x2="${padding.left + chartWidth}" y2="${y}"></line>
        <text class="line-axis" x="${padding.left - 10}" y="${y + 4}" text-anchor="end">${formatCompact(value)}</text>
      `;
    })
    .join("");

  chart.innerHTML = `
    <div class="line-legend">
      ${orderStatuses
        .map(
          (status) => `
            <span>
              <i style="background: ${statusColors[status]}"></i>
              ${statusLabels[status]}
            </span>
          `,
        )
        .join("")}
    </div>
    <svg class="line-svg" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" aria-hidden="true">
      ${gridLines}
      ${orderStatuses
        .map((status) => {
          const points = pointsByStatus[status];
          const linePoints = points.map((point) => `${point.x},${point.y}`).join(" ");
          return `
            <polyline class="line-stroke" style="stroke: ${statusColors[status]}" points="${linePoints}"></polyline>
            ${points
              .map(
                (point) => `
                  <circle class="line-point" style="stroke: ${statusColors[status]}" cx="${point.x}" cy="${point.y}" r="3.5">
                    <title>${point.date} ${statusLabels[status]}: ${formatAmount(point.total_amount)} (${point.order_count} 单)</title>
                  </circle>
                `,
              )
              .join("")}
          `;
        })
        .join("")}
      ${series
        .map((item, index) =>
          index % labelEvery === 0 || index === series.length - 1
            ? `<text class="line-label" x="${padding.left + index * pointGap}" y="${height - 14}" text-anchor="middle">${formatDay(item.date)}</text>`
            : "",
        )
        .join("")}
    </svg>
  `;
}

function fillDateSeries(start, days, items) {
  const byDateStatus = new Map(items.map((item) => [`${item.date}|${item.status}`, item]));
  const series = [];
  for (let i = 0; i < days; i++) {
    const date = new Date(start);
    date.setDate(start.getDate() + i);
    const key = toDateKey(date);
    const statuses = Object.fromEntries(
      orderStatuses.map((status) => {
        const item = byDateStatus.get(`${key}|${status}`);
        return [
          status,
          {
            total_amount: Number(item?.total_amount || 0),
            order_count: Number(item?.order_count || 0),
          },
        ];
      }),
    );
    series.push({
      date: key,
      statuses,
    });
  }
  return series;
}

function readFormValues(form) {
  const values = {};
  const data = new FormData(form);
  for (const [key, value] of data.entries()) {
    values[key] = String(value).trim();
  }
  return values;
}

function setNotice(message, isError = false) {
  notice.textContent = message;
  notice.classList.toggle("is-visible", message !== "");
  notice.classList.toggle("is-error", isError);
}

function toDateKey(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatDay(date) {
  return date.slice(5).replace("-", "/");
}

function formatAmount(value) {
  return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
}

function formatCompact(value) {
  return new Intl.NumberFormat("zh-CN", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(Number(value || 0));
}
