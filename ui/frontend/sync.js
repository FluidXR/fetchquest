// FetchQuest — Sync view
(function () {
  'use strict';

  var deviceCard = document.getElementById('devices-list');
  var refreshBtn = document.getElementById('refresh-btn');
  var syncBtn = document.getElementById('sync-btn');
  var previewBtn = document.getElementById('preview-btn');
  var syncSummary = document.getElementById('sync-summary');
  var syncSummaryContent = document.getElementById('sync-summary-content');
  var syncProgress = document.getElementById('sync-progress');
  var syncResults = document.getElementById('sync-results');
  var syncResultsContent = document.getElementById('sync-results-content');
  var missingDeps = document.getElementById('missing-deps');
  var cleanBtn = document.getElementById('clean-btn');

  function setActions(disabled) {
    syncBtn.disabled = disabled;
    previewBtn.disabled = disabled;
  }

  var configCache = null;

  function renderDevices(devices) {
    if (!devices || devices.length === 0) {
      deviceCard.innerHTML = '<p class="no-devices">No adventurer has entered the tavern... Connect your Quest via USB.</p>';
      setActions(true);
      return;
    }
    var online = devices.filter(function (d) { return d.status === 'device'; });
    if (online.length === 0) {
      deviceCard.innerHTML = '<p class="no-devices">Headset detected but not ready. Check the headset and allow USB debugging if prompted.</p>';
      setActions(true);
      return;
    }
    deviceCard.innerHTML = devices.map(function (d) {
      var name = d.nickname || d.model || d.serial;
      var meta = d.serial;
      var stats = d.stats ? '<p class="device-stats">' + FQ.escapeHtml(d.stats) + '</p>' : '';
      return '<div class="device"><p class="device-name">' + FQ.escapeHtml(name) + '</p>' +
        '<p class="device-meta">' + FQ.escapeHtml(meta) + '</p>' + stats + '</div>';
    }).join('');

    // Enable buttons only if destinations exist
    var hasDests = configCache && configCache.hasDestinations;
    setActions(!hasDests);

    if (!hasDests && online.length > 0) {
      syncSummary.hidden = false;
      syncSummaryContent.innerHTML = '<p class="text-muted">Your quest has no destination. Open a portal to begin. ' +
        '<a href="#" id="go-to-dests" style="color:var(--accent)">Go to Places</a></p>';
      var link = document.getElementById('go-to-dests');
      if (link) link.addEventListener('click', function (e) { e.preventDefault(); FQ.showView('destinations'); });
    }
  }

  function loadConfig() {
    var b = FQ.backend();
    if (!b) return;
    b.GetConfig().then(function (cfg) {
      configCache = cfg;
      // Sync dir (for settings view too)
      var dirInput = document.getElementById('sync-dir-input');
      if (dirInput) dirInput.value = cfg.syncDir || '';
      // Missing deps
      if (cfg.missingDeps && cfg.missingDeps.length > 0) {
        missingDeps.hidden = false;
        missingDeps.innerHTML = '<div class="card-label">Missing required tools</div>' +
          cfg.missingDeps.map(function (dep) {
            return '<p><strong>' + FQ.escapeHtml(dep.name) + '</strong><br><code>' + FQ.escapeHtml(dep.install) + '</code></p>';
          }).join('');
      } else {
        missingDeps.hidden = true;
      }
      // Re-check button state
      var hasDests = cfg.hasDestinations;
      if (FQ.deviceState.hasOnline) {
        setActions(!hasDests);
      }
    });
  }

  function loadAll() {
    FQ.refreshDevices().then(function (devs) {
      renderDevices(devs);
    });
    loadConfig();
  }

  // ── Sync (current blocking approach — will be replaced by SyncAsync in Phase 2) ──

  var phaseLabel = document.getElementById('sync-phase-label');
  var progressFill = document.getElementById('sync-progress-fill');
  var progressCount = document.getElementById('sync-progress-count');
  var progressFile = document.getElementById('sync-progress-file');

  var dotCount = 0;
  var dotTimer = null;

  function startDots() {
    stopDots();
    dotCount = 0;
    dotTimer = setInterval(function () {
      dotCount = (dotCount + 1) % 4;
      var base = phaseLabel.textContent.replace(/\.+$/, '');
      phaseLabel.textContent = base + '.'.repeat(dotCount);
    }, 400);
  }

  function stopDots() {
    if (dotTimer) { clearInterval(dotTimer); dotTimer = null; }
  }

  function onSyncProgress(data) {
    var phases = { scan: 'Scanning device', pull: 'Pulling from Quest', push: 'Pushing to destinations' };
    var base = phases[data.phase] || 'Syncing';
    phaseLabel.textContent = base + '...';
    startDots();
    var fileText = data.file || '';
    if (fileText && data.filePercent > 0) {
      fileText += '  ' + data.filePercent + '%';
    }
    progressFile.textContent = fileText;
    if (data.total > 0) {
      var pct = Math.round((data.current / data.total) * 100);
      progressFill.style.width = pct + '%';
      progressCount.textContent = data.current + ' / ' + data.total;
    } else {
      progressFill.style.width = '0%';
      progressCount.textContent = '';
    }
  }

  function runSync() {
    var b = FQ.backend();
    if (!b) return;

    setActions(true);
    syncSummary.hidden = true;
    syncResults.hidden = true;
    syncProgress.hidden = false;

    phaseLabel.textContent = 'Scanning devices...';
    progressFill.style.width = '0%';
    progressCount.textContent = '';
    progressFile.textContent = '';

    window.runtime.EventsOn('sync:progress', onSyncProgress);

    b.Sync()
      .then(function (msg) {
        window.runtime.EventsOff('sync:progress');
        stopDots();
        syncProgress.hidden = true;
        syncResults.hidden = false;
        syncResultsContent.innerHTML = '<p class="text-success" style="font-weight:600">Quest Complete!</p>' +
          '<p class="text-muted text-sm">' + FQ.escapeHtml(msg) + '</p>' +
          '<div style="margin-top:10px;display:flex;gap:8px">' +
          '<button class="btn-text" id="sync-open-folder">View Loot</button>' +
          '<button class="btn-text" id="sync-again">Another Quest!</button></div>';
        var openBtn = document.getElementById('sync-open-folder');
        if (openBtn) openBtn.addEventListener('click', function () {
          if (b.OpenSyncFolder) b.OpenSyncFolder();
        });
        var againBtn = document.getElementById('sync-again');
        if (againBtn) againBtn.addEventListener('click', function () {
          syncResults.hidden = true;
          setActions(false);
        });
        FQ.setStatus('', '');
        loadAll();
      })
      .catch(function (err) {
        window.runtime.EventsOff('sync:progress');
        stopDots();
        syncProgress.hidden = true;
        FQ.setStatus(err.message || String(err), 'error');
        setActions(false);
      });
  }

  // ── Preview ──

  previewBtn.addEventListener('click', function () {
    var b = FQ.backend();
    if (!b || !b.PreviewSync) return;

    setActions(true);
    syncResults.hidden = true;
    syncSummary.hidden = false;
    syncSummaryContent.innerHTML = '<p class="text-muted">Scouting ahead...</p>';

    b.PreviewSync()
      .then(function (preview) {
        var lines = [];

        if (preview.devices && preview.devices.length > 0) {
          preview.devices.forEach(function (d) {
            lines.push('<p>' + FQ.escapeHtml(d.label) + ': <strong>' + d.newFiles + '</strong> new file' + (d.newFiles !== 1 ? 's' : '') + ' to pull</p>');
          });
        } else {
          lines.push('<p class="text-muted">No devices connected.</p>');
        }

        if (preview.destinations && preview.destinations.length > 0) {
          preview.destinations.forEach(function (d) {
            lines.push('<p>' + FQ.escapeHtml(d.label) + ': <strong>' + d.pending + '</strong> file' + (d.pending !== 1 ? 's' : '') + ' to push</p>');
          });
        }

        if (preview.totalNew === 0 && preview.totalPending === 0) {
          lines = ['<p class="text-success">All caught up! Nothing to sync.</p>'];
        }

        syncSummary.hidden = false;
        syncSummaryContent.innerHTML = '<div class="card-label">Scout Report</div>' + lines.join('');
        setActions(false);
      })
      .catch(function (err) {
        syncSummary.hidden = true;
        FQ.setStatus(err.message || String(err), 'error');
        setActions(false);
      });
  });

  // ── Events ──

  refreshBtn.addEventListener('click', function () {
    refreshBtn.disabled = true;
    loadAll();
    setTimeout(function () { refreshBtn.disabled = false; }, 500);
  });

  syncBtn.addEventListener('click', runSync);

  var cleanModal = document.getElementById('clean-modal');
  var cleanModalBody = document.getElementById('clean-modal-body');
  var cleanModalConfirm = document.getElementById('clean-modal-confirm');
  var cleanModalCancel = document.getElementById('clean-modal-cancel');

  cleanBtn.addEventListener('click', function () {
    var b = FQ.backend();
    if (!b || !b.PreviewClean) return;

    cleanBtn.disabled = true;
    b.PreviewClean()
      .then(function (info) {
        cleanBtn.disabled = false;
        if (info.eligible === 0) {
          FQ.setStatus('Nothing to clean — no fully backed-up files on Quest.', '');
          return;
        }
        cleanModalBody.innerHTML =
          '<p><strong>' + info.eligible + '</strong> file' + (info.eligible !== 1 ? 's' : '') +
          ' (' + FQ.formatSize(info.totalSize) + ') fully backed up and safe to delete.</p>' +
          (info.unsynced > 0 ? '<p class="text-muted">' + info.unsynced + ' file' + (info.unsynced !== 1 ? 's' : '') + ' on Quest are not yet fully synced and will be kept.</p>' : '') +
          '<p style="color:#d47a7a;margin-top:8px">This cannot be undone.</p>';
        cleanModal.hidden = false;
      })
      .catch(function (err) {
        cleanBtn.disabled = false;
        FQ.setStatus(err.message || String(err), 'error');
      });
  });

  cleanModalConfirm.addEventListener('click', function () {
    var b = FQ.backend();
    if (!b) return;

    cleanModalConfirm.disabled = true;
    cleanModalConfirm.textContent = 'Cleaning...';

    b.CleanQuest()
      .then(function (msg) {
        cleanModalBody.innerHTML = '<p class="text-success" style="font-weight:600">' + FQ.escapeHtml(msg) + '</p>';
        cleanModalConfirm.hidden = true;
        cleanModalCancel.textContent = 'Done';
        loadAll();
      })
      .catch(function (err) {
        cleanModalBody.innerHTML = '<p style="color:#e05555">' + FQ.escapeHtml(err.message || String(err)) + '</p>';
        cleanModalConfirm.hidden = true;
        cleanModalCancel.textContent = 'Close';
      });
  });

  cleanModalCancel.addEventListener('click', function () {
    cleanModal.hidden = true;
    // Reset modal state
    cleanModalConfirm.hidden = false;
    cleanModalConfirm.disabled = false;
    cleanModalConfirm.textContent = 'Delete Files';
    cleanModalCancel.textContent = 'Cancel';
  });

  FQ.onViewActive('sync', loadAll);

  // Initial load
  loadAll();
})();
