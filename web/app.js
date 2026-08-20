// Dashboard controller: polls /health and /api/summary and renders KPIs.
(function () {
  "use strict";

  const $ = (id) => document.getElementById(id);

  function setHealth(ok) {
    const dot = $("health-dot");
    const text = $("health-text");
    dot.className = "dot " + (ok ? "dot-ok" : "dot-warn");
    text.textContent = ok ? "服务正常" : "服务异常";
  }

  function fmt(n) {
    if (n === null || n === undefined) return "—";
    if (typeof n === "number") {
      if (Math.abs(n) >= 1e6) return (n / 1e6).toFixed(2) + "M";
      if (Math.abs(n) >= 1e3) return (n / 1e3).toFixed(1) + "k";
      return String(Math.round(n));
    }
    return String(n);
  }

  function setText(id, val) {
    const el = $(id);
    if (el) el.textContent = fmt(val);
  }

  async function checkHealth() {
    try {
      const r = await fetch("/health");
      setHealth(r.ok);
    } catch (e) {
      setHealth(false);
    }
  }

  async function loadSummary() {
    try {
      const r = await fetch("/api/summary");
      if (!r.ok) {
        setHealth(false);
        return;
      }
      const s = await r.json();
      setHealth(true);

      setText("kpi-segments", s.network.segments);
      setText("kpi-stations", s.network.stations);
      setText("kpi-compressors", s.network.compressors);
      setText("kpi-valves", s.network.valves);
      setText("kpi-points", s.network.points);

      setText("kpi-readings", s.scada.readings);
      setText("kpi-alarms", s.scada.alarms);
      setText("kpi-active-alarms", s.scada.active_alarms);

      setText("kpi-meters", s.metering.meters);
      setText("kpi-today-total", s.metering.today_total_std);
      setText("kpi-settlements", s.metering.settlements);

      setText("kpi-incidents", s.open_incidents);
      setText("kpi-permits", s.active_permits);
      setText("kpi-orders", s.active_orders);
      setText("kpi-notifications", s.pending_notifications);

      if (s.site) $("site").textContent = s.site;
      if (s.generated_at) $("generated-at").textContent = s.generated_at;

      $("raw-summary").textContent = JSON.stringify(s, null, 2);
    } catch (e) {
      setHealth(false);
      $("raw-summary").textContent = "加载失败: " + e.message;
    }
  }

  async function refresh() {
    await checkHealth();
    await loadSummary();
  }

  refresh();
  setInterval(refresh, 5000);
})();
