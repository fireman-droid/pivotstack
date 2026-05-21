// Package notif 实现 PivotStack 跨端通知（admin 发布 → user 接收）。
//
// 设计要点：
//   - 独立持久化文件 data/notifications.json，避免与高频 config.json 互锁
//   - 推送机制 v1 polling 60s；schema 不变，v2 可平滑迁移 SSE
//   - 软删除：DeletedAt 非零时对 user 全部不可见，admin list 可选过滤
package notif

import (
	"errors"
	"strings"
)

// 通知级别
const (
	LevelInfo     = "info"
	LevelWarn     = "warn"
	LevelCritical = "critical"
)

// 目标类型
const (
	TargetAll     = "all"
	TargetPlan    = "plan"
	TargetGroup   = "group"
	TargetUserIDs = "userIds"
)

// 状态
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

const (
	MaxTitle = 80
	MaxBody  = 2000
)

// Notification 一条系统通知
type Notification struct {
	ID          string   `json:"id"`           // ntf_<unix>_<short>
	Title       string   `json:"title"`        // ≤ MaxTitle
	Body        string   `json:"body"`         // markdown，≤ MaxBody
	Level       string   `json:"level"`        // info | warn | critical
	TargetType  string   `json:"targetType"`   // all | plan | group | userIds
	TargetValue []string `json:"targetValue,omitempty"`

	Status      string `json:"status"`                // draft | published
	PublishAt   int64  `json:"publishAt,omitempty"`   // 0 = 立即（status published 时）
	ExpireAt    int64  `json:"expireAt,omitempty"`    // 0 = 永久
	Dismissible bool   `json:"dismissible"`

	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	UpdatedBy string `json:"updatedBy,omitempty"`
	DeletedAt int64  `json:"deletedAt,omitempty"` // 软删除
}

// Delivery 单 user 对单 notification 的阅读/隐藏状态
type Delivery struct {
	NotificationID string `json:"notificationId"`
	UserID         string `json:"userId"` // ApiKeyInfo.ID
	FirstSeenAt    int64  `json:"firstSeenAt,omitempty"`
	ReadAt         int64  `json:"readAt,omitempty"`
	DismissedAt    int64  `json:"dismissedAt,omitempty"`
	UpdatedAt      int64  `json:"updatedAt,omitempty"`
}

// File 落盘结构
type File struct {
	SchemaVersion int            `json:"schemaVersion"`
	Notifications []Notification `json:"notifications"`
	Deliveries    []Delivery     `json:"deliveries,omitempty"`
	UpdatedAt     int64          `json:"updatedAt,omitempty"`
}

// CurrentSchemaVersion 当前 schema 版本，迁移需 bump
const CurrentSchemaVersion = 1

// Validate 检查 notification 字段合法性。
//
// id / createdAt / status 由 service 层填充，这里只校验 admin 可填字段。
func (n Notification) Validate() error {
	title := strings.TrimSpace(n.Title)
	if title == "" {
		return errors.New("title is required")
	}
	if len([]rune(title)) > MaxTitle {
		return errors.New("title too long (> 80 chars)")
	}
	if strings.TrimSpace(n.Body) == "" {
		return errors.New("body is required")
	}
	if len([]rune(n.Body)) > MaxBody {
		return errors.New("body too long (> 2000 chars)")
	}
	switch n.Level {
	case LevelInfo, LevelWarn, LevelCritical:
	default:
		return errors.New("invalid level: must be info | warn | critical")
	}
	switch n.TargetType {
	case TargetAll:
		// targetValue ignored
	case TargetPlan, TargetGroup, TargetUserIDs:
		if len(n.TargetValue) == 0 {
			return errors.New("targetValue is required when targetType != all")
		}
	default:
		return errors.New("invalid targetType")
	}
	switch n.Status {
	case "", StatusDraft, StatusPublished:
	default:
		return errors.New("invalid status: must be draft | published")
	}
	if n.PublishAt > 0 && n.ExpireAt > 0 && n.ExpireAt <= n.PublishAt {
		return errors.New("expireAt must be greater than publishAt")
	}
	return nil
}
