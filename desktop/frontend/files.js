// FetchQuest — Files view
(function () {
  'use strict';

  var filesContent = document.getElementById('files-content');
  var openFolderBtn = document.getElementById('open-folder-btn');
  var activeTab = 'quest';
  var questTab = document.querySelector('.tab[data-tab="quest"]');
  var localTab = document.querySelector('.tab[data-tab="local"]');

  function updateTabCounts(b) {
    if (!b) return;
    if (b.GetDeviceFiles && FQ.deviceState.hasOnline) {
      var serial = '';
      FQ.deviceState.devices.forEach(function (d) { if (d.status === 'device' && !serial) serial = d.serial; });
      b.GetDeviceFiles(serial).then(function (files) {
        questTab.textContent = 'Quest (' + (files ? files.length : 0) + ')';
      });
    }
    if (b.GetLocalFiles) {
      b.GetLocalFiles().then(function (files) {
        localTab.textContent = 'Local (' + (files ? files.length : 0) + ')';
      });
    }
  }

  // Tab switching
  document.querySelectorAll('.tab-bar .tab').forEach(function (tab) {
    tab.addEventListener('click', function () {
      document.querySelectorAll('.tab-bar .tab').forEach(function (t) { t.classList.remove('active'); });
      tab.classList.add('active');
      activeTab = tab.dataset.tab;
      loadFiles();
    });
  });

  function statusBadge(entry) {
    if (!entry.isPulled) return '<span class="badge badge-quest">On Device</span>';
    if (!entry.syncedDests || entry.syncedDests.length === 0) return '<span class="badge badge-local">Local</span>';
    if (entry.syncedDests.length < entry.totalDests) return '<span class="badge badge-partial">' + entry.syncedDests.length + ' of ' + entry.totalDests + ' places</span>';
    return '<span class="badge badge-synced">Backed Up</span>';
  }

  function renderFileTable(files) {
    if (!files || files.length === 0) {
      var msg = activeTab === 'quest'
        ? 'No files found on Quest.'
        : 'No files synced locally yet.';
      filesContent.innerHTML = '<p class="text-muted" style="padding:20px;margin:0">' + msg + '</p>';
      return;
    }

    // Group by mediaType
    var groups = {};
    files.forEach(function (f) {
      var type = f.mediaType || 'Other';
      if (!groups[type]) groups[type] = [];
      groups[type].push(f);
    });

    // Flatten all groups, sort newest first
    var allFiles = [];
    Object.keys(groups).forEach(function (type) {
      groups[type].forEach(function (f) { allFiles.push(f); });
    });
    allFiles.sort(function (a, b) { return (b.mtime || 0) - (a.mtime || 0); });

    var showLocate = activeTab === 'local';
    var html = '<table class="file-table"><thead><tr><th>Name</th><th>Date</th><th>Size</th><th>Status</th><th></th></tr></thead><tbody>';
    allFiles.forEach(function (f) {
      var filePath = f.localPath || f.path;
      html += '<tr data-path="' + FQ.escapeHtml(filePath) + '">' +
        '<td>' + FQ.escapeHtml(f.fileName) + '</td>' +
        '<td>' + FQ.formatDate(f.mtime) + '</td>' +
        '<td>' + FQ.formatSize(f.size) + '</td>' +
        '<td>' + statusBadge(f) + '</td>' +
        '<td>' + (showLocate && filePath ? '<button class="btn-text btn-locate" data-locate="' + FQ.escapeHtml(filePath) + '">Show</button>' : '') + '</td>' +
        '</tr>';
    });
    html += '</tbody></table>';

    filesContent.innerHTML = html;

    // Row click → select
    filesContent.querySelectorAll('.file-table tr[data-path]').forEach(function (row) {
      row.addEventListener('click', function () {
        filesContent.querySelectorAll('.file-table tr').forEach(function (r) { r.classList.remove('selected'); });
        row.classList.add('selected');
      });
    });

    // Show in folder buttons
    filesContent.querySelectorAll('.btn-locate').forEach(function (btn) {
      btn.addEventListener('click', function (e) {
        e.stopPropagation();
        var b = FQ.backend();
        if (b && b.ShowInFolder) b.ShowInFolder(btn.dataset.locate);
      });
    });
  }

  function loadFiles() {
    var b = FQ.backend();
    if (!b) {
      filesContent.innerHTML = '<p class="text-muted" style="padding:20px;margin:0">Backend not ready.</p>';
      return;
    }

    if (activeTab === 'quest') {
      if (!FQ.deviceState.hasOnline) {
        filesContent.innerHTML = '<p class="text-muted" style="padding:20px;margin:0">Connect your Quest to browse files on the headset.</p>';
        return;
      }
      if (!b.GetDeviceFiles) {
        filesContent.innerHTML = '<p class="text-muted" style="padding:20px;margin:0">File browsing will be available soon.</p>';
        return;
      }
      var serial = '';
      FQ.deviceState.devices.forEach(function (d) { if (d.status === 'device' && !serial) serial = d.serial; });
      b.GetDeviceFiles(serial).then(renderFileTable).catch(function (err) {
        filesContent.innerHTML = '<p class="text-error" style="padding:20px;margin:0">Error: ' + FQ.escapeHtml(err.message || String(err)) + '</p>';
      });
    } else {
      if (!b.GetLocalFiles) {
        filesContent.innerHTML = '<p class="text-muted" style="padding:20px;margin:0">File browsing will be available soon.</p>';
        return;
      }
      b.GetLocalFiles().then(renderFileTable).catch(function (err) {
        filesContent.innerHTML = '<p class="text-error" style="padding:20px;margin:0">Error: ' + FQ.escapeHtml(err.message || String(err)) + '</p>';
      });
    }
  }

  // Open sync folder
  if (openFolderBtn) {
    openFolderBtn.addEventListener('click', function () {
      var b = FQ.backend();
      if (b && b.OpenSyncFolder) b.OpenSyncFolder();
    });
  }

  FQ.onViewActive('files', function () {
    FQ.refreshDevices().then(function () {
      loadFiles();
      updateTabCounts(FQ.backend());
    });
  });
})();
