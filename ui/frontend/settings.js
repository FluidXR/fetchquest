// FetchQuest — Settings view
(function () {
  'use strict';

  var browseSyncDirBtn = document.getElementById('browse-sync-dir-btn');
  var settingsDevices = document.getElementById('settings-devices');

  // ── Sync folder ──

  if (browseSyncDirBtn) {
    browseSyncDirBtn.addEventListener('click', function () {
      var b = FQ.backend();
      if (!b) return;
      b.OpenFolderDialog().then(function (path) {
        if (!path) return;
        return b.SetSyncDir(path).then(function () {
          var input = document.getElementById('sync-dir-input');
          if (input) input.value = path;
        });
      }).catch(function (err) {
        FQ.setStatus(err.message || String(err), 'error');
      });
    });
  }

  // ── Device settings ──

  function loadDeviceSettings() {
    var b = FQ.backend();
    if (!b) return;

    Promise.all([b.GetDevices(), b.GetConfig()]).then(function (results) {
      var devices = results[0] || [];
      var cfg = results[1];
      var cfgDevices = (cfg && cfg.devices) || {};

      if (devices.length === 0) {
        settingsDevices.innerHTML = '<p class="text-muted text-sm">No devices seen yet. Connect a Quest to configure it.</p>';
        return;
      }

      settingsDevices.innerHTML = devices.map(function (d) {
        var nickname = d.nickname || '';
        return '<div class="setting-device">' +
          '<div class="setting-device-serial">' + FQ.escapeHtml(d.serial) +
            (d.model ? ' <span class="text-muted">(' + FQ.escapeHtml(d.model) + ')</span>' : '') +
            (d.status === 'device' ? ' <span class="badge badge-synced">Connected</span>' : '') +
          '</div>' +
          '<div class="setting-device-field">' +
            '<label>Nickname</label>' +
            '<div style="display:flex;gap:8px">' +
              '<input class="input" type="text" value="' + FQ.escapeHtml(nickname) + '" data-serial="' + FQ.escapeHtml(d.serial) + '" data-field="nickname" placeholder="e.g. My Quest 3" style="flex:1">' +
              '<button type="button" class="btn-text btn-save-nickname" data-serial="' + FQ.escapeHtml(d.serial) + '">Save</button>' +
            '</div>' +
          '</div>' +
        '</div>';
      }).join('');

      // Save nickname
      settingsDevices.querySelectorAll('.btn-save-nickname').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var serial = btn.dataset.serial;
          var input = settingsDevices.querySelector('input[data-serial="' + serial + '"]');
          if (!input || !b.SetDeviceNickname) return;
          var val = input.value.trim();
          btn.textContent = 'Saving...';
          btn.disabled = true;
          b.SetDeviceNickname(serial, val)
            .then(function () { btn.textContent = 'Saved!'; setTimeout(function () { btn.textContent = 'Save'; btn.disabled = false; }, 1500); })
            .catch(function (err) { btn.textContent = 'Save'; btn.disabled = false; FQ.setStatus(err.message || String(err), 'error'); });
        });
      });
    });
  }

  FQ.onViewActive('settings', function () {
    loadDeviceSettings();
    // Refresh sync dir display
    var b = FQ.backend();
    if (b) b.GetConfig().then(function (cfg) {
      var input = document.getElementById('sync-dir-input');
      if (input && cfg) input.value = cfg.syncDir || '';
    });
  });
})();
