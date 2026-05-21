// proxy/handler_v6_admin_test.go
package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kiro-api-proxy/notif"
	"kiro-api-proxy/users"
)

func v6CreateAdminNotification(t *testing.T, h *Handler, in notif.CreateInput) notif.Notification {
	t.Helper()

	rr := httptest.NewRecorder()
	h.apiCreateNotification(rr, v6JSONRequest(t, http.MethodPost, "/admin/api/notifications", in))
	v6AssertStatus(t, rr, http.StatusCreated)
	return v6DecodeJSON[notif.Notification](t, rr)
}

func v6ListAdminNotifications(t *testing.T, h *Handler, status string) notif.AdminListResult {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/notifications?status="+status+"&limit=500", nil)
	rr := httptest.NewRecorder()
	h.apiListNotifications(rr, req)
	v6AssertStatus(t, rr, http.StatusOK)
	return v6DecodeJSON[notif.AdminListResult](t, rr)
}

func v6FindAdminNotification(items []notif.AdminItem, id string) (notif.AdminItem, bool) {
	for _, it := range items {
		if it.Notification.ID == id {
			return it, true
		}
	}
	return notif.AdminItem{}, false
}

func v6FindUserNotification(items []notif.UserView, id string) (notif.UserView, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return notif.UserView{}, false
}

func TestApiAdminRechargesEmptyRecords(t *testing.T) {
	h := seedV6Test(t)
	v6WriteRechargeRecords(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/recharges", nil)
	rr := httptest.NewRecorder()
	h.apiAdminRecharges(rr, req)

	v6AssertStatus(t, rr, http.StatusOK)
	resp := v6DecodeJSON[struct {
		Records []RechargeRecord      `json:"records"`
		Total   int                   `json:"total"`
		Summary adminRechargesSummary `json:"summary"`
	}](t, rr)

	if resp.Total != 0 || len(resp.Records) != 0 {
		t.Fatalf("expected empty records, got total=%d records=%+v", resp.Total, resp.Records)
	}
	v6AssertFloat(t, resp.Summary.TodayCNY, 0)
	v6AssertFloat(t, resp.Summary.MonthCNY, 0)
	v6AssertFloat(t, resp.Summary.AvgCNY, 0)
	v6AssertFloat(t, resp.Summary.ReturningRate, 0)
}

func TestAdminRechargeFilterFromRequestAndFiltering(t *testing.T) {
	base := time.Now().Unix()
	records := []RechargeRecord{
		v6RechargeRecord(base-300, "key-a", "code_redeem", "Alpha customer", "ALPHA-CODE", "first recharge", 10),
		v6RechargeRecord(base-200, "key-b", "admin_balance", "Beta customer", "", "manual topup", 20),
		v6RechargeRecord(base-100, "key-c", "admin_gift", "Gamma customer", "GIFT-CODE", "gift note", 30),
	}

	if got := filterAdminRecharges(records, adminRechargeFilter{typ: "code_redeem"}); len(got) != 1 || got[0].KeyID != "key-a" {
		t.Fatalf("type filter mismatch: %+v", got)
	}
	if got := filterAdminRecharges(records, adminRechargeFilter{typ: "missing"}); len(got) != 0 {
		t.Fatalf("missing type filter = %+v, want empty", got)
	}

	searchCases := []struct {
		name   string
		search string
		keyID  string
	}{
		{name: "key note", search: "alpha customer", keyID: "key-a"},
		{name: "code", search: "alpha-code", keyID: "key-a"},
		{name: "note", search: "manual topup", keyID: "key-b"},
	}
	for _, tc := range searchCases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterAdminRecharges(records, adminRechargeFilter{search: tc.search})
			if len(got) != 1 || got[0].KeyID != tc.keyID {
				t.Fatalf("search %q = %+v, want keyID=%s", tc.search, got, tc.keyID)
			}
		})
	}

	got := filterAdminRecharges(records, adminRechargeFilter{from: base - 250, to: base - 150})
	if len(got) != 1 || got[0].KeyID != "key-b" {
		t.Fatalf("time range filter mismatch: %+v", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/recharges?type=admin_balance&search=MiXeD&from=-1&to=123", nil)
	f := adminRechargeFilterFromRequest(req)
	if f.typ != "admin_balance" || f.search != "mixed" || f.from != 0 || f.to != 123 {
		t.Fatalf("request filter mismatch: %+v", f)
	}
}

func TestApiAdminRechargesPaginationBoundaries(t *testing.T) {
	h := seedV6Test(t)

	base := time.Now().Unix() - 60
	v6WriteRechargeRecords(t, []RechargeRecord{
		v6RechargeRecord(base+1, "key-old", "code_redeem", "old", "OLD", "", 10),
		v6RechargeRecord(base+3, "key-new", "code_redeem", "new", "NEW", "", 30),
		v6RechargeRecord(base+2, "key-mid", "admin_balance", "mid", "MID", "", 20),
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/api/recharges?limit=2&offset=1", nil)
	rr := httptest.NewRecorder()
	h.apiAdminRecharges(rr, req)
	v6AssertStatus(t, rr, http.StatusOK)

	resp := v6DecodeJSON[struct {
		Records []RechargeRecord      `json:"records"`
		Total   int                   `json:"total"`
		Summary adminRechargesSummary `json:"summary"`
	}](t, rr)

	if resp.Total != 3 || len(resp.Records) != 2 {
		t.Fatalf("pagination total/len mismatch: total=%d records=%+v", resp.Total, resp.Records)
	}
	if resp.Records[0].Code != "MID" || resp.Records[1].Code != "OLD" {
		t.Fatalf("pagination order mismatch: %+v", resp.Records)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/api/recharges?limit=2&offset=99", nil)
	rr = httptest.NewRecorder()
	h.apiAdminRecharges(rr, req)
	v6AssertStatus(t, rr, http.StatusOK)

	resp = v6DecodeJSON[struct {
		Records []RechargeRecord      `json:"records"`
		Total   int                   `json:"total"`
		Summary adminRechargesSummary `json:"summary"`
	}](t, rr)
	if resp.Total != 3 || len(resp.Records) != 0 {
		t.Fatalf("offset beyond total mismatch: total=%d records=%+v", resp.Total, resp.Records)
	}

	limit, offset := adminRechargePagination(httptest.NewRequest(http.MethodGet, "/admin/api/recharges?limit=5000&offset=-10", nil))
	if limit != 1000 || offset != 0 {
		t.Fatalf("pagination boundary = limit=%d offset=%d, want 1000/0", limit, offset)
	}
}

func TestSummarizeAdminRechargesRevenueAndReturningRate(t *testing.T) {
	ts := time.Now().Unix()
	records := []RechargeRecord{
		v6RechargeRecord(ts, "key-a", "code_redeem", "", "C1", "", 10),
		v6RechargeRecord(ts, "key-a", "admin_balance", "", "", "", 20),
		v6RechargeRecord(ts, "key-b", "code_redeem", "", "C2", "", 30),
		v6RechargeRecord(ts, "key-c", "admin_gift", "", "", "", 40),
		v6RechargeRecord(ts, "key-d", "code_redeem_days", "", "DAYS", "", 50),
	}

	summary := summarizeAdminRecharges(records)

	v6AssertFloat(t, summary.TodayCNY, 60)
	v6AssertFloat(t, summary.MonthCNY, 60)
	v6AssertFloat(t, summary.AvgCNY, 30)
	v6AssertFloat(t, summary.ReturningRate, 0.5)
}

func TestAdminListUsersOmitsPasswordHashAndToggleUser(t *testing.T) {
	h := seedV6Test(t)

	email := v6TestEmail("admin-user-list")
	u, _ := v6RegisterUser(t, email, "listed-user", "password-123")

	req := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	rr := httptest.NewRecorder()
	h.apiListUsers(rr, req)
	v6AssertStatus(t, rr, http.StatusOK)

	resp := v6DecodeJSON[struct {
		Users []map[string]any `json:"users"`
		Total int              `json:"total"`
	}](t, rr)

	var row map[string]any
	for _, candidate := range resp.Users {
		if candidate["email"] == email {
			row = candidate
			break
		}
	}
	if row == nil {
		t.Fatalf("registered user %s not found in response: %+v", email, resp.Users)
	}
	if _, ok := row["passwordHash"]; ok {
		t.Fatalf("apiListUsers leaked passwordHash: %+v", row)
	}

	beforeAudit := v6AuditSize(t)
	rr = httptest.NewRecorder()
	h.apiToggleUser(rr, httptest.NewRequest(http.MethodPost, "/admin/api/users/"+u.ID+"/disable", nil), u.ID, true)
	v6AssertStatus(t, rr, http.StatusOK)
	v6AssertAuditContains(t, beforeAudit, "user_disable")

	got, ok := users.Default().FindByID(u.ID)
	if !ok || !got.Disabled {
		t.Fatalf("user should be disabled: ok=%v user=%+v", ok, got)
	}

	beforeAudit = v6AuditSize(t)
	rr = httptest.NewRecorder()
	h.apiToggleUser(rr, httptest.NewRequest(http.MethodPost, "/admin/api/users/"+u.ID+"/enable", nil), u.ID, false)
	v6AssertStatus(t, rr, http.StatusOK)
	v6AssertAuditContains(t, beforeAudit, "user_enable")

	got, ok = users.Default().FindByID(u.ID)
	if !ok || got.Disabled {
		t.Fatalf("user should be enabled: ok=%v user=%+v", ok, got)
	}
}

func TestAdminUserPolicyGetSetAndAudit(t *testing.T) {
	h := seedV6Test(t)

	beforeAudit := v6AuditSize(t)
	rr := httptest.NewRecorder()
	h.apiSetUserPolicy(rr, v6JSONRequest(t, http.MethodPut, "/admin/api/users/policy", map[string]any{
		"allowSelfRegister":     false,
		"requireActivationCode": true,
	}))
	v6AssertStatus(t, rr, http.StatusOK)
	v6AssertAuditContains(t, beforeAudit, "user_policy_update")

	setResp := v6DecodeJSON[map[string]bool](t, rr)
	if setResp["allowSelfRegister"] || !setResp["requireActivationCode"] {
		t.Fatalf("set policy response mismatch: %+v", setResp)
	}

	rr = httptest.NewRecorder()
	h.apiGetUserPolicy(rr, httptest.NewRequest(http.MethodGet, "/admin/api/users/policy", nil))
	v6AssertStatus(t, rr, http.StatusOK)

	getResp := v6DecodeJSON[map[string]bool](t, rr)
	if getResp["allowSelfRegister"] || !getResp["requireActivationCode"] {
		t.Fatalf("get policy response mismatch: %+v", getResp)
	}
}

func TestAdminNotificationsCreateDraftPublishedAndInvalid(t *testing.T) {
	h := seedV6Test(t)
	plan := v6TestID("notif-create-plan")

	draft := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "draft " + v6TestID("notif"),
		Body:        "draft body",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusDraft,
		Dismissible: true,
	})
	if draft.Status != notif.StatusDraft || draft.PublishAt != 0 {
		t.Fatalf("draft notification mismatch: %+v", draft)
	}

	published := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "published " + v6TestID("notif"),
		Body:        "published body",
		Level:       notif.LevelWarn,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
		Dismissible: true,
	})
	if published.Status != notif.StatusPublished || published.PublishAt == 0 {
		t.Fatalf("published notification mismatch: %+v", published)
	}

	rr := httptest.NewRecorder()
	h.apiCreateNotification(rr, v6JSONRequest(t, http.MethodPost, "/admin/api/notifications", notif.CreateInput{
		Body:       "missing title",
		Level:      notif.LevelInfo,
		TargetType: notif.TargetAll,
		Status:     notif.StatusDraft,
	}))
	v6AssertStatus(t, rr, http.StatusBadRequest)

	rr = httptest.NewRecorder()
	h.apiCreateNotification(rr, v6JSONRequest(t, http.MethodPost, "/admin/api/notifications", notif.CreateInput{
		Title:      "invalid level",
		Body:       "body",
		Level:      "loud",
		TargetType: notif.TargetAll,
		Status:     notif.StatusDraft,
	}))
	v6AssertStatus(t, rr, http.StatusBadRequest)
}

func TestAdminNotificationsListStatusFiltersAndSoftDelete(t *testing.T) {
	h := seedV6Test(t)

	plan := v6TestID("notif-list-plan")
	now := time.Now().Unix()
	draft := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "draft " + v6TestID("notif-list"),
		Body:        "draft",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusDraft,
	})
	published := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "published " + v6TestID("notif-list"),
		Body:        "published",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
	})
	expired := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "expired " + v6TestID("notif-list"),
		Body:        "expired",
		Level:       notif.LevelWarn,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
		PublishAt:   now - 3600,
		ExpireAt:    now - 60,
	})

	all := v6ListAdminNotifications(t, h, "all")
	for _, id := range []string{draft.ID, published.ID, expired.ID} {
		if _, ok := v6FindAdminNotification(all.Items, id); !ok {
			t.Fatalf("status=all missing %s in %+v", id, all.Items)
		}
	}

	drafts := v6ListAdminNotifications(t, h, "draft")
	if _, ok := v6FindAdminNotification(drafts.Items, draft.ID); !ok {
		t.Fatalf("status=draft missing draft notification")
	}
	if _, ok := v6FindAdminNotification(drafts.Items, published.ID); ok {
		t.Fatalf("status=draft should not include published notification")
	}

	publishedList := v6ListAdminNotifications(t, h, "published")
	if _, ok := v6FindAdminNotification(publishedList.Items, published.ID); !ok {
		t.Fatalf("status=published missing active published notification")
	}
	if _, ok := v6FindAdminNotification(publishedList.Items, expired.ID); ok {
		t.Fatalf("status=published should not include expired notification")
	}

	expiredList := v6ListAdminNotifications(t, h, "expired")
	if _, ok := v6FindAdminNotification(expiredList.Items, expired.ID); !ok {
		t.Fatalf("status=expired missing expired notification")
	}

	rr := httptest.NewRecorder()
	h.apiDeleteNotification(rr, httptest.NewRequest(http.MethodDelete, "/admin/api/notifications/"+draft.ID, nil), draft.ID)
	v6AssertStatus(t, rr, http.StatusOK)

	deleted, ok := notif.Default().GetNotification(draft.ID)
	if !ok || deleted.DeletedAt == 0 {
		t.Fatalf("notification should be soft deleted: ok=%v notification=%+v", ok, deleted)
	}

	allAfterDelete := v6ListAdminNotifications(t, h, "all")
	if _, ok := v6FindAdminNotification(allAfterDelete.Items, draft.ID); ok {
		t.Fatalf("status=all should hide soft-deleted notification")
	}
}

func TestAdminNotificationUpdateMissingReturnsNotFound(t *testing.T) {
	h := seedV6Test(t)

	rr := httptest.NewRecorder()
	h.apiUpdateNotification(rr, v6JSONRequest(t, http.MethodPut, "/admin/api/notifications/missing", notif.CreateInput{
		Title:       "missing update",
		Body:        "body",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{v6TestID("missing-plan")},
		Status:      notif.StatusDraft,
	}), "missing-"+v6TestID("notification"))
	v6AssertStatus(t, rr, http.StatusNotFound)
}

func TestAdminNotificationStatsCountsTargetReadAndDismissed(t *testing.T) {
	h := seedV6Test(t)

	plan := v6TestID("notif-stats-plan")
	key1 := v6AddApiKeyWithPlan(t, "notif-stats-key-1", "stats one", plan)
	key2 := v6AddApiKeyWithPlan(t, "notif-stats-key-2", "stats two", plan)

	n := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "stats " + v6TestID("notif"),
		Body:        "stats body",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
		Dismissible: true,
	})

	rr := httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodPost, "/user/api/notifications/"+n.ID+"/read", nil), &key1)
	v6AssertStatus(t, rr, http.StatusOK)

	rr = httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodPost, "/user/api/notifications/"+n.ID+"/dismiss", nil), &key2)
	v6AssertStatus(t, rr, http.StatusOK)

	rr = httptest.NewRecorder()
	h.apiNotificationStats(rr, httptest.NewRequest(http.MethodGet, "/admin/api/notifications/"+n.ID+"/stats", nil), n.ID)
	v6AssertStatus(t, rr, http.StatusOK)

	stats := v6DecodeJSON[notif.Stats](t, rr)
	if stats.TargetCount != 2 || stats.ReadCount != 2 || stats.DismissedCount != 1 || stats.UnreadCount != 0 {
		t.Fatalf("stats mismatch: %+v", stats)
	}
}

func TestUserNotificationsListReadDismissAndReadAll(t *testing.T) {
	h := seedV6Test(t)

	plan := v6TestID("user-notif-plan")
	key := v6AddApiKeyWithPlan(t, "user-notif-key", "user notification key", plan)

	readable := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "readable " + v6TestID("user-notif"),
		Body:        "readable body",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
		Dismissible: true,
	})
	locked := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "locked " + v6TestID("user-notif"),
		Body:        "locked body",
		Level:       notif.LevelWarn,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
		Dismissible: false,
	})
	batch := v6CreateAdminNotification(t, h, notif.CreateInput{
		Title:       "batch " + v6TestID("user-notif"),
		Body:        "batch body",
		Level:       notif.LevelInfo,
		TargetType:  notif.TargetPlan,
		TargetValue: []string{plan},
		Status:      notif.StatusPublished,
		Dismissible: true,
	})

	rr := httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodGet, "/user/api/notifications?limit=10", nil), &key)
	v6AssertStatus(t, rr, http.StatusOK)

	initial := v6DecodeJSON[notif.UserListResult](t, rr)
	if initial.UnreadCount != 3 || len(initial.Items) != 3 {
		t.Fatalf("initial user notification list mismatch: %+v", initial)
	}
	for _, id := range []string{readable.ID, locked.ID, batch.ID} {
		if _, ok := v6FindUserNotification(initial.Items, id); !ok {
			t.Fatalf("initial list missing notification %s: %+v", id, initial.Items)
		}
	}

	for i := 0; i < 2; i++ {
		rr = httptest.NewRecorder()
		h.handleUserNotifications(rr, httptest.NewRequest(http.MethodPost, "/user/api/notifications/"+readable.ID+"/read", nil), &key)
		v6AssertStatus(t, rr, http.StatusOK)
	}

	rr = httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodPost, "/user/api/notifications/"+locked.ID+"/dismiss", nil), &key)
	v6AssertStatus(t, rr, http.StatusBadRequest)

	rr = httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodPost, "/user/api/notifications/"+readable.ID+"/dismiss", nil), &key)
	v6AssertStatus(t, rr, http.StatusOK)

	rr = httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodPost, "/user/api/notifications/read-all", nil), &key)
	v6AssertStatus(t, rr, http.StatusOK)

	readAllResp := v6DecodeJSON[struct {
		Success bool `json:"success"`
		Marked  int  `json:"marked"`
	}](t, rr)
	if !readAllResp.Success || readAllResp.Marked != 2 {
		t.Fatalf("read-all response mismatch: %+v", readAllResp)
	}

	rr = httptest.NewRecorder()
	h.handleUserNotifications(rr, httptest.NewRequest(http.MethodGet, "/user/api/notifications?limit=10", nil), &key)
	v6AssertStatus(t, rr, http.StatusOK)

	final := v6DecodeJSON[notif.UserListResult](t, rr)
	if final.UnreadCount != 0 {
		t.Fatalf("final unread count mismatch: %+v", final)
	}
	if _, ok := v6FindUserNotification(final.Items, readable.ID); ok {
		t.Fatalf("dismissed notification should be hidden: %+v", final.Items)
	}
	if lockedItem, ok := v6FindUserNotification(final.Items, locked.ID); !ok || !lockedItem.Read {
		t.Fatalf("locked notification should remain visible and read: ok=%v item=%+v list=%+v", ok, lockedItem, final.Items)
	}
	if batchItem, ok := v6FindUserNotification(final.Items, batch.ID); !ok || !batchItem.Read {
		t.Fatalf("batch notification should be read: ok=%v item=%+v list=%+v", ok, batchItem, final.Items)
	}
}
