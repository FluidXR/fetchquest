// FetchQuest Desktop — shared utilities and router
var FQ = (function () {
  'use strict';

  function backend() {
    if (window.go && window.go.main) return window.go.main.App || window.go.main.app;
    return null;
  }

  function escapeHtml(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  function formatSize(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    var units = ['B', 'KB', 'MB', 'GB'];
    var i = 0;
    var v = bytes;
    while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
    return (i === 0 ? v : v.toFixed(1)) + ' ' + units[i];
  }

  function formatDate(epoch) {
    if (!epoch) return '';
    var d = new Date(epoch * 1000);
    var now = new Date();
    var months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
    if (d.getFullYear() === now.getFullYear()) {
      return months[d.getMonth()] + ' ' + d.getDate();
    }
    return d.getFullYear() + '-' + String(d.getMonth()+1).padStart(2,'0') + '-' + String(d.getDate()).padStart(2,'0');
  }

  // ── Router ──

  var views = {};
  var currentView = 'sync';

  function initRouter() {
    document.querySelectorAll('.view').forEach(function (el) {
      var id = el.id.replace('view-', '');
      views[id] = el;
    });
    document.querySelectorAll('.nav-item[data-view]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        showView(btn.dataset.view);
      });
    });
  }

  function showView(name) {
    if (!views[name]) return;
    currentView = name;
    Object.keys(views).forEach(function (k) {
      views[k].hidden = (k !== name);
    });
    document.querySelectorAll('.nav-item').forEach(function (el) {
      el.classList.toggle('active', el.dataset.view === name);
    });
    // Notify view that it became active
    if (viewCallbacks[name]) viewCallbacks[name]();
  }

  var viewCallbacks = {};
  function onViewActive(name, fn) { viewCallbacks[name] = fn; }

  // ── Status line ──

  var statusEl;
  function setStatus(text, type) {
    if (!statusEl) statusEl = document.getElementById('sync-status');
    if (!statusEl) return;
    statusEl.textContent = text || '';
    statusEl.className = 'sync-status' + (type ? ' ' + type : '');
  }

  // ── Device state (shared) ──

  var deviceState = { devices: [], hasOnline: false };

  function refreshDevices() {
    var b = backend();
    if (!b) return Promise.resolve();
    return b.GetDevices().then(function (devs) {
      deviceState.devices = devs || [];
      deviceState.hasOnline = devs && devs.some(function (d) { return d.status === 'device'; });
      // Update sidebar indicator
      var dot = document.querySelector('.device-dot');
      if (dot) {
        dot.className = 'device-dot ' + (deviceState.hasOnline ? 'online' : 'offline');
        dot.parentElement.title = deviceState.hasOnline ? 'Quest connected' : 'No device connected';
      }
      return devs;
    });
  }

  // ── Init ──

  function init() {
    initRouter();
    showView('sync');
  }

  // Wait for DOM
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  return {
    backend: backend,
    escapeHtml: escapeHtml,
    formatSize: formatSize,
    formatDate: formatDate,
    showView: showView,
    onViewActive: onViewActive,
    setStatus: setStatus,
    deviceState: deviceState,
    refreshDevices: refreshDevices
  };
})();
