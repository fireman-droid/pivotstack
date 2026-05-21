package notif

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kiro-api-proxy/config"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "notif_test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "MkdirTemp: %v\n", err)
		os.Exit(2)
	}
	if err := config.Init(filepath.Join(dir, "config.json")); err != nil {
		_ = os.RemoveAll(dir)
		fmt.Fprintf(os.Stderr, "config.Init: %v\n", err)
		os.Exit(2)
	}

	defaultStore = newStore(filepath.Join(dir, "notifications.json"))
	once.Do(func() {})

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func resetDefaultStore(t *testing.T) *Store {
	t.Helper()
	s := Default()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.path = filepath.Join(t.TempDir(), "notifications.json")
	s.file = File{
		SchemaVersion: CurrentSchemaVersion,
		Notifications: []Notification{},
		Deliveries:    []Delivery{},
	}
	return s
}

func testID(t *testing.T, suffix string) string {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_", "-", "_", ":", "_").Replace(t.Name())
	return "ntf_" + name + "_" + suffix
}

func testKey(t *testing.T, suffix, plan string, enabled bool, prefs map[string]string) config.ApiKeyInfo {
	t.Helper()
	id := testID(t, suffix)
	return config.ApiKeyInfo{
		ID:                 id,
		Key:                "sk-" + id,
		Plan:               plan,
		Enabled:            enabled,
		ChannelPreferences: prefs,
		CreatedAt:          time.Now().Unix(),
	}
}

func seedNotifAPIKeys(t *testing.T, keys ...config.ApiKeyInfo) []config.ApiKeyInfo {
	t.Helper()

	preexistingKeys := config.GetAllApiKeys()
	for _, k := range preexistingKeys {
		_ = config.DeleteApiKey(k.ID)
	}
	t.Cleanup(func() {
		current := config.GetAllApiKeys()
		for _, k := range current {
			_ = config.DeleteApiKey(k.ID)
		}
		for _, k := range preexistingKeys {
			_ = config.AddApiKey(k)
		}
	})

	now := time.Now().Unix()
	for i := range keys {
		if keys[i].ID == "" {
			keys[i].ID = testID(t, fmt.Sprintf("key_%d", i))
		}
		if keys[i].Key == "" {
			keys[i].Key = "sk-" + keys[i].ID
		}
		if keys[i].CreatedAt == 0 {
			keys[i].CreatedAt = now
		}
		if err := config.AddApiKey(keys[i]); err != nil {
			t.Fatalf("AddApiKey(%q): %v", keys[i].ID, err)
		}
	}

	return config.GetAllApiKeys()
}

func testNotification(t *testing.T, suffix string, now int64) Notification {
	t.Helper()
	return Notification{
		ID:          testID(t, suffix),
		Title:       "Title " + suffix,
		Body:        "Body " + suffix,
		Level:       LevelInfo,
		TargetType:  TargetAll,
		Status:      StatusPublished,
		PublishAt:   now - 60,
		ExpireAt:    now + 3600,
		Dismissible: true,
		CreatedAt:   now - 120,
	}
}

func validCreateInput() CreateInput {
	return CreateInput{
		Title:       "Notice",
		Body:        "Body",
		Level:       LevelInfo,
		TargetType:  TargetAll,
		Status:      StatusDraft,
		Dismissible: true,
	}
}

func mustUpsert(t *testing.T, s *Store, n Notification) {
	t.Helper()
	if _, err := s.UpsertNotification(n); err != nil {
		t.Fatalf("UpsertNotification(%q): %v", n.ID, err)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
}

func userViewIDs(items []UserView) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func adminItemIDs(items []AdminItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Notification.ID)
	}
	return out
}

func matchingAdminStatusIDs(notes []Notification, status string, now int64) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		if matchAdminStatus(n, status, now) {
			out = append(out, n.ID)
		}
	}
	return out
}

func TestNotificationValidate(t *testing.T) {
	valid := func() Notification {
		return Notification{
			ID:          "ntf_valid",
			Title:       "Valid title",
			Body:        "Valid body",
			Level:       LevelInfo,
			TargetType:  TargetAll,
			Status:      StatusPublished,
			PublishAt:   100,
			ExpireAt:    200,
			Dismissible: true,
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Notification)
		wantErr bool
	}{
		{name: "valid", wantErr: false},
		{name: "empty_title", mutate: func(n *Notification) { n.Title = "   " }, wantErr: true},
		{name: "title_too_long", mutate: func(n *Notification) { n.Title = strings.Repeat("x", MaxTitle+1) }, wantErr: true},
		{name: "empty_body", mutate: func(n *Notification) { n.Body = "" }, wantErr: true},
		{name: "body_too_long", mutate: func(n *Notification) { n.Body = strings.Repeat("b", MaxBody+1) }, wantErr: true},
		{name: "invalid_level", mutate: func(n *Notification) { n.Level = "debug" }, wantErr: true},
		{name: "invalid_target_type", mutate: func(n *Notification) { n.TargetType = "team" }, wantErr: true},
		{name: "plan_target_without_value", mutate: func(n *Notification) {
			n.TargetType = TargetPlan
			n.TargetValue = nil
		}, wantErr: true},
		{name: "invalid_status", mutate: func(n *Notification) { n.Status = "archived" }, wantErr: true},
		{name: "expire_at_not_after_publish_at", mutate: func(n *Notification) {
			n.PublishAt = 200
			n.ExpireAt = 200
		}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := valid()
			if tt.mutate != nil {
				tt.mutate(&n)
			}
			err := n.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestStoreNotificationCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.json")
	s := newStore(path)
	now := int64(1000)

	n := testNotification(t, "store", now)
	n.TargetType = TargetPlan
	n.TargetValue = []string{"pro"}

	got, err := s.UpsertNotification(n)
	if err != nil {
		t.Fatalf("UpsertNotification insert: %v", err)
	}
	if got.ID != n.ID {
		t.Fatalf("inserted ID = %q, want %q", got.ID, n.ID)
	}

	stored, ok := s.GetNotification(n.ID)
	if !ok {
		t.Fatal("GetNotification() ok = false, want true")
	}
	if stored.Title != n.Title {
		t.Fatalf("stored title = %q, want %q", stored.Title, n.Title)
	}

	snap := s.Snapshot()
	snap.Notifications[0].TargetValue[0] = "mutated"
	stored, _ = s.GetNotification(n.ID)
	if stored.TargetValue[0] != "pro" {
		t.Fatalf("Snapshot leaked TargetValue mutation: %#v", stored.TargetValue)
	}

	n.Body = "updated body"
	if _, err := s.UpsertNotification(n); err != nil {
		t.Fatalf("UpsertNotification update: %v", err)
	}
	snap = s.Snapshot()
	if len(snap.Notifications) != 1 || snap.Notifications[0].Body != "updated body" {
		t.Fatalf("snapshot after update = %+v", snap.Notifications)
	}

	reloaded := newStore(path)
	reloadedN, ok := reloaded.GetNotification(n.ID)
	if !ok || reloadedN.Body != "updated body" {
		t.Fatalf("reloaded notification = %+v ok=%v", reloadedN, ok)
	}

	changed, err := s.SoftDelete("missing", now+1, "admin")
	if err != nil {
		t.Fatalf("SoftDelete missing: %v", err)
	}
	if changed {
		t.Fatal("SoftDelete missing changed = true, want false")
	}

	changed, err = s.SoftDelete(n.ID, now+2, "admin")
	if err != nil {
		t.Fatalf("SoftDelete existing: %v", err)
	}
	if !changed {
		t.Fatal("SoftDelete existing changed = false, want true")
	}
	deleted, ok := s.GetNotification(n.ID)
	if !ok {
		t.Fatal("deleted notification missing")
	}
	if deleted.DeletedAt != now+2 || deleted.UpdatedAt != now+2 || deleted.UpdatedBy != "admin" {
		t.Fatalf("soft delete fields wrong: %+v", deleted)
	}

	changed, err = s.SoftDelete(n.ID, now+3, "admin")
	if err != nil {
		t.Fatalf("SoftDelete repeat: %v", err)
	}
	if changed {
		t.Fatal("SoftDelete repeat changed = true, want false")
	}
}

func TestStoreDeliveryCRUD(t *testing.T) {
	s := newStore(filepath.Join(t.TempDir(), "notifications.json"))

	if err := s.MarkDelivery("n1", "u1", "seen", 100); err != nil {
		t.Fatalf("MarkDelivery seen: %v", err)
	}
	d := s.DeliveryMap("u1")["n1"]
	if d.FirstSeenAt != 100 || d.ReadAt != 0 || d.DismissedAt != 0 || d.UpdatedAt != 100 {
		t.Fatalf("seen delivery wrong: %+v", d)
	}

	if err := s.MarkDelivery("n1", "u1", "read", 200); err != nil {
		t.Fatalf("MarkDelivery read: %v", err)
	}
	if err := s.MarkDelivery("n1", "u1", "read", 300); err != nil {
		t.Fatalf("MarkDelivery read idempotent: %v", err)
	}
	d = s.DeliveryMap("u1")["n1"]
	if d.FirstSeenAt != 100 || d.ReadAt != 200 || d.UpdatedAt != 300 {
		t.Fatalf("read delivery wrong: %+v", d)
	}

	if err := s.MarkDelivery("n1", "u1", "dismiss", 400); err != nil {
		t.Fatalf("MarkDelivery dismiss: %v", err)
	}
	d = s.DeliveryMap("u1")["n1"]
	if d.FirstSeenAt != 100 || d.ReadAt != 200 || d.DismissedAt != 400 || d.UpdatedAt != 400 {
		t.Fatalf("dismiss delivery wrong: %+v", d)
	}

	if err := s.MarkDelivery("n2", "u2", "read", 500); err != nil {
		t.Fatalf("MarkDelivery other user: %v", err)
	}
	if err := s.MarkDelivery("n3", "u3", "bad", 600); err == nil {
		t.Fatal("MarkDelivery bad kind error = nil, want error")
	}
	if len(s.DeliveryMap("u3")) != 0 {
		t.Fatalf("bad kind should not create delivery: %+v", s.DeliveryMap("u3"))
	}

	dels := s.DeliveriesFor("n1")
	if len(dels) != 1 || dels[0].UserID != "u1" {
		t.Fatalf("DeliveriesFor(n1) = %+v", dels)
	}
	byUser := s.DeliveryMap("u1")
	if len(byUser) != 1 || byUser["n1"].NotificationID != "n1" {
		t.Fatalf("DeliveryMap(u1) = %+v", byUser)
	}

	changed, err := s.MarkAllRead("u1", []string{"n1", "n2", "n4"}, 700)
	if err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if changed != 2 {
		t.Fatalf("MarkAllRead changed = %d, want 2", changed)
	}
	byUser = s.DeliveryMap("u1")
	if byUser["n1"].ReadAt != 200 || byUser["n2"].ReadAt != 700 || byUser["n4"].ReadAt != 700 {
		t.Fatalf("MarkAllRead deliveries wrong: %+v", byUser)
	}

	changed, err = s.MarkAllRead("u1", []string{"n1", "n2", "n4"}, 800)
	if err != nil {
		t.Fatalf("MarkAllRead repeat: %v", err)
	}
	if changed != 0 {
		t.Fatalf("MarkAllRead repeat changed = %d, want 0", changed)
	}
}

func TestVisibleForUserTargetsAndGuards(t *testing.T) {
	now := int64(1000)
	baseKey := config.ApiKeyInfo{
		ID:                 "user-1",
		Plan:               "pro",
		Enabled:            true,
		ChannelPreferences: map[string]string{"group-a": "channel-a"},
	}
	base := testNotification(t, "visible", now)

	tests := []struct {
		name   string
		mutN   func(*Notification)
		mutKey func(*config.ApiKeyInfo)
		want   bool
	}{
		{name: "deleted", mutN: func(n *Notification) { n.DeletedAt = now }, want: false},
		{name: "draft", mutN: func(n *Notification) { n.Status = StatusDraft }, want: false},
		{name: "not_yet_published", mutN: func(n *Notification) { n.PublishAt = now + 1 }, want: false},
		{name: "expired_at_now", mutN: func(n *Notification) { n.ExpireAt = now }, want: false},
		{name: "disabled_key", mutKey: func(k *config.ApiKeyInfo) { k.Enabled = false }, want: false},
		{name: "all", want: true},
		{name: "plan_match", mutN: func(n *Notification) {
			n.TargetType = TargetPlan
			n.TargetValue = []string{"pro"}
		}, want: true},
		{name: "plan_miss", mutN: func(n *Notification) {
			n.TargetType = TargetPlan
			n.TargetValue = []string{"free"}
		}, want: false},
		{name: "group_match", mutN: func(n *Notification) {
			n.TargetType = TargetGroup
			n.TargetValue = []string{"group-a"}
		}, want: true},
		{name: "group_miss", mutN: func(n *Notification) {
			n.TargetType = TargetGroup
			n.TargetValue = []string{"group-b"}
		}, want: false},
		{name: "user_ids_match", mutN: func(n *Notification) {
			n.TargetType = TargetUserIDs
			n.TargetValue = []string{"user-1"}
		}, want: true},
		{name: "user_ids_miss", mutN: func(n *Notification) {
			n.TargetType = TargetUserIDs
			n.TargetValue = []string{"user-2"}
		}, want: false},
		{name: "unknown_target_type", mutN: func(n *Notification) { n.TargetType = "unknown" }, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := base
			key := baseKey
			if tt.mutN != nil {
				tt.mutN(&n)
			}
			if tt.mutKey != nil {
				tt.mutKey(&key)
			}
			if got := VisibleForUser(n, key, now); got != tt.want {
				t.Fatalf("VisibleForUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTargetUserIDsFiltersVisibleKeys(t *testing.T) {
	now := int64(1000)
	keys := []config.ApiKeyInfo{
		{ID: "pro-enabled", Plan: "pro", Enabled: true},
		{ID: "pro-disabled", Plan: "pro", Enabled: false},
		{ID: "free-enabled", Plan: "free", Enabled: true},
	}
	n := testNotification(t, "resolve", now)
	n.TargetType = TargetPlan
	n.TargetValue = []string{"pro"}

	got := resolveTargetUserIDs(n, keys, now)
	assertStringSlice(t, got, []string{"pro-enabled"})
}

func TestListForUserSortsLimitsDismissesAndCountsUnread(t *testing.T) {
	s := resetDefaultStore(t)
	now := time.Now().Unix()
	key := testKey(t, "user", "pro", true, nil)

	nOld := testNotification(t, "old", now)
	nOld.PublishAt = now - 30
	nNew := testNotification(t, "new", now)
	nNew.PublishAt = now - 10
	nRead := testNotification(t, "read", now)
	nRead.PublishAt = now - 20
	nDismissed := testNotification(t, "dismissed", now)
	nDismissed.PublishAt = now - 5

	for _, n := range []Notification{nOld, nNew, nRead, nDismissed} {
		mustUpsert(t, s, n)
	}
	if err := s.MarkDelivery(nRead.ID, key.ID, "read", now-15); err != nil {
		t.Fatalf("MarkDelivery read: %v", err)
	}
	if err := s.MarkDelivery(nDismissed.ID, key.ID, "dismiss", now-4); err != nil {
		t.Fatalf("MarkDelivery dismiss: %v", err)
	}

	res := ListForUser(key, 2, false)
	if res.UnreadCount != 2 {
		t.Fatalf("UnreadCount = %d, want 2", res.UnreadCount)
	}
	assertStringSlice(t, userViewIDs(res.Items), []string{nNew.ID, nRead.ID})
	if !res.Items[1].Read || res.Items[1].Dismissed {
		t.Fatalf("read item flags wrong: %+v", res.Items[1])
	}

	withDismissed := ListForUser(key, 0, true)
	if withDismissed.UnreadCount != 2 {
		t.Fatalf("include dismissed UnreadCount = %d, want 2", withDismissed.UnreadCount)
	}
	assertStringSlice(t, userViewIDs(withDismissed.Items), []string{nDismissed.ID, nNew.ID, nRead.ID, nOld.ID})
	if !withDismissed.Items[0].Dismissed || !withDismissed.Items[0].Read {
		t.Fatalf("dismissed item flags wrong: %+v", withDismissed.Items[0])
	}
}

func TestMarkUserReadBehavior(t *testing.T) {
	s := resetDefaultStore(t)
	now := time.Now().Unix()
	key := testKey(t, "reader", "pro", true, nil)

	readable := testNotification(t, "readable", now)
	readable.Dismissible = true
	notDismissible := testNotification(t, "not_dismissible", now)
	notDismissible.Dismissible = false
	wrongTarget := testNotification(t, "wrong_target", now)
	wrongTarget.TargetType = TargetUserIDs
	wrongTarget.TargetValue = []string{"someone-else"}

	for _, n := range []Notification{readable, notDismissible, wrongTarget} {
		mustUpsert(t, s, n)
	}

	if _, err := MarkUserRead("missing", key, false); err == nil {
		t.Fatal("MarkUserRead missing error = nil, want error")
	}
	if _, err := MarkUserRead(wrongTarget.ID, key, false); err == nil {
		t.Fatal("MarkUserRead inaccessible error = nil, want error")
	}

	readAt, err := MarkUserRead(readable.ID, key, false)
	if err != nil {
		t.Fatalf("MarkUserRead read: %v", err)
	}
	if readAt == 0 {
		t.Fatal("readAt = 0, want non-zero")
	}
	d := s.DeliveryMap(key.ID)[readable.ID]
	if d.ReadAt == 0 || d.DismissedAt != 0 {
		t.Fatalf("read delivery wrong: %+v", d)
	}

	if _, err := MarkUserRead(notDismissible.ID, key, true); err == nil {
		t.Fatal("MarkUserRead non-dismissible dismiss error = nil, want error")
	}
	if _, err := MarkUserRead(readable.ID, key, true); err != nil {
		t.Fatalf("MarkUserRead dismiss: %v", err)
	}
	d = s.DeliveryMap(key.ID)[readable.ID]
	if d.DismissedAt == 0 || d.ReadAt == 0 {
		t.Fatalf("dismiss delivery wrong: %+v", d)
	}
}

func TestMarkAllUserReadMarksVisibleUnreadOnly(t *testing.T) {
	s := resetDefaultStore(t)
	now := time.Now().Unix()
	key := testKey(t, "mark_all", "pro", true, nil)

	n1 := testNotification(t, "unread_1", now)
	n2 := testNotification(t, "unread_2", now)
	nRead := testNotification(t, "already_read", now)
	nDismissed := testNotification(t, "dismissed", now)

	for _, n := range []Notification{n1, n2, nRead, nDismissed} {
		mustUpsert(t, s, n)
	}
	if err := s.MarkDelivery(nRead.ID, key.ID, "read", now-10); err != nil {
		t.Fatalf("MarkDelivery read: %v", err)
	}
	if err := s.MarkDelivery(nDismissed.ID, key.ID, "dismiss", now-9); err != nil {
		t.Fatalf("MarkDelivery dismiss: %v", err)
	}

	changed, err := MarkAllUserRead(key)
	if err != nil {
		t.Fatalf("MarkAllUserRead: %v", err)
	}
	if changed != 2 {
		t.Fatalf("MarkAllUserRead changed = %d, want 2", changed)
	}

	delivery := s.DeliveryMap(key.ID)
	for _, id := range []string{n1.ID, n2.ID, nRead.ID} {
		if delivery[id].ReadAt == 0 {
			t.Fatalf("notification %s not marked read: %+v", id, delivery[id])
		}
	}
	if delivery[nDismissed.ID].DismissedAt == 0 {
		t.Fatalf("dismissed notification state lost: %+v", delivery[nDismissed.ID])
	}

	changed, err = MarkAllUserRead(key)
	if err != nil {
		t.Fatalf("MarkAllUserRead repeat: %v", err)
	}
	if changed != 0 {
		t.Fatalf("MarkAllUserRead repeat changed = %d, want 0", changed)
	}
}

func TestAdminCreateUpdateDelete(t *testing.T) {
	resetDefaultStore(t)

	draftInput := validCreateInput()
	draft, err := Create(draftInput, "admin-a")
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}
	if draft.ID == "" || !strings.HasPrefix(draft.ID, "ntf_") {
		t.Fatalf("draft ID = %q", draft.ID)
	}
	if draft.Status != StatusDraft || draft.PublishAt != 0 || draft.CreatedBy != "admin-a" {
		t.Fatalf("draft fields wrong: %+v", draft)
	}

	updateInput := validCreateInput()
	updateInput.Title = "Updated"
	updateInput.Body = "Updated body"
	updateInput.Level = LevelWarn
	updateInput.Status = StatusPublished
	updateInput.Dismissible = false
	updated, err := Update(draft.ID, updateInput, "admin-b")
	if err != nil {
		t.Fatalf("Update existing: %v", err)
	}
	if updated.ID != draft.ID || updated.Title != "Updated" || updated.Level != LevelWarn {
		t.Fatalf("updated fields wrong: %+v", updated)
	}
	if updated.PublishAt == 0 || updated.UpdatedAt == 0 || updated.UpdatedBy != "admin-b" {
		t.Fatalf("updated audit/publish fields wrong: %+v", updated)
	}

	publishedInput := validCreateInput()
	publishedInput.Status = StatusPublished
	before := time.Now().Unix()
	published, err := Create(publishedInput, "admin-c")
	after := time.Now().Unix()
	if err != nil {
		t.Fatalf("Create published: %v", err)
	}
	if published.PublishAt < before || published.PublishAt > after {
		t.Fatalf("published PublishAt = %d, want between %d and %d", published.PublishAt, before, after)
	}

	invalid := validCreateInput()
	invalid.Level = "bad"
	if _, err := Create(invalid, "admin"); err == nil {
		t.Fatal("Create invalid error = nil, want error")
	}

	if _, err := Update("missing", validCreateInput(), "admin"); err == nil {
		t.Fatal("Update missing error = nil, want error")
	}

	if err := Delete(updated.ID, "admin-delete"); err != nil {
		t.Fatalf("Delete existing: %v", err)
	}
	deleted, ok := Default().GetNotification(updated.ID)
	if !ok {
		t.Fatal("deleted notification not found")
	}
	if deleted.DeletedAt == 0 || deleted.UpdatedBy != "admin-delete" {
		t.Fatalf("soft delete fields wrong: %+v", deleted)
	}

	if _, err := Update(updated.ID, validCreateInput(), "admin"); err == nil {
		t.Fatal("Update deleted error = nil, want error")
	}
	if err := Delete(updated.ID, "admin-delete"); err == nil {
		t.Fatal("Delete repeat error = nil, want error")
	}
}

func TestMatchAdminStatusClassifiesNotifications(t *testing.T) {
	now := int64(1000)
	notes := []Notification{
		{ID: "draft", Status: StatusDraft},
		{ID: "published", Status: StatusPublished, ExpireAt: now + 1},
		{ID: "expired", Status: StatusPublished, ExpireAt: now},
		{ID: "deleted", Status: StatusPublished, DeletedAt: now},
	}

	tests := []struct {
		status string
		want   []string
	}{
		{status: "all", want: []string{"draft", "published", "expired"}},
		{status: "", want: []string{"draft", "published", "expired"}},
		{status: "draft", want: []string{"draft"}},
		{status: "published", want: []string{"published"}},
		{status: "expired", want: []string{"expired"}},
		{status: "deleted", want: []string{"deleted"}},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := matchingAdminStatusIDs(notes, tt.status, now)
			assertStringSlice(t, got, tt.want)
		})
	}
}

func TestListForAdminFiltersSortsPaginatesAndAddsStats(t *testing.T) {
	s := resetDefaultStore(t)
	now := time.Now().Unix()

	key1 := testKey(t, "admin_key_1", "pro", true, nil)
	key2 := testKey(t, "admin_key_2", "free", true, nil)
	disabled := testKey(t, "admin_key_disabled", "pro", false, nil)
	seedNotifAPIKeys(t, key1, key2, disabled)

	nDraft := testNotification(t, "draft", now)
	nDraft.Status = StatusDraft
	nDraft.PublishAt = 0
	nPubOld := testNotification(t, "pub_old", now)
	nPubOld.PublishAt = now - 100
	nPubNew := testNotification(t, "pub_new", now)
	nPubNew.PublishAt = now - 10
	nExpired := testNotification(t, "expired", now)
	nExpired.PublishAt = now - 200
	nExpired.ExpireAt = now - 1
	nDeleted := testNotification(t, "deleted", now)
	nDeleted.PublishAt = now - 5
	nDeleted.DeletedAt = now

	for _, n := range []Notification{nDraft, nPubOld, nPubNew, nExpired, nDeleted} {
		mustUpsert(t, s, n)
	}
	if err := s.MarkDelivery(nPubNew.ID, key1.ID, "read", now-1); err != nil {
		t.Fatalf("MarkDelivery: %v", err)
	}

	all := ListForAdmin("all", 0, 0)
	if all.Total != 4 {
		t.Fatalf("all Total = %d, want 4", all.Total)
	}
	assertStringSlice(t, adminItemIDs(all.Items), []string{nPubNew.ID, nPubOld.ID, nExpired.ID, nDraft.ID})

	published := ListForAdmin("published", 1, 0)
	if published.Total != 2 {
		t.Fatalf("published Total = %d, want 2", published.Total)
	}
	assertStringSlice(t, adminItemIDs(published.Items), []string{nPubNew.ID})
	if published.Items[0].Stats.TargetCount != 2 || published.Items[0].Stats.ReadCount != 1 || published.Items[0].Stats.UnreadCount != 1 {
		t.Fatalf("published stats wrong: %+v", published.Items[0].Stats)
	}

	nextPage := ListForAdmin("published", 1, 1)
	assertStringSlice(t, adminItemIDs(nextPage.Items), []string{nPubOld.ID})

	expired := ListForAdmin("expired", 0, 0)
	assertStringSlice(t, adminItemIDs(expired.Items), []string{nExpired.ID})

	deleted := ListForAdmin("deleted", 0, 0)
	assertStringSlice(t, adminItemIDs(deleted.Items), []string{nDeleted.ID})
}

func TestComputeStatsCountsOnlyCurrentTargets(t *testing.T) {
	s := resetDefaultStore(t)
	now := time.Now().Unix()

	proRead := testKey(t, "pro_read", "pro", true, nil)
	proDismiss := testKey(t, "pro_dismiss", "pro", true, nil)
	proUnread := testKey(t, "pro_unread", "pro", true, nil)
	freeRead := testKey(t, "free_read", "free", true, nil)
	proDisabled := testKey(t, "pro_disabled", "pro", false, nil)
	allKeys := seedNotifAPIKeys(t, proRead, proDismiss, proUnread, freeRead, proDisabled)

	n := testNotification(t, "stats", now)
	n.TargetType = TargetPlan
	n.TargetValue = []string{"pro"}
	mustUpsert(t, s, n)

	if err := s.MarkDelivery(n.ID, proRead.ID, "read", now-3); err != nil {
		t.Fatalf("MarkDelivery proRead: %v", err)
	}
	if err := s.MarkDelivery(n.ID, proDismiss.ID, "dismiss", now-2); err != nil {
		t.Fatalf("MarkDelivery proDismiss: %v", err)
	}
	if err := s.MarkDelivery(n.ID, freeRead.ID, "dismiss", now-1); err != nil {
		t.Fatalf("MarkDelivery freeRead: %v", err)
	}
	if err := s.MarkDelivery(n.ID, proDisabled.ID, "read", now-1); err != nil {
		t.Fatalf("MarkDelivery proDisabled: %v", err)
	}

	st := computeStats(n, allKeys, now)
	if st.NotificationID != n.ID {
		t.Fatalf("NotificationID = %q, want %q", st.NotificationID, n.ID)
	}
	if st.TargetCount != 3 || st.ReadCount != 2 || st.DismissedCount != 1 || st.UnreadCount != 1 {
		t.Fatalf("computeStats = %+v, want target=3 read=2 dismissed=1 unread=1", st)
	}

	got, err := GetStats(n.ID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if got.TargetCount != 3 || got.ReadCount != 2 || got.DismissedCount != 1 || got.UnreadCount != 1 {
		t.Fatalf("GetStats = %+v, want target=3 read=2 dismissed=1 unread=1", got)
	}

	if _, err := GetStats("missing"); err == nil {
		t.Fatal("GetStats missing error = nil, want error")
	}
}
