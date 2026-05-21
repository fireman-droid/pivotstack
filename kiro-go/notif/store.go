package notif

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kiro-api-proxy/config"
)

// Store 是单进程内的 notification 持久化封装。
//
//   - 所有读写串行化（RWMutex），保证 list/CRUD 与 polling 不互相覆盖
//   - 写入用 tmp + rename，避免崩溃时半截文件
//   - 全量加载到内存（数据量小：通知 <1000 / deliveries <100k）
type Store struct {
	mu   sync.RWMutex
	file File
	path string
}

var (
	defaultStore *Store
	once         sync.Once
)

// Default 返回进程级单例。
func Default() *Store {
	once.Do(func() {
		defaultStore = newStore(filepath.Join(config.GetDataDir(), "notifications.json"))
	})
	return defaultStore
}

func newStore(path string) *Store {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		// 启动期失败用空 file 兜底，下次写入会自动覆盖
		fmt.Printf("[notif] load %s failed: %v (starting empty)\n", path, err)
		s.file = File{SchemaVersion: CurrentSchemaVersion}
	}
	return s
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.file = File{SchemaVersion: CurrentSchemaVersion}
			return nil
		}
		return err
	}
	if len(data) == 0 {
		s.file = File{SchemaVersion: CurrentSchemaVersion}
		return nil
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if f.SchemaVersion == 0 {
		f.SchemaVersion = CurrentSchemaVersion
	}
	if f.Notifications == nil {
		f.Notifications = []Notification{}
	}
	s.file = f
	return nil
}

// flushLocked 写盘 — 调用方持有写锁。
func (s *Store) flushLocked() error {
	s.file.SchemaVersion = CurrentSchemaVersion
	s.file.UpdatedAt = time.Now().Unix()
	data, err := json.MarshalIndent(s.file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Snapshot 读快照（深拷贝 slice 头，notification 值类型本身安全复制）。
func (s *Store) Snapshot() File {
	s.mu.RLock()
	defer s.mu.RUnlock()
	notifs := make([]Notification, len(s.file.Notifications))
	copy(notifs, s.file.Notifications)
	for i := range notifs {
		if len(notifs[i].TargetValue) > 0 {
			cp := make([]string, len(notifs[i].TargetValue))
			copy(cp, notifs[i].TargetValue)
			notifs[i].TargetValue = cp
		}
	}
	dels := make([]Delivery, len(s.file.Deliveries))
	copy(dels, s.file.Deliveries)
	return File{
		SchemaVersion: s.file.SchemaVersion,
		Notifications: notifs,
		Deliveries:    dels,
		UpdatedAt:     s.file.UpdatedAt,
	}
}

// UpsertNotification 插入或更新。返回写入后的副本。
func (s *Store) UpsertNotification(n Notification) (Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.file.Notifications {
		if s.file.Notifications[i].ID == n.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.file.Notifications = append(s.file.Notifications, n)
	} else {
		s.file.Notifications[idx] = n
	}
	if err := s.flushLocked(); err != nil {
		return Notification{}, err
	}
	return n, nil
}

// SoftDelete 标记 deletedAt；返回是否实际被改动。
func (s *Store) SoftDelete(id string, now int64, operator string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.file.Notifications {
		if s.file.Notifications[i].ID == id && s.file.Notifications[i].DeletedAt == 0 {
			s.file.Notifications[i].DeletedAt = now
			s.file.Notifications[i].UpdatedAt = now
			s.file.Notifications[i].UpdatedBy = operator
			if err := s.flushLocked(); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// GetNotification 按 ID 获取（含软删，调用方自行过滤 DeletedAt）。
func (s *Store) GetNotification(id string) (Notification, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.file.Notifications {
		if s.file.Notifications[i].ID == id {
			return s.file.Notifications[i], true
		}
	}
	return Notification{}, false
}

// MarkDelivery 写入 user 对 notification 的状态变化。kind ∈ {"seen","read","dismiss"}。
//
// 幂等：已读再点 read 不更新 readAt，但更新 updatedAt 以做幂等回执。
func (s *Store) MarkDelivery(notifID, userID, kind string, now int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.file.Deliveries {
		d := s.file.Deliveries[i]
		if d.NotificationID == notifID && d.UserID == userID {
			idx = i
			break
		}
	}
	var d Delivery
	if idx < 0 {
		d = Delivery{NotificationID: notifID, UserID: userID}
	} else {
		d = s.file.Deliveries[idx]
	}
	switch kind {
	case "seen":
		if d.FirstSeenAt == 0 {
			d.FirstSeenAt = now
		}
	case "read":
		if d.ReadAt == 0 {
			d.ReadAt = now
		}
		if d.FirstSeenAt == 0 {
			d.FirstSeenAt = now
		}
	case "dismiss":
		if d.DismissedAt == 0 {
			d.DismissedAt = now
		}
		if d.ReadAt == 0 {
			d.ReadAt = now
		}
		if d.FirstSeenAt == 0 {
			d.FirstSeenAt = now
		}
	default:
		return errors.New("unknown delivery kind")
	}
	d.UpdatedAt = now
	if idx < 0 {
		s.file.Deliveries = append(s.file.Deliveries, d)
	} else {
		s.file.Deliveries[idx] = d
	}
	return s.flushLocked()
}

// MarkAllRead 一次性把 user 对 ids 列表全部置为已读，返回实际改动条数。
func (s *Store) MarkAllRead(userID string, ids []string, now int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	target := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		target[id] = struct{}{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := 0
	existing := make(map[string]int, len(s.file.Deliveries))
	for i, d := range s.file.Deliveries {
		if d.UserID == userID {
			existing[d.NotificationID] = i
		}
	}
	for nid := range target {
		if idx, ok := existing[nid]; ok {
			if s.file.Deliveries[idx].ReadAt == 0 {
				s.file.Deliveries[idx].ReadAt = now
				if s.file.Deliveries[idx].FirstSeenAt == 0 {
					s.file.Deliveries[idx].FirstSeenAt = now
				}
				s.file.Deliveries[idx].UpdatedAt = now
				changed++
			}
		} else {
			s.file.Deliveries = append(s.file.Deliveries, Delivery{
				NotificationID: nid,
				UserID:         userID,
				FirstSeenAt:    now,
				ReadAt:         now,
				UpdatedAt:      now,
			})
			changed++
		}
	}
	if changed == 0 {
		return 0, nil
	}
	if err := s.flushLocked(); err != nil {
		return 0, err
	}
	return changed, nil
}

// DeliveriesFor 返回某 notification 的所有 delivery 记录。
func (s *Store) DeliveriesFor(notifID string) []Delivery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Delivery, 0, 4)
	for _, d := range s.file.Deliveries {
		if d.NotificationID == notifID {
			out = append(out, d)
		}
	}
	return out
}

// DeliveryMap 返回 user 视角的 delivery 索引：notifID → delivery。
func (s *Store) DeliveryMap(userID string) map[string]Delivery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Delivery, 16)
	for _, d := range s.file.Deliveries {
		if d.UserID == userID {
			out[d.NotificationID] = d
		}
	}
	return out
}
