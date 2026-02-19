package manifest

import (
	"database/sql"
	"fmt"
	"time"
)

// Entry represents a synced file in the manifest.
type Entry struct {
	ID           int64
	DeviceSerial string
	RemotePath   string
	LocalPath    string
	Size         int64
	MTime        int64
	SHA256       string
	PulledAt     *time.Time
}

// DestSync records that a file has been synced to a destination.
type DestSync struct {
	ID          int64
	FileID      int64
	Destination string
	SyncedAt    time.Time
}

// IsPulled returns true if the file has been pulled (device_serial, remote_path, size, mtime match).
func (m *DB) IsPulled(deviceSerial, remotePath string, size, mtime int64) (bool, error) {
	var count int
	err := m.db.QueryRow(
		`SELECT COUNT(*) FROM files WHERE device_serial = ? AND remote_path = ? AND size = ? AND mtime = ?`,
		deviceSerial, remotePath, size, mtime,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check pulled: %w", err)
	}
	return count > 0, nil
}

// RecordPull inserts or updates a file entry after pulling.
func (m *DB) RecordPull(deviceSerial, remotePath, localPath string, size, mtime int64) (int64, error) {
	now := time.Now()
	res, err := m.db.Exec(
		`INSERT INTO files (device_serial, remote_path, local_path, size, mtime, pulled_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(device_serial, remote_path) DO UPDATE SET
		   local_path = excluded.local_path,
		   size = excluded.size,
		   mtime = excluded.mtime,
		   pulled_at = excluded.pulled_at`,
		deviceSerial, remotePath, localPath, size, mtime, now,
	)
	if err != nil {
		return 0, fmt.Errorf("record pull: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		// If it was an update, fetch the ID
		return m.getFileID(deviceSerial, remotePath)
	}
	return id, nil
}

// RecordDestSync marks a file as synced to a destination.
func (m *DB) RecordDestSync(fileID int64, destination string) error {
	_, err := m.db.Exec(
		`INSERT INTO dest_syncs (file_id, destination, synced_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(file_id, destination) DO UPDATE SET synced_at = excluded.synced_at`,
		fileID, destination, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("record dest sync: %w", err)
	}
	return nil
}

// IsFullySynced checks if a file has been synced to all given destinations.
func (m *DB) IsFullySynced(deviceSerial, remotePath string, destinations []string) (bool, error) {
	if len(destinations) == 0 {
		return false, nil
	}
	fileID, err := m.getFileID(deviceSerial, remotePath)
	if err != nil {
		return false, nil // file not in manifest
	}
	var count int
	err = m.db.QueryRow(
		`SELECT COUNT(DISTINCT destination) FROM dest_syncs WHERE file_id = ?`, fileID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check synced: %w", err)
	}
	return count >= len(destinations), nil
}

// GetFileID returns the file ID for a device+path combo.
func (m *DB) getFileID(deviceSerial, remotePath string) (int64, error) {
	var id int64
	err := m.db.QueryRow(
		`SELECT id FROM files WHERE device_serial = ? AND remote_path = ?`,
		deviceSerial, remotePath,
	).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("file not found")
		}
		return 0, err
	}
	return id, nil
}

// GetUnpushedFiles returns files that have been pulled but not synced to the given destination.
func (m *DB) GetUnpushedFiles(destination string) ([]Entry, error) {
	rows, err := m.db.Query(
		`SELECT f.id, f.device_serial, f.remote_path, f.local_path, f.size, f.mtime, f.sha256
		 FROM files f
		 WHERE f.pulled_at IS NOT NULL
		   AND f.id NOT IN (SELECT file_id FROM dest_syncs WHERE destination = ?)`,
		destination,
	)
	if err != nil {
		return nil, fmt.Errorf("get unpushed: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.DeviceSerial, &e.RemotePath, &e.LocalPath, &e.Size, &e.MTime, &e.SHA256); err != nil {
			return nil, fmt.Errorf("scan unpushed: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetFullySyncedFiles returns files synced to ALL destinations for a device (safe to clean).
func (m *DB) GetFullySyncedFiles(deviceSerial string, destinations []string) ([]Entry, error) {
	if len(destinations) == 0 {
		return nil, nil
	}
	rows, err := m.db.Query(
		`SELECT f.id, f.device_serial, f.remote_path, f.local_path, f.size, f.mtime
		 FROM files f
		 WHERE f.device_serial = ?
		   AND (SELECT COUNT(DISTINCT ds.destination) FROM dest_syncs ds WHERE ds.file_id = f.id) >= ?`,
		deviceSerial, len(destinations),
	)
	if err != nil {
		return nil, fmt.Errorf("get fully synced: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.DeviceSerial, &e.RemotePath, &e.LocalPath, &e.Size, &e.MTime); err != nil {
			return nil, fmt.Errorf("scan fully synced: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetAnySyncedFiles returns files synced to at least one destination for a device.
func (m *DB) GetAnySyncedFiles(deviceSerial string) ([]Entry, error) {
	rows, err := m.db.Query(
		`SELECT f.id, f.device_serial, f.remote_path, f.local_path, f.size, f.mtime
		 FROM files f
		 WHERE f.device_serial = ?
		   AND (SELECT COUNT(*) FROM dest_syncs ds WHERE ds.file_id = f.id) >= 1`,
		deviceSerial,
	)
	if err != nil {
		return nil, fmt.Errorf("get any synced: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.DeviceSerial, &e.RemotePath, &e.LocalPath, &e.Size, &e.MTime); err != nil {
			return nil, fmt.Errorf("scan any synced: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DeviceStats returns sync statistics for a device.
type DeviceStats struct {
	TotalFiles  int
	PulledFiles int
	SyncedFiles int // fully synced to all destinations
}

// GetDeviceStats returns sync statistics for a device.
func (m *DB) GetDeviceStats(deviceSerial string, numDestinations int) (DeviceStats, error) {
	var stats DeviceStats
	err := m.db.QueryRow(
		`SELECT COUNT(*) FROM files WHERE device_serial = ?`, deviceSerial,
	).Scan(&stats.TotalFiles)
	if err != nil {
		return stats, err
	}
	err = m.db.QueryRow(
		`SELECT COUNT(*) FROM files WHERE device_serial = ? AND pulled_at IS NOT NULL`, deviceSerial,
	).Scan(&stats.PulledFiles)
	if err != nil {
		return stats, err
	}
	if numDestinations > 0 {
		err = m.db.QueryRow(
			`SELECT COUNT(*) FROM files f
			 WHERE f.device_serial = ?
			   AND (SELECT COUNT(DISTINCT ds.destination) FROM dest_syncs ds WHERE ds.file_id = f.id) >= ?`,
			deviceSerial, numDestinations,
		).Scan(&stats.SyncedFiles)
		if err != nil {
			return stats, err
		}
	}
	return stats, nil
}
