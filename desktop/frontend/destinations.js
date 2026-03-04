// FetchQuest — Destinations view (cards + wizard)
(function () {
  'use strict';

  var destCardsEl = document.getElementById('dest-cards');

  // ── Destination cards ──

  function renderDestCards(cfg) {
    var list = cfg.destinationsList || [];
    if (list.length === 0) {
      destCardsEl.innerHTML = '<p class="text-muted text-sm" style="margin-bottom:16px">No portals open yet. Configure one below.</p>';
      return;
    }
    destCardsEl.innerHTML = list.map(function (d) {
      return '<div class="dest-card">' +
        '<div class="dest-card-info">' +
          '<div class="dest-card-name">' + FQ.escapeHtml(d.name) + '</div>' +
          '<div class="dest-card-remote">' + FQ.escapeHtml(d.remote) + '</div>' +
          '<div class="dest-card-stats" id="dest-stats-' + FQ.escapeHtml(d.name) + '"></div>' +
        '</div>' +
        '<div class="dest-status" id="dest-reach-' + FQ.escapeHtml(d.name) + '">' +
          '<span class="dest-status-dot loading"></span> Checking' +
        '</div>' +
        '<button type="button" class="btn-remove" data-name="' + FQ.escapeHtml(d.name) + '">Remove</button>' +
      '</div>';
    }).join('');

    // Remove handlers
    destCardsEl.querySelectorAll('.btn-remove').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var name = btn.dataset.name;
        if (!name) return;
        FQ.backend().RemoveDestination(name).then(loadDestinations).catch(function (err) {
          FQ.setStatus(err.message || String(err), 'error');
        });
      });
    });

    // Check reachability async (if backend supports it)
    var b = FQ.backend();
    if (b && b.GetDestinationStatuses) {
      b.GetDestinationStatuses().then(function (statuses) {
        (statuses || []).forEach(function (s) {
          var reachEl = document.getElementById('dest-reach-' + s.name);
          if (reachEl) {
            reachEl.innerHTML = s.reachable
              ? '<span class="dest-status-dot ok"></span> Portal Open'
              : '<span class="dest-status-dot fail"></span> Portal Sealed';
          }
          var statsEl = document.getElementById('dest-stats-' + s.name);
          if (statsEl && s.fileCount > 0) {
            statsEl.textContent = s.fileCount + ' files synced';
          }
        });
      });
    }
  }

  function loadDestinations() {
    var b = FQ.backend();
    if (!b) return;
    b.GetConfig().then(function (cfg) {
      renderDestCards(cfg);
    });
  }

  // ── Destination wizard ──

  var wizard = { destType: '', remoteName: '', rootFolderID: '', busy: false };
  var wizStepIds = ['wiz-pick', 'wiz-oauth', 'wiz-smb', 'wiz-other', 'wiz-folder'];

  function showWizStep(id) {
    wizStepIds.forEach(function (sid) {
      var el = document.getElementById(sid);
      if (el) el.hidden = (sid !== id);
    });
  }

  function resetWizard() {
    wizard = { destType: '', remoteName: '', rootFolderID: '', busy: false };
    showWizStep('wiz-pick');
    ['wiz-smb-host', 'wiz-smb-user', 'wiz-smb-pass', 'wiz-other-name'].forEach(function (id) {
      var el = document.getElementById(id); if (el) el.value = '';
    });
    ['wiz-folder-input', 'wiz-other-folder'].forEach(function (id) {
      var el = document.getElementById(id); if (el) el.value = 'FetchQuest';
    });
    var st = document.getElementById('wiz-oauth-status');
    if (st) { st.textContent = ''; st.className = 'wiz-status'; }
    var ob = document.getElementById('wiz-oauth-btn');
    if (ob) { ob.disabled = false; ob.textContent = 'Authorize'; }
    var br = document.getElementById('wiz-browser');
    if (br) br.hidden = true;
  }

  // Step 1: Type selection
  document.querySelectorAll('.wiz-type-btn').forEach(function (btn) {
    btn.addEventListener('click', function () {
      wizard.destType = btn.dataset.type;
      if (wizard.destType === 'drive' || wizard.destType === 'dropbox') {
        var title = document.getElementById('wiz-oauth-title');
        if (title) title.textContent = 'Connect ' + (wizard.destType === 'drive' ? 'Google Drive' : 'Dropbox');
        showWizStep('wiz-oauth');
      } else if (wizard.destType === 'smb') {
        showWizStep('wiz-smb');
      } else {
        loadRemotesForWizard();
        showWizStep('wiz-other');
      }
    });
  });

  // Step 2a: OAuth
  document.getElementById('wiz-oauth-btn').addEventListener('click', function () {
    if (wizard.busy) return;
    wizard.busy = true;
    var btn = document.getElementById('wiz-oauth-btn');
    var status = document.getElementById('wiz-oauth-status');
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner"></span> Waiting\u2026';
    status.textContent = 'Check your browser to sign in.';
    status.className = 'wiz-status';

    FQ.backend().SetupOAuthRemote(wizard.destType)
      .then(function (rn) {
        wizard.remoteName = rn; wizard.busy = false;
        btn.disabled = false; btn.textContent = 'Authorize';
        var ft = document.getElementById('wiz-folder-title');
        if (ft) ft.textContent = 'Connected to ' + (wizard.destType === 'drive' ? 'Google Drive' : 'Dropbox');
        showWizStep('wiz-folder');
      })
      .catch(function (err) {
        wizard.busy = false; btn.disabled = false; btn.textContent = 'Authorize';
        status.textContent = err.message || String(err);
        status.className = 'wiz-status error';
      });
  });

  // Step 2b: SMB
  document.getElementById('wiz-smb-btn').addEventListener('click', function () {
    if (wizard.busy) return;
    var host = document.getElementById('wiz-smb-host').value.trim();
    if (!host) { FQ.setStatus('Server address is required.', 'error'); return; }
    wizard.busy = true;
    var btn = document.getElementById('wiz-smb-btn');
    btn.disabled = true; btn.textContent = 'Connecting\u2026';

    FQ.backend().SetupSMBRemote(host, document.getElementById('wiz-smb-user').value.trim(), document.getElementById('wiz-smb-pass').value)
      .then(function (rn) {
        wizard.remoteName = rn; wizard.busy = false;
        btn.disabled = false; btn.textContent = 'Connect';
        var ft = document.getElementById('wiz-folder-title');
        if (ft) ft.textContent = 'Connected to ' + host;
        showWizStep('wiz-folder');
      })
      .catch(function (err) {
        wizard.busy = false; btn.disabled = false; btn.textContent = 'Connect';
        FQ.setStatus(err.message || String(err), 'error');
      });
  });

  // Step 2c: Other — remotes
  function loadRemotesForWizard() {
    var b = FQ.backend();
    if (!b) return;
    b.GetConfig().then(function (cfg) {
      var sel = document.getElementById('wiz-other-select');
      if (!sel) return;
      var remotes = cfg.rcloneRemotes || [];
      sel.innerHTML = '<option value="">\u2014 Choose one \u2014</option>' +
        remotes.map(function (r) {
          return '<option value="' + FQ.escapeHtml(r) + '">' + FQ.escapeHtml(r.replace(/:$/, '')) + '</option>';
        }).join('');
    });
  }

  document.getElementById('wiz-other-refresh').addEventListener('click', loadRemotesForWizard);

  document.getElementById('wiz-other-add').addEventListener('click', function () {
    var remote = (document.getElementById('wiz-other-select').value || '').trim().replace(/:$/, '');
    var name = document.getElementById('wiz-other-name').value.trim();
    var folder = document.getElementById('wiz-other-folder').value.trim();
    if (!remote) { FQ.setStatus('Choose a remote from the list.', 'error'); return; }
    if (!name) { FQ.setStatus('Enter a name for this destination.', 'error'); return; }
    var remoteStr = folder ? remote + ':' + folder : remote + ':';
    FQ.backend().AddDestination(name, remoteStr)
      .then(function () { FQ.setStatus('Destination added.', 'success'); resetWizard(); loadDestinations(); })
      .catch(function (err) { FQ.setStatus(err.message || String(err), 'error'); });
  });

  // Folder browser
  var browserPath = '';

  function loadBrowserFolders(path) {
    var list = document.getElementById('wiz-browser-list');
    var pathEl = document.getElementById('wiz-browser-path');
    if (!list) return;
    list.innerHTML = '<p class="text-muted">Loading\u2026</p>';
    pathEl.textContent = '/' + (path || '');
    browserPath = path || '';

    FQ.backend().ListRemoteFolders(wizard.remoteName, path, wizard.destType, wizard.rootFolderID || '')
      .then(function (folders) {
        if (!folders || folders.length === 0) {
          list.innerHTML = '<p class="text-muted">No subfolders here.</p>';
          return;
        }
        list.innerHTML = folders.map(function (f) {
          return '<button type="button" class="wiz-browser-item" data-path="' + FQ.escapeHtml(f.path) + '">' + FQ.escapeHtml(f.name) + '</button>';
        }).join('');
        list.querySelectorAll('.wiz-browser-item').forEach(function (item) {
          item.addEventListener('click', function () { loadBrowserFolders(item.dataset.path); });
        });
      })
      .catch(function () { list.innerHTML = '<p class="text-muted">Could not list folders.</p>'; });
  }

  document.getElementById('wiz-browse-btn').addEventListener('click', function () {
    var browser = document.getElementById('wiz-browser');
    if (!browser) return;
    if (!browser.hidden) { browser.hidden = true; return; }
    browser.hidden = false;
    loadBrowserFolders('');
  });

  document.getElementById('wiz-browser-up').addEventListener('click', function () {
    if (!browserPath) return;
    var idx = browserPath.lastIndexOf('/');
    loadBrowserFolders(idx < 0 ? '' : browserPath.substring(0, idx));
  });

  document.getElementById('wiz-browser-select').addEventListener('click', function () {
    var input = document.getElementById('wiz-folder-input');
    if (input) input.value = browserPath || '';
    var browser = document.getElementById('wiz-browser');
    if (browser) browser.hidden = true;
  });

  // URL paste detection on folder input
  var folderInput = document.getElementById('wiz-folder-input');
  if (folderInput) {
    folderInput.addEventListener('paste', function () {
      setTimeout(function () {
        var val = folderInput.value.trim();
        var m;
        if ((m = /drive\.google\.com\/drive\/.*folders\/([a-zA-Z0-9_-]+)/.exec(val))) {
          wizard.rootFolderID = m[1];
          folderInput.value = '';
          var browser = document.getElementById('wiz-browser');
          if (browser) browser.hidden = false;
          loadBrowserFolders('');
        } else if ((m = /dropbox\.com\/home(.*)/.exec(val))) {
          var path = (m[1] || '').replace(/^\//, '');
          folderInput.value = path || '';
          var browser = document.getElementById('wiz-browser');
          if (browser) browser.hidden = false;
          loadBrowserFolders(path);
        }
      }, 0);
    });
  }

  // Step 3: Folder → finish
  document.getElementById('wiz-folder-add').addEventListener('click', function () {
    if (wizard.busy) return;
    var folder = document.getElementById('wiz-folder-input').value.trim();
    wizard.busy = true;
    var btn = document.getElementById('wiz-folder-add');
    btn.disabled = true; btn.textContent = 'Adding\u2026';

    FQ.backend().AddDestinationAuto(wizard.remoteName, folder, wizard.destType, wizard.rootFolderID || '')
      .then(function () {
        wizard.busy = false; btn.disabled = false; btn.textContent = 'Add destination';
        FQ.setStatus('Destination added.', 'success');
        resetWizard(); loadDestinations();
      })
      .catch(function (err) {
        wizard.busy = false; btn.disabled = false; btn.textContent = 'Add destination';
        FQ.setStatus(err.message || String(err), 'error');
      });
  });

  // Cancel buttons
  ['wiz-oauth-cancel', 'wiz-smb-cancel', 'wiz-other-cancel', 'wiz-folder-cancel'].forEach(function (id) {
    var btn = document.getElementById(id);
    if (btn) btn.addEventListener('click', resetWizard);
  });

  FQ.onViewActive('destinations', loadDestinations);
})();
