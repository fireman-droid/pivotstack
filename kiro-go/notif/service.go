package notif

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"kiro-api-proxy/config"
)

// ───────────────── helpers ─────────────────

func newID(now int64) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("ntf_%d_%s", now, hex.EncodeToString(b[:]))
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ───────────────── targeting ─────────────────

// VisibleForUser 判断单条 notification 对单 user 是否可见。
//
// 不考虑 read/dismissed 状态，只考虑发布、目标、生效窗口。
func VisibleForUser(n Notification, key config.ApiKeyInfo, now int64) bool {
	if !key.Enabled {
		return false
	}
	if n.DeletedAt != 0 || n.Status != StatusPublished {
		return false
	}
	if n.PublishAt > 0 && now < n.PublishAt {
		return false
	}
	if n.ExpireAt > 0 && now >= n.ExpireAt {
		return false
	}
	switch n.TargetType {
	case TargetAll:
		return true
	case TargetPlan:
		return contains(n.TargetValue, key.Plan)
	case TargetGroup:
		for _, g := range n.TargetValue {
			if _, ok := key.ChannelPreferences[g]; ok {
				return true
			}
		}
		return false
	case TargetUserIDs:
		return contains(n.TargetValue, key.ID)
	}
	return false
}

// resolveTargetUserIDs 枚举所有命中 notification 的 ApiKeyInfo.ID（用于 stats / 已读率）。
func resolveTargetUserIDs(n Notification, allKeys []config.ApiKeyInfo, now int64) []string {
	out := make([]string, 0, len(allKeys))
	for _, k := range allKeys {
		if VisibleForUser(n, k, now) {
			out = append(out, k.ID)
		}
	}
	return out
}

// ───────────────── user-facing list ─────────────────

// UserView 单条 notification 给 user 的渲染视图。
type UserView struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Level       string `json:"level"`
	PublishAt   int64  `json:"publishAt,omitempty"`
	ExpireAt    int64  `json:"expireAt,omitempty"`
	Dismissible bool   `json:"dismissible"`

	Read      bool  `json:"read"`
	Dismissed bool  `json:"dismissed"`
	ReadAt    int64 `json:"readAt,omitempty"`
}

// UserListResult 是 GET /user/api/notifications 的响应。
type UserListResult struct {
	UnreadCount int        `json:"unreadCount"`
	Items       []UserView `json:"items"`
}

// ListForUser 取某 user 可见的全部通知（按 publishAt desc 排序），并标注 read/dismissed。
//
// includeDismissed=false 时已 dismiss 的通知不出现。limit ≤ 0 表示不截断。
func ListForUser(key config.ApiKeyInfo, limit int, includeDismissed bool) UserListResult {
	now := time.Now().Unix()
	snap := Default().Snapshot()
	delivery := Default().DeliveryMap(key.ID)

	items := make([]UserView, 0, 8)
	unread := 0
	for _, n := range snap.Notifications {
		if !VisibleForUser(n, key, now) {
			continue
		}
		d := delivery[n.ID]
		if !includeDismissed && d.DismissedAt != 0 {
			continue
		}
		read := d.ReadAt != 0
		if !read {
			unread++
		}
		items = append(items, UserView{
			ID:          n.ID,
			Title:       n.Title,
			Body:        n.Body,
			Level:       n.Level,
			PublishAt:   n.PublishAt,
			ExpireAt:    n.ExpireAt,
			Dismissible: n.Dismissible,
			Read:        read,
			Dismissed:   d.DismissedAt != 0,
			ReadAt:      d.ReadAt,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		// 用 PublishAt 排序；若 0（草稿不该进来）退化到 ID
		return items[i].PublishAt > items[j].PublishAt
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return UserListResult{UnreadCount: unread, Items: items}
}

// MarkUserRead 单条标已读。返回是否实际改变（已读再标 = false 但仍写时间戳？这里返回是否首次置位）。
func MarkUserRead(notifID string, key config.ApiKeyInfo, dismiss bool) (int64, error) {
	now := time.Now().Unix()
	n, ok := Default().GetNotification(notifID)
	if !ok {
		return 0, fmt.Errorf("notification %s not found", notifID)
	}
	if !VisibleForUser(n, key, now) {
		return 0, fmt.Errorf("notification %s not accessible", notifID)
	}
	kind := "read"
	if dismiss {
		if !n.Dismissible {
			return 0, fmt.Errorf("notification is not dismissible")
		}
		kind = "dismiss"
	}
	if err := Default().MarkDelivery(notifID, key.ID, kind, now); err != nil {
		return 0, err
	}
	return now, nil
}

// MarkAllUserRead 把 user 当前可见的所有未读通知一次性置为已读。返回改动条数。
func MarkAllUserRead(key config.ApiKeyInfo) (int, error) {
	res := ListForUser(key, 0, false)
	ids := make([]string, 0, len(res.Items))
	for _, it := range res.Items {
		if !it.Read {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return Default().MarkAllRead(key.ID, ids, time.Now().Unix())
}

// ───────────────── admin-facing list + stats ─────────────────

// Stats 是单条 notification 的发布统计。
type Stats struct {
	NotificationID string `json:"notificationId"`
	TargetCount    int    `json:"targetCount"`
	ReadCount      int    `json:"readCount"`
	DismissedCount int    `json:"dismissedCount"`
	UnreadCount    int    `json:"unreadCount"`
}

// AdminItem 是单条 notification + stats 的组合视图。
type AdminItem struct {
	Notification Notification `json:"notification"`
	Stats        Stats        `json:"stats"`
}

// AdminListResult 是 GET /admin/api/notifications 的响应。
type AdminListResult struct {
	Items []AdminItem `json:"items"`
	Total int         `json:"total"`
}

// computeStats 计算单条 notification 的已读统计。allKeys 由调用方传入避免重复加锁。
func computeStats(n Notification, allKeys []config.ApiKeyInfo, now int64) Stats {
	target := resolveTargetUserIDs(n, allKeys, now)
	st := Stats{NotificationID: n.ID, TargetCount: len(target)}
	if st.TargetCount == 0 {
		return st
	}
	idx := make(map[string]struct{}, st.TargetCount)
	for _, id := range target {
		idx[id] = struct{}{}
	}
	for _, d := range Default().DeliveriesFor(n.ID) {
		if _, ok := idx[d.UserID]; !ok {
			// user 不再属于目标群（plan 改了 / group 偏好换了），不统计
			continue
		}
		if d.ReadAt != 0 {
			st.ReadCount++
		}
		if d.DismissedAt != 0 {
			st.DismissedCount++
		}
	}
	st.UnreadCount = st.TargetCount - st.ReadCount
	if st.UnreadCount < 0 {
		st.UnreadCount = 0
	}
	return st
}

// ListForAdmin 返回 admin 视角的 notification 列表 + stats，支持状态过滤。
//
// status ∈ {"all","draft","published","expired","deleted"}。limit ≤ 0 不截断。
func ListForAdmin(status string, limit, offset int) AdminListResult {
	now := time.Now().Unix()
	snap := Default().Snapshot()
	allKeys := snapshotApiKeys()

	filtered := make([]Notification, 0, len(snap.Notifications))
	status = strings.ToLower(strings.TrimSpace(status))
	for _, n := range snap.Notifications {
		if !matchAdminStatus(n, status, now) {
			continue
		}
		filtered = append(filtered, n)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		ai, aj := filtered[i].PublishAt, filtered[j].PublishAt
		if ai == aj {
			return filtered[i].CreatedAt > filtered[j].CreatedAt
		}
		return ai > aj
	})

	total := len(filtered)
	if offset > 0 {
		if offset >= total {
			filtered = nil
		} else {
			filtered = filtered[offset:]
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	items := make([]AdminItem, 0, len(filtered))
	for _, n := range filtered {
		items = append(items, AdminItem{
			Notification: n,
			Stats:        computeStats(n, allKeys, now),
		})
	}
	return AdminListResult{Items: items, Total: total}
}

func matchAdminStatus(n Notification, status string, now int64) bool {
	deleted := n.DeletedAt != 0
	switch status {
	case "", "all":
		return !deleted
	case "deleted":
		return deleted
	case "draft":
		return !deleted && n.Status == StatusDraft
	case "published":
		return !deleted && n.Status == StatusPublished &&
			(n.ExpireAt == 0 || now < n.ExpireAt)
	case "expired":
		return !deleted && n.Status == StatusPublished &&
			n.ExpireAt > 0 && now >= n.ExpireAt
	}
	return !deleted
}

// snapshotApiKeys 取所有 ApiKey 的副本（屏蔽 key 字段）。
//
// notif 包不能直接锁 config，所以借用现有 ListApiKeys 接口。
func snapshotApiKeys() []config.ApiKeyInfo {
	return config.GetAllApiKeys()
}

// CreateInput 是 admin POST 入参。
type CreateInput struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Level       string   `json:"level"`
	TargetType  string   `json:"targetType"`
	TargetValue []string `json:"targetValue"`
	Status      string   `json:"status"`
	PublishAt   int64    `json:"publishAt"`
	ExpireAt    int64    `json:"expireAt"`
	Dismissible bool     `json:"dismissible"`
}

// Create 新建。draft 状态不要求 publishAt；published 状态 publishAt=0 时填 now。
func Create(in CreateInput, operator string) (Notification, error) {
	now := time.Now().Unix()
	n := Notification{
		ID:          newID(now),
		Title:       strings.TrimSpace(in.Title),
		Body:        in.Body,
		Level:       strings.ToLower(strings.TrimSpace(in.Level)),
		TargetType:  strings.ToLower(strings.TrimSpace(in.TargetType)),
		TargetValue: dedupNonEmpty(in.TargetValue),
		Status:      strings.ToLower(strings.TrimSpace(in.Status)),
		PublishAt:   in.PublishAt,
		ExpireAt:    in.ExpireAt,
		Dismissible: in.Dismissible,
		CreatedAt:   now,
		CreatedBy:   operator,
	}
	if n.Status == "" {
		n.Status = StatusDraft
	}
	if n.Status == StatusPublished && n.PublishAt == 0 {
		n.PublishAt = now
	}
	if err := n.Validate(); err != nil {
		return Notification{}, err
	}
	return Default().UpsertNotification(n)
}

// Update 更新；只允许改 admin 字段，read state 不重置。
func Update(id string, in CreateInput, operator string) (Notification, error) {
	existing, ok := Default().GetNotification(id)
	if !ok || existing.DeletedAt != 0 {
		return Notification{}, fmt.Errorf("notification %s not found", id)
	}
	now := time.Now().Unix()
	merged := existing
	merged.Title = strings.TrimSpace(in.Title)
	merged.Body = in.Body
	merged.Level = strings.ToLower(strings.TrimSpace(in.Level))
	merged.TargetType = strings.ToLower(strings.TrimSpace(in.TargetType))
	merged.TargetValue = dedupNonEmpty(in.TargetValue)
	merged.Status = strings.ToLower(strings.TrimSpace(in.Status))
	merged.PublishAt = in.PublishAt
	merged.ExpireAt = in.ExpireAt
	merged.Dismissible = in.Dismissible
	merged.UpdatedAt = now
	merged.UpdatedBy = operator
	if merged.Status == StatusPublished && merged.PublishAt == 0 {
		merged.PublishAt = now
	}
	if err := merged.Validate(); err != nil {
		return Notification{}, err
	}
	return Default().UpsertNotification(merged)
}

// Delete 软删除。
func Delete(id, operator string) error {
	now := time.Now().Unix()
	changed, err := Default().SoftDelete(id, now, operator)
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("notification %s not found or already deleted", id)
	}
	return nil
}

// GetStats 单条统计。
func GetStats(id string) (Stats, error) {
	n, ok := Default().GetNotification(id)
	if !ok {
		return Stats{}, fmt.Errorf("notification %s not found", id)
	}
	return computeStats(n, snapshotApiKeys(), time.Now().Unix()), nil
}

func dedupNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
